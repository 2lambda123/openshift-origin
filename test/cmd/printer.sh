#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Test that resource printer includes resource kind on multiple resources
os::test::junit::declare_suite_start "cmd/basicresources/printer"
os::cmd::expect_success 'oc create imagestream test1'
os::cmd::expect_success 'oc new-app registry.centos.org/centos/php-72-centos7:latest'
os::cmd::expect_success_and_text 'oc get all' 'imagestream.image.openshift.io/test1'
os::cmd::expect_success_and_not_text 'oc get is' 'imagestream.image.openshift.io/test1'

# Test that resource printer includes namespaces for buildconfigs with custom strategies
os::cmd::expect_success 'oc create -f examples/sample-app/application-template-custombuild.json'
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample' 'deploymentconfig.apps.openshift.io "frontend" created'
os::cmd::expect_success_and_text 'oc get all --all-namespaces' 'cmd-printer[\ ]+buildconfig.build.openshift.io\/ruby\-sample\-build'

# Test that infos printer supports all outputFormat options
os::cmd::expect_success_and_text 'oc new-app registry.centos.org/centos/php-72-centos7:latest -o yaml | oc set env -f - MYVAR=value' 'deploymentconfig.apps.openshift.io/php-72-centos7 updated'
# FIXME: what to do with this?
# os::cmd::expect_success 'oc new-app php -o yaml | oc set env -f - MYVAR=value -o custom-colums="NAME:.metadata.name"'
os::cmd::expect_success_and_text 'oc new-app registry.centos.org/centos/php-72-centos7:latest -o yaml | oc set env -f - MYVAR=value -o yaml' 'apiVersion: apps.openshift.io/v1'
os::cmd::expect_success_and_text 'oc new-app registry.centos.org/centos/php-72-centos7:latest -o yaml | oc set env -f - MYVAR=value -o json' '"apiVersion": "apps.openshift.io/v1"'
echo "resource printer: ok"
os::test::junit::declare_suite_end
