#!/bin/bash

set -e

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

TMPDIR="${TMPDIR:-"/tmp"}"
LOG_DIR="${LOG_DIR:-$(mktemp -d ${TMPDIR}/openshift.assets.logs.XXXX)}"

function cmd() {
  local cmd="$1"
  local log_file=$(mktemp ${LOG_DIR}/install-assets.XXXX)}
  echo "[install-assets] ${cmd}"
  rc="0"
  $cmd &> ${log_file} || rc=$?
  if [ "$rc" != "0" ]; then
    echo "[ERROR] Command '${cmd}' failed with ${rc}, logs:" && cat ${log_file}
    exit $rc
  fi
}

# If we are running inside of Travis then do not run the rest of this
# script unless we want to TEST_ASSETS
if [[ "${TRAVIS-}" == "true" && "${TEST_ASSETS-}" == "false" ]]; then
  exit
fi

# Lock version of npm to work around https://github.com/npm/npm/issues/6309
if [[ "${TRAVIS-}" == "true" ]]; then
  cmd "npm install npm@2.1.3" "npm.log"
fi

# Install bower if needed
if ! which bower > /dev/null 2>&1 ; then
  if [[ "${TRAVIS-}" == "true" ]]; then
    cmd "npm install -g bower"
  else
    cmd "sudo npm install -g bower"
  fi
fi
 
# Install grunt if needed
if ! which grunt > /dev/null 2>&1 ; then
  if [[ "${TRAVIS-}" == "true" ]]; then
    cmd "npm install -g grunt-cli"
  else
    cmd "sudo npm install -g grunt-cli"
  fi
fi

pushd ${OS_ROOT}/assets > /dev/null
  cmd "npm install"

  # In case upstream components change things without incrementing versions
  cmd "bower cache clean"
  cmd "bower install"

  cmd "bundle install --path ${OS_ROOT}/assets/.bundle"
popd > /dev/null

pushd ${OS_ROOT}/Godeps/_workspace > /dev/null
  godep_path=$(pwd)
  pushd src/github.com/jteeuwen/go-bindata > /dev/null
    GOPATH=$godep_path go install ./...
  popd > /dev/null
popd > /dev/null
