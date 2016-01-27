#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

source "${OS_ROOT}/hack/lib/util/environment.sh"
os::util::environment::setup_time_vars

ROUTER_TESTS_ENABLED="${ROUTER_TESTS_ENABLED:-true}"
TEST_ASSETS="${TEST_ASSETS:-false}"

export VERBOSE=true

function wait_for_app() {
  echo "[INFO] Waiting for app in namespace $1"
  echo "[INFO] Waiting for database pod to start"
  os::cmd::try_until_text "oc get -n $1 pods -l name=database" 'Running'
  os::cmd::expect_success "oc logs dc/database -n $1 --follow"

  echo "[INFO] Waiting for database service to start"
  os::cmd::try_until_text "oc get -n $1 services" 'database' "$(( 2 * TIME_MIN ))"
  DB_IP=$(oc get -n $1 --output-version=v1beta3 --template="{{ .spec.portalIP }}" service database)

  echo "[INFO] Waiting for frontend pod to start"
  os::cmd::try_until_text "oc get -n $1 pods" 'frontend.+Running' "$(( 2 * TIME_MIN ))"
  os::cmd::expect_success "oc logs dc/frontend -n $1 --follow"

  echo "[INFO] Waiting for frontend service to start"
  os::cmd::try_until_text "oc get -n $1 services" 'frontend' "$(( 2 * TIME_MIN ))"
  FRONTEND_IP=$(oc get -n $1 --output-version=v1beta3 --template="{{ .spec.portalIP }}" service frontend)

  echo "[INFO] Waiting for database to start..."
  wait_for_url_timed "http://${DB_IP}:5434" "[INFO] Database says: " "$(( 3 * TIME_MIN ))"

  echo "[INFO] Waiting for app to start..."
  wait_for_url_timed "http://${FRONTEND_IP}:5432" "[INFO] Frontend says: " "$(( 2 * TIME_MIN ))"

  echo "[INFO] Testing app"
  os::cmd::try_until_text "curl -s -X POST http://${FRONTEND_IP}:5432/keys/foo -d value=1337" 'Key created'
  os::cmd::try_until_text "curl -s http://${FRONTEND_IP}:5432/keys/foo" '1337'
}

# service dns entry is visible via master service
# find the IP of the master service by asking the API_HOST to verify DNS is running there
MASTER_SERVICE_IP="$(dig @${API_HOST} "kubernetes.default.svc.cluster.local." +short A | head -n 1)"
# find the IP of the master service again by asking the IP of the master service, to verify port 53 tcp/udp is routed by the service
os::cmd::expect_success_and_text "dig +tcp @${MASTER_SERVICE_IP} kubernetes.default.svc.cluster.local. +short A | head -n 1" "${MASTER_SERVICE_IP}"
os::cmd::expect_success_and_text "dig +notcp @${MASTER_SERVICE_IP} kubernetes.default.svc.cluster.local. +short A | head -n 1" "${MASTER_SERVICE_IP}"

# add e2e-user as a viewer for the default namespace so we can see infrastructure pieces appear
os::cmd::expect_success 'openshift admin policy add-role-to-user view e2e-user --namespace=default'

# pre-load some image streams and templates
os::cmd::expect_success 'oc create -f examples/image-streams/image-streams-centos7.json --namespace=openshift'
os::cmd::expect_success 'oc create -f examples/sample-app/application-template-stibuild.json --namespace=openshift'
os::cmd::expect_success 'oc create -f examples/jenkins/application-template.json --namespace=openshift'
os::cmd::expect_success 'oc create -f examples/db-templates/mongodb-ephemeral-template.json --namespace=openshift'
os::cmd::expect_success 'oc create -f examples/db-templates/mysql-ephemeral-template.json --namespace=openshift'
os::cmd::expect_success 'oc create -f examples/db-templates/postgresql-ephemeral-template.json --namespace=openshift'

# create test project so that this shows up in the console
os::cmd::expect_success "openshift admin new-project test --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"
os::cmd::expect_success "openshift admin new-project docker --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"
os::cmd::expect_success "openshift admin new-project custom --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"
os::cmd::expect_success "openshift admin new-project cache --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"

echo "The console should be available at ${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}/console."
echo "Log in as 'e2e-user' to see the 'test' project."

install_router
install_registry

echo "[INFO] Pre-pulling and pushing ruby-22-centos7"
os::cmd::expect_success 'docker pull centos/ruby-22-centos7:latest'
echo "[INFO] Pulled ruby-22-centos7"

echo "[INFO] Waiting for Docker registry pod to start"
wait_for_registry

