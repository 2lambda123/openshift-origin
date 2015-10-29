#!/bin/bash

source $(dirname $0)/provision-config.sh

os::util::base-provision

echo "Building and installing openshift"
${ORIGIN_ROOT}/hack/build-go.sh
os::util::install-cmds "${ORIGIN_ROOT}"
${ORIGIN_ROOT}/hack/install-etcd.sh
os::util::install-sdn "${ORIGIN_ROOT}"

# Running an openshift node on the master ensures connectivity between
# the openshift service and pods.  This supports kube API calls that
# query a service and require that the endpoints of the service be
# reachable from the master.
#
# TODO(marun) This is required for connectivity with openshift-sdn,
# but may not make sense for other plugins.
NODE_NAMES+=(${SDN_NODE_NAME})
NODE_IPS+=(127.0.0.1)
# Force the addition of a hosts entry for the sdn node.
os::util::add-to-hosts-file "${MASTER_IP}" "${SDN_NODE_NAME}" 1

os::util::init-certs "${CONFIG_ROOT}" "${NETWORK_PLUGIN}" "${MASTER_NAME}" \
  "${MASTER_IP}" NODE_NAMES NODE_IPS

echo "Launching openshift daemons"
NODE_LIST=$(os::util::join , ${NODE_NAMES[@]})
cmd="/usr/bin/openshift start master --loglevel=${LOG_LEVEL} \
 --master=https://${MASTER_IP}:8443 --nodes=${NODE_LIST} \
 --network-plugin=${NETWORK_PLUGIN}"
os::util::start-os-service "openshift-master" "OpenShift Master" "${cmd}"
os::util::start-node-service "${SDN_NODE_NAME}"

# TODO(marun) Need to disable scheduling on sdn daemon

os::util::set-os-env "${ORIGIN_ROOT}" "${CONFIG_ROOT}"
