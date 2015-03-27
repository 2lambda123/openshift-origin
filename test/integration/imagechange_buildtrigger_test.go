// +build integration,!no-etcd

package integration

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	watchapi "github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
)

func init() {
	testutil.RequireEtcd()
}

func TestSimpleImageChangeBuildTrigger(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	openshift := NewTestOpenshift(t)
	defer openshift.Close()

	imageRepo := &imageapi.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "test-image-trigger-repo"},
		DockerImageRepository: "registry:8080/openshift/test-image-trigger",
		Tags: map[string]string{
			"latest": "latest",
		},
	}

	config := imageChangeBuildConfig()

	created, err := openshift.Client.BuildConfigs(testutil.Namespace()).Create(config)
	if err != nil {
		t.Fatalf("Couldn't create BuildConfig: %v", err)
	}

	watch, err := openshift.Client.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), created.ResourceVersion)
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer watch.Stop()

	watch2, err := openshift.Client.BuildConfigs(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), created.ResourceVersion)
	if err != nil {
		t.Fatalf("Couldn't subscribe to BuildConfigs %v", err)
	}
	defer watch2.Stop()

	imageRepo, err = openshift.Client.ImageRepositories(testutil.Namespace()).Create(imageRepo)
	if err != nil {
		t.Fatalf("Couldn't create ImageRepository: %v", err)
	}

	// wait for initial build event from the creation of the imagerepo with tag latest
	event := <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild := event.Object.(*buildapi.Build)
	if newBuild.Parameters.Strategy.DockerStrategy.Image != "registry:8080/openshift/test-image-trigger:latest" {
		i, _ := openshift.Client.ImageRepositories(testutil.Namespace()).Get(imageRepo.Name)
		bc, _ := openshift.Client.BuildConfigs(testutil.Namespace()).Get(config.Name)
		t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", "registry:8080/openshift/test-image-trigger:latest", newBuild.Parameters.Strategy.DockerStrategy.Image, i, bc.Triggers[0].ImageChange)
	}
	event = <-watch.ResultChan()
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	if newBuild.Parameters.Output.DockerImageReference != "registry:8080/openshift/test-image-trigger:outputtag" {
		t.Fatalf("Expected build with output image %s, got %s", "registry:8080/openshift/test-image-trigger:outputtag", newBuild.Parameters.Output.DockerImageReference)
	}
	if newBuild.Labels["testlabel"] != "testvalue" {
		t.Fatalf("Expected build with label %s=%s from build config got %s=%s", "testlabel", "testvalue", "testlabel", newBuild.Labels["testlabel"])
	}

	// wait for build config to be updated
	<-watch2.ResultChan()
	updatedConfig, err := openshift.Client.BuildConfigs(testutil.Namespace()).Get(config.Name)
	if err != nil {
		t.Fatalf("Couldn't get BuildConfig: %v", err)
	}
	// the first tag did not have an image id, so the last trigger field is the pull spec
	if updatedConfig.Triggers[0].ImageChange.LastTriggeredImageID != "registry:8080/openshift/test-image-trigger:latest" {
		t.Errorf("Expected imageID equal to pull spec, got %#v", updatedConfig.Triggers[0].ImageChange)
	}

	// trigger a build by posting a new image
	if err := openshift.Client.ImageRepositoryMappings(testutil.Namespace()).Create(&imageapi.ImageRepositoryMapping{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: testutil.Namespace(),
			Name:      imageRepo.Name,
		},
		Tag: "latest",
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "ref-2-random",
			},
			DockerImageReference: "registry:8080/openshift/test-image-trigger:ref-2",
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	event = <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	if newBuild.Parameters.Strategy.DockerStrategy.Image != "registry:8080/openshift/test-image-trigger:ref-2" {
		i, _ := openshift.Client.ImageRepositories(testutil.Namespace()).Get(imageRepo.Name)
		bc, _ := openshift.Client.BuildConfigs(testutil.Namespace()).Get(config.Name)
		t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", "registry:8080/openshift/test-image-trigger:ref-2", newBuild.Parameters.Strategy.DockerStrategy.Image, i, bc.Triggers[0].ImageChange)
	}
	event = <-watch.ResultChan()
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	if newBuild.Parameters.Output.DockerImageReference != "registry:8080/openshift/test-image-trigger:outputtag" {
		t.Fatalf("Expected build with output image %s, got %s", "registry:8080/openshift/test-image-trigger:outputtag", newBuild.Parameters.Output.DockerImageReference)
	}
	if newBuild.Labels["testlabel"] != "testvalue" {
		t.Fatalf("Expected build with label %s=%s from build config got %s=%s", "testlabel", "testvalue", "testlabel", newBuild.Labels["testlabel"])
	}

	<-watch2.ResultChan()
	updatedConfig, err = openshift.Client.BuildConfigs(testutil.Namespace()).Get(config.Name)
	if err != nil {
		t.Fatalf("Couldn't get BuildConfig: %v", err)
	}
	if updatedConfig.Triggers[0].ImageChange.LastTriggeredImageID != "ref-2-random" {
		t.Errorf("unexpected trigger id: %#v", updatedConfig.Triggers[0].ImageChange)
	}
}

