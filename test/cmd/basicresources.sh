#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
source "${OS_ROOT}/hack/lib/test/junit.sh"
os::log::install_errexit
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates,secrets,pods,jobs --all
  oc delete image v1-image
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/basicresources"
# This test validates basic resource retrieval and command interaction

os::test::junit::declare_suite_start "cmd/basicresources/versionreporting"
# Test to make sure that we're reporting the correct version information from endpoints and the correct
# User-Agent information from our clients regardless of which resources they're trying to access
os::build::get_version_vars
OS_GIT_VERSION_TO_MICRO=${OS_GIT_VERSION%%-*}
KUBE_GIT_VERSION_TO_MICRO=${KUBE_GIT_VERSION%%-*}
os::cmd::expect_success_and_text 'oc version' "oc ${OS_GIT_VERSION_TO_MICRO}"
os::cmd::expect_success_and_text 'oc version' "kubernetes ${KUBE_GIT_VERSION}"
os::cmd::expect_success_and_text 'openshift version' "openshift ${OS_GIT_VERSION_TO_MICRO}"
os::cmd::expect_success_and_text 'openshift version' "kubernetes ${KUBE_GIT_VERSION}"
os::cmd::expect_success_and_text 'curl -k ${API_SCHEME}://${API_HOST}:${API_PORT}/version' "${KUBE_GIT_VERSION}"
if [[ "${KUBE_GIT_VERSION_TO_MICRO}" != "${OS_GIT_VERSION_TO_MICRO}" ]]; then
  os::cmd::expect_success_and_not_text 'curl -k ${API_SCHEME}://${API_HOST}:${API_PORT}/version' "${OS_GIT_VERSION_TO_MICRO}"
fi
# variants I know I have to worry about
# 1. oc (kube and openshift resources)
# 2. openshift kubectl (kube and openshift resources)
# 3. oadm (kube and openshift resources)
# 4  openshift cli (kube and openshift resources)