# services can end up on any IP.  Make sure we get the IP we need for the docker registry
DOCKER_REGISTRY=$(oc get --output-version=v1beta3 --template="{{ .spec.portalIP }}:{{ with index .spec.ports 0 }}{{ .port }}{{ end }}" service docker-registry)

os::cmd::expect_success_and_text "dig @${API_HOST} docker-registry.default.svc.cluster.local. +short A | head -n 1 | xargs printf '%s:5000'" "${DOCKER_REGISTRY}"

echo "[INFO] Verifying the docker-registry is up at ${DOCKER_REGISTRY}"
wait_for_url_timed "http://${DOCKER_REGISTRY}" "[INFO] Docker registry says: " "$(( 2 * TIME_MIN ))"
# ensure original healthz route works as well
os::cmd::expect_success "curl -f http://${DOCKER_REGISTRY}/healthz"

os::cmd::expect_success "dig @${API_HOST} docker-registry.default.local. A"

# Client setup (log in as e2e-user and set 'test' as the default project)
# This is required to be able to push to the registry!
echo "[INFO] Logging in as a regular user (e2e-user:pass) with project 'test'..."
os::cmd::expect_success 'oc login -u e2e-user -p pass'
os::cmd::expect_success_and_text 'oc whoami' 'e2e-user'
 
# make sure viewers can see oc status
os::cmd::expect_success 'oc status -n default'

# check to make sure a project admin can push an image
os::cmd::expect_success 'oc project cache'
e2e_user_token=$(oc config view --flatten --minify -o template --template='{{with index .users 0}}{{.user.token}}{{end}}')
os::cmd::expect_success_and_text "echo ${e2e_user_token}" '.+'

echo "[INFO] Docker login as e2e-user to ${DOCKER_REGISTRY}"
os::cmd::expect_success "docker login -u e2e-user -p ${e2e_user_token} -e e2e-user@openshift.com ${DOCKER_REGISTRY}"
echo "[INFO] Docker login successful"

echo "[INFO] Tagging and pushing ruby-22-centos7 to ${DOCKER_REGISTRY}/cache/ruby-22-centos7:latest"
os::cmd::expect_success "docker tag -f centos/ruby-22-centos7:latest ${DOCKER_REGISTRY}/cache/ruby-22-centos7:latest"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/cache/ruby-22-centos7:latest"
echo "[INFO] Pushed ruby-22-centos7"

# check to make sure an image-pusher can push an image
os::cmd::expect_success 'oc policy add-role-to-user system:image-pusher pusher'
os::cmd::expect_success 'oc login -u pusher -p pass'
pusher_token=$(oc config view --flatten --minify -o template --template='{{with index .users 0}}{{.user.token}}{{end}}')
os::cmd::expect_success_and_text "echo ${pusher_token}" '.+'

echo "[INFO] Docker login as pusher to ${DOCKER_REGISTRY}"
os::cmd::expect_success "docker login -u e2e-user -p ${pusher_token} -e pusher@openshift.com ${DOCKER_REGISTRY}"
echo "[INFO] Docker login successful"

# log back into docker as e2e-user again
os::cmd::expect_success "docker login -u e2e-user -p ${e2e_user_token} -e e2e-user@openshift.com ${DOCKER_REGISTRY}"

echo "[INFO] Back to 'default' project with 'admin' user..."
os::cmd::expect_success "oc project ${CLUSTER_ADMIN_CONTEXT}"
os::cmd::expect_success_and_text 'oc whoami' 'system:admin'

# The build requires a dockercfg secret in the builder service account in order
# to be able to push to the registry.  Make sure it exists first.
echo "[INFO] Waiting for dockercfg secrets to be generated in project 'test' before building"
os::cmd::try_until_text 'oc get -n test serviceaccount/builder -o yaml' 'dockercfg'

# Process template and create
echo "[INFO] Submitting application template json for processing..."
STI_CONFIG_FILE="${ARTIFACT_DIR}/stiAppConfig.json"
DOCKER_CONFIG_FILE="${ARTIFACT_DIR}/dockerAppConfig.json"
CUSTOM_CONFIG_FILE="${ARTIFACT_DIR}/customAppConfig.json"
os::cmd::expect_success "oc process -n test -f examples/sample-app/application-template-stibuild.json > '${STI_CONFIG_FILE}'"
os::cmd::expect_success "oc process -n docker -f examples/sample-app/application-template-dockerbuild.json > '${DOCKER_CONFIG_FILE}'"
os::cmd::expect_success "oc process -n custom -f examples/sample-app/application-template-custombuild.json > '${CUSTOM_CONFIG_FILE}'"

