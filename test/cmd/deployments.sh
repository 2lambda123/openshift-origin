#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::install_errexit
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/deployments"
# This test validates deployments and the env command

os::cmd::expect_success 'oc get deploymentConfigs'
os::cmd::expect_success 'oc get dc'
os::cmd::expect_success 'oc create -f test/integration/fixtures/test-deployment-config.yaml'
os::cmd::expect_success 'oc describe deploymentConfigs test-deployment-config'
os::cmd::expect_success_and_text 'oc get dc -o name' 'deploymentconfig/test-deployment-config'
os::cmd::try_until_success 'oc get rc/test-deployment-config-1'
os::cmd::expect_success_and_text 'oc describe dc test-deployment-config' 'deploymentconfig=test-deployment-config'

os::test::junit::declare_suite_start "cmd/deployments/env"
# Patch a nil list
os::cmd::expect_success 'oc env dc/test-deployment-config TEST=value'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'TEST=value'
# Remove only env in the list
os::cmd::expect_success 'oc env dc/test-deployment-config TEST-'
os::cmd::expect_success_and_not_text 'oc env dc/test-deployment-config --list' 'TEST=value'
# Add back to empty list
os::cmd::expect_success 'oc env dc/test-deployment-config TEST=value'
os::cmd::expect_success_and_not_text 'oc env dc/test-deployment-config TEST=foo --list' 'TEST=value'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config TEST=foo --list' 'TEST=foo'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config OTHER=foo --list' 'TEST=value'
os::cmd::expect_success_and_not_text 'oc env dc/test-deployment-config OTHER=foo -c ruby --list' 'OTHER=foo'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config OTHER=foo -c ruby*   --list' 'OTHER=foo'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config OTHER=foo -c *hello* --list' 'OTHER=foo'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config OTHER=foo -c *world  --list' 'OTHER=foo'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config OTHER=foo --list' 'OTHER=foo'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config OTHER=foo -o yaml' 'name: OTHER'
os::cmd::expect_success_and_text 'echo OTHER=foo | oc env dc/test-deployment-config -e - --list' 'OTHER=foo'
os::cmd::expect_success_and_not_text 'echo #OTHER=foo | oc env dc/test-deployment-config -e - --list' 'OTHER=foo'
os::cmd::expect_success 'oc env dc/test-deployment-config TEST=bar OTHER=baz BAR-'
os::cmd::expect_success_and_not_text 'oc env -f test/integration/fixtures/test-deployment-config.yaml TEST=VERSION -o yaml' 'v1beta3'
os::cmd::expect_success 'oc env dc/test-deployment-config A=a B=b C=c D=d E=e F=f G=g'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'A=a'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'B=b'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'C=c'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'D=d'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'E=e'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'F=f'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'G=g'
os::cmd::expect_success 'oc env dc/test-deployment-config H=h G- E=updated C- A-'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'B=b'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'D=d'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'E=updated'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'F=f'
os::cmd::expect_success_and_text 'oc env dc/test-deployment-config --list' 'H=h'
os::cmd::expect_success_and_not_text 'oc env dc/test-deployment-config --list' 'A=a'
os::cmd::expect_success_and_not_text 'oc env dc/test-deployment-config --list' 'C=c'
os::cmd::expect_success_and_not_text 'oc env dc/test-deployment-config --list' 'G=g'
echo "env: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/deployments/config"
os::cmd::expect_success 'oc deploy test-deployment-config'
os::cmd::expect_success 'oc deploy dc/test-deployment-config'
os::cmd::expect_success 'oc delete deploymentConfigs test-deployment-config'
echo "deploymentConfigs: ok"
os::test::junit::declare_suite_end

os::cmd::expect_success 'oc delete all --all'
# TODO: remove, flake caused by deployment controller updating the following dc
sleep 1
os::cmd::expect_success 'oc delete all --all'

os::cmd::expect_success 'oc process -f examples/sample-app/application-template-dockerbuild.json -l app=dockerbuild | oc create -f -'
os::cmd::try_until_success 'oc get rc/database-1'

os::test::junit::declare_suite_start "cmd/deployments/rollback"
os::cmd::expect_success 'oc rollback database --to-version=1 -o=yaml'
os::cmd::expect_success 'oc rollback dc/database --to-version=1 -o=yaml'
os::cmd::expect_success 'oc rollback dc/database --to-version=1 --dry-run'
os::cmd::expect_success 'oc rollback database-1 -o=yaml'
os::cmd::expect_success 'oc rollback rc/database-1 -o=yaml'
# should fail because there's no previous deployment
os::cmd::expect_failure 'oc rollback database -o yaml'
echo "rollback: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/deployments/stop"
os::cmd::expect_success 'oc get dc/database'
os::cmd::expect_success 'oc expose dc/database --name=fromdc'
# should be a service
os::cmd::expect_success 'oc get svc/fromdc'
os::cmd::expect_success 'oc delete svc/fromdc'
os::cmd::expect_failure_and_text 'oc stop dc/database' 'delete'
os::cmd::expect_success 'oc delete dc/database'
os::cmd::expect_failure 'oc get dc/database'
os::cmd::expect_failure 'oc get rc/database-1'
echo "stop: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/deployments/autoscale"
os::cmd::expect_success 'oc create -f test/integration/fixtures/test-deployment-config.yaml'
os::cmd::expect_success 'oc autoscale dc/test-deployment-config --max 5'
os::cmd::expect_success_and_text "oc get hpa/test-deployment-config --template='{{.spec.maxReplicas}}'" "5"
os::cmd::expect_success 'oc delete dc/test-deployment-config'
os::cmd::expect_success 'oc delete hpa/test-deployment-config'
echo "autoscale: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end
