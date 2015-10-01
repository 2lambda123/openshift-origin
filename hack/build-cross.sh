#!/bin/bash

# Build all cross compile targets and the base binaries

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# Build the primary client/server for all platforms
OS_BUILD_PLATFORMS=("${OS_CROSS_COMPILE_PLATFORMS[@]}")
os::build::build_binaries "${OS_CROSS_COMPILE_TARGETS[@]}"

# Build image binaries for a subset of platforms. Image binaries are currently
# linux-only, and are compiled with flags to make them static for use in Docker
# images "FROM scratch".
OS_BUILD_PLATFORMS=("${OS_IMAGE_COMPILE_PLATFORMS[@]-}")
CGO_ENABLED=0 OS_GOFLAGS="-a" os::build::build_binaries "${OS_IMAGE_COMPILE_TARGETS[@]-}"
CGO_ENABLED=0 OS_GOFLAGS="-a -installsuffix cgo" os::build::build_binaries "${OS_SCRATCH_IMAGE_COMPILE_TARGETS[@]-}"

# Make the primary client/server release.
OS_RELEASE_ARCHIVE="openshift-origin"
OS_BUILD_PLATFORMS=("${OS_CROSS_COMPILE_PLATFORMS[@]}")
os::build::place_bins "${OS_CROSS_COMPILE_BINARIES[@]}"

# Make the image binaries release.
OS_RELEASE_ARCHIVE="openshift-origin-image"
OS_BUILD_PLATFORMS=("${OS_IMAGE_COMPILE_PLATFORMS[@]-}")
os::build::place_bins "${OS_IMAGE_COMPILE_BINARIES[@]}"

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