echo "[INFO] Back to 'test' context with 'e2e-user' user"
os::cmd::expect_success 'oc login -u e2e-user'
os::cmd::expect_success 'oc project test'
os::cmd::expect_success 'oc whoami'

echo "[INFO] Running a CLI command in a container using the service account"
os::cmd::expect_success 'oc policy add-role-to-user view -z default'
os::cmd::expect_success "oc run cli-with-token --attach --env=POD_NAMESPACE=test --image=openshift/origin:${TAG} --restart=Never -- cli status --loglevel=4 > '${LOG_DIR}/cli-with-token.log' 2>&1"
os::cmd::expect_success_and_text "cat '${LOG_DIR}/cli-with-token.log'" 'Using in-cluster configuration'
os::cmd::expect_success_and_text "cat '${LOG_DIR}/cli-with-token.log'" 'In project test'
os::cmd::expect_success 'oc delete pod cli-with-token'

os::cmd::expect_success "oc run cli-with-token-2 --attach --env=POD_NAMESPACE=test --image=openshift/origin:${TAG} --restart=Never -- cli whoami --loglevel=4 > '${LOG_DIR}/cli-with-token2.log' 2>&1"
os::cmd::expect_success_and_text "cat '${LOG_DIR}/cli-with-token2.log'" 'system:serviceaccount:test:default'
os::cmd::expect_success 'oc delete pod cli-with-token-2'

os::cmd::expect_success "oc run kubectl-with-token --attach --env=POD_NAMESPACE=test --image=openshift/origin:${TAG} --restart=Never --command -- kubectl get pods --loglevel=4 > '${LOG_DIR}/kubectl-with-token.log' 2>&1"
os::cmd::expect_success_and_text "cat '${LOG_DIR}/kubectl-with-token.log'" 'Using in-cluster configuration'
os::cmd::expect_success_and_text "cat '${LOG_DIR}/kubectl-with-token.log'" 'kubectl-with-token'

echo "[INFO] Streaming the logs from a deployment twice..."
os::cmd::expect_success 'oc create -f test/fixtures/failing-dc.yaml'
os::cmd::try_until_success 'oc get rc/failing-dc-1'
os::cmd::expect_success 'oc logs -f dc/failing-dc'
os::cmd::try_until_text 'oc get rc/failing-dc-1 --template={{.metadata.annotations}}' 'openshift.io/deployment.phase:Failed'
os::cmd::expect_success_and_text 'oc logs dc/failing-dc' 'test pre hook executed'
os::cmd::expect_success 'oc deploy failing-dc --latest'
os::cmd::expect_success 'oc logs --version=1 dc/failing-dc'

echo "[INFO] Run pod diagnostics"
# Requires a node to run the pod; uses origin-deployer pod, expects registry deployed
openshift ex diagnostics DiagnosticPod --images="${USE_IMAGES}"

echo "[INFO] Applying STI application config"
os::cmd::expect_success "oc create -f ${STI_CONFIG_FILE}"

# Wait for build which should have triggered automatically
echo "[INFO] Starting build from ${STI_CONFIG_FILE} and streaming its logs..."
os::cmd::try_until_text 'oc get -n test builds' 'Running'
 
# Ensure that the build pod doesn't allow exec
BUILD_ID=$(oc get -n test builds  --output-version=v1 --template="{{with index .items 0}}{{.metadata.name}}{{end}}")
os::cmd::expect_failure_and_text "oc rsh ${BUILD_ID}-build" 'forbidden'
os::cmd::try_until_text 'oc get -n test builds' 'Complete' $((10*TIME_MIN))
os::cmd::expect_success "oc build-logs -n test ${BUILD_ID} > '${LOG_DIR}/testbuild.log'"
wait_for_app "test"

# logs can't be tested without a node, so has to be in e2e
POD_NAME=$(oc get pods -n test --template='{{(index .items 0).metadata.name}}')
os::cmd::expect_success "oc logs pod/${POD_NAME} --loglevel=6"
os::cmd::expect_success "oc logs ${POD_NAME} --loglevel=6"

BUILD_NAME=$(oc get builds -n test --template='{{(index .items 0).metadata.name}}')
os::cmd::expect_success "oc logs build/${BUILD_NAME} --loglevel=6"
os::cmd::expect_success "oc logs build/${BUILD_NAME} --loglevel=6"
os::cmd::expect_success 'oc logs bc/ruby-sample-build --loglevel=6'
os::cmd::expect_success 'oc logs buildconfigs/ruby-sample-build --loglevel=6'
os::cmd::expect_success 'oc logs buildconfig/ruby-sample-build --loglevel=6'
echo "logs: ok"

