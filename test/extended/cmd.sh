#!/bin/bash
#
# This scripts starts the OpenShift server with a default configuration.
# No registry or router is setup.
# It is intended to test cli commands that may require docker and therefore
# cannot be run under Travis.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::install_errexit
os::util::environment::setup_time_vars

cd "${OS_ROOT}"

os::build::setup_env

function cleanup()
{
	out=$?
	docker rmi test/scratchimage
	cleanup_openshift
	echo "[INFO] Exiting"
	return "${out}"
}

trap "exit" INT TERM
trap "cleanup" EXIT

echo "[INFO] Starting server"

os::util::environment::setup_all_server_vars "test-extended/cmd/"
os::util::environment::use_sudo
reset_tmp_dir

os::log::start_system_logger

configure_os_server
start_os_server

export KUBECONFIG="${ADMIN_KUBECONFIG}"

oc login -u system:admin -n default
# let everyone be able to see stuff in the default namespace
oadm policy add-role-to-group view system:authenticated -n default

install_registry
wait_for_registry
docker_registry="$( oc get service/docker-registry -n default -o jsonpath='{.spec.clusterIP}:{.spec.ports[0].port}' )"

os::test::junit::declare_suite_start "extended/cmd"

os::test::junit::declare_suite_start "extended/cmd/new-app"
echo "[INFO] Running newapp extended tests"
oc login "${MASTER_ADDR}" -u new-app -p password --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt"
oc new-project new-app
oc delete all --all

# create a local-only docker image for testing
# image is removed in cleanup()
tmp=$(mktemp -d)
pushd "${tmp}"
cat <<-EOF >> Dockerfile
	FROM scratch
	EXPOSE 80
EOF
docker build -t test/scratchimage .
popd
rm -rf "${tmp}"


# ensure a local-only image gets a docker image(not imagestream) reference created.
VERBOSE=true os::cmd::expect_success "oc new-project test-scratchimage"
os::cmd::expect_success "oc new-app test/scratchimage~https://github.com/openshift/ruby-hello-world.git --strategy=docker"
os::cmd::expect_success_and_text "oc get bc ruby-hello-world -o jsonpath={.spec.strategy.dockerStrategy.from.kind}" "DockerImage"
os::cmd::expect_success_and_text "oc get bc ruby-hello-world -o jsonpath={.spec.strategy.dockerStrategy.from.name}" "test/scratchimage:latest"
os::cmd::expect_success "oc delete project test-scratchimage"
VERBOSE=true os::cmd::expect_success "oc project new-app"
# error due to partial match
os::cmd::expect_failure_and_text "oc new-app test/scratchimage2 -o yaml" "partial match"
# success with exact match	
os::cmd::expect_success "oc new-app test/scratchimage"
echo "[INFO] newapp: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "extended/cmd/variable-expansion"
echo "[INFO] Running env variable expansion tests"
VERBOSE=true os::cmd::expect_success "oc new-project envtest"
os::cmd::expect_success "oc create -f test/extended/fixtures/test-env-pod.json"
os::cmd::try_until_text "oc get pods" "Running"
os::cmd::expect_success_and_text "oc exec test-pod env" "podname=test-pod"
os::cmd::expect_success_and_text "oc exec test-pod env" "podname_composed=test-pod_composed"
os::cmd::expect_success_and_text "oc exec test-pod env" "var1=value1"
os::cmd::expect_success_and_text "oc exec test-pod env" "var2=value1"
os::cmd::expect_success_and_text "oc exec test-pod ps ax" "sleep 120"
echo "[INFO] variable-expansion: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "extended/cmd/image-pull-secrets"
echo "[INFO] Running image pull secrets tests"
VERBOSE=true os::cmd::expect_success "oc login '${MASTER_ADDR}' -u pull-secrets-user -p password --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt'"

# create a new project and push a busybox image in there
VERBOSE=true os::cmd::expect_success "oc new-project image-ns"
os::cmd::expect_success "oc delete all --all"
token="$( oc sa get-token builder )"
os::cmd::expect_success "docker login -u imagensbuilder -p ${token} -e fake@example.org ${docker_registry}"
os::cmd::expect_success "oc import-image busybox:latest --confirm"
os::cmd::expect_success "docker pull busybox"
os::cmd::expect_success "docker tag -f docker.io/busybox:latest ${docker_registry}/image-ns/busybox:latest"
os::cmd::expect_success "docker push ${docker_registry}/image-ns/busybox:latest"
os::cmd::expect_success "docker rmi -f ${docker_registry}/image-ns/busybox:latest"


