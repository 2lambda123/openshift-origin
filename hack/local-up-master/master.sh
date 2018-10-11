#!/usr/bin/env bash

set -e

# preserve etcd data. you also need to set ETCD_DIR.
PRESERVE_ETCD="${PRESERVE_ETCD:-false}"
API_PORT=${API_PORT:-8080}
API_SECURE_PORT=${API_SECURE_PORT:-8443}

# WARNING: For DNS to work on most setups you should export API_HOST as the docker0 ip address,
API_HOST=${API_HOST:-localhost}
API_HOST_IP=${API_HOST_IP:-"127.0.0.1"}
ADVERTISE_ADDRESS=${ADVERTISE_ADDRESS:-""}
FIRST_SERVICE_CLUSTER_IP=${FIRST_SERVICE_CLUSTER_IP:-10.0.0.1}
HOSTNAME_OVERRIDE=${HOSTNAME_OVERRIDE:-"127.0.0.1"}
CONTROLPLANE_SUDO=
LOG_LEVEL=${LOG_LEVEL:-3}
# Use to increase verbosity on particular files, e.g. LOG_SPEC=token_controller*=5,other_controller*=4
LOG_SPEC=${LOG_SPEC:-""}
WAIT_FOR_URL_API_SERVER=${WAIT_FOR_URL_API_SERVER:-60}
MAX_TIME_FOR_URL_API_SERVER=${MAX_TIME_FOR_URL_API_SERVER:-1}


source "$(dirname "${BASH_SOURCE}")/../lib/init.sh"
KUBE_ROOT=OS_ROOT
source "${OS_ROOT}/hack/local-up-master/logging.sh"
source "${OS_ROOT}/hack/local-up-master/util.sh"
source "${OS_ROOT}/hack/local-up-master/etcd.sh"


CONFIG=$(pwd)/openshift.local.masterup
mkdir -p ${CONFIG}/logs
ETCD_DIR=$(pwd)/openshift.local.masterup/etcd
CERT_DIR=${CONFIG}/kube-apiserver
LOG_DIR=${CONFIG}/logs
ROOT_CA_FILE=${CERT_DIR}/server-ca.crt



cleanup()
{
  echo "Cleaning up..."
  set +e
  # Check if the API server is still running
  [[ -n "${KUBE_APISERVER_PID-}" ]] && KUBE_APISERVER_PIDS=$(pgrep -P ${KUBE_APISERVER_PID} ; ps -o pid= -p ${KUBE_APISERVER_PID})
  [[ -n "${KUBE_APISERVER_PIDS-}" ]] && sudo kill ${KUBE_APISERVER_PIDS} 2>/dev/null

  # Check if the controller-manager is still running
  [[ -n "${KUBE_CONTROLLER_MANAGER_PID-}" ]] && KUBE_CONTROLLER_MANAGER_PIDS=$(pgrep -P ${KUBE_CONTROLLER_MANAGER_PID} ; ps -o pid= -p ${KUBE_CONTROLLER_MANAGER_PID})
  [[ -n "${KUBE_CONTROLLER_MANAGER_PIDS-}" ]] && sudo kill ${KUBE_CONTROLLER_MANAGER_PIDS} 2>/dev/null

  [[ -n "${OPENSHIFT_APISERVER_PID-}" ]] && OPENSHIFT_APISERVER_PIDS=$(pgrep -P ${OPENSHIFT_APISERVER_PID} ; ps -o pid= -p ${OPENSHIFT_APISERVER_PID})
  [[ -n "${OPENSHIFT_APISERVER_PIDS-}" ]] && sudo kill ${OPENSHIFT_APISERVER_PIDS} 2>/dev/null

  [[ -n "${OPENSHIFT_CONTROLLER_MANAGER_PID-}" ]] && OPENSHIFT_CONTROLLER_MANAGER_PIDS=$(pgrep -P ${OPENSHIFT_CONTROLLER_MANAGER_PID} ; ps -o pid= -p ${OPENSHIFT_CONTROLLER_MANAGER_PID})
  [[ -n "${OPENSHIFT_CONTROLLER_MANAGER_PIDS-}" ]] && sudo kill ${OPENSHIFT_CONTROLLER_MANAGER_PIDS} 2>/dev/null


  # Check if the etcd is still running
  [[ -n "${ETCD_PID-}" ]] && kube::etcd::stop
  if [[ "${PRESERVE_ETCD}" == "false" ]]; then
    [[ -n "${ETCD_DIR-}" ]] && kube::etcd::clean_etcd_dir
  fi
  exit 0
}

