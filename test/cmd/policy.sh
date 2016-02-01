#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
source "${OS_ROOT}/hack/lib/util/trap.sh"
source "${OS_ROOT}/hack/lib/log/stacktrace.sh"

os::util::trap::init_err
os::log::stacktrace::install

# This test validates user level policy

os::cmd::expect_success 'oc policy add-role-to-group cluster-admin system:unauthenticated'
os::cmd::expect_success 'oc policy add-role-to-user cluster-admin system:no-user'
os::cmd::expect_success 'oc get rolebinding/cluster-admin --no-headers'
os::cmd::expect_success_and_text 'oc get rolebinding/cluster-admin --no-headers' 'system:no-user'

os::cmd::expect_success 'oc policy add-role-to-user cluster-admin -z=one,two --serviceaccount=three,four'
os::cmd::expect_success 'oc get rolebinding/cluster-admin --no-headers'
os::cmd::expect_success_and_text 'oc get rolebinding/cluster-admin --no-headers' 'one'
os::cmd::expect_success_and_text 'oc get rolebinding/cluster-admin --no-headers' 'four'

os::cmd::expect_success 'oc policy remove-role-from-group cluster-admin system:unauthenticated'

os::cmd::expect_success 'oc policy remove-role-from-user cluster-admin system:no-user'
os::cmd::expect_success 'oc policy remove-role-from-user cluster-admin -z=one,two --serviceaccount=three,four'
os::cmd::expect_success 'oc get rolebinding/cluster-admin --no-headers'
os::cmd::expect_success_and_not_text 'oc get rolebinding/cluster-admin --no-headers' 'four'

os::cmd::expect_success 'oc policy remove-group system:unauthenticated'
os::cmd::expect_success 'oc policy remove-user system:no-user'
echo "policy: ok"