# example User-Agent: oc/v1.2.0 (linux/amd64) kubernetes/bc4550d
# this is probably broken and should be `oc/<oc version>... openshift/...`
os::cmd::expect_success_and_text 'oc get pods --loglevel=7  2>&1 | grep -A4 "pods" | grep User-Agent' "oc/${KUBE_GIT_VERSION_TO_MICRO} .* kubernetes/"
# example User-Agent: oc/v1.1.3 (linux/amd64) openshift/b348c2f
os::cmd::expect_success_and_text 'oc get dc --loglevel=7  2>&1 | grep -A4 "deploymentconfig" | grep User-Agent' "oc/${OS_GIT_VERSION_TO_MICRO} .* openshift/"
# example User-Agent: openshift/v1.2.0 (linux/amd64) kubernetes/bc4550d
# this is probably broken and should be `kubectl/<kube version> kubernetes/...`
os::cmd::expect_success_and_text 'openshift kubectl get pods --loglevel=7  2>&1 | grep -A4 "pods" | grep User-Agent' "openshift/${KUBE_GIT_VERSION_TO_MICRO} .* kubernetes/"
# example User-Agent: openshift/v1.1.3 (linux/amd64) openshift/b348c2f
# this is probably broken and should be `kubectl/<kube version> openshift/...`
os::cmd::expect_success_and_text 'openshift kubectl get dc --loglevel=7  2>&1 | grep -A4 "deploymentconfig" | grep User-Agent' "openshift/${OS_GIT_VERSION_TO_MICRO} .* openshift/"
# example User-Agent: oadm/v1.2.0 (linux/amd64) kubernetes/bc4550d
# this is probably broken and should be `oadm/<oc version>... openshift/...`
os::cmd::expect_success_and_text 'oadm policy reconcile-sccs --loglevel=7  2>&1 | grep -A4 "securitycontextconstraints" | grep User-Agent' "oadm/${KUBE_GIT_VERSION_TO_MICRO} .* kubernetes/"
# example User-Agent: oadm/v1.1.3 (linux/amd64) openshift/b348c2f
os::cmd::expect_success_and_text 'oadm policy who-can get pods --loglevel=7  2>&1 | grep -A4 "localresourceaccessreviews" | grep User-Agent' "oadm/${OS_GIT_VERSION_TO_MICRO} .* openshift/"
# example User-Agent: openshift/v1.2.0 (linux/amd64) kubernetes/bc4550d
# this is probably broken and should be `oc/<oc version>... openshift/...`
os::cmd::expect_success_and_text 'openshift cli get pods --loglevel=7  2>&1 | grep -A4 "pods" | grep User-Agent' "openshift/${KUBE_GIT_VERSION_TO_MICRO} .* kubernetes/"
# example User-Agent: openshift/v1.1.3 (linux/amd64) openshift/b348c2f
os::cmd::expect_success_and_text 'openshift cli get dc --loglevel=7  2>&1 | grep -A4 "deploymentconfig" | grep User-Agent' "openshift/${OS_GIT_VERSION_TO_MICRO} .* openshift/"
echo "version reporting: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/basicresources/explain"
os::cmd::expect_success_and_text 'oc types' 'Deployment Configuration'
os::cmd::expect_failure_and_text 'oc get' 'deploymentconfig'
os::cmd::expect_success_and_text 'oc get all --loglevel=6' 'buildconfigs'
os::cmd::expect_success_and_text 'oc explain pods' 'Pod is a collection of containers that can run on a host'
os::cmd::expect_success_and_text 'oc explain pods.spec' 'SecurityContext holds pod-level security attributes'
os::cmd::expect_success_and_text 'oc explain deploymentconfig' 'a desired deployment state'
os::cmd::expect_success_and_text 'oc explain deploymentconfig.spec' 'ensures that this deployment config will have zero replicas'
echo "explain: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/basicresources/resource-builder"
# Test resource builder filtering of files with expected extensions inside directories, and individual files without expected extensions
os::cmd::expect_success 'oc create -f test/fixtures/resource-builder/directory -f test/fixtures/resource-builder/json-no-extension -f test/fixtures/resource-builder/yml-no-extension'
# Explicitly specified extensionless files
os::cmd::expect_success 'oc get secret json-no-extension yml-no-extension'
# Scanned files with extensions inside directories
os::cmd::expect_success 'oc get secret json-with-extension yml-with-extension'
# Ensure extensionless files inside directories are not processed by resource-builder
os::cmd::expect_failure_and_text 'oc get secret json-no-extension-in-directory' 'not found'
echo "resource-builder: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/basicresources/pods"
os::cmd::expect_success 'oc get pods --match-server-version'
os::cmd::expect_success_and_text 'oc create -f examples/hello-openshift/hello-pod.json' 'pod "hello-openshift" created'
os::cmd::expect_success 'oc describe pod hello-openshift'
os::cmd::expect_success 'oc delete pods hello-openshift --grace-period=0'
echo "pods: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/basicresources/label"
os::cmd::expect_success_and_text 'oc create -f examples/hello-openshift/hello-pod.json -o name' 'pod/hello-openshift'
os::cmd::try_until_success 'oc label pod/hello-openshift acustom=label' # can race against scheduling and status updates
os::cmd::expect_success_and_text 'oc describe pod/hello-openshift' 'acustom=label'
os::cmd::try_until_success 'oc annotate pod/hello-openshift foo=bar' # can race against scheduling and status updates
os::cmd::expect_success_and_text 'oc get -o yaml pod/hello-openshift' 'foo: bar'
os::cmd::expect_success 'oc delete pods -l acustom=label --grace-period=0'
os::cmd::expect_failure 'oc get pod/hello-openshift'
echo "label: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/basicresources/services"
os::cmd::expect_success 'oc get services'
os::cmd::expect_success 'oc create -f test/integration/fixtures/test-service.json'
os::cmd::expect_success 'oc delete services frontend'
echo "services: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/basicresources/list-version-conversion"
os::cmd::expect_success 'oc create   -f test/fixtures/mixed-api-versions.yaml'
os::cmd::expect_success 'oc get      -f test/fixtures/mixed-api-versions.yaml -o yaml'
os::cmd::expect_success 'oc label    -f test/fixtures/mixed-api-versions.yaml mylabel=a'
os::cmd::expect_success 'oc annotate -f test/fixtures/mixed-api-versions.yaml myannotation=b'
# Make sure all six resources, with different API versions, got labeled and annotated
os::cmd::expect_success_and_text 'oc get -f test/fixtures/mixed-api-versions.yaml --output-version=v1 --output=jsonpath="{..metadata.labels.mylabel}"'           '^a a a a a a$'
os::cmd::expect_success_and_text 'oc get -f test/fixtures/mixed-api-versions.yaml --output-version=v1 --output=jsonpath="{..metadata.annotations.myannotation}"' '^b b b b b b$'
os::cmd::expect_success 'oc delete   -f test/fixtures/mixed-api-versions.yaml'
echo "list version conversion: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/basicresources/nodes"
os::cmd::expect_success 'oc get nodes'
(
  # subshell so we can unset kubeconfig
  cfg="${KUBECONFIG}"
  unset KUBECONFIG
  os::cmd::expect_success 'kubectl get nodes --kubeconfig="${cfg}"'
)
echo "nodes: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/basicresources/routes"
os::cmd::expect_success 'oc get routes'
os::cmd::expect_success 'oc create -f test/integration/fixtures/test-route.json'
os::cmd::expect_success 'oc delete routes testroute'
os::cmd::expect_success 'oc create -f test/integration/fixtures/test-service.json'
os::cmd::expect_success 'oc create route passthrough --service=svc/frontend'
os::cmd::expect_success 'oc delete routes frontend'
os::cmd::expect_success 'oc create route edge --path /test --service=services/non-existent --port=80'
os::cmd::expect_success 'oc delete routes non-existent'
os::cmd::expect_success 'oc create route edge test-route --service=frontend'
os::cmd::expect_success 'oc delete routes test-route'
os::cmd::expect_failure 'oc create route edge new-route'
os::cmd::expect_success 'oc delete services frontend'
echo "routes: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/basicresources/setprobe"
# Validate the probe command
arg="-f examples/hello-openshift/hello-pod.json"
os::cmd::expect_failure_and_text "oc set probe" "error: one or more resources"
os::cmd::expect_failure_and_text "oc set probe ${arg}" "error: you must specify one of --readiness or --liveness"
os::cmd::expect_success_and_text "oc set probe ${arg} --liveness -o yaml" 'livenessProbe: \{\}'
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness --initial-delay-seconds=10 -o yaml" "livenessProbe:"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness --initial-delay-seconds=10 -o yaml" "initialDelaySeconds: 10"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness -- echo test" "livenessProbe:"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --readiness -- echo test" "readinessProbe:"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness -- echo test" "exec:"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness -- echo test" "\- echo"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness -- echo test" "\- test"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness --open-tcp=3306" "tcpSocket:"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness --open-tcp=3306" "port: 3306"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness --open-tcp=port" "port: port"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness --get-url=https://127.0.0.1:port/path" "port: port"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness --get-url=https://127.0.0.1:8080/path" "port: 8080"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness --get-url=https://127.0.0.1/path" 'port: ""'
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness --get-url=https://127.0.0.1:port/path" "path: /path"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness --get-url=https://127.0.0.1:port/path" "scheme: HTTPS"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness --get-url=http://127.0.0.1:port/path" "scheme: HTTP"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness --get-url=https://127.0.0.1:port/path" "host: 127.0.0.1"
os::cmd::expect_success_and_text "oc set probe ${arg} -o yaml --liveness --get-url=https://127.0.0.1:port/path" "port: port"
os::cmd::expect_success "oc create -f test/integration/fixtures/test-deployment-config.yaml"
os::cmd::expect_failure_and_text "oc set probe dc/test-deployment-config --liveness" "Required value: must specify a handler type"
os::cmd::expect_success_and_text "oc set probe dc test-deployment-config --liveness --open-tcp=8080" "updated"
os::cmd::expect_success_and_text "oc set probe dc/test-deployment-config --liveness --open-tcp=8080" "was not changed"
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o yaml" "livenessProbe:"
os::cmd::expect_success_and_text "oc set probe dc/test-deployment-config --liveness --initial-delay-seconds=10" "updated"
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o yaml" "initialDelaySeconds: 10"
os::cmd::expect_success_and_text "oc set probe dc/test-deployment-config --liveness --initial-delay-seconds=20" "updated"
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o yaml" "initialDelaySeconds: 20"
os::cmd::expect_success_and_text "oc set probe dc/test-deployment-config --liveness --failure-threshold=2" "updated"
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o yaml" "initialDelaySeconds: 20"
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o yaml" "failureThreshold: 2"
os::cmd::expect_success_and_text "oc set probe dc/test-deployment-config --readiness --success-threshold=4 -- echo test" "updated"
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o yaml" "initialDelaySeconds: 20"
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o yaml" "successThreshold: 4"
os::cmd::expect_success_and_text "oc set probe dc test-deployment-config --liveness --period-seconds=5" "updated"
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o yaml" "periodSeconds: 5"
os::cmd::expect_success_and_text "oc set probe dc/test-deployment-config --liveness --timeout-seconds=6" "updated"
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o yaml" "timeoutSeconds: 6"
os::cmd::expect_success_and_text "oc set probe dc --all --liveness --timeout-seconds=7" "updated"
os::cmd::expect_success_and_text "oc get dc -o yaml" "timeoutSeconds: 7"
os::cmd::expect_success_and_text "oc set probe dc/test-deployment-config --liveness --remove" "updated"
os::cmd::expect_success_and_not_text "oc get dc/test-deployment-config -o yaml" "livenessProbe"
os::cmd::expect_success "oc delete dc/test-deployment-config"
echo "set probe: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/basicresources/setenv"
os::cmd::expect_success "oc create -f test/integration/fixtures/test-deployment-config.yaml"
os::cmd::expect_success "oc create -f test/integration/fixtures/test-buildcli.json"
os::cmd::expect_success_and_text "oc set env dc/test-deployment-config FOO=bar" "updated"
os::cmd::expect_success_and_text "oc set env dc/test-deployment-config --list" "FOO=bar"
os::cmd::expect_success_and_text "oc set env bc --all FOO=bar" "updated"
os::cmd::expect_success_and_text "oc set env bc --all --list" "FOO=bar"
os::cmd::expect_success_and_text "oc set env bc --all FOO-" "updated"
os::cmd::expect_success "oc delete dc/test-deployment-config"
os::cmd::expect_success "oc delete bc/ruby-sample-build-validtag"
echo "set env: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/basicresources/expose"
# Expose service as a route
os::cmd::expect_success 'oc create -f test/integration/fixtures/test-service.json'
os::cmd::expect_failure 'oc expose service frontend --create-external-load-balancer'
os::cmd::expect_failure 'oc expose service frontend --port=40 --type=NodePort'
os::cmd::expect_success 'oc expose service frontend --path=/test'
os::cmd::expect_success_and_text "oc get route frontend --output-version=v1 --template='{{.spec.path}}'" "/test"
os::cmd::expect_success_and_text "oc get route frontend --output-version=v1 --template='{{.spec.to.name}}'" "frontend"           # routes to correct service
os::cmd::expect_success_and_text "oc get route frontend --output-version=v1 --template='{{.spec.port.targetPort}}'" "<no value>" # no target port for services with unnamed ports
os::cmd::expect_success 'oc delete svc,route -l name=frontend'
# Test that external services are exposable
os::cmd::expect_success 'oc create -f test/fixtures/external-service.yaml'
os::cmd::expect_success 'oc expose svc/external'
os::cmd::expect_success_and_text 'oc get route external' 'external=service'
os::cmd::expect_success 'oc delete route external'
os::cmd::expect_success 'oc delete svc external'
# Expose multiport service and verify we set a port in the route
os::cmd::expect_success 'oc create -f test/fixtures/multiport-service.yaml'
os::cmd::expect_success 'oc expose svc/frontend --name route-with-set-port'
os::cmd::expect_success_and_text "oc get route route-with-set-port --template='{{.spec.port.targetPort}}' --output-version=v1" "web"
echo "expose: ok"
os::test::junit::declare_suite_end

