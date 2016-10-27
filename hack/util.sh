#!/bin/bash

# Provides simple utility functions

# ensure_iptables_or_die tests if the testing machine has iptables available
# and in PATH. Also test whether current user has sudo privileges.
function ensure_iptables_or_die() {
	if [[ -z "$(which iptables)" ]]; then
		echo "IPTables not found - the end-to-end test requires a system with iptables for Kubernetes services."
		exit 1
	fi

	set +e

	iptables --list > /dev/null 2>&1
	if [ $? -ne 0 ]; then
		sudo iptables --list > /dev/null 2>&1
		if [ $? -ne 0 ]; then
			echo "You do not have iptables or sudo privileges. Kubernetes services will not work without iptables access.	See https://github.com/kubernetes/kubernetes/issues/1859.	Try 'sudo hack/test-end-to-end.sh'."
			exit 1
		fi
	fi

	set -e
}
readonly -f ensure_iptables_or_die

# wait_for_command executes a command and waits for it to
# complete or times out after max_wait.
#
# $1 - The command to execute (e.g. curl -fs http://redhat.com)
# $2 - Optional maximum time to wait in ms before giving up (Default: 10000ms)
# $3 - Optional alternate command to determine if the wait should
#		exit before the max_wait
function wait_for_command() {
	STARTTIME=$(date +%s)
	cmd=$1
	msg="Waiting for command to finish: '${cmd}'..."
	max_wait=${2:-10*TIME_SEC}
	fail=${3:-""}
	wait=0.2

	echo "[INFO] $msg"
	expire=$(($(time_now) + $max_wait))
	set +e
	while [[ $(time_now) -lt $expire ]]; do
		eval $cmd
		if [ $? -eq 0 ]; then
			set -e
			ENDTIME=$(date +%s)
			echo "[INFO] Success running command: '$cmd' after $(($ENDTIME - $STARTTIME)) seconds"
			return 0
		fi
		#check a failure condition where the success
		#command may never be evaluated before timing
		#out
		if [[ ! -z $fail ]]; then
			eval $fail
			if [ $? -eq 0 ]; then
				set -e
				echo "[FAIL] Returning early. Command Failed '$cmd'"
				return 1
			fi
		fi
		sleep $wait
	done
	echo "[ ERR] Gave up waiting for: '$cmd'"
	set -e
	return 1
}
readonly -f wait_for_command

# wait_for_url_timed attempts to access a url in order to
# determine if it is available to service requests.
#
# $1 - The URL to check
# $2 - Optional prefix to use when echoing a successful result
# $3 - Optional maximum time to wait before giving up (Default: 10s)
function wait_for_url_timed() {
	STARTTIME=$(date +%s)
	url=$1
	prefix=${2:-}
	max_wait=${3:-10*TIME_SEC}
	wait=0.2
	expire=$(($(time_now) + $max_wait))
	set +e
	while [[ $(time_now) -lt $expire ]]; do
		out=$(curl --max-time 2 -fs $url 2>/dev/null)
		if [ $? -eq 0 ]; then
			set -e
			echo ${prefix}${out}
			ENDTIME=$(date +%s)
			echo "[INFO] Success accessing '$url' after $(($ENDTIME - $STARTTIME)) seconds"
			return 0
		fi
		sleep $wait
	done
	echo "ERROR: gave up waiting for $url"
	set -e
	return 1
}
readonly -f wait_for_url_timed

# wait_for_file returns 0 if a file exists, 1 if it does not exist
#
# $1 - The file to check for existence
# $2 - Optional time to sleep between attempts (Default: 0.2s)
# $3 - Optional number of attemps to make (Default: 10)
function wait_for_file() {
	file=$1
	wait=${2:-0.2}
	times=${3:-10}
	for i in $(seq 1 $times); do
		if [ -f "${file}" ]; then
			return 0
		fi
		sleep $wait
	done
	echo "ERROR: gave up waiting for file ${file}"
	return 1
}
readonly -f wait_for_file

