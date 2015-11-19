package strategy

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

func TestDockerCreateBuildPod(t *testing.T) {
	strategy := DockerBuildStrategy{
		Image: "docker-test-image",
		Codec: latest.Codec,
	}

	expected := mockDockerBuild()
	actual, err := strategy.CreateBuildPod(expected)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if expected, actual := buildutil.GetBuildPodName(expected), actual.ObjectMeta.Name; expected != actual {
		t.Errorf("Expected %s, but got %s!", expected, actual)
	}
	if !reflect.DeepEqual(map[string]string{buildapi.BuildLabel: expected.Name}, actual.Labels) {
		t.Errorf("Pod Labels does not match Build Labels!")
	}
	container := actual.Spec.Containers[0]
	if container.Name != "docker-build" {
		t.Errorf("Expected docker-build, but got %s!", container.Name)
	}
	if container.Image != strategy.Image {
		t.Errorf("Expected %s image, got %s!", container.Image, strategy.Image)
	}
	if container.ImagePullPolicy != kapi.PullIfNotPresent {
		t.Errorf("Expected %v, got %v", kapi.PullIfNotPresent, container.ImagePullPolicy)
	}
	if actual.Spec.RestartPolicy != kapi.RestartPolicyNever {
		t.Errorf("Expected never, got %#v", actual.Spec.RestartPolicy)
	}
	if len(container.Env) != 8 {
		t.Fatalf("Expected 8 elements in Env table, got %d: %+v", len(container.Env), container.Env)
	}
	if len(container.VolumeMounts) != 4 {
		t.Fatalf("Expected 4 volumes in container, got %d", len(container.VolumeMounts))
	}
	if *actual.Spec.ActiveDeadlineSeconds != 60 {
		t.Errorf("Expected ActiveDeadlineSeconds 60, got %d", *actual.Spec.ActiveDeadlineSeconds)
	}
	for i, expected := range []string{dockerSocketPath, DockerPushSecretMountPath, DockerPullSecretMountPath, sourceSecretMountPath} {
		if container.VolumeMounts[i].MountPath != expected {
			t.Fatalf("Expected %s in VolumeMount[%d], got %s", expected, i, container.VolumeMounts[i].MountPath)
		}
	}
	if len(actual.Spec.Volumes) != 4 {
		t.Fatalf("Expected 4 volumes in Build pod, got %d", len(actual.Spec.Volumes))
	}
	if !kapi.Semantic.DeepEqual(container.Resources, expected.Spec.Resources) {
		t.Fatalf("Expected actual=expected, %v != %v", container.Resources, expected.Spec.Resources)
	}
	found := false
	foundIllegal := false
	for _, v := range container.Env {
		if v.Name == "BUILD_LOGLEVEL" && v.Value == "bar" {
			found = true
		}
		if v.Name == "ILLEGAL" {
			foundIllegal = true
		}
	}
	if !found {
		t.Fatalf("Expected variable BUILD_LOGLEVEL be defined for the container")
	}
	if foundIllegal {
		t.Fatalf("Found illegal environment variable 'ILLEGAL' defined on container")
	}

	buildJSON, _ := latest.Codec.Encode(expected)
	errorCases := map[int][]string{
		0: {"BUILD", string(buildJSON)},
	}
	for index, exp := range errorCases {
		if e := container.Env[index]; e.Name != exp[0] || e.Value != exp[1] {
			t.Errorf("Expected %s:%s, got %s:%s!\n", exp[0], exp[1], e.Name, e.Value)
		}
	}
}

func mockDockerBuild() *buildapi.Build {
	timeout := int64(60)
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "dockerBuild",
			Labels: map[string]string{
				"name": "dockerBuild",
			},
		},
		Spec: buildapi.BuildSpec{
			Revision: &buildapi.SourceRevision{
				Git: &buildapi.GitSourceRevision{},
			},
			Source: buildapi.BuildSource{
				Git: &buildapi.GitBuildSource{
					URI: "http://my.build.com/the/dockerbuild/Dockerfile",
					Ref: "master",
				},
				ContextDir:   "my/test/dir",
				SourceSecret: &kapi.LocalObjectReference{Name: "secretFoo"},
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					PullSecret: &kapi.LocalObjectReference{Name: "bar"},
					Env: []kapi.EnvVar{
						{Name: "ILLEGAL", Value: "foo"},
						{Name: "BUILD_LOGLEVEL", Value: "bar"},
					},
				},
			},
			Output: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "docker-registry/repository/dockerBuild",
				},
				PushSecret: &kapi.LocalObjectReference{Name: "foo"},
			},
			Resources: kapi.ResourceRequirements{
				Limits: kapi.ResourceList{
					kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
					kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("10G"),
				},
			},
			CompletionDeadlineSeconds: &timeout,
		},
		Status: buildapi.BuildStatus{
			Phase: buildapi.BuildPhaseNew,
		},
	}
}