os::cmd::expect_success 'oc delete all --all'

os::test::junit::declare_suite_start "cmd/basicresources/projectadmin"
# switch to test user to be sure that default project admin policy works properly
new="$(mktemp -d)/tempconfig"
os::cmd::expect_success "oc config view --raw > $new"
export KUBECONFIG=$new
project=$(oc project -q)
os::cmd::expect_success 'oc policy add-role-to-user admin test-user'
os::cmd::expect_success 'oc login -u test-user -p anything'
os::cmd::try_until_success 'oc project ${project}'

os::cmd::expect_success 'oc run --image=openshift/hello-openshift test'
os::cmd::expect_success 'oc run --image=openshift/hello-openshift --generator=run-controller/v1 test2'
os::cmd::expect_success 'oc run --image=openshift/hello-openshift --restart=Never test3'
os::cmd::expect_success 'oc run --image=openshift/hello-openshift --generator=job/v1beta1 --restart=Never test4'
os::cmd::expect_success 'oc delete dc/test rc/test2 pod/test3 job/test4'

os::cmd::expect_success_and_text 'oc run --dry-run foo --image=bar -o "go-template={{.kind}} {{.apiVersion}}"'                                'DeploymentConfig v1'
os::cmd::expect_success_and_text 'oc run --dry-run foo --image=bar -o "go-template={{.kind}} {{.apiVersion}}" --restart=Always'               'DeploymentConfig v1'
os::cmd::expect_success_and_text 'oc run --dry-run foo --image=bar -o "go-template={{.kind}} {{.apiVersion}}" --restart=Never'                'Pod v1'
# TODO: version ordering is unstable between Go 1.4 and Go 1.6 because of import order
os::cmd::expect_success_and_text 'oc run --dry-run foo --image=bar -o "go-template={{.kind}} {{.apiVersion}}" --output-version=extensions/v1beta1 --generator=job/v1beta1'        'Job extensions/v1beta1'
os::cmd::expect_success_and_text 'oc run --dry-run foo --image=bar -o "go-template={{.kind}} {{.apiVersion}}" --generator=job/v1'              'Job batch/v1'
os::cmd::expect_success_and_text 'oc run --dry-run foo --image=bar -o "go-template={{.kind}} {{.apiVersion}}" --generator=deploymentconfig/v1' 'DeploymentConfig v1'
os::cmd::expect_success_and_text 'oc run --dry-run foo --image=bar -o "go-template={{.kind}} {{.apiVersion}}" --generator=run-controller/v1'   'ReplicationController v1'
os::cmd::expect_success_and_text 'oc run --dry-run foo --image=bar -o "go-template={{.kind}} {{.apiVersion}}" --generator=run/v1'              'ReplicationController v1'
os::cmd::expect_success_and_text 'oc run --dry-run foo --image=bar -o "go-template={{.kind}} {{.apiVersion}}" --generator=run-pod/v1'          'Pod v1'
os::cmd::expect_success_and_text 'oc run --dry-run foo --image=bar -o "go-template={{.kind}} {{.apiVersion}}" --generator=deployment/v1beta1'  'Deployment extensions/v1beta1'