# wait_for_url attempts to access a url in order to
# determine if it is available to service requests.
#
# $1 - The URL to check
# $2 - Optional prefix to use when echoing a successful result
# $3 - Optional time to sleep between attempts (Default: 0.2s)
# $4 - Optional number of attemps to make (Default: 10)
function wait_for_url() {
	url=$1
	prefix=${2:-}
	wait=${3:-0.2}
	times=${4:-10}

	set_curl_args $wait $times

	set +e
	cmd="env -i CURL_CA_BUNDLE=${CURL_CA_BUNDLE:-} $(which curl) ${clientcert_args} -fs ${url}"
	for i in $(seq 1 $times); do
		out=$(${cmd})
		if [ $? -eq 0 ]; then
			set -e
			echo "${prefix}${out}"
			return 0
		fi
		sleep $wait
	done
	echo "ERROR: gave up waiting for ${url}" 1>&2
	${cmd} 1>&2
	set -e
	return 1
}
readonly -f wait_for_url

# set_curl_args tries to export CURL_ARGS for a program to use.
# will do a wait for the files to exist when using curl with
# SecureTransport (because we must convert the keys to a different
# form).
#
# $1 - Optional time to sleep between attempts (Default: 0.2s)
# $2 - Optional number of attemps to make (Default: 10)
function set_curl_args() {
	wait=${1:-0.2}
	times=${2:-10}

	CURL_CERT=${CURL_CERT:-}
	CURL_KEY=${CURL_KEY:-}
	clientcert_args="${CURL_EXTRA:-} "

	if [ -n "${CURL_CERT}" ]; then
		if [ -n "${CURL_KEY}" ]; then
			if [[ `curl -V` == *"SecureTransport"* ]]; then
					# Convert to a p12 cert for SecureTransport
					export CURL_CERT_DIR=$(dirname "${CURL_CERT}")
					export CURL_CERT_P12=${CURL_CERT_P12:-${CURL_CERT_DIR}/cert.p12}
					export CURL_CERT_P12_PASSWORD=${CURL_CERT_P12_PASSWORD:-password}
					if [ ! -f "${CURL_CERT_P12}" ]; then
					wait_for_file "${CURL_CERT}" $wait $times
					wait_for_file "${CURL_KEY}" $wait $times
					openssl pkcs12 -export -inkey "${CURL_KEY}" -in "${CURL_CERT}" -out "${CURL_CERT_P12}" -password "pass:${CURL_CERT_P12_PASSWORD}"
					fi
					clientcert_args="--cert ${CURL_CERT_P12}:${CURL_CERT_P12_PASSWORD} ${CURL_EXTRA:-}"
			else
				clientcert_args="--cert ${CURL_CERT} --key ${CURL_KEY} ${CURL_EXTRA:-}"
			fi
		fi
	fi
	export CURL_ARGS="${clientcert_args}"
}
readonly -f set_curl_args

# Search for a regular expression in a HTTP response.
#
# $1 - a valid URL (e.g.: http://127.0.0.1:8080)
# $2 - a regular expression or text
function validate_response() {
	url=$1
	expected_response=$2
	wait=${3:-0.2}
	times=${4:-10}

	set +e
	for i in $(seq 1 $times); do
		response=`curl $url`
		echo $response | grep -q "$expected_response"
		if [ $? -eq 0 ]; then
			echo "[INFO] Response is valid."
			set -e
			return 0
		fi
		sleep $wait
	done

	echo "[INFO] Response is invalid: $response"
	set -e
	return 1
}
readonly -f validate_response

# reset_tmp_dir will try to delete the testing directory.
# If it fails will unmount all the mounts associated with
# the test.
#
# $1 expression for which the mounts should be checked
function reset_tmp_dir() {
	local sudo="${USE_SUDO:+sudo}"

	set +e
	${sudo} rm -rf ${BASETMPDIR} &>/dev/null
	if [[ $? != 0 ]]; then
		echo "[INFO] Unmounting previously used volumes ..."
		findmnt -lo TARGET | grep ${BASETMPDIR} | xargs -r ${sudo} umount
		${sudo} rm -rf ${BASETMPDIR}
	fi

	mkdir -p ${BASETMPDIR} ${LOG_DIR} ${ARTIFACT_DIR} ${FAKE_HOME_DIR} ${VOLUME_DIR}
	set -e
}
readonly -f reset_tmp_dir

