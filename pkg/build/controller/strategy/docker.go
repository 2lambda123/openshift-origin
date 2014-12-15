package strategy

import (
	"encoding/json"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// DockerBuildStrategy creates a Docker build using a Docker builder image.
type DockerBuildStrategy struct {
	Image          string
	UseLocalImages bool
}

// CreateBuildPod creates the pod to be used for the Docker build
// TODO: Make the Pod definition configurable
func (bs *DockerBuildStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	buildJSON, err := json.Marshal(build)
	if err != nil {
		return nil, err
	}

	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: build.PodName,
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:  "docker-build",
					Image: bs.Image,
					Env: []kapi.EnvVar{
						{Name: "BUILD", Value: string(buildJSON)},
					},
				},
			},
			RestartPolicy: kapi.RestartPolicy{
				Never: &kapi.RestartPolicyNever{},
			},
		},
	}

	if bs.UseLocalImages {
		pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent
	}

	setupDockerSocket(pod)
	setupDockerConfig(pod)
	return pod, nil
}
