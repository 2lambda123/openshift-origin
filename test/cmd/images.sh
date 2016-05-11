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


defaultimage="openshift/origin-\${component}:latest"
USE_IMAGES=${USE_IMAGES:-$defaultimage}

os::test::junit::declare_suite_start "cmd/images"
# This test validates images and image streams along with the tag and import-image commands

os::test::junit::declare_suite_start "cmd/images/images"
os::cmd::expect_success 'oc get images'
os::cmd::expect_success 'oc create -f test/integration/fixtures/test-image.json'
os::cmd::expect_success 'oc delete images test'
echo "images: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/images/imagestreams"
os::cmd::expect_success 'oc get imageStreams'
os::cmd::expect_success 'oc create -f test/integration/fixtures/test-image-stream.json'
# verify that creating a registry fills out .status.dockerImageRepository
if os::cmd::expect_success_and_not_text "oc get imageStreams test --template='{{.status.dockerImageRepository}}'" '.'; then
  # create the registry
  os::cmd::expect_success "oadm registry --images='${USE_IMAGES}' -n default"
  # make sure stream.status.dockerImageRepository IS set
  os::cmd::expect_success_and_text "oc get imageStreams test --template='{{.status.dockerImageRepository}}'" 'test'
fi
os::cmd::expect_success 'oc delete imageStreams test'
os::cmd::expect_failure 'oc get imageStreams test'

os::cmd::expect_success 'oc create -f examples/image-streams/image-streams-centos7.json'
os::cmd::expect_success_and_text "oc get imageStreams ruby --template='{{.status.dockerImageRepository}}'" 'ruby'
os::cmd::expect_success_and_text "oc get imageStreams nodejs --template='{{.status.dockerImageRepository}}'" 'nodejs'
os::cmd::expect_success_and_text "oc get imageStreams wildfly --template='{{.status.dockerImageRepository}}'" 'wildfly'
os::cmd::expect_success_and_text "oc get imageStreams mysql --template='{{.status.dockerImageRepository}}'" 'mysql'
os::cmd::expect_success_and_text "oc get imageStreams postgresql --template='{{.status.dockerImageRepository}}'" 'postgresql'
os::cmd::expect_success_and_text "oc get imageStreams mongodb --template='{{.status.dockerImageRepository}}'" 'mongodb'
# verify the image repository had its tags populated
os::cmd::try_until_success 'oc get imagestreamtags wildfly:latest'
os::cmd::expect_success_and_text "oc get imageStreams wildfly --template='{{ index .metadata.annotations \"openshift.io/image.dockerRepositoryCheck\"}}'" '[0-9]{4}\-[0-9]{2}\-[0-9]{2}' # expect a date like YYYY-MM-DD
os::cmd::expect_success_and_text 'oc get istag' 'wildfly'

# test image stream tag operations
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.generation}' '2'
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.tag.from.kind}' 'ImageStreamTag'
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.tag.from.name}' '10.0'
os::cmd::expect_success 'oc annotate istag/wildfly:latest foo=bar'
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.metadata.annotations.foo}' 'bar'
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.tag.annotations.foo}' 'bar'
os::cmd::expect_success 'oc annotate istag/wildfly:latest foo-'
os::cmd::expect_success_and_not_text 'oc get istag/wildfly:latest -o jsonpath={.metadata.annotations}' 'bar'
os::cmd::expect_success_and_not_text 'oc get istag/wildfly:latest -o jsonpath={.tag.annotations}' 'bar'
os::cmd::expect_success "oc patch istag/wildfly:latest -p='{\"tag\":{\"from\":{\"kind\":\"DockerImage\",\"name\":\"mysql:latest\"}}}'"
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.tag.from.kind}' 'DockerImage'
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.tag.from.name}' 'mysql:latest'
os::cmd::expect_success_and_not_text 'oc get istag/wildfly:latest -o jsonpath={.tag.generation}' '2'
os::cmd::expect_success "oc patch istag/wildfly:latest -p='{\"tag\":{\"importPolicy\":{\"scheduled\":true}}}'"
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.tag.importPolicy.scheduled}' 'true'

