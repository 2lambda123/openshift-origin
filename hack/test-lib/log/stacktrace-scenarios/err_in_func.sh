#!/bin/bash
#
# This test case generates an error inside of a function

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../../..
source "${OS_ROOT}/hack/lib/util/trap.sh"
source "${OS_ROOT}/hack/lib/log/stacktrace.sh"

os::util::trap::init
os::log::stacktrace::install

function grandparent() {
	parent
}

function parent() {
	child
}

function child() {
	grandchild
}

function grandchild() {
	grep >/dev/null 2>&1
}

grandparent
