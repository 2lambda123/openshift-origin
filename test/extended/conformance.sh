#!/bin/bash
#
# Runs the conformance extended tests for OpenShift

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/test/extended/setup.sh"
cd "${OS_ROOT}"

os::test::extended::setup
os::test::extended::focus "$@"

function join { local IFS="$1"; shift; echo "$*"; }

parallel_only=( "${CONFORMANCE_TESTS[@]}" )
parallel_exclude=( "${EXCLUDED_TESTS[@]}" "${SERIAL_TESTS[@]}" )
serial_only=( "${SERIAL_TESTS[@]}" )
serial_exclude=( "${EXCLUDED_TESTS[@]}" )

pf=$(join '|' "${parallel_only[@]}")
ps=$(join '|' "${parallel_exclude[@]}")
sf=$(join '|' "${serial_only[@]}")
ss=$(join '|' "${serial_exclude[@]}")

exitstatus=0

# run parallel tests
nodes="${PARALLEL_NODES:-5}"
echo "[INFO] Running parallel tests N=${nodes}"
TEST_REPORT_FILE_NAME=conformance_parallel ${GINKGO} -v "-focus=${pf}" "-skip=${ps}" -p -nodes "${nodes}" ${EXTENDEDTEST} -- -ginkgo.v -test.timeout 6h || exitstatus=$?

# run tests in serial
echo "[INFO] Running serial tests"
TEST_REPORT_FILE_NAME=conformance_serial ${GINKGO} -v "-focus=${sf}" "-skip=${ss}" ${EXTENDEDTEST} -- -ginkgo.v -test.timeout 2h || exitstatus=$?

exit $exitstatus
