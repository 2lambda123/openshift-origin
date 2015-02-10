#!/bin/bash
set -e

# ID to be used as the k8s id and also appended to the container name. Defaults to router1
ROUTER_ID="${1}"
# Full address to connect to the master.
MASTER_URL="${2}"
# openshift executable - optional, will try to find it on the path if not specified
OPENSHIFT="${3}"

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
if [[ -f /usr/share/openshift/examples/router.json ]]; then
  ROUTER_JSON="/usr/share/openshift/examples/router.json"
else
  ROUTER_JSON="${OS_ROOT}/images/router/haproxy/pod.json"
fi

if [[ "${ROUTER_ID}" == "" ]]; then
	echo "No router id provided, cannot create router..."
	exit
fi

if [[ "${MASTER_URL}" == "" ]]; then
	echo "No master url provided, cannot create router..."
	exit
fi
if [[ "${MASTER_URL}" != "http"* ]]; then
	echo "Master url must include protocol, e.g. https://localhost:8443"
	exit
fi

if [[ "${OPENSHIFT}" == "" ]]; then
    if [[ "$(which osc)" != "" ]]; then
        OPENSHIFT=$(which osc)
    fi
fi

OPENSHIFT_INSECURE="${OPENSHIFT_INSECURE:-false}"
OPENSHIFT_CA_DATA="${OPENSHIFT_CA_DATA:-}"
if [[ "${OPENSHIFT_CA_DATA}" == "" ]] && \
   [ -f /var/lib/openshift/openshift.local.certificates/master/root.crt ]; then
  OPENSHIFT_CA_DATA=$(</var/lib/openshift/openshift.local.certificates/master/root.crt)
fi

if [[ "${MASTER_URL}" == "https"* ]]; then
	if [[ "$OPENSHIFT_CA_DATA" == "" ]]; then
		echo "Running against an HTTPS master (${MASTER_URL}) without a trusted certificate bundle."
		echo "Set \$OPENSHIFT_CA_DATA to the contents of the root certificate bundle to start securely next time."
		echo "Starting insecurely..."
		OPENSHIFT_INSECURE=true
	fi

	# TODO: remove this once services can provide root cert data to pods
	# Escape cert data for json
	OPENSHIFT_CA_DATA="${OPENSHIFT_CA_DATA//$'\n'/\\\\n}"
else
	OPENSHIFT_INSECURE=""
	OPENSHIFT_CA_DATA=""
fi

# update the template file
echo "Creating router file and starting pod..."
cp ${ROUTER_JSON} /tmp/router.json
sed -i "s|ROUTER_ID|${ROUTER_ID}|g" /tmp/router.json
sed -i "s|\${OPENSHIFT_MASTER}|${MASTER_URL}|"       /tmp/router.json
sed -i "s|\${OPENSHIFT_INSECURE}|${OPENSHIFT_INSECURE}|"   /tmp/router.json
sed -i "s|\${OPENSHIFT_CA_DATA}|${OPENSHIFT_CA_DATA}|"     /tmp/router.json
# TODO: provide security context to client inside router pod

# create the pod if we can find openshift
if [ "${OPENSHIFT}" == "" ]; then
    echo "Unable to find openshift binary"
    echo "/tmp/router.json has been created.  In order to start the router please run:"
    echo "osc create -f /tmp/router.json"
else
    "${OPENSHIFT}" --server="${MASTER_URL}" create -f /tmp/router.json
fi