# kill_all_processes function will kill all
# all processes created by the test script.
function kill_all_processes() {
	local sudo="${USE_SUDO:+sudo}"

	pids=($(jobs -pr))
	for i in ${pids[@]-}; do
		pgrep -P "${i}" | xargs $sudo kill &> /dev/null
		$sudo kill ${i} &> /dev/null
	done
}
readonly -f kill_all_processes

# time_now return the time since the epoch in millis
function time_now() {
	echo $(date +%s000)
}
readonly -f time_now

# dump_container_logs writes container logs to $LOG_DIR
function dump_container_logs() {
	if ! docker version >/dev/null 2>&1; then
		return
	fi

	mkdir -p ${LOG_DIR}

	echo "[INFO] Dumping container logs to ${LOG_DIR}"
	for container in $(docker ps -aq); do
		container_name=$(docker inspect -f "{{.Name}}" $container)
		# strip off leading /
		container_name=${container_name:1}
		if [[ "$container_name" =~ ^k8s_ ]]; then
			pod_name=$(echo $container_name | awk 'BEGIN { FS="[_.]+" }; { print $4 }')
			container_name=${pod_name}-$(echo $container_name | awk 'BEGIN { FS="[_.]+" }; { print $2 }')
		fi
		docker logs "$container" >&"${LOG_DIR}/container-${container_name}.log"
	done
}
readonly -f dump_container_logs

# delete_empty_logs deletes empty logs
function delete_empty_logs() {
	# Clean up zero byte log files
	find "${ARTIFACT_DIR}" "${LOG_DIR}" -type f -name '*.log' \( -empty \) -delete
}
readonly -f delete_empty_logs

# truncate_large_logs truncates large logs so we only download the last 50MB
function truncate_large_logs() {
	# Clean up large log files so they don't end up on jenkins
	local large_files=$(find "${ARTIFACT_DIR}" "${LOG_DIR}" -type f -name '*.log' \( -size +50M \))
	for file in ${large_files}; do
		mv "${file}" "${file}.tmp"
		echo "LOGFILE TOO LONG ($(du -h "${file}.tmp")), PREVIOUS BYTES TRUNCATED. LAST 50M OF LOGFILE:" > "${file}"
		tail -c 50M "${file}.tmp" >> "${file}"
		rm "${file}.tmp"
	done
}
readonly -f truncate_large_logs

######
# start of common functions for extended test group's run.sh scripts
######

# exit run if ginkgo not installed
function ensure_ginkgo_or_die() {
	which ginkgo &>/dev/null || (echo 'Run: "go get github.com/onsi/ginkgo/ginkgo"' && exit 1)
}
readonly -f ensure_ginkgo_or_die

# cleanup_openshift saves container logs, saves resources, and kills all processes and containers
function cleanup_openshift() {
	LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
	ARTIFACT_DIR="${ARTIFACT_DIR:-${LOG_DIR}}"
	API_HOST="${API_HOST:-127.0.0.1}"
	API_SCHEME="${API_SCHEME:-https}"
	ETCD_PORT="${ETCD_PORT:-4001}"

	set +e
	dump_container_logs

	# pull information out of the server log so that we can get failure management in jenkins to highlight it and
	# really have it smack people in their logs.  This is a severe correctness problem
	grep -a5 "CACHE.*ALTERED" ${LOG_DIR}/openshift.log

	echo "[INFO] Dumping etcd contents to ${ARTIFACT_DIR}/etcd_dump.json"
	set_curl_args 0 1
	curl -s ${clientcert_args} -L "${API_SCHEME}://${API_HOST}:${ETCD_PORT}/v2/keys/?recursive=true" > "${ARTIFACT_DIR}/etcd_dump.json"
	echo

	if [[ -z "${SKIP_TEARDOWN-}" ]]; then
		echo "[INFO] Tearing down test"
		kill_all_processes

		if docker version >/dev/null 2>&1; then
			echo "[INFO] Stopping k8s docker containers"; docker ps | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker stop -t 1 >/dev/null
			if [[ -z "${SKIP_IMAGE_CLEANUP-}" ]]; then
				echo "[INFO] Removing k8s docker containers"; docker ps -a | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker rm -v >/dev/null
			fi
		fi

		echo "[INFO] Pruning etcd data directory..."
		local sudo="${USE_SUDO:+sudo}"
		${sudo} rm -rf "${ETCD_DATA_DIR}"

		set -u
	fi

	# TODO soltysh: restore the if back once #8399 is resolved
	# if grep -q 'no Docker socket found' "${LOG_DIR}/openshift.log"; then
		# the Docker daemon crashed, we need the logs
	# journalctl --unit docker.service --since -4hours > "${LOG_DIR}/docker.log"
	# fi
	journalctl --unit docker.service --since -15minutes > "${LOG_DIR}/docker.log"

	delete_empty_logs
	truncate_large_logs

	echo "[INFO] Cleanup complete"
	set -e
}
readonly -f cleanup_openshift

