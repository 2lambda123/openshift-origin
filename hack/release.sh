#!/bin/bash

# This script builds and pushes a release to DockerHub.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

if [[ -z "${OS_TAG}" ]]; then
  echo "You must specify the OS_TAG variable as the name of the tag to create, e.g. 'v1.0.1'."
  exit 1
fi
tag="${OS_TAG}"

if [[ "$(git name-rev --name-only --tags HEAD)" != "${tag}^0" ]]; then
  if git rev-parse -q --short "${tag}" &>/dev/null; then
    echo "Tag ${tag} already exists"
    exit 1
  else
    git tag "${tag}" -a -m "${tag}" HEAD
  fi
fi

function removeimage() {
  for i in $@; do
    if docker inspect $i &>/dev/null; then
      docker rmi $i
    fi
    if docker inspect docker.io/$i &>/dev/null; then
      docker rmi docker.io/$i
    fi
  done
}

removeimage openshift/origin-base openshift/origin-release openshift/origin-haproxy-router-base
docker pull openshift/origin-base
docker pull openshift/origin-release
docker pull openshift/origin-haproxy-router-base

hack/build-release.sh
hack/build-images.sh
OS_PUSH_TAG="${tag}" OS_TAG="" OS_PUSH_LOCAL="1" hack/push-release.sh

echo
echo "Pushed ${tag} to DockerHub"
echo "1. Push tag to GitHub with: git push origin --tags # (ensure you have no extra tags in your environment)"
echo "2. Create a new release on the releases page and upload the built binaries in _output/local/releases"
echo "   Note: you should untar the Windows binary and recompress it as a zip"
echo "3. Send an email"