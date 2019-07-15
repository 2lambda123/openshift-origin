#!/usr/bin/env bash

# This command checks that the built commands can function together for
# simple scenarios.  It does not require Docker so it can run in travis.
export LOG_DIR="$(dirname ${BASH_SOURCE})/logs"

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
os::util::environment::setup_time_vars

function cleanup() {
  return_code=$?
  os::test::junit::generate_report
  os::util::describe_return_code "${return_code}"

  exit "${return_code}"
}
trap "cleanup" EXIT

function find_tests() {
    local test_regex="${1}"
    local full_test_list=()
    local selected_tests=()

    full_test_list=( $(find "${TESTS_DIR}" -name '*.sh') )
    for test in "${full_test_list[@]}"; do
        if grep -q -E "${test_regex}" <<< "${test}"; then
            selected_tests+=( "${test}" )
        fi
    done

    if [[ "${#selected_tests[@]}" -eq 0 ]]; then
        os::log::fatal "No tests were selected due to invalid regex."
    else
        echo "${selected_tests[@]}"
    fi
}

if [ -z "${USER_TOKEN:-}" ]; then
  os::log::error "Please provide a token of a user to run the tests with"
  exit 1
fi
export USER_CREDENTIALS="--token=${USER_TOKEN}"
export ADMIN_KUBECONFIG="/tmp/admin.kubeconfig"
export KUBECONFIG="/tmp/kubeconfig"

cp "$KUBECONFIG_TESTS" "$KUBECONFIG"
cp "$KUBECONFIG_TESTS" "$ADMIN_KUBECONFIG"

CLUSTER_ADMIN_CONTEXT=$(oc config view --config="${ADMIN_KUBECONFIG}" --flatten -o template --template='{{index . "current-context"}}'); export CLUSTER_ADMIN_CONTEXT

tests=( $(find_tests ${1:-.*}) )

# NOTE: Do not add tests here, add them to test/cmd/*.
# Tests should assume they run in an empty project, and should be reentrant if possible
# to make it easy to run individual tests
for test in "${tests[@]}"; do
  echo
  echo "++ ${test}"
  name=$(basename ${test} .sh)
  namespace="cmd-${name}"

  os::test::junit::declare_suite_start "cmd/${namespace}-namespace-setup"
  # switch back to a standard identity. This prevents individual tests from changing contexts and messing up other tests
  os::cmd::expect_success "oc login --server=${KUBERNETES_MASTER} ${KUBERNETES_CA_OPTION:-} ${USER_CREDENTIALS}"
  os::cmd::expect_success "oc project ${CLUSTER_ADMIN_CONTEXT}"
  os::cmd::expect_success "oc new-project '${namespace}'"
  # wait for the project cache to catch up and correctly list us in the new project
  os::cmd::try_until_text "oc get projects -o name" "project.project.openshift.io/${namespace}"
  os::test::junit::declare_suite_end

   if ! ${test}; then
     failed="true"
   fi

  os::test::junit::declare_suite_start "cmd/${namespace}-namespace-teardown"
  os::cmd::expect_success "oc project '${CLUSTER_ADMIN_CONTEXT}'"
  os::cmd::expect_success "oc delete project '${namespace}'"
  cp ${KUBECONFIG_TESTS} ${KUBECONFIG}  # since nothing ever gets deleted from kubeconfig, reset it
  os::test::junit::declare_suite_end
done

os::log::debug "Metrics information logged to ${LOG_DIR}/metrics.log"
oc get --raw /metrics --kubeconfig="${ADMIN_KUBECONFIG}" > "${LOG_DIR}/metrics.log"

if [[ -n "${failed:-}" ]]; then
    exit 1
fi
echo "test-cmd: ok"