# create a .gitconfig for test-cmd secrets
function create_gitconfig() {
	USERNAME=sample-user
	PASSWORD=password
	BASETMPDIR="${BASETMPDIR:-"/tmp"}"
	GITCONFIG_DIR=$(mktemp -d ${BASETMPDIR}/test-gitconfig.XXXX)
	touch ${GITCONFIG_DIR}/.gitconfig
	git config --file ${GITCONFIG_DIR}/.gitconfig user.name ${USERNAME}
	git config --file ${GITCONFIG_DIR}/.gitconfig user.token ${PASSWORD}
	echo ${GITCONFIG_DIR}/.gitconfig
}

function create_valid_file() {
	BASETMPDIR="${BASETMPDIR:-"/tmp"}"
	FILE_DIR=$(mktemp -d ${BASETMPDIR}/test-file.XXXX)
	echo test_data >${FILE_DIR}/${1}
	echo ${FILE_DIR}/${1}
}

# install the router for the extended tests
function install_router() {
	echo "[INFO] Installing the router"
	oadm policy add-scc-to-user privileged -z router --config="${ADMIN_KUBECONFIG}"
	# Create a TLS certificate for the router
	if [[ -n "${CREATE_ROUTER_CERT:-}" ]]; then
		echo "[INFO] Generating router TLS certificate"
		oadm ca create-server-cert --signer-cert=${MASTER_CONFIG_DIR}/ca.crt \
			--signer-key=${MASTER_CONFIG_DIR}/ca.key \
			--signer-serial=${MASTER_CONFIG_DIR}/ca.serial.txt \
			--hostnames="*.${API_HOST}.xip.io" \
			--cert=${MASTER_CONFIG_DIR}/router.crt --key=${MASTER_CONFIG_DIR}/router.key
		cat ${MASTER_CONFIG_DIR}/router.crt ${MASTER_CONFIG_DIR}/router.key \
			${MASTER_CONFIG_DIR}/ca.crt > ${MASTER_CONFIG_DIR}/router.pem
		ROUTER_DEFAULT_CERT="--default-cert=${MASTER_CONFIG_DIR}/router.pem"
	fi
	openshift admin router --config="${ADMIN_KUBECONFIG}" --images="${USE_IMAGES}" --service-account=router ${ROUTER_DEFAULT_CERT-}

	# Set the SYN eater to make router reloads more robust
	if [[ -n "${DROP_SYN_DURING_RESTART:-}" ]]; then
		# Rewrite the DC for the router to add the environment variable into the pod definition
		echo "[INFO] Changing the router DC to drop SYN packets during a reload"
		oc set env dc/router -c router DROP_SYN_DURING_RESTART=true
	fi
}
readonly -f create_gitconfig

# install registry for the extended tests
function install_registry() {
	# The --mount-host option is provided to reuse local storage.
	echo "[INFO] Installing the registry"
	# For testing purposes, ensure the quota objects are always up to date in the registry by
	# disabling project cache.
	openshift admin registry --config="${ADMIN_KUBECONFIG}" --images="${USE_IMAGES}" --enforce-quota -o json | \
		oc env -f - --output json "REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_PROJECTCACHETTL=0" | \
		oc create -f -
}
readonly -f install_registry