trap cleanup EXIT

# Check if all processes are still running. Prints a warning once each time
# a process dies unexpectedly.
function healthcheck {
  if [[ -n "${KUBE_APISERVER_PID-}" ]] && ! sudo kill -0 ${KUBE_APISERVER_PID} 2>/dev/null; then
    warning_log "API server terminated unexpectedly, see ${KUBE_APISERVER_LOG}"
    KUBE_APISERVER_PID=
  fi

  if [[ -n "${KUBE_CONTROLLER_MANAGER_PID-}" ]] && ! sudo kill -0 ${KUBE_CONTROLLER_MANAGER_PID} 2>/dev/null; then
    warning_log "kube-controller-manager terminated unexpectedly, see ${KUBE_CONTROLLER_MANAGER_LOG}"
    KUBE_CONTROLLER_MANAGER_PID=
  fi

  if [[ -n "${OPENSHIFT_APISERVER_PID-}" ]] && ! sudo kill -0 ${OPENSHIFT_APISERVER_PID} 2>/dev/null; then
    warning_log "API server terminated unexpectedly, see ${OPENSHIFT_APISERVER_LOG}"
    OPENSHIFT_APISERVER_PID=
  fi

  if [[ -n "${OPENSHIFT_CONTROLLER_MANAGER_PID-}" ]] && ! sudo kill -0 ${OPENSHIFT_CONTROLLER_MANAGER_PID} 2>/dev/null; then
    warning_log "kube-controller-manager terminated unexpectedly, see ${OPENSHIFT_CONTROLLER_MANAGER_LOG}"
    OPENSHIFT_CONTROLLER_MANAGER_PID=
  fi


  if [[ -n "${ETCD_PID-}" ]] && ! sudo kill -0 ${ETCD_PID} 2>/dev/null; then
    warning_log "etcd terminated unexpectedly"
    ETCD_PID=
  fi
}

function print_color {
  message=$1
  prefix=${2:+$2: } # add colon only if defined
  color=${3:-1}     # default is red
  echo -n $(tput bold)$(tput setaf ${color})
  echo "${prefix}${message}"
  echo -n $(tput sgr0)
}

function warning_log {
  print_color "$1" "W$(date "+%m%d %H:%M:%S")]" 1
}


function generate_etcd_certs {
    # Create CA signers
    kube::util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${ETCD_DIR}" server '"client auth","server auth"'
    cp "${ETCD_DIR}/server-ca.key" "${ETCD_DIR}/client-ca.key"
    cp "${ETCD_DIR}/server-ca.crt" "${ETCD_DIR}/client-ca.crt"
    cp "${ETCD_DIR}/server-ca-config.json" "${ETCD_DIR}/client-ca-config.json"

    # Create client certs signed with client-ca, given id, given CN and a number of groups
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${ETCD_DIR}" 'client-ca' etcd-client etcd-clients

    # Create matching certificates for kube-aggregator
    kube::util::create_serving_certkey "${CONTROLPLANE_SUDO}" "${ETCD_DIR}" "server-ca" etcd-server "localhost" "127.0.0.1" ${API_HOST_IP}
}

