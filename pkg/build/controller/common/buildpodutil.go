package common

import (
	"errors"
	"fmt"

	"github.com/openshift/origin/pkg/build/buildscheme"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

// GetBuildFromPod returns a build object encoded in a pod's BUILD environment variable along with
// its encoding version
func GetBuildFromPod(pod *v1.Pod) (*buildapi.Build, error) {
	if len(pod.Spec.Containers) == 0 {
		return nil, errors.New("unable to get build from pod: pod has no containers")
	}

	buildEnvVar := getEnvVar(&pod.Spec.Containers[0], "BUILD")
	if len(buildEnvVar) == 0 {
		return nil, errors.New("unable to get build from pod: BUILD environment variable is empty")
	}

	obj, err := runtime.Decode(buildscheme.Decoder, []byte(buildEnvVar))
	if err != nil {
		return nil, fmt.Errorf("unable to get build from pod: %v", err)
	}
	build, ok := obj.(*buildapi.Build)
	if !ok {
		return nil, fmt.Errorf("unable to get build from pod: %v", errors.New("decoded object is not of type Build"))
	}
	return build, nil
}

// SetBuildInPod encodes a build object and sets it in a pod's BUILD environment variable
func SetBuildInPod(pod *v1.Pod, build *buildapi.Build) error {
	encodedBuild, err := runtime.Encode(buildscheme.Encoder, build)
	if err != nil {
		return fmt.Errorf("unable to set build in pod: %v", err)
	}

	for i := range pod.Spec.Containers {
		setEnvVar(&pod.Spec.Containers[i], "BUILD", string(encodedBuild))
	}
	for i := range pod.Spec.InitContainers {
		setEnvVar(&pod.Spec.InitContainers[i], "BUILD", string(encodedBuild))
	}

	return nil
}

func getEnvVar(c *v1.Container, name string) string {
	for _, envVar := range c.Env {
		if envVar.Name == name {
			return envVar.Value
		}
	}

	return ""
}

func setEnvVar(c *v1.Container, name, value string) {
	for i, envVar := range c.Env {
		if envVar.Name == name {
			c.Env[i] = v1.EnvVar{Name: name, Value: value}
			return
		}
	}

	c.Env = append(c.Env, v1.EnvVar{Name: name, Value: value})
}