function wait_for_registry() {
	local generation="$(oc get dc/docker-registry -o 'jsonpath={.metadata.generation}')"
	local onereplicajs='{.status.observedGeneration},{.status.replicas},{.status.updatedReplicas},{.status.availableReplicas}'
	wait_for_command "oc get dc/docker-registry -o 'jsonpath=${onereplicajs}' --config='${ADMIN_KUBECONFIG}' | grep '^${generation},1,1,1$'"  "$((5*TIME_MIN))"
	local readyjs='{.items[*].status.conditions[?(@.type=="Ready")].status}'
	wait_for_command "oc get pod -l deploymentconfig=docker-registry -o 'jsonpath=${readyjs}' --config='${ADMIN_KUBECONFIG}' | grep -qi true" "${TIME_MIN}"
}
readonly -f wait_for_registry

# Wait for builds to start
# $1 namespace
function os::build:wait_for_start() {
	echo "[INFO] Waiting for $1 namespace build to start"
	wait_for_command "oc get -n $1 builds | grep -i running" $((10*TIME_MIN)) "oc get -n $1 builds | grep -i -e failed -e error"
	BUILD_ID=`oc get -n $1 builds  --output-version=v1 --template="{{with index .items 0}}{{.metadata.name}}{{end}}"`
	echo "[INFO] Build ${BUILD_ID} started"
}
readonly -f os::build:wait_for_start

# Wait for builds to complete
# $1 namespace
function os::build:wait_for_end() {
	echo "[INFO] Waiting for $1 namespace build to complete"
	wait_for_command "oc get -n $1 builds | grep -i complete" $((10*TIME_MIN)) "oc get -n $1 builds | grep -i -e failed -e error"
	BUILD_ID=`oc get -n $1 builds --output-version=v1 --template="{{with index .items 0}}{{.metadata.name}}{{end}}"`
	echo "[INFO] Build ${BUILD_ID} finished"
	# TODO: fix
	set +e
	oc build-logs -n $1 $BUILD_ID > $LOG_DIR/$1build.log
	set -e
}
readonly -f os::build:wait_for_end

# enable-selinux/disable-selinux use the shared control variable
# SELINUX_DISABLED to determine whether to re-enable selinux after it
# has been disabled.  The goal is to allow temporary disablement of
# selinux enforcement while avoiding enabling enforcement in an
# environment where it is not already enabled.
SELINUX_DISABLED=0

function enable-selinux() {
	if [ "${SELINUX_DISABLED}" = "1" ]; then
		os::log::info "Re-enabling selinux enforcement"
		sudo setenforce 1
		SELINUX_DISABLED=0
	fi
}
readonly -f enable-selinux

function disable-selinux() {
	if selinuxenabled && [ "$(getenforce)" = "Enforcing" ]; then
		os::log::info "Temporarily disabling selinux enforcement"
		sudo setenforce 0
		SELINUX_DISABLED=1
	fi
}
readonly -f disable-selinux

######
# end of common functions for extended test group's run.sh scripts
######

function os::log::with-severity() {
	local msg=$1
	local severity=$2

	echo "[$2] ${1}"
}
readonly -f os::log::with-severity

function os::log::info() {
	os::log::with-severity "${1}" "INFO"
}
readonly -f os::log::info

function os::log::warn() {
	os::log::with-severity "${1}" "WARNING" 1>&2
}
readonly -f os::log::warn

function os::log::error() {
	os::log::with-severity "${1}" "ERROR" 1>&2
}
readonly -f os::log::error

function find_files() {
	find . -not \( \
		\( \
		-wholename './_output' \
		-o -wholename './.*' \
		-o -wholename './pkg/assets/bindata.go' \
		-o -wholename './pkg/assets/*/bindata.go' \
		-o -wholename './pkg/bootstrap/bindata.go' \
		-o -wholename './openshift.local.*' \
		-o -wholename '*/vendor/*' \
		-o -wholename './assets/bower_components/*' \
		\) -prune \
	\) -name '*.go' | sort -u
}
readonly -f find_files

# Asks golang what it thinks the host platform is.  The go tool chain does some
# slightly different things when the target platform matches the host platform.
function os::util::host_platform() {
	echo "$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
}
readonly -f os::util::host_platform