os::cmd::expect_success 'oc delete imageStreams ruby'
os::cmd::expect_success 'oc delete imageStreams nodejs'
os::cmd::expect_success 'oc delete imageStreams wildfly'
os::cmd::expect_success 'oc delete imageStreams postgresql'
os::cmd::expect_success 'oc delete imageStreams mongodb'
os::cmd::expect_failure 'oc get imageStreams ruby'
os::cmd::expect_failure 'oc get imageStreams nodejs'
os::cmd::expect_failure 'oc get imageStreams postgresql'
os::cmd::expect_failure 'oc get imageStreams mongodb'
os::cmd::expect_failure 'oc get imageStreams wildfly'
os::cmd::try_until_success 'oc get imagestreamTags mysql:5.5'
os::cmd::try_until_success 'oc get imagestreamTags mysql:5.6'
os::cmd::expect_success_and_text "oc get imagestreams mysql --template='{{ index .metadata.annotations \"openshift.io/image.dockerRepositoryCheck\"}}'" '[0-9]{4}\-[0-9]{2}\-[0-9]{2}' # expect a date like YYYY-MM-DD
os::cmd::expect_success 'oc describe istag/mysql:latest'
os::cmd::expect_success_and_text 'oc describe istag/mysql:latest' 'Environment:'
os::cmd::expect_success_and_text 'oc describe istag/mysql:latest' 'Image Created:'
os::cmd::expect_success_and_text 'oc describe istag/mysql:latest' 'Image Name:'
name=$(oc get istag/mysql:latest --template='{{ .image.metadata.name }}')
imagename="isimage/mysql@${name:0:15}"
os::cmd::expect_success "oc describe ${imagename}"
os::cmd::expect_success_and_text "oc describe ${imagename}" 'Environment:'
os::cmd::expect_success_and_text "oc describe ${imagename}" 'Image Created:'
os::cmd::expect_success_and_text "oc describe ${imagename}" 'Image Name:'
echo "imageStreams: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/images/import-image"
# should follow the latest reference to 5.6 and update that, and leave latest unchanged
os::cmd::expect_success_and_text "oc get is/mysql --template='{{(index .spec.tags 1).from.kind}}'" 'DockerImage'
os::cmd::expect_success_and_text "oc get is/mysql --template='{{(index .spec.tags 2).from.kind}}'" 'ImageStreamTag'
os::cmd::expect_success_and_text "oc get is/mysql --template='{{(index .spec.tags 2).from.name}}'" '5.6'
# import existing tag (implicit latest)
os::cmd::expect_success_and_text 'oc import-image mysql' 'sha256:'
os::cmd::expect_success_and_text "oc get is/mysql --template='{{(index .spec.tags 1).from.kind}}'" 'DockerImage'
os::cmd::expect_success_and_text "oc get is/mysql --template='{{(index .spec.tags 2).from.kind}}'" 'ImageStreamTag'
os::cmd::expect_success_and_text "oc get is/mysql --template='{{(index .spec.tags 2).from.name}}'" '5.6'
# should prevent changing source
os::cmd::expect_failure_and_text 'oc import-image mysql --from=docker.io/mysql' "use the 'tag' command if you want to change the source"
os::cmd::expect_success 'oc describe is/mysql'
# import existing tag (explicit)
os::cmd::expect_success_and_text 'oc import-image mysql:5.6' "sha256:"
os::cmd::expect_success_and_text 'oc import-image mysql:latest' "sha256:"
# import existing image stream creating new tag
os::cmd::expect_success_and_text 'oc import-image mysql:external --from=docker.io/mysql' "sha256:"
os::cmd::expect_success_and_text "oc get istag/mysql:external --template='{{.tag.from.kind}}'" 'DockerImage'
os::cmd::expect_success_and_text "oc get istag/mysql:external --template='{{.tag.from.name}}'" 'docker.io/mysql'
os::cmd::expect_success 'oc delete is/mysql'
# import creates new image stream with single tag
os::cmd::expect_failure_and_text 'oc import-image mysql:latest --from=docker.io/mysql:latest' '\-\-confirm'
os::cmd::expect_success_and_text 'oc import-image mysql:latest --from=docker.io/mysql:latest --confirm' 'sha256:'
os::cmd::expect_success_and_text "oc get is/mysql --template='{{(len .spec.tags)}}'" '1'
os::cmd::expect_success 'oc delete is/mysql'
# import creates new image stream with all tags
os::cmd::expect_failure_and_text 'oc import-image mysql --from=mysql --all' '\-\-confirm'
os::cmd::expect_success_and_text 'oc import-image mysql --from=mysql --all --confirm' 'sha256:'
name=$(oc get istag/mysql:latest --template='{{ .image.metadata.name }}')
echo "import-image: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/images/tag"
# oc tag
os::cmd::expect_success 'oc tag mysql:latest tagtest:tag1 --alias'
os::cmd::expect_success_and_text "oc get is/tagtest --template='{{(index .spec.tags 0).from.kind}}'" 'ImageStreamTag'

os::cmd::expect_success "oc tag mysql@${name} tagtest:tag2 --alias"
os::cmd::expect_success_and_text "oc get is/tagtest --template='{{(index .spec.tags 1).from.kind}}'" 'ImageStreamImage'

os::cmd::expect_success 'oc tag mysql:notfound tagtest:tag3 --alias'
os::cmd::expect_success_and_text "oc get is/tagtest --template='{{(index .spec.tags 2).from.kind}}'" 'ImageStreamTag'

os::cmd::expect_success 'oc tag --source=imagestreamtag mysql:latest tagtest:tag4 --alias'
os::cmd::expect_success_and_text "oc get is/tagtest --template='{{(index .spec.tags 3).from.kind}}'" 'ImageStreamTag'

os::cmd::expect_success 'oc tag --source=istag mysql:latest tagtest:tag5 --alias'
os::cmd::expect_success_and_text "oc get is/tagtest --template='{{(index .spec.tags 4).from.kind}}'" 'ImageStreamTag'

