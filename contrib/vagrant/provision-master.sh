#!/bin/bash

source $(dirname $0)/provision-config.sh

os::provision::base-provision

os::provision::build-origin "${ORIGIN_ROOT}" "${SKIP_BUILD}"
os::provision::build-etcd "${ORIGIN_ROOT}" "${SKIP_BUILD}"

echo "Installing openshift"
os::provision::install-cmds "${ORIGIN_ROOT}"
os::provision::install-sdn "${ORIGIN_ROOT}"

if [ "${SDN_NODE}" = "true" ]; then
  # Running an sdn node on the master when using an openshift sdn
  # plugin ensures connectivity between the openshift service and
  # pods.  This enables kube API calls that query a service and
  # require that the endpoints of the service be reachable from the
  # master.  This capability is used extensively in the kube e2e
  # tests.
  NODE_NAMES+=(${SDN_NODE_NAME})
  NODE_IPS+=(127.0.0.1)
  # Force the addition of a hosts entry for the sdn node.
  os::provision::add-to-hosts-file "${MASTER_IP}" "${SDN_NODE_NAME}" 1
fi

os::provision::init-certs "${CONFIG_ROOT}" "${NETWORK_PLUGIN}" \
  "${MASTER_NAME}" "${MASTER_IP}" NODE_NAMES NODE_IPS

echo "Launching openshift daemons"
NODE_LIST=$(os::provision::join , ${NODE_NAMES[@]})
cmd="/usr/bin/openshift start master --loglevel=${LOG_LEVEL} \
 --master=https://${MASTER_IP}:8443 \
 --network-plugin=${NETWORK_PLUGIN}"
os::provision::start-os-service "openshift-master" "OpenShift Master" "${cmd}"

if [ "${SDN_NODE}" = "true" ]; then
  os::provision::start-node-service "${CONFIG_ROOT}" "${SDN_NODE_NAME}"

  # Disable scheduling for the sdn node - it's purpose is only to ensure
  # pod network connectivity on the master.
  os::provision::disable-sdn-node "${CONFIG_ROOT}" "${SDN_NODE_NAME}"
fi

os::provision::set-os-env "${ORIGIN_ROOT}" "${CONFIG_ROOT}"