DOCKER_CONFIG_JSON="${HOME}/.docker/config.json"
VERBOSE=true os::cmd::expect_success "oc new-project dc-ns"
os::cmd::expect_success "oc delete all --all"
os::cmd::expect_success "oc delete secrets --all"
os::cmd::expect_success "oc secrets new image-ns-pull .dockerconfigjson=${DOCKER_CONFIG_JSON}"
os::cmd::expect_success "oc secrets new-dockercfg image-ns-pull-old --docker-email=fake@example.org --docker-username=imagensbuilder --docker-server=${docker_registry} --docker-password=${token}"

os::cmd::expect_success "oc process -f test/extended/fixtures/image-pull-secrets/pod-with-no-pull-secret.yaml --value=DOCKER_REGISTRY=${docker_registry} | oc create -f - "
os::cmd::try_until_text "oc describe pod/no-pull-pod" "Back-off pulling image"
os::cmd::expect_success "oc delete pods --all"

os::cmd::expect_success "oc process -f test/extended/fixtures/image-pull-secrets/pod-with-new-pull-secret.yaml --value=DOCKER_REGISTRY=${docker_registry} | oc create -f - "
os::cmd::try_until_text "oc get pods/new-pull-pod -o jsonpath='{.status.containerStatuses[0].imageID}'" "docker"
os::cmd::expect_success "oc delete pods --all"
os::cmd::expect_success "docker rmi -f ${docker_registry}/image-ns/busybox:latest"

os::cmd::expect_success "oc process -f test/extended/fixtures/image-pull-secrets/pod-with-old-pull-secret.yaml --value=DOCKER_REGISTRY=${docker_registry} | oc create -f - "
os::cmd::try_until_text "oc get pods/old-pull-pod -o jsonpath='{.status.containerStatuses[0].imageID}'" "docker"
os::cmd::expect_success "oc delete pods --all"
os::cmd::expect_success "docker rmi -f ${docker_registry}/image-ns/busybox:latest"

os::cmd::expect_success "oc process -f test/extended/fixtures/image-pull-secrets/dc-with-old-pull-secret.yaml --value=DOCKER_REGISTRY=${docker_registry} | oc create -f - "
os::cmd::try_until_text "oc get pods/my-dc-old-1-hook-pre -o jsonpath='{.status.containerStatuses[0].imageID}'" "docker"
os::cmd::expect_success "oc delete all --all"
os::cmd::expect_success "docker rmi -f ${docker_registry}/image-ns/busybox:latest"

os::cmd::expect_success "oc process -f test/extended/fixtures/image-pull-secrets/dc-with-new-pull-secret.yaml --value=DOCKER_REGISTRY=${docker_registry} | oc create -f - "
os::cmd::try_until_text "oc get pods/my-dc-1-hook-pre -o jsonpath='{.status.containerStatuses[0].imageID}'" "docker"
os::cmd::expect_success "oc delete all --all"
os::cmd::expect_success "docker rmi -f ${docker_registry}/image-ns/busybox:latest"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "extended/cmd/service-signer"
# check to make sure that service serving cert signing works correctly
# nginx currently needs to run as root
os::cmd::expect_success "oc login -u system:admin -n default"
os::cmd::expect_success "oadm policy add-scc-to-user anyuid system:serviceaccount:service-serving-cert-generation:default"

os::cmd::expect_success "oc login -u serving-cert -p asdf"
VERBOSE=true os::cmd::expect_success "oc new-project service-serving-cert-generation"

os::cmd::expect_success 'oc create dc nginx --image=nginx -- sh -c "nginx -c /etc/nginx/nginx.conf && sleep 86400"'
os::cmd::expect_success "oc expose dc/nginx --port=443"
os::cmd::expect_success "oc annotate svc/nginx service.alpha.openshift.io/serving-cert-secret-name=nginx-ssl-key"
os::cmd::expect_success "oc volumes dc/nginx --add --secret-name=nginx-ssl-key  --mount-path=/etc/serving-cert"
os::cmd::expect_success "oc create configmap default-conf --from-file=test/extended/fixtures/service-serving-cert/nginx-serving-cert.conf"
os::cmd::expect_success "oc set volumes dc/nginx --add --configmap-name=default-conf --mount-path=/etc/nginx/conf.d"
os::cmd::try_until_text "oc get pods -l deployment-config.name=nginx" 'Running'

# break mac os
service_ip=$(oc get service/nginx -o=jsonpath={.spec.clusterIP})
os::cmd::try_until_success "curl --cacert ${MASTER_CONFIG_DIR}/service-signer.crt --resolve nginx.service-serving-cert-generation.svc:443:${service_ip} https://nginx.service-serving-cert-generation.svc:443"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end