echo "[INFO] Starting a deployment to test scaling..."
os::cmd::expect_success 'oc create -f test/integration/fixtures/test-deployment-config.json'
# scaling which might conflict with the deployment should work
os::cmd::expect_success 'oc scale dc/test-deployment-config --replicas=2'
os::cmd::try_until_text 'oc get rc/test-deployment-config-1 -o yaml' 'Complete'
# scale rc via deployment configuration
os::cmd::expect_success 'oc scale dc/test-deployment-config --replicas=3 --timeout=1m'
os::cmd::expect_success 'oc delete dc/test-deployment-config'
echo "scale: ok"

echo "[INFO] Starting build from ${STI_CONFIG_FILE} with non-existing commit..."
os::cmd::expect_failure 'oc start-build test --commit=fffffff --wait'

# Remote command execution
echo "[INFO] Validating exec"
frontend_pod=$(oc get pod -l deploymentconfig=frontend --template='{{(index .items 0).metadata.name}}')
# when running as a restricted pod the registry will run with a pre-allocated
# user in the neighborhood of 1000000+.  Look for a substring of the pre-allocated uid range
os::cmd::expect_success_and_text "oc exec -p ${frontend_pod} id" '1000'
os::cmd::expect_success_and_text "oc rsh ${frontend_pod} id -u" '1000'
os::cmd::expect_success_and_text "oc rsh -T ${frontend_pod} id -u" '1000'

# Port forwarding
echo "[INFO] Validating port-forward"
os::cmd::expect_success "oc port-forward -p ${frontend_pod} 10080:8080  &> '${LOG_DIR}/port-forward.log' &"
wait_for_url_timed "http://localhost:10080" "[INFO] Frontend says: " $((10*TIME_SEC))

# Rsync
echo "[INFO] Validating rsync"
os::cmd::expect_success "oc rsync examples/sample-app ${frontend_pod}:/tmp"
os::cmd::expect_success_and_text "oc rsh ${frontend_pod} ls /tmp/sample-app" 'application-template-stibuild'

# echo "[INFO] Applying Docker application config"
# os::cmd::expect_success "oc create -n docker -f ${DOCKER_CONFIG_FILE}"
# echo "[INFO] Invoking generic web hook to trigger new docker build using curl"
# os::cmd::expect_success "curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/osapi/v1beta3/namespaces/docker/buildconfigs/ruby-sample-build/webhooks/secret101/generic"
# sleep 3
# os::cmd::try_until_text 'oc get -n docker builds' 'Complete'
# BUILD_ID=$(oc get -n docker builds --output-version=v1 --template="{{with index .items 0}}{{.metadata.name}}{{end}}")
# os::cmd::expect_success "oc build-logs -n docker $BUILD_ID > '$LOG_DIR/dockerbuild.log'"
# wait_for_app "docker"

# echo "[INFO] Applying Custom application config"
# os::cmd::expect_success "oc create -n custom -f ${CUSTOM_CONFIG_FILE}"
# echo "[INFO] Invoking generic web hook to trigger new custom build using curl"
# os::cmd::expect_success "curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/osapi/v1beta3/namespaces/custom/buildconfigs/ruby-sample-build/webhooks/secret101/generic"
# sleep 3
# os::cmd::try_until_text 'oc get -n custom builds' 'Complete'
# BUILD_ID=$(oc get -n custom builds --output-version=v1 --template="{{with index .items 0}}{{.metadata.name}}{{end}}")
# os::cmd::expect_success "oc build-logs -n custom $BUILD_ID > '$LOG_DIR/custombuild.log'"
# wait_for_app "custom"

echo "[INFO] Back to 'default' project with 'admin' user..."
os::cmd::expect_success "oc project ${CLUSTER_ADMIN_CONTEXT}"

# ensure the router is started
# TODO: simplify when #4702 is fixed upstream
os::cmd::try_until_text "oc get endpoints router --output-version=v1beta3 --template='{{ if .subsets }}{{ len .subsets }}{{ else }}0{{ end }}'" '[1-9]+' $((5*TIME_MIN))

# Check for privileged exec limitations.
echo "[INFO] Validating privileged pod exec"
router_pod=$(oc get pod -n default -l deploymentconfig=router --template='{{(index .items 0).metadata.name}}')
os::cmd::expect_success 'oc policy add-role-to-user admin e2e-default-admin'
# login as a user that can't run privileged pods
os::cmd::expect_success 'oc login -u e2e-default-admin -p pass'
os::cmd::expect_failure_and_text "oc exec -n default -tip ${router_pod} ls" 'unable to validate against any security context constraint'
# system:admin should be able to exec into it
os::cmd::expect_success "oc project ${CLUSTER_ADMIN_CONTEXT}"
os::cmd::expect_success "oc exec -n default -tip ${router_pod} ls"


