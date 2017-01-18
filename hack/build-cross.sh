#!/bin/bash

# Build all cross compile targets and the base binaries
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

host_platform="$(os::build::host_platform)"

# Set build tags for these binaries
readonly OS_GOFLAGS_TAGS="include_gcs include_oss"
readonly OS_GOFLAGS_TAGS_LINUX_PPC64LE="gssapi"
readonly OS_GOFLAGS_TAGS_LINUX_AMD64="gssapi"

# by default, build for these platforms
platforms=(
  linux/amd64
  darwin/amd64
  windows/amd64
  linux/386
)
image_platforms=( )
test_platforms=( "${host_platform}" )

targets=( "${OS_CROSS_COMPILE_TARGETS[@]}" )

# Special case ppc64le
if [[ "${host_platform}" == "linux/ppc64le" ]]; then
  platforms+=( "linux/ppc64le" )
fi

# On linux platforms, build images
if [[ "${host_platform}" == linux/* ]]; then
  image_platforms+=( "${host_platform}" )
fi

# filter platform list
if [[ -n "${OS_ONLY_BUILD_PLATFORMS-}" ]]; then
  filtered=( )
  for platform in ${platforms[@]}; do
    if [[ "${platform}" =~ "${OS_ONLY_BUILD_PLATFORMS}" ]]; then
      filtered+=("${platform}")
    fi
  done
  platforms=("${filtered[@]}")

  filtered=( )
  for platform in ${image_platforms[@]}; do
    if [[ "${platform}" =~ "${OS_ONLY_BUILD_PLATFORMS}" ]]; then
      filtered+=("${platform}")
    fi
  done
  image_platforms=("${filtered[@]}")

  filtered=( )
  for platform in ${test_platforms[@]}; do
    if [[ "${platform}" =~ "${OS_ONLY_BUILD_PLATFORMS}" ]]; then
      filtered+=("${platform}")
    fi
  done
  test_platforms=("${filtered[@]}")
fi

# Build image binaries for a subset of platforms. Image binaries are currently
# linux-only, and are compiled with flags to make them static for use in Docker
# images "FROM scratch".
OS_BUILD_PLATFORMS=("${image_platforms[@]}")
os::build::build_static_binaries "${OS_IMAGE_COMPILE_TARGETS_LINUX[@]-}" "${OS_SCRATCH_IMAGE_COMPILE_TARGETS_LINUX[@]-}"

# Build the primary client/server for all platforms
OS_BUILD_PLATFORMS=("${platforms[@]}")
os::build::build_binaries "${OS_CROSS_COMPILE_TARGETS[@]}"

# Build the test binaries for the host platform
OS_BUILD_PLATFORMS=("${test_platforms[@]}")
os::build::build_binaries "${OS_TEST_TARGETS[@]}"

# Make the primary client/server release.
OS_BUILD_PLATFORMS=("${platforms[@]}")
OS_RELEASE_ARCHIVE="openshift-origin" \
  os::build::place_bins "${OS_CROSS_COMPILE_BINARIES[@]}"

# Make the image binaries release.
OS_BUILD_PLATFORMS=("${image_platforms[@]}")
OS_RELEASE_ARCHIVE="openshift-origin-image" \
  os::build::place_bins "${OS_IMAGE_COMPILE_BINARIES[@]}"

os::build::release_sha

if [[ "${OS_GIT_TREE_STATE:-dirty}" == "clean"  ]]; then
	# only when we are building from a clean state can we claim to
	# have created a valid set of binaries that can resemble a release
	echo "${OS_GIT_COMMIT}" > "${OS_LOCAL_RELEASEPATH}/.commit"
fi

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
