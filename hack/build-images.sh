#!/bin/bash

# This script builds all images locally except the base and release images,
# which are handled by hack/build-base-images.sh.

# NOTE:  you only need to run this script if your code changes are part of
# any images OpenShift runs internally such as origin-sti-builder, origin-docker-builder,
# origin-deployer, etc.
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
source "${OS_ROOT}/contrib/node/install-sdn.sh"

if [[ "${OS_RELEASE:-}" == "n" ]]; then
	# Use local binaries
	imagedir="${OS_OUTPUT_BINPATH}/linux/amd64"
	# identical to build-cross.sh
	os::build::os_version_vars
	OS_RELEASE_COMMIT="${OS_GIT_VERSION//+/-}"
	platform="$(os::build::host_platform)"
	OS_BUILD_PLATFORMS=("${OS_IMAGE_COMPILE_PLATFORMS[@]:-${platform}}")
	OS_IMAGE_COMPILE_TARGETS=("${OS_IMAGE_COMPILE_TARGETS[@]:-${OS_IMAGE_COMPILE_TARGETS_LINUX[@]}}")
	OS_SCRATCH_IMAGE_COMPILE_TARGETS=("${OS_SCRATCH_IMAGE_COMPILE_TARGETS[@]:-${OS_SCRATCH_IMAGE_COMPILE_TARGETS_LINUX[@]}}")

	echo "Building images from source ${OS_RELEASE_COMMIT}:"
	echo
	os::build::build_static_binaries "${OS_IMAGE_COMPILE_TARGETS[@]-}" "${OS_SCRATCH_IMAGE_COMPILE_TARGETS[@]-}"
	os::build::place_bins "${OS_IMAGE_COMPILE_BINARIES[@]}"
	echo
else
	# Get the latest Linux release
	if [[ ! -d _output/local/releases ]]; then
		echo "No release has been built. Run hack/build-release.sh"
		exit 1
	fi

	# Extract the release archives to a staging area.
	os::build::detect_local_release_tars "linux-64bit"

	echo "Building images from release tars for commit ${OS_RELEASE_COMMIT}:"
	echo " primary: $(basename ${OS_PRIMARY_RELEASE_TAR})"
	echo " image:   $(basename ${OS_IMAGE_RELEASE_TAR})"

	imagedir="${OS_OUTPUT}/images"
	rm -rf ${imagedir}
	mkdir -p ${imagedir}
	os::build::extract_tar "${OS_PRIMARY_RELEASE_TAR}" "${imagedir}"
	os::build::extract_tar "${OS_IMAGE_RELEASE_TAR}" "${imagedir}"
fi

# Create link to file if the FS supports hardlinks, otherwise copy the file
function ln_or_cp {
	local src_file=$1
	local dst_dir=$2
	if os::build::is_hardlink_supported "${dst_dir}" ; then
		ln -f "${src_file}" "${dst_dir}"
	else
		cp -pf "${src_file}" "${dst_dir}"
	fi
}


# image-build is wrapped to allow output to be captured
function image-build() {
	local tag=$1
	local dir=$2
	local dest="${tag}"
	if [[ ! "${tag}" == *":"* ]]; then
		dest="${tag}:latest"
	fi

	local STARTTIME
	local ENDTIME
	STARTTIME="$(date +%s)"

	# build the image
	if ! os::build::image "${dir}" "${dest}"; then
		os::log::warn "Retrying build once"
		os::build::image "${dir}" "${dest}"
	fi

	# tag to release commit unless we specified a hardcoded tag
	if [[ ! "${tag}" == *":"* ]]; then
		docker tag "${dest}" "${tag}:${OS_RELEASE_COMMIT}"
	fi
	# ensure the temporary contents are cleaned up
	git clean -fdx "${dir}"

	ENDTIME="$(date +%s)"
	echo "Finished in $(($ENDTIME - $STARTTIME)) seconds"
}

# builds an image and tags it two ways - with latest, and with the release tag
function image() {
	local tag=$1
	local dir=$2
	local out
	mkdir -p "${BASETMPDIR}"
	out="$( mktemp "${BASETMPDIR}/imagelogs.XXXXX" )"
	if ! image-build "${tag}" "${dir}" > "${out}" 2>&1; then
		sed -e "s|^|$1: |" "${out}" 1>&2
		os::log::error "Failed to build $1"
		return 1
	fi
	sed -e "s|^|$1: |" "${out}"
	return 0
}

# Link or copy primary binaries to the appropriate locations.
ln_or_cp "${imagedir}/openshift" images/origin/bin

# Link or copy image binaries to the appropriate locations.
ln_or_cp "${imagedir}/pod"             images/pod/bin
ln_or_cp "${imagedir}/hello-openshift" examples/hello-openshift/bin
ln_or_cp "${imagedir}/gitserver"       examples/gitserver/bin
ln_or_cp "${imagedir}/dockerregistry"  images/dockerregistry/bin

# Copy SDN scripts into images/node
os::provision::install-sdn "${OS_ROOT}" "${imagedir}" "${OS_ROOT}/images/node"
mkdir -p images/node/conf/
cp -pf "${OS_ROOT}/contrib/systemd/openshift-sdn-ovs.conf" images/node/conf/

# images that depend on scratch / centos
image openshift/origin-pod                   images/pod
image openshift/openvswitch                  images/openvswitch
# images that depend on openshift/origin-base
image openshift/origin                       images/origin
image openshift/origin-haproxy-router        images/router/haproxy
image openshift/origin-keepalived-ipfailover images/ipfailover/keepalived
image openshift/origin-docker-registry       images/dockerregistry
image openshift/origin-egress-router         images/router/egress

# images that depend on openshift/origin
image openshift/origin-gitserver             examples/gitserver
image openshift/origin-deployer              images/deployer
image openshift/origin-recycler              images/recycler
image openshift/origin-docker-builder        images/builder/docker/docker-builder
image openshift/origin-sti-builder           images/builder/docker/sti-builder
image openshift/origin-f5-router             images/router/f5
image openshift/node                         images/node

# extra images (not part of infrastructure)
image openshift/hello-openshift       examples/hello-openshift

ln_or_cp "${imagedir}/deployment" examples/deployment/bin
image openshift/deployment-example:v1 examples/deployment
ln_or_cp "${imagedir}/deployment" examples/deployment/bin
image openshift/deployment-example:v2 examples/deployment examples/deployment/Dockerfile.v2

echo
echo
echo "++ Active images"

docker images | grep openshift/ | grep ${OS_RELEASE_COMMIT} | sort
echo

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
