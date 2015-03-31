package strategy

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

func TestSetupDockerSocketHostSocket(t *testing.T) {
	pod := kapi.Pod{
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{},
			},
		},
	}

	setupDockerSocket(&pod)

	if len(pod.Spec.Volumes) != 1 {
		t.Fatalf("Expected 1 volume, got: %#v", pod.Spec.Volumes)
	}
	volume := pod.Spec.Volumes[0]
	if e, a := "docker-socket", volume.Name; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if volume.Name == "" {
		t.Fatalf("Unexpected empty volume source name")
	}
	if isVolumeSourceEmpty(volume.VolumeSource) {
		t.Fatalf("Unexpected nil volume source")
	}
	if volume.HostPath == nil {
		t.Fatalf("Unexpected nil host directory")
	}
	if volume.EmptyDir != nil {
		t.Errorf("Unexpected non-nil empty directory: %#v", volume.EmptyDir)
	}
	if e, a := "/var/run/docker.sock", volume.HostPath.Path; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}

	if len(pod.Spec.Containers[0].VolumeMounts) != 1 {
		t.Fatalf("Expected 1 volume mount, got: %#v", pod.Spec.Containers[0].VolumeMounts)
	}
	mount := pod.Spec.Containers[0].VolumeMounts[0]
	if e, a := "docker-socket", mount.Name; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if e, a := "/var/run/docker.sock", mount.MountPath; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if pod.Spec.Containers[0].Privileged {
		t.Error("Expected privileged to be false")
	}
}

func isVolumeSourceEmpty(volumeSource kapi.VolumeSource) bool {

	if volumeSource.EmptyDir == nil &&
		volumeSource.HostPath == nil &&
		volumeSource.GCEPersistentDisk == nil &&
		volumeSource.GitRepo == nil {
		return true
	}

	return false
}

func TestSetupBuildEnvFails(t *testing.T) {
	build := mockCustomBuild()
	containerEnv := []kapi.EnvVar{
		{Name: "BUILD", Value: ""},
		{Name: "SOURCE_REPOSITORY", Value: build.Parameters.Source.Git.URI},
	}
	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: buildutil.GetBuildPodName(build),
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:  "custom-build",
					Image: build.Parameters.Strategy.CustomStrategy.Image,
					Env:   containerEnv,
					// TODO: run unprivileged https://github.com/openshift/origin/issues/662
					Privileged: true,
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}
	if err := setupBuildEnv(build, pod); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	build.Parameters.Output.DockerImageReference = ""
	if err := setupBuildEnv(build, pod); err == nil {
		t.Errorf("unexpected non-error: %v", err)
	}
}

func TestMergeEnvWithoutDuplicates(t *testing.T) {
	input := []kapi.EnvVar{
		{Name: "foo", Value: "bar"},
		{Name: "input", Value: "inputVal"},
	}
	output := []kapi.EnvVar{
		{Name: "foo", Value: "test"},
	}

	mergeEnvWithoutDuplicates(input, &output)

	if len(output) != 2 {
		t.Errorf("Expected output to contain input items len!=2 (%d)", len(output))
	}

	if output[0].Name != "foo" {
		t.Errorf("Expected output to have env 'foo', got %+v", output[0])
	}
	if output[0].Value != "bar" {
		t.Errorf("Expected output env 'foo' to have value 'bar', got %+v", output[0])
	}
}