function generate_kubeapiserver_certs {
    openssl genrsa -out "${CERT_DIR}/service-account" 2048 2>/dev/null

    # Create CA signers
    kube::util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" server '"client auth","server auth"'
    cp "${CERT_DIR}/server-ca.key" "${CERT_DIR}/client-ca.key"
    cp "${CERT_DIR}/server-ca.crt" "${CERT_DIR}/client-ca.crt"
    cp "${CERT_DIR}/server-ca-config.json" "${CERT_DIR}/client-ca-config.json"

    # Create auth proxy client ca
    kube::util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" request-header '"client auth"'

    # serving cert for kube-apiserver
    kube::util::create_serving_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "server-ca" kube-apiserver kubernetes.default kubernetes.default.svc "localhost" ${API_HOST_IP} ${API_HOST} ${FIRST_SERVICE_CLUSTER_IP}

    # Create client certs signed with client-ca, given id, given CN and a number of groups
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' kubelet system:node:${HOSTNAME_OVERRIDE} system:nodes
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' controller system:kube-controller-manager
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' admin system:admin system:masters
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' openshift-apiserver openshift-apiserver system:masters
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' openshift-controller-manager openshift-controller-manager system:masters

    # Create matching certificates for kube-aggregator
    kube::util::create_serving_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "server-ca" kube-aggregator api.kube-public.svc "localhost" ${API_HOST_IP}
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" request-header-ca auth-proxy system:auth-proxy
    # TODO remove masters and add rolebinding
    kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' kube-aggregator system:kube-aggregator system:masters
    kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${ROOT_CA_FILE}" "${API_HOST}" "${API_SECURE_PORT}" kube-aggregator

    cp ${ETCD_DIR}/server-ca.crt ${CERT_DIR}/etcd-serving-ca.crt
    cp ${ETCD_DIR}/client-etcd-client.crt ${CERT_DIR}/client-etcd-client.crt
    cp ${ETCD_DIR}/client-etcd-client.key ${CERT_DIR}/client-etcd-client.key
}

function generate_kubecontrollermanager_certs {
    cp ${CONFIG}/kube-apiserver/service-account ${CONFIG}/kube-controller-manager/etcd-serving-ca.crt
    cp ${CONFIG}/kube-apiserver/client-controller.crt ${CONFIG}/kube-controller-manager/client-controller.crt
    cp ${CONFIG}/kube-apiserver/client-controller.key ${CONFIG}/kube-controller-manager/client-controller.key
    kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CONFIG}/kube-controller-manager" "${ROOT_CA_FILE}" "${API_HOST}" "${API_SECURE_PORT}" controller
}


function generate_openshiftapiserver_certs {
    # Create CA signers
    kube::util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${CONFIG}/openshift-apiserver" server '"client auth","server auth"'

    # serving cert for kube-apiserver
    kube::util::create_serving_certkey "${CONTROLPLANE_SUDO}" "${CONFIG}/openshift-apiserver" "server-ca" openshift-apiserver openshift.default openshift.default.svc "localhost" ${API_HOST_IP} ${API_HOST} ${FIRST_SERVICE_CLUSTER_IP}

    cp ${CONFIG}/kube-apiserver/client-openshift-apiserver.crt ${CONFIG}/openshift-apiserver/client-openshift-apiserver.crt
    cp ${CONFIG}/kube-apiserver/client-openshift-apiserver.key ${CONFIG}/openshift-apiserver/client-openshift-apiserver.key
    kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CONFIG}/openshift-apiserver" "${ROOT_CA_FILE}" "${API_HOST}" "${API_SECURE_PORT}" openshift-apiserver

    cp ${ETCD_DIR}/server-ca.crt ${CONFIG}/openshift-apiserver/etcd-serving-ca.crt
    cp ${ETCD_DIR}/client-etcd-client.crt ${CONFIG}/openshift-apiserver/client-etcd-client.crt
    cp ${ETCD_DIR}/client-etcd-client.key ${CONFIG}/openshift-apiserver/client-etcd-client.key
}

