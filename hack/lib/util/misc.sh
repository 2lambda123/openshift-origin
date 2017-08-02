#!/bin/bash
#
# This library holds miscellaneous utility functions. If there begin to be groups of functions in this
# file that share intent or are thematically similar, they should be split into their own files.

# os::util::describe_return_code describes an exit code
#
# Globals:
#  - OS_SCRIPT_START_TIME
# Arguments:
#  - 1: exit code to describe
# Returns:
#  None
function os::util::describe_return_code() {
	local return_code=$1
	local message="$( os::util::repository_relative_path $0 ) exited with code ${return_code} "

	if [[ -n "${OS_SCRIPT_START_TIME:-}" ]]; then
		local end_time
        end_time="$(date +%s)"
		local elapsed_time
        elapsed_time="$(( end_time - OS_SCRIPT_START_TIME ))"
		local formatted_time
        formatted_time="$( os::util::format_seconds "${elapsed_time}" )"
		message+="after ${formatted_time}"
	fi

	if [[ "${return_code}" = "0" ]]; then
		os::log::info "${message}"
	else
		os::log::error "${message}"
	fi
}
readonly -f os::util::describe_return_code

# os::util::install_describe_return_code installs the return code describer for the EXIT trap
# If the EXIT trap is not initialized, installing this plugin will initialize it.
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  - export OS_DESCRIBE_RETURN_CODE
#  - export OS_SCRIPT_START_TIME
function os::util::install_describe_return_code() {
	export OS_DESCRIBE_RETURN_CODE="true"
	OS_SCRIPT_START_TIME="$( date +%s )"; export OS_SCRIPT_START_TIME
	os::util::trap::init_exit
}
readonly -f os::util::install_describe_return_code

# OS_ORIGINAL_WD is the original working directory the script sourcing this utility file was called
# from. This is an important directory as if $0 is a relative path, we cannot use the following path
# utility without knowing from where $0 is relative.
if [[ -z "${OS_ORIGINAL_WD:-}" ]]; then
	# since this could be sourced in a context where the utilities are already loaded,
	# we want to ensure that this is re-entrant, so we only set $OS_ORIGINAL_WD if it
	# is not set already
	OS_ORIGINAL_WD="$( pwd )"
	readonly OS_ORIGINAL_WD
	export OS_ORIGINAL_WD
fi

# os::util::repository_relative_path returns the relative path from the $OS_ROOT directory to the
# given file, if the file is inside of the $OS_ROOT directory. If the file is outside of $OS_ROOT,
# this function will return the absolute path to the file
#
# Globals:
#  - OS_ROOT
# Arguments:
#  - 1: the path to relativize
# Returns:
#  None
function os::util::repository_relative_path() {
	local filename=$1
	local directory; directory="$( dirname "${filename}" )"
	filename="$( basename "${filename}" )"

	if [[ "${directory}" != "${OS_ROOT}"* ]]; then
		pushd "${OS_ORIGINAL_WD}" >/dev/null 2>&1
		directory="$( os::util::absolute_path "${directory}" )"
		popd >/dev/null 2>&1
	fi

	directory="${directory##*${OS_ROOT}/}"

	echo "${directory}/${filename}"
}
readonly -f os::util::repository_relative_path

# os::util::format_seconds formats a duration of time in seconds to print in HHh MMm SSs
#
# Globals:
#  None
# Arguments:
#  - 1: time in seconds to format
# Return:
#  None
function os::util::format_seconds() {
	local raw_seconds=$1

	local hours minutes seconds
	(( hours=raw_seconds/3600 ))
	(( minutes=(raw_seconds%3600)/60 ))
	(( seconds=raw_seconds%60 ))

	printf '%02dh %02dm %02ds' "${hours}" "${minutes}" "${seconds}"
}
readonly -f os::util::format_seconds

# os::util::sed attempts to make our Bash scripts agnostic to the platform
# on which they run `sed` by glossing over a discrepancy in flag use in GNU.
#
# Globals:
#  None
# Arguments:
#  - all: arguments to pass to `sed -i`
# Return:
#  None
function os::util::sed() {
	local sudo="${USE_SUDO:+sudo}"
	if LANG=C sed --help 2>&1 | grep -q "GNU sed"; then
		${sudo} sed -i'' "$@"
	else
		${sudo} sed -i '' "$@"
	fi
}
readonly -f os::util::sed