os::cmd::expect_success 'oc process -f examples/sample-app/application-template-stibuild.json -l name=mytemplate | oc create -f -'
os::cmd::expect_success 'oc delete all -l name=mytemplate'
os::cmd::expect_success 'oc new-app https://github.com/openshift/ruby-hello-world'
os::cmd::expect_success 'oc get dc/ruby-hello-world'

os::cmd::expect_success_and_text "oc get dc/ruby-hello-world --template='{{ .spec.replicas }}'" '1'
patch='{"spec": {"replicas": 2}}'
os::cmd::expect_success "oc patch dc/ruby-hello-world -p '${patch}'"
os::cmd::expect_success_and_text "oc get dc/ruby-hello-world --template='{{ .spec.replicas }}'" '2'

os::cmd::expect_success 'oc delete all -l app=ruby-hello-world'
os::cmd::expect_failure 'oc get dc/ruby-hello-world'
echo "delete all: ok"
os::test::junit::declare_suite_end

# service accounts should not be allowed to request new projects
os::cmd::expect_failure_and_text "oc new-project --token="$( oc sa get-token builder )" will-fail" 'Error from server: You may not request a new project via this API'

os::test::junit::declare_suite_start "cmd/basicresources/patch"
# Validate patching works correctly
oc login -u system:admin
# Clean up group if needed to be re-entrant
oc delete group patch-group || true
group_json='{"kind":"Group","apiVersion":"v1","metadata":{"name":"patch-group"}}'
os::cmd::expect_success          "echo '${group_json}' | oc create -f -"
os::cmd::expect_success_and_text 'oc get group patch-group -o yaml' 'users: null'
os::cmd::expect_success          "oc patch group patch-group -p 'users: [\"myuser\"]' --loglevel=8"
os::cmd::expect_success_and_text 'oc get group patch-group -o yaml' 'myuser'
os::cmd::expect_success          "oc patch group patch-group -p 'users: []' --loglevel=8"
os::cmd::expect_success_and_text 'oc get group patch-group -o yaml' 'users: \[\]'
echo "patch: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end