echo "[INFO] Validating routed app response..."
# use the docker bridge ip address until there is a good way to get the auto-selected address from master
# this address is considered stable
# used as a resolve IP to test routing
CONTAINER_ACCESSIBLE_API_HOST="${CONTAINER_ACCESSIBLE_API_HOST:-172.17.42.1}"
os::cmd::try_until_text "curl -s -k --resolve www.example.com:443:${CONTAINER_ACCESSIBLE_API_HOST} https://www.example.com" "Hello from OpenShift"


# Pod node selection
echo "[INFO] Validating pod.spec.nodeSelector rejections"
# Create a project that enforces an impossible to satisfy nodeSelector, and two pods, one of which has an explicit node name
os::cmd::expect_success "openshift admin new-project node-selector --description='This is an example project to test node selection prevents deployment' --admin='e2e-user' --node-selector='impossible-label=true'"
NODE_NAME=`oc get node --no-headers | awk '{print $1}'`
os::cmd::expect_success "oc process -n node-selector -v NODE_NAME='${NODE_NAME}' -f test/fixtures/node-selector/pods.json | oc create -n node-selector -f -"
# The pod without a node name should fail to schedule
os::cmd::try_until_text 'oc get events -n node-selector' 'pod-without-node-name.+FailedScheduling' $((20*TIME_SEC))
# The pod with a node name should be rejected by the kubelet
os::cmd::try_until_text 'oc get events -n node-selector' 'pod-with-node-name.+NodeSelectorMismatching' $((20*TIME_SEC))


# Image pruning
echo "[INFO] Validating image pruning"
os::cmd::expect_success 'docker pull busybox'
os::cmd::expect_success 'docker pull gcr.io/google_containers/pause'
os::cmd::expect_success 'docker pull openshift/hello-openshift'

# tag and push 1st image - layers unique to this image will be pruned
os::cmd::expect_success "docker tag -f busybox ${DOCKER_REGISTRY}/cache/prune"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/cache/prune"

# tag and push 2nd image - layers unique to this image will be pruned
os::cmd::expect_success "docker tag -f openshift/hello-openshift ${DOCKER_REGISTRY}/cache/prune"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/cache/prune"

# tag and push 3rd image - it won't be pruned
os::cmd::expect_success "docker tag -f gcr.io/google_containers/pause ${DOCKER_REGISTRY}/cache/prune"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/cache/prune"

# record the storage before pruning
registry_pod=$(oc get pod -l deploymentconfig=docker-registry --template='{{(index .items 0).metadata.name}}')
os::cmd::expect_success "oc exec -p ${registry_pod} du /registry > '${LOG_DIR}/prune-images.before.txt'"

# set up pruner user
os::cmd::expect_success 'oadm policy add-cluster-role-to-user system:image-pruner e2e-pruner'
os::cmd::expect_success 'oc login -u e2e-pruner -p pass'

# run image pruning
os::cmd::expect_success "oadm prune images --keep-younger-than=0 --keep-tag-revisions=1 --confirm &> '${LOG_DIR}/prune-images.log'"
os::cmd::expect_success_and_not_text "cat ${LOG_DIR}/prune-images.log" 'error'

os::cmd::expect_success "oc project ${CLUSTER_ADMIN_CONTEXT}"
# record the storage after pruning
os::cmd::expect_success "oc exec -p ${registry_pod} du /registry > '${LOG_DIR}/prune-images.after.txt'"

# make sure there were changes to the registry's storage
os::cmd::expect_code "diff ${LOG_DIR}/prune-images.before.txt ${LOG_DIR}/prune-images.after.txt" 1

unset VERBOSE

# UI e2e tests can be found in assets/test/e2e
if [[ "$TEST_ASSETS" == "true" ]]; then

  if [[ "$TEST_ASSETS_HEADLESS" == "true" ]]; then
    echo "[INFO] Starting virtual framebuffer for headless tests..."
    export DISPLAY=:10
    Xvfb :10 -screen 0 1024x768x24 -ac &
  fi

  echo "[INFO] Running UI e2e tests at time..."
  echo `date`
  pushd ${OS_ROOT}/assets > /dev/null
    grunt test-integration
  echo "UI  e2e done at time "
  echo `date`

  popd > /dev/null

fi