# os::util::base64decode attempts to make our Bash scripts agnostic to the platform
# on which they run `base64decode` by glossing over a discrepancy in flag use in GNU.
#
# Globals:
#  None
# Arguments:
#  - all: arguments to pass to `base64decode`
# Return:
#  None
function os::util::base64decode() {
	if [[ "$(go env GOHOSTOS)" == "darwin" ]]; then
		base64 -D "$@"
	else
		base64 -d "$@"
	fi
}
readonly -f os::util::base64decode

# os::util::curl_etcd sends a request to the backing etcd store for the master.
# We use the administrative client cert and key for access and re-encode them
# as necessary for OSX clients.
#
# Globals:
#  MASTER_CONFIG_DIR
#  API_SCHEME
#  API_HOST
#  ETCD_PORT
# Arguments:
#  - 1: etcd-relative URL to curl, with leading slash
# Returns:
#  None
function os::util::curl_etcd() {
	local url="$1"
	local full_url="${API_SCHEME}://${API_HOST}:${ETCD_PORT}${url}"

	local etcd_client_cert="${MASTER_CONFIG_DIR}/master.etcd-client.crt"
	local etcd_client_key="${MASTER_CONFIG_DIR}/master.etcd-client.key"
	local ca_bundle="${MASTER_CONFIG_DIR}/ca-bundle.crt"

	if curl -V | grep -q 'SecureTransport'; then
		# on newer OSX `curl` implementations, SSL is not used and client certs
		# and keys are expected to be encoded in P12 format instead of PEM format,
		# so we need to convert the secrets that the server wrote if we haven't
		# already done so
		local etcd_client_cert_p12="${MASTER_CONFIG_DIR}/master.etcd-client.crt.p12"
		local etcd_client_cert_p12_password="${CURL_CERT_P12_PASSWORD:-'password'}"
		if [[ ! -f "${etcd_client_cert_p12}" ]]; then
			openssl pkcs12 -export                        \
			               -in "${etcd_client_cert}"      \
			               -inkey "${etcd_client_key}"    \
			               -out "${etcd_client_cert_p12}" \
			               -password "pass:${etcd_client_cert_p12_password}"
		fi

		curl --fail --silent --cacert "${ca_bundle}" \
		     --cert "${etcd_client_cert_p12}:${etcd_client_cert_p12_password}" "${full_url}"
	else
		curl --fail --silent --cacert "${ca_bundle}" \
		     --cert "${etcd_client_cert}" --key "${etcd_client_key}" "${full_url}"
	fi
}

# os::util::host_platform determines what the host OS and architecture
# are, as Golang sees it. The go tool chain does some slightly different
# things when the target platform matches the host platform.
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::util::host_platform() {
	echo "$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
}
readonly -f os::util::host_platform

# os::util::go_arch determines what the host architecture is, as Golang
# sees it.
function os::util::go_arch() {
	arch=${1:-}

	if [[ -z "${arch}" ]]; then
		# No arch requested, query go
		echo "$(go env GOHOSTARCH)"
	elif [[ "${!OS_BUILD_GOARCH_TO_SYSARCH_MAP[@]}" =~ "${arch}" ]]; then
		# Requested arch is a known go arch
		echo "${arch}"
	elif [[ "${!OS_BUILD_SYSARCH_TO_GOARCH_MAP[@]}" =~ "${arch}" ]]; then
		# Requested arch is a known system arch
		echo "${OS_BUILD_SYSARCH_TO_GOARCH_MAP[${arch}]}"
	else
		os::log::error "Unkown arch: ${arch}"
		exit 1
	fi
}
readonly -f os::util::go_arch

function os::util::sys_arch() {
	arch=${1:-}

	if [[ -z "${arch}" ]]; then
		# No arch requested, query system
		echo "$(uname -m)"
	elif [[ "${!OS_BUILD_SYSARCH_TO_GOARCH_MAP[@]}" =~ "${arch}" ]]; then
		# Requested arch is a known system arch
		echo "${arch}"
	elif [[ "${!OS_BUILD_GOARCH_TO_SYSARCH_MAP[@]}" =~ "${arch}" ]]; then
		# Requested arch is a known go arch
		echo "${OS_BUILD_GOARCH_TO_SYSARCH_MAP[${arch}]}"
	else
		os::log::error "Unkown arch: ${arch}"
		exit 1
	fi
}
readonly -f os::util::sys_arch