os::cmd::expect_success "oc tag --source=imagestreamimage mysql@${name} tagtest:tag6 --alias"
os::cmd::expect_success_and_text "oc get is/tagtest --template='{{(index .spec.tags 5).from.kind}}'" 'ImageStreamImage'

os::cmd::expect_success "oc tag --source=isimage mysql@${name} tagtest:tag7 --alias"
os::cmd::expect_success_and_text "oc get is/tagtest --template='{{(index .spec.tags 6).from.kind}}'" 'ImageStreamImage'

os::cmd::expect_success 'oc tag --source=docker mysql:latest tagtest:tag8 --alias'
os::cmd::expect_success_and_text "oc get is/tagtest --template='{{(index .spec.tags 7).from.kind}}'" 'DockerImage'

os::cmd::expect_success 'oc tag mysql:latest tagtest:zzz tagtest2:zzz --alias'
os::cmd::expect_success_and_text "oc get is/tagtest --template='{{(index .spec.tags 8).from.kind}}'" 'ImageStreamTag'
os::cmd::expect_success_and_text "oc get is/tagtest2 --template='{{(index .spec.tags 0).from.kind}}'" 'ImageStreamTag'

os::cmd::expect_success 'oc tag registry-1.docker.io/openshift/origin:v1.0.4 newrepo:latest'
os::cmd::expect_success_and_text "oc get is/newrepo --template='{{(index .spec.tags 0).from.kind}}'" 'DockerImage'
os::cmd::try_until_success 'oc get istag/mysql:5.5'
os::cmd::expect_success 'oc tag mysql:5.5 newrepo:latest'
os::cmd::expect_success_and_text "oc get is/newrepo --template='{{(index .spec.tags 0).from.kind}}'" 'ImageStreamImage'
os::cmd::expect_success 'oc tag mysql:5.5 newrepo:latest --alias'
os::cmd::expect_success_and_text "oc get is/newrepo --template='{{(index .spec.tags 0).from.kind}}'" 'ImageStreamTag'

# test scheduled and insecure tagging
os::cmd::expect_success 'oc tag --source=docker mysql:5.7 newrepo:latest --scheduled'
os::cmd::expect_success_and_text "oc get is/newrepo --template='{{(index .spec.tags 0).importPolicy.scheduled}}'" 'true'
os::cmd::expect_success_and_text "oc describe is/newrepo" '\* tag is scheduled'
os::cmd::expect_success 'oc tag --source=docker mysql:5.7 newrepo:latest --insecure'
os::cmd::expect_success_and_text "oc describe is/newrepo" '\! tag is insecure'
os::cmd::expect_success_and_not_text "oc describe is/newrepo" '\* tag is scheduled'
os::cmd::expect_success_and_text "oc get is/newrepo --template='{{(index .spec.tags 0).importPolicy.insecure}}'" 'true'

# test creating streams that don't exist
os::cmd::expect_failure_and_text 'oc get imageStreams tagtest3' 'not found'
os::cmd::expect_failure_and_text 'oc get imageStreams tagtest4' 'not found'
os::cmd::expect_success 'oc tag mysql:latest tagtest3:latest tagtest4:latest --alias'
os::cmd::expect_success_and_text "oc get is/tagtest3 --template='{{(index .spec.tags 0).from.kind}}'" 'ImageStreamTag'
os::cmd::expect_success_and_text "oc get is/tagtest4 --template='{{(index .spec.tags 0).from.kind}}'" 'ImageStreamTag'
os::cmd::expect_success 'oc delete is/tagtest is/tagtest2 is/tagtest3 is/tagtest4'
os::cmd::expect_success_and_text 'oc tag mysql:latest tagtest:new1 --alias' 'Tag tagtest:new1 set up to track mysql:latest.'
os::cmd::expect_success_and_text 'oc tag mysql:latest tagtest:new1' 'Tag tagtest:new1 set to mysql@sha256:'

# test deleting a spec tag using oc tag
os::cmd::expect_success 'oc create -f test/fixtures/test-stream.yaml'
os::cmd::expect_success_and_text 'oc tag test-stream:latest -d' 'Deleted'
os::cmd::expect_success 'oc delete is/test-stream'
echo "tag: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/images/delete-istag"
# test deleting a tag using oc delete
os::cmd::expect_success_and_text "oc get is perl --template '{{(index .spec.tags 0).name}}'" '5.16'
os::cmd::expect_success_and_text "oc get is perl --template '{{(index .status.tags 0).tag}}'" '5.16'
os::cmd::expect_success 'oc delete istag/perl:5.16'
os::cmd::expect_success_and_not_text 'oc get is/perl --template={{.spec.tags}}' 'version:5.16'
os::cmd::expect_success_and_not_text 'oc get is/perl --template={{.status.tags}}' 'version:5.16'
os::cmd::expect_success 'oc delete all --all'

echo "delete istag: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end