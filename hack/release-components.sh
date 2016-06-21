#!/bin/bash

# This script builds and pushes a release to DockerHub.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/lib/init.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

tag="${OS_TAG:-}"
if [[ -z "${tag}" ]]; then
  if [[ "$( git tag --points-at HEAD | wc -l )" -ne 1 ]]; then
    echo "error: Specify OS_TAG or ensure the current git HEAD is tagged."
    exit 1
  fi
  tag=":$( git tag --points-at HEAD )"
fi

# release_component is the standard release pattern for subcomponents
function release_component() {
  local STARTTIME=$(date +%s)
  echo "--- $1 $2 ---"
  mkdir -p "_output/components"
  (
    pushd _output/components/
    git clone --recursive "$2" "$1" -b "${tag}"
    OS_TAG="${tag}" hack/release.sh
  )
  local ENDTIME=$(date +%s); echo "--- $1 took $(($ENDTIME - $STARTTIME)) seconds ---"
}

release_component logging https://github.com/openshift/origin-aggregated-logging
release_component metrics https://github.com/openshift/origin-metrics