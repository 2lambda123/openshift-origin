#!/bin/bash

# This command checks that the built commands can function together for
# simple scenarios.  It does not require Docker so it can run in travis.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

os::log::install_errexit

function cleanup()
{
    out=$?
    pkill -P $$

    if [ $out -ne 0 ]; then
        echo "[FAIL] !!!!! Test Failed !!!!"
    else
        echo
        echo "Complete"
    fi
    exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

set -e

USE_LOCAL_IMAGES=${USE_LOCAL_IMAGES:-true}

ETCD_HOST=${ETCD_HOST:-127.0.0.1}
ETCD_PORT=${ETCD_PORT:-4001}
API_SCHEME=${API_SCHEME:-https}
API_PORT=${API_PORT:-8443}
API_HOST=${API_HOST:-127.0.0.1}
KUBELET_SCHEME=${KUBELET_SCHEME:-http}
KUBELET_PORT=${KUBELET_PORT:-10250}

TEMP_DIR=$(mktemp -d /tmp/openshift-cmd.XXXX)
ETCD_DATA_DIR="${TEMP_DIR}/etcd"
VOLUME_DIR="${TEMP_DIR}/volumes"
CERT_DIR="${TEMP_DIR}/certs"
mkdir -p "${ETCD_DATA_DIR}" "${VOLUME_DIR}" "${CERT_DIR}"

# handle profiling defaults
profile="${OPENSHIFT_PROFILE-}"
unset OPENSHIFT_PROFILE
if [[ -n "${profile}" ]]; then
    if [[ "${TEST_PROFILE-}" == "cli" ]]; then
        export CLI_PROFILE="${profile}"
    else
        export WEB_PROFILE="${profile}"
    fi
fi

# set path so OpenShift is available
GO_OUT="${OS_ROOT}/_output/local/go/bin"
export PATH="${GO_OUT}:${PATH}"

# Check openshift version
out=$(openshift version)
echo openshift: $out

# profile the web
export OPENSHIFT_PROFILE="${WEB_PROFILE-}"

# Start openshift
OPENSHIFT_ON_PANIC=crash openshift start --master="${API_SCHEME}://${API_HOST}:${API_PORT}" --listen="${API_SCHEME}://${API_HOST}:${API_PORT}" --hostname="${API_HOST}" --volume-dir="${VOLUME_DIR}" --cert-dir="${CERT_DIR}" --etcd-dir="${ETCD_DATA_DIR}" 1>&2 &
OS_PID=$!

if [[ "${API_SCHEME}" == "https" ]]; then
    export CURL_CA_BUNDLE="${CERT_DIR}/admin/root.crt"
fi
wait_for_url "http://${API_HOST}:${KUBELET_PORT}/healthz" "kubelet: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1beta1/minions/127.0.0.1" "apiserver(minions): " 0.25 80

# Set KUBERNETES_MASTER for osc
export KUBERNETES_MASTER="${API_SCHEME}://${API_HOST}:${API_PORT}"
if [[ "${API_SCHEME}" == "https" ]]; then
	# Make osc use ${CERT_DIR}/admin/.kubeconfig, and ignore anything in the running user's $HOME dir
	export HOME="${CERT_DIR}/admin"
	export KUBECONFIG="${CERT_DIR}/admin/.kubeconfig"
fi

# profile the cli commands
export OPENSHIFT_PROFILE="${CLI_PROFILE-}"

#
# Begin tests
#

# verify some default commands
[ "$(openshift cli)" ]
[ "$(openshift ex)" ]
[ "$(openshift ex config 2>&1)" ]
[ "$(openshift ex tokens)" ]
[ "$(openshift ex policy  2>&1)" ]
[ "$(openshift kubectl)" ]
[ "$(openshift kube 2>&1)" ]

# help for root commands must be consistent
[ "$(openshift | grep 'OpenShift for Admins')" ]
[ "$(osc | grep 'OpenShift Client')" ]
[ "$(openshift cli | grep 'OpenShift Client')" ]
[ "$(openshift kubectl | grep 'OpenShift Client')" ]

# help for root commands with --help flag must be consistent
[ "$(openshift --help 2>&1 | grep 'OpenShift for Admins')" ]
[ "$(osc --help 2>&1 | grep 'OpenShift Client')" ]
[ "$(openshift cli --help 2>&1 | grep 'OpenShift Client')" ]
[ "$(openshift kubectl --help 2>&1 | grep 'OpenShift Client')" ]

# help for root commands through help command must be consistent
[ "$(openshift help cli 2>&1 | grep 'OpenShift Client')" ]
[ "$(openshift help kubectl 2>&1 | grep 'OpenShift Client')" ]

# help for given command with --help flag must be consistent
[ "$(osc get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift cli get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift kubectl get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift start --help 2>&1 | grep 'Start an OpenShift server')" ]

# help for given command through help command must be consistent
[ "$(osc help get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift cli help get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift kubectl help get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift help start 2>&1 | grep 'Start an OpenShift server')" ]

# runnable commands with required flags must error consistently
[ "$(osc get 2>&1 | grep 'you must provide one or more resources')" ]
[ "$(openshift cli get 2>&1 | grep 'you must provide one or more resources')" ]
[ "$(openshift kubectl get 2>&1 | grep 'you must provide one or more resources')" ]

osc get pods --match-server-version
osc create -f examples/hello-openshift/hello-pod.json
osc delete pods hello-openshift
echo "pods: ok"

osc get services
osc create -f test/integration/fixtures/test-service.json
osc delete services frontend
echo "services: ok"

osc get minions
echo "minions: ok"

osc get images
osc create -f test/integration/fixtures/test-image.json
osc delete images test
echo "images: ok"

osc get imageRepositories
osc create -f test/integration/fixtures/test-image-repository.json
[ -z "$(osc get imageRepositories test -t "{{.status.dockerImageRepository}}")" ]
osc create -f test/integration/fixtures/docker-registry-config.json
[ -n "$(osc get imageRepositories test -t "{{.status.dockerImageRepository}}")" ]
osc delete -f test/integration/fixtures/docker-registry-config.json
osc delete imageRepositories test
[ -z "$(osc get imageRepositories test -t "{{.status.dockerImageRepository}}")" ]
osc create -f examples/image-repositories/image-repositories.json
[ -n "$(osc get imageRepositories ruby-20-centos -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageRepositories nodejs-0-10-centos -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageRepositories wildfly-8-centos -t "{{.status.dockerImageRepository}}")" ]
osc delete imageRepositories ruby-20-centos
osc delete imageRepositories nodejs-0-10-centos
osc delete imageRepositories wildfly-8-centos
[ -z "$(osc get imageRepositories ruby-20-centos -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageRepositories nodejs-0-10-centos -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageRepositories wildfly-8-centos -t "{{.status.dockerImageRepository}}")" ]
echo "imageRepositories: ok"

osc create -f test/integration/fixtures/test-image-repository.json
osc create -f test/integration/fixtures/test-mapping.json
osc get images
osc get imageRepositories
osc delete imageRepositories test
echo "imageRepositoryMappings: ok"

osc get routes
osc create -f test/integration/fixtures/test-route.json create routes
osc delete routes testroute
echo "routes: ok"

osc get deploymentConfigs
osc get dc
osc create -f test/integration/fixtures/test-deployment-config.json
osc describe deploymentConfigs test-deployment-config
osc delete deploymentConfigs test-deployment-config
echo "deploymentConfigs: ok"

osc process -f test/templates/fixtures/guestbook.json --parameters --value="ADMIN_USERNAME=admin"
osc process -f test/templates/fixtures/guestbook.json | osc create -f -
echo "template+config: ok"

openshift kube resize --replicas=2 rc guestbook
osc get pods
echo "resize: ok"

osc process -f examples/sample-app/application-template-dockerbuild.json | osc create -f -
osc get buildConfigs
osc get bc
osc get builds
echo "buildConfig: ok"

started=$(osc start-build ruby-sample-build)
echo "start-build: ok"

osc cancel-build "${started}" --dump-logs --restart
echo "cancel-build: ok"

osc get minions,pods

openshift ex policy add-group cluster-admin system:unauthenticated
openshift ex policy remove-group cluster-admin system:unauthenticated
openshift ex policy remove-group-from-project system:unauthenticated
openshift ex policy add-user cluster-admin system:no-user
openshift ex policy remove-user cluster-admin system:no-user
openshift ex policy remove-user-from-project system:no-user
echo "ex policy: ok"