func TestSimpleImageChangeBuildTriggerFromRef(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	openshift := NewTestOpenshift(t)
	defer openshift.Close()

	imageRepo := &imageapi.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "test-image-trigger-repo"},
		DockerImageRepository: "registry:8080/openshift/test-image-trigger",
		Tags: map[string]string{
			"latest": "latest",
		},
	}

	config := imageChangeBuildConfigFromRef()

	created, err := openshift.Client.BuildConfigs(testutil.Namespace()).Create(config)
	if err != nil {
		t.Fatalf("Couldn't create BuildConfig: %v", err)
	}

	watch, err := openshift.Client.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), created.ResourceVersion)
	if err != nil {
		t.Fatalf("Couldn't subscribe to Builds %v", err)
	}
	defer watch.Stop()

	watch2, err := openshift.Client.BuildConfigs(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), created.ResourceVersion)
	if err != nil {
		t.Fatalf("Couldn't subscribe to BuildConfigs %v", err)
	}
	defer watch2.Stop()

	imageRepo, err = openshift.Client.ImageRepositories(testutil.Namespace()).Create(imageRepo)
	if err != nil {
		t.Fatalf("Couldn't create ImageRepository: %v", err)
	}

	// wait for initial build event from the creation of the imagerepo with tag latest
	event := <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild := event.Object.(*buildapi.Build)
	if newBuild.Parameters.Strategy.STIStrategy.Image != "registry:8080/openshift/test-image-trigger:latest" {
		i, _ := openshift.Client.ImageRepositories(testutil.Namespace()).Get(imageRepo.Name)
		bc, _ := openshift.Client.BuildConfigs(testutil.Namespace()).Get(config.Name)
		t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\ntrigger is %s\n", "registry:8080/openshift/test-image-trigger:latest", newBuild.Parameters.Strategy.STIStrategy.Image, i, bc.Triggers[0].ImageChange)
	}
	event = <-watch.ResultChan()
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	if newBuild.Parameters.Output.DockerImageReference != "registry:8080/openshift/test-image-trigger:outputtag" {
		t.Fatalf("Expected build with output image %s, got %s", "registry:8080/openshift/test-image-trigger:outputtag", newBuild.Parameters.Output.DockerImageReference)
	}
	if newBuild.Labels["testlabel"] != "testvalue" {
		t.Fatalf("Expected build with label %s=%s from build config got %s=%s", "testlabel", "testvalue", "testlabel", newBuild.Labels["testlabel"])
	}

	// wait for build config to be updated
	<-watch2.ResultChan()
	updatedConfig, err := openshift.Client.BuildConfigs(testutil.Namespace()).Get(config.Name)
	if err != nil {
		t.Fatalf("Couldn't get BuildConfig: %v", err)
	}
	// the first tag did not have an image id, so the last trigger field is the pull spec
	if updatedConfig.Triggers[0].ImageChange.LastTriggeredImageID != "registry:8080/openshift/test-image-trigger:latest" {
		t.Errorf("Expected imageID equal to pull spec, got %s", updatedConfig.Triggers[0].ImageChange)
	}

	// trigger a build by posting a new image
	if err := openshift.Client.ImageRepositoryMappings(testutil.Namespace()).Create(&imageapi.ImageRepositoryMapping{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: testutil.Namespace(),
			Name:      imageRepo.Name,
		},
		Tag: "latest",
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "ref-2-random",
			},
			DockerImageReference: "registry:8080/openshift/test-image-trigger:ref-2",
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	event = <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	if newBuild.Parameters.Strategy.STIStrategy.Image != "registry:8080/openshift/test-image-trigger:ref-2" {
		i, _ := openshift.Client.ImageRepositories(testutil.Namespace()).Get(imageRepo.Name)
		bc, _ := openshift.Client.BuildConfigs(testutil.Namespace()).Get(config.Name)
		t.Fatalf("Expected build with base image %s, got %s\n, imagerepo is %v\trigger is %s\n", "registry:8080/openshift/test-image-trigger:ref-2", newBuild.Parameters.Strategy.STIStrategy.Image, i, bc.Triggers[0].ImageChange)
	}
	event = <-watch.ResultChan()
	if e, a := watchapi.Modified, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newBuild = event.Object.(*buildapi.Build)
	if newBuild.Parameters.Output.DockerImageReference != "registry:8080/openshift/test-image-trigger:outputtag" {
		t.Fatalf("Expected build with output image %s, got %s", "registry:8080/openshift/test-image-trigger:outputtag", newBuild.Parameters.Output.DockerImageReference)
	}
	if newBuild.Labels["testlabel"] != "testvalue" {
		t.Fatalf("Expected build with label %s=%s from build config got %s=%s", "testlabel", "testvalue", "testlabel", newBuild.Labels["testlabel"])
	}

	// wait for build config to be updated
	<-watch2.ResultChan()
	updatedConfig, err = openshift.Client.BuildConfigs(testutil.Namespace()).Get(config.Name)
	if err != nil {
		t.Fatalf("Couldn't get BuildConfig: %v", err)
	}
	if updatedConfig.Triggers[0].ImageChange.LastTriggeredImageID != "ref-2-random" {
		t.Errorf("unexpected trigger id: %#v", updatedConfig.Triggers[0].ImageChange)
	}
}

