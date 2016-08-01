#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/authentication"

os::test::junit::declare_suite_start "cmd/authentication/scopedtokens"
os::cmd::expect_success 'oadm policy add-role-to-user admin scoped-user'

# initialize the user object
os::cmd::expect_success 'oc login -u scoped-user -p asdf'
os::cmd::expect_success 'oc login -u system:admin'
username="$(oc get user/scoped-user -o jsonpath='{.metadata.name}')"
useruid="$(oc get user/scoped-user -o jsonpath='{.metadata.uid}')"
os::cmd::expect_success_and_text "oc policy can-i --list -n cmd-authentication --as=scoped-user" 'get.*pods'

whoamitoken="$(oc process -f "${OS_ROOT}/test/testdata/authentication/scoped-token-template.yaml" TOKEN_PREFIX=whoami SCOPE=user:info USER_NAME="${username}" USER_UID="${useruid}" | oc create -f - -o name | awk -F/ '{print $2}')"
os::cmd::expect_success_and_text "oc get user/~ --token='${whoamitoken}'" "${username}"
os::cmd::expect_failure_and_text "oc get pods --token='${whoamitoken}' -n cmd-authentication" 'prevent this action; User "scoped-user" cannot list pods in project "cmd-authentication"'

listprojecttoken="$(oc process -f "${OS_ROOT}/test/testdata/authentication/scoped-token-template.yaml" TOKEN_PREFIX=listproject SCOPE=user:list-projects USER_NAME="${username}" USER_UID="${useruid}" | oc create -f - -o name | awk -F/ '{print $2}')"
os::cmd::expect_success_and_text "oc get projects --token='${listprojecttoken}'" 'cmd-authentication'
os::cmd::expect_failure_and_text "oc get user/~ --token='${listprojecttoken}'" 'prevent this action; User "scoped-user" cannot get users at the cluster scope'
os::cmd::expect_failure_and_text "oc get pods --token='${listprojecttoken}' -n cmd-authentication" 'prevent this action; User "scoped-user" cannot list pods in project "cmd-authentication"'

adminnonescalatingpowerstoken="$(oc process -f "${OS_ROOT}/test/testdata/authentication/scoped-token-template.yaml" TOKEN_PREFIX=admin SCOPE=role:admin:* USER_NAME="${username}" USER_UID="${useruid}" | oc create -f - -o name | awk -F/ '{print $2}')"
os::cmd::expect_failure_and_text "oc get user/~ --token='${adminnonescalatingpowerstoken}'" 'prevent this action; User "scoped-user" cannot get users at the cluster scope'
os::cmd::expect_failure_and_text "oc get secrets --token='${adminnonescalatingpowerstoken}' -n cmd-authentication" 'prevent this action; User "scoped-user" cannot list secrets in project "cmd-authentication"'
os::cmd::expect_success_and_text "oc get projects/cmd-authentication --token='${adminnonescalatingpowerstoken}' -n cmd-authentication" 'cmd-authentication'

allescalatingpowerstoken="$(oc process -f "${OS_ROOT}/test/testdata/authentication/scoped-token-template.yaml" TOKEN_PREFIX=clusteradmin SCOPE='role:cluster-admin:*:!' USER_NAME="${username}" USER_UID="${useruid}" | oc create -f - -o name | awk -F/ '{print $2}')"
os::cmd::expect_success_and_text "oc get user/~ --token='${allescalatingpowerstoken}'" "${username}"
os::cmd::expect_success "oc get secrets --token='${allescalatingpowerstoken}' -n cmd-authentication"
# scopes allow it, but authorization doesn't
os::cmd::expect_failure_and_text "oc get secrets --token='${allescalatingpowerstoken}' -n default" 'cannot list secrets in project'
os::cmd::expect_success_and_text "oc get projects --token='${allescalatingpowerstoken}'" 'cmd-authentication'
os::cmd::expect_success_and_text "oc policy can-i --list --token='${allescalatingpowerstoken}' -n cmd-authentication" 'get.*pods'

accesstoken="$(oc process -f "${OS_ROOT}/test/testdata/authentication/scoped-token-template.yaml" TOKEN_PREFIX=access SCOPE=user:check-access USER_NAME="${username}" USER_UID="${useruid}" | oc create -f - -o name | awk -F/ '{print $2}')"
os::cmd::expect_success_and_text "curl -k -XPOST -H 'Content-Type: application/json' -H 'Authorization: Bearer ${accesstoken}' ${API_SCHEME}://${API_HOST}:${API_PORT}/oapi/v1/namespaces/cmd-authentication/localsubjectaccessreviews -d @${OS_ROOT}/test/testdata/authentication/localsubjectaccessreview.json" '"kind": "SubjectAccessReviewResponse"'
os::cmd::expect_success_and_text "oc policy can-i create pods --token='${accesstoken}' -n cmd-authentication --ignore-scopes" 'yes'
os::cmd::expect_success_and_text "oc policy can-i create pods --token='${accesstoken}' -n cmd-authentication" 'no'
os::cmd::expect_success_and_text "oc policy can-i create subjectaccessreviews --token='${accesstoken}' -n cmd-authentication" 'no'
os::cmd::expect_success_and_text "oc policy can-i create subjectaccessreviews --token='${accesstoken}' -n cmd-authentication --ignore-scopes" 'yes'
os::cmd::expect_success_and_text "oc policy can-i create pods --token='${accesstoken}' -n cmd-authentication --scopes='role:admin:*'" 'yes'
os::cmd::expect_success_and_text "oc policy can-i --list --token='${accesstoken}' -n cmd-authentication --scopes='role:admin:*'" 'get.*pods'
os::cmd::expect_success_and_not_text "oc policy can-i --list --token='${accesstoken}' -n cmd-authentication" 'get.*pods'


os::test::junit::declare_suite_end

os::test::junit::declare_suite_end