function generate_openshiftcontrollermanager_certs {
    # Create CA signers
    kube::util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${CONFIG}/openshift-controller-manager" server '"client auth","server auth"'

    # serving cert for kube-apiserver
    kube::util::create_serving_certkey "${CONTROLPLANE_SUDO}" "${CONFIG}/openshift-controller-manager" "server-ca" openshift-controller-manager openshift.default openshift.default.svc "localhost" ${API_HOST_IP} ${API_HOST} ${FIRST_SERVICE_CLUSTER_IP}

    cp ${CONFIG}/kube-apiserver/client-ca.crt ${CONFIG}/openshift-controller-manager/client-ca.crt
    cp ${CONFIG}/kube-apiserver/client-openshift-controller-manager.crt ${CONFIG}/openshift-controller-manager/client-openshift-controller-manager.crt
    cp ${CONFIG}/kube-apiserver/client-openshift-controller-manager.key ${CONFIG}/openshift-controller-manager/client-openshift-controller-manager.key
    kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CONFIG}/openshift-controller-manager" "${ROOT_CA_FILE}" "${API_HOST}" "${API_SECURE_PORT}" openshift-controller-manager
}

function start_etcd {
    if [ ! -d "${CONFIG}/etcd" ]; then
        mkdir -p ${CONFIG}/etcd
        generate_etcd_certs
    fi
    echo "Starting etcd"
    ETCD_LOGFILE=${LOG_DIR}/etcd.log
    kube::etcd::start
}

function start_kubeapiserver {
    if [ ! -d "${CONFIG}/kube-apiserver" ]; then
        mkdir -p ${CONFIG}/kube-apiserver
        cp ${OS_ROOT}/hack/local-up-master/kube-apiserver.yaml ${CONFIG}/kube-apiserver
        generate_kubeapiserver_certs
    fi

    KUBE_APISERVER_LOG=${LOG_DIR}/kube-apiserver.log
    hypershift openshift-kube-apiserver \
      --v=${LOG_LEVEL} \
      --vmodule="${LOG_SPEC}" \
      --config=${CONFIG}/kube-apiserver/kube-apiserver.yaml >"${KUBE_APISERVER_LOG}" 2>&1 &
    KUBE_APISERVER_PID=$!

    # Wait for kube-apiserver to come up before launching the rest of the components.
    echo "Waiting for kube-apiserver to come up"
    kube::util::wait_for_url "https://${API_HOST_IP}:${API_SECURE_PORT}/healthz" "kube-apiserver: " 1 ${WAIT_FOR_URL_API_SERVER} ${MAX_TIME_FOR_URL_API_SERVER} \
        || { echo "check kube-apiserver logs: ${KUBE_APISERVER_LOG}" ; exit 1 ; }

    # Create kubeconfigs for all components, using client certs
    kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${ROOT_CA_FILE}" "${API_HOST}" "${API_SECURE_PORT}" admin
    chown "${USER}" "${CERT_DIR}/client-admin.key" # make readable for kubectl
}

function start_kubecontrollermanager {
    if [ ! -d "${CONFIG}/kube-controller-manager" ]; then
        mkdir -p ${CONFIG}/kube-controller-manager
        generate_kubecontrollermanager_certs
    fi

    KUBE_CONTROLLER_MANAGER_LOG=${LOG_DIR}/kube-controller-manager.log
    hyperkube controller-manager \
      --v=${LOG_LEVEL} \
      --vmodule="${LOG_SPEC}" \
      --service-account-private-key-file="${CONFIG}/kube-controller-manager/etcd-serving-ca.crt" \
      --root-ca-file="${ROOT_CA_FILE}" \
      --kubeconfig  ${CONFIG}/kube-controller-manager/controller.kubeconfig \
      --use-service-account-credentials \
      --leader-elect=false >"${KUBE_CONTROLLER_MANAGER_LOG}" 2>&1 &
    KUBE_CONTROLLER_MANAGER_PID=$!

    echo "Waiting for kube-controller-manager to come up"
    kube::util::wait_for_url "http://localhost:10252/healthz" "kube-controller-manager: " 1 ${WAIT_FOR_URL_API_SERVER} ${MAX_TIME_FOR_URL_API_SERVER} \
        || { echo "check kube-controller-manager logs: ${KUBE_CONTROLLER_MANAGER_LOG}" ; exit 1 ; }
}