func imageChangeBuildConfig() *buildapi.BuildConfig {
	buildcfg := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name:   "test-build-cfg",
			Labels: map[string]string{"testlabel": "testvalue"},
		},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: "Git",
				Git: &buildapi.GitBuildSource{
					URI: "git://github.com/openshift/ruby-hello-world.git",
				},
				ContextDir: "contextimage",
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					Image: "registry:8080/openshift/test-image-trigger",
				},
			},
			Output: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Name: "test-image-trigger-repo",
				},
				Tag: "outputtag",
			},
		},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					Image: "registry:8080/openshift/test-image-trigger",
					From: kapi.ObjectReference{
						Name: "test-image-trigger-repo",
					},
					Tag: "latest",
				},
			},
		},
	}
	return buildcfg
}

func imageChangeBuildConfigFromRef() *buildapi.BuildConfig {
	buildcfg := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name:   "test-build-cfg",
			Labels: map[string]string{"testlabel": "testvalue"},
		},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: "Git",
				Git: &buildapi.GitBuildSource{
					URI: "git://github.com/openshift/ruby-hello-world.git",
				},
				ContextDir: "contextimage",
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.STIBuildStrategyType,
				STIStrategy: &buildapi.STIBuildStrategy{
					From: &kapi.ObjectReference{
						Name: "test-image-trigger-repo",
					},
				},
			},
			Output: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Name: "test-image-trigger-repo",
				},
				Tag: "outputtag",
			},
		},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					Image: "registry:8080/openshift/test-image-trigger",
					From: kapi.ObjectReference{
						Name: "test-image-trigger-repo",
					},
					Tag: "latest",
				},
			},
		},
	}
	return buildcfg
}