# os::util::centos_image returns the image prefix for
# the centos image based on host architecture
function os::util::centos_image() {
  local host_arch=${1:-}
  if [[ -z "${host_arch}" ]]; then
    host_arch=$(os::util::go_arch)
  fi

  if [[ $host_arch == "arm64" ]]; then
    echo "centos/aarch64:latest"
  elif [[ $host_arch == "ppc64le" ]]; then
    echo "ppc64le/centos:7"
  #elif [[ $host_arch == "s390x" ]]; then
  #  echo "centos/s390x:latest"
  else
    echo centos:7
  fi
}
readonly -f os::util::centos_image


# os::util::official_docker_image_prefix returns the image prefix for
# the "official" docker hub images based on host architecture
function os::util::official_docker_image_prefix() {
  local host_arch=${1:-}
  if [[ -z "${host_arch}" ]]; then
    host_arch=$(os::util::go_arch)
  fi

  if [[ $host_arch == "arm64" ]]; then
    echo "arm64v8/"
  elif [[ $host_arch == "ppc64le" ]]; then
    echo "ppc64le/"
  elif [[ $host_arch == "s390x" ]]; then
    echo "s390x/"
  fi
}
readonly -f os::util::official_docker_image_prefix

# os::util::busybox_base_img determines which busybox base image to use
# based on host architecture
function os::util::busybox_base_img() {
  echo "$(os::util::official_docker_image_prefix)busybox"
}
readonly -f os::util::busybox_base_img

# Create a user friendly version of host_platform for end users
function os::util::host_platform_friendly() {
  local platform=${1:-}
  if [[ -z "${platform}" ]]; then
    platform=$(os::util::host_platform)
  fi
  if [[ $platform == "windows/amd64" ]]; then
    echo "windows"
  elif [[ $platform == "darwin/amd64" ]]; then
    echo "mac"
  elif [[ $platform == "linux/386" ]]; then
    echo "linux-32bit"
  elif [[ $platform == "linux/amd64" ]]; then
    echo "linux-64bit"
  elif [[ $platform == "linux/ppc64le" ]]; then
    echo "linux-powerpc64"
  elif [[ $platform == "linux/arm64" ]]; then
    echo "linux-arm64"
  elif [[ $platform == "linux/s390x" ]]; then
    echo "linux-s390"
  else
    echo "$(go env GOHOSTOS)-$(go env GOHOSTARCH)"
  fi
}
readonly -f os::util::host_platform_friendly

# This converts from platform/arch to PLATFORM_ARCH, host platform will be
# considered if no parameter passed
function os::util::platform_arch() {
  local platform=${1:-}
  if [[ -z "${platform}" ]]; then
    platform=$(os::util::host_platform)
  fi

  echo $(echo ${platform} | tr '[:lower:]/' '[:upper:]_')
}
readonly -f os::util::platform_arch

# os::util::list_go_src_files lists files we consider part of our project
# source code, useful for tools that iterate over source to provide vet-
# ting or linting, etc.
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::util::list_go_src_files() {
	find . -not \( \
		\( \
		-wholename './_output' \
		-o -wholename './.*' \
		-o -wholename './pkg/assets/bindata.go' \
		-o -wholename './pkg/assets/*/bindata.go' \
		-o -wholename './pkg/bootstrap/bindata.go' \
		-o -wholename './openshift.local.*' \
		-o -wholename './test/extended/testdata/bindata.go' \
		-o -wholename '*/vendor/*' \
		-o -wholename './cmd/service-catalog/*' \
		-o -wholename './cmd/cluster-capacity/*' \
		-o -wholename './assets/bower_components/*' \
		\) -prune \
	\) -name '*.go' | sort -u
}
readonly -f os::util::list_go_src_files

# os::util::list_go_src_dirs lists dirs in origin/ and cmd/ dirs excluding
# cmd/cluster-capacity and cmd/service-catalog and doc.go useful for tools that
# iterate over source to provide vetting or linting, or for godep-save etc.
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::util::list_go_src_dirs() {
	os::util::list_go_src_files | cut -d '/' -f 1-2 | grep -v ".go$" | grep -v "^./cmd" | LC_ALL=C sort -u
	os::util::list_go_src_files | grep "^./cmd/"| cut -d '/' -f 1-3 | grep -v ".go$" | LC_ALL=C sort -u
}
readonly -f os::util::list_go_src_dirs