function start_openshiftapiserver {
    if [ ! -d "${CONFIG}/openshift-apiserver" ]; then
        mkdir -p ${CONFIG}/openshift-apiserver
        cp ${OS_ROOT}/hack/local-up-master/openshift-apiserver.yaml ${CONFIG}/openshift-apiserver
        generate_openshiftapiserver_certs
    fi

    OPENSHIFT_APISERVER_LOG=${LOG_DIR}/openshift-apiserver.log
    hypershift openshift-apiserver \
      --v=${LOG_LEVEL} \
      --vmodule="${LOG_SPEC}" \
      --config=${CONFIG}/openshift-apiserver/openshift-apiserver.yaml >"${OPENSHIFT_APISERVER_LOG}" 2>&1 &
    OPENSHIFT_APISERVER_PID=$!

    # Wait for openshift-apiserver to come up before launching the rest of the components.
    echo "Waiting for openshift-apiserver to come up"
    kube::util::wait_for_url "https://${API_HOST_IP}:8444/healthz" "openshift-apiserver: " 1 ${WAIT_FOR_URL_API_SERVER} ${MAX_TIME_FOR_URL_API_SERVER} \
        || { echo "check kube-apiserver logs: ${OPENSHIFT_APISERVER_LOG}" ; exit 1 ; }

    NON_LOOPBACK_IPV4=$(ifconfig | grep -A1 -e "wlp\|enp" | grep -Eo 'inet (addr:)?([0-9]*\.){3}[0-9]*' | grep -Eo '([0-9]*\.){3}[0-9]*' | grep -v '127.0.0.1' | head -n 1)
    for filename in ${OS_ROOT}/hack/local-up-master/openshift-apiserver-manifests/*.yaml; do
        sed "s/NON_LOOPBACK_HOST/${NON_LOOPBACK_IPV4}/g" ${filename} | oc --config=${CONFIG}/openshift-apiserver/openshift-apiserver.kubeconfig apply -f -
    done
}

function start_openshiftcontrollermanager {
#    if [ ! -d "${CONFIG}/openshift-controller-manager" ]; then
        mkdir -p ${CONFIG}/openshift-controller-manager
        cp ${OS_ROOT}/hack/local-up-master/openshift-controller-manager.yaml ${CONFIG}/openshift-controller-manager
        generate_openshiftcontrollermanager_certs
#    fi

    OPENSHIFT_CONTROLLER_MANAGER_LOG=${LOG_DIR}/openshift-controller-manager.log
    hypershift openshift-controller-manager \
      --v=${LOG_LEVEL} \
      --vmodule="${LOG_SPEC}" \
      --config=${CONFIG}/openshift-controller-manager/openshift-controller-manager.yaml >"${OPENSHIFT_CONTROLLER_MANAGER_LOG}" 2>&1 &
    OPENSHIFT_CONTROLLER_MANAGER_PID=$!

    echo "Waiting for openshift-controller-manager to come up"
    kube::util::wait_for_url "https://localhost:8445/healthz" "openshift-controller-manager: " 1 ${WAIT_FOR_URL_API_SERVER} ${MAX_TIME_FOR_URL_API_SERVER} \
        || { echo "check openshift-controller-manager logs: ${OPENSHIFT_CONTROLLER_MANAGER_LOG}" ; exit 1 ; }
}



kube::util::test_openssl_installed
kube::util::ensure-cfssl

start_etcd
start_kubeapiserver
start_kubecontrollermanager
start_openshiftapiserver
start_openshiftcontrollermanager

cp ${CONFIG}/kube-apiserver/admin.kubeconfig ${CONFIG}/admin.kubeconfig

echo
echo "Cluster is available, the following kubeconfig to interact with it"
echo "export KUBECONFIG=${CONFIG}/admin.kubeconfig"
echo "Press ctrl+C to finish"

while true; do sleep 1; healthcheck; done
