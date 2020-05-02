#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

project="$( oc project -q )"

os::test::junit::declare_suite_start "cmd/policy-storage-admin"

# Test storage-admin role and impersonation
os::cmd::expect_success 'oc adm policy add-cluster-role-to-user storage-admin storage-adm'
os::cmd::expect_success 'oc adm policy add-cluster-role-to-user storage-admin storage-adm2'
os::cmd::expect_success 'oc adm policy add-role-to-user admin storage-adm2'
os::cmd::expect_success_and_text 'oc policy who-can impersonate storage-admin' 'cluster-admin'

# Test storage-admin role as user level
#os::cmd::expect_success 'oc login -u storage-adm -p pw'
#os::cmd::expect_success_and_text 'oc whoami' "storage-adm"
#os::cmd::expect_failure 'oc whoami --as=basic-user'
#os::cmd::expect_failure 'oc whoami --as=cluster-admin'

# Test storage-admin can not do normal project scoped tasks
os::cmd::expect_failure_and_text 'oc auth can-i --as=storage-adm create pods --all-namespaces' 'no'
os::cmd::expect_failure_and_text 'oc auth can-i --as=storage-adm create projects' 'no'
os::cmd::expect_failure_and_text 'oc auth can-i --as=storage-adm create pvc' 'no'

# Test storage-admin can read pvc and pods, and create pv and storageclass
os::cmd::expect_success_and_text 'oc auth can-i --as=storage-adm get pvc --all-namespaces' 'yes'
os::cmd::expect_success_and_text 'oc auth can-i --as=storage-adm get storageclass' 'yes'
os::cmd::expect_success_and_text 'oc auth can-i --as=storage-adm create pv' 'yes'
os::cmd::expect_success_and_text 'oc auth can-i --as=storage-adm create storageclass' 'yes'
os::cmd::expect_success_and_text 'oc auth can-i --as=storage-adm get pods --all-namespaces' 'yes'

# Test failure to change policy on users for storage-admin
os::cmd::expect_failure_and_text 'oc policy --as=storage-adm add-role-to-user admin storage-adm' ' cannot list resource "rolebindings" in API group "rbac.authorization.k8s.io"'
os::cmd::expect_failure_and_text 'oc policy --as=storage-adm remove-user screeley' ' cannot list resource "rolebindings" in API group "rbac.authorization.k8s.io"'
#os::cmd::expect_success 'oc logout'

# Test that scoped storage-admin now an admin in project foo
#os::cmd::expect_success 'oc login -u storage-adm2 -p pw'
#os::cmd::expect_success_and_text 'oc whoami' "storage-adm2"
os::cmd::expect_success 'oc new-project --as=storage-adm2 --as-group=system:authenticated:oauth --as-group=system:authenticated policy-can-i'
os::cmd::expect_failure_and_text 'oc auth can-i --as=storage-adm2 create pod --all-namespaces' 'no'
os::cmd::expect_success_and_text 'oc auth can-i --as=storage-adm2 create pod' 'yes'
os::cmd::expect_success_and_text 'oc auth can-i --as=storage-adm2 create pvc' 'yes'
os::cmd::expect_success_and_text 'oc auth can-i --as=storage-adm2 create endpoints' 'yes'
os::cmd::expect_success 'oc delete project policy-can-i'
