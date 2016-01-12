package strategy

import (
	"fmt"
	"io/ioutil"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

// SourceBuildStrategy creates STI(source to image) builds
type SourceBuildStrategy struct {
	Image                string
	TempDirectoryCreator TempDirectoryCreator
	// Codec is the codec to use for encoding the output pod.
	// IMPORTANT: This may break backwards compatibility when
	// it changes.
	Codec            runtime.Codec
	AdmissionControl admission.Interface
}

type TempDirectoryCreator interface {
	CreateTempDirectory() (string, error)
}

type tempDirectoryCreator struct{}

func (tc *tempDirectoryCreator) CreateTempDirectory() (string, error) {
	return ioutil.TempDir("", "stibuild")
}

var STITempDirectoryCreator = &tempDirectoryCreator{}

// CreateBuildPod creates a pod that will execute the STI build
// TODO: Make the Pod definition configurable
func (bs *SourceBuildStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	data, err := bs.Codec.Encode(build)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the Build %s/%s: %v", build.Namespace, build.Name, err)
	}

	containerEnv := []kapi.EnvVar{
		{Name: "BUILD", Value: string(data)},
		{Name: "BUILD_LOGLEVEL", Value: fmt.Sprintf("%d", cmdutil.GetLogLevel())},
	}

	addSourceEnvVars(build.Spec.Source, &containerEnv)

	strategy := build.Spec.Strategy.SourceStrategy
	if len(strategy.Env) > 0 {
		mergeTrustedEnvWithoutDuplicates(strategy.Env, &containerEnv)
	}

	// check if can run container as root
	if !bs.canRunAsRoot(build) {
		containerEnv = append(containerEnv, kapi.EnvVar{Name: "ALLOWED_UIDS", Value: "1-"})
	}

	privileged := true
	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name:      buildutil.GetBuildPodName(build),
			Namespace: build.Namespace,
			Labels:    getPodLabels(build),
		},
		Spec: kapi.PodSpec{
			ServiceAccountName: build.Spec.ServiceAccount,
			Containers: []kapi.Container{
				{
					Name:  "sti-build",
					Image: bs.Image,
					Env:   containerEnv,
					// TODO: run unprivileged https://github.com/openshift/origin/issues/662
					SecurityContext: &kapi.SecurityContext{
						Privileged: &privileged,
					},
					Args: []string{"--loglevel=" + getContainerVerbosity(containerEnv)},
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}
	pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent
	pod.Spec.Containers[0].Resources = build.Spec.Resources

	if build.Spec.CompletionDeadlineSeconds != nil {
		pod.Spec.ActiveDeadlineSeconds = build.Spec.CompletionDeadlineSeconds
	}
	if build.Spec.Source.Binary != nil {
		pod.Spec.Containers[0].Stdin = true
		pod.Spec.Containers[0].StdinOnce = true
	}

	setupDockerSocket(pod)
	var sourceImageSecret *kapi.LocalObjectReference
	if build.Spec.Source.Image != nil {
		sourceImageSecret = build.Spec.Source.Image.PullSecret
	}
	setupDockerSecrets(pod, build.Spec.Output.PushSecret, strategy.PullSecret, sourceImageSecret)
	setupSourceSecrets(pod, build.Spec.Source.SourceSecret)
	return pod, nil
}

func (bs *SourceBuildStrategy) canRunAsRoot(build *buildapi.Build) bool {
	var rootUser int64
	rootUser = 0
	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name:      buildutil.GetBuildPodName(build),
			Namespace: build.Namespace,
		},
		Spec: kapi.PodSpec{
			ServiceAccountName: build.Spec.ServiceAccount,
			Containers: []kapi.Container{
				{
					Name:  "sti-build",
					Image: bs.Image,
					SecurityContext: &kapi.SecurityContext{
						RunAsUser: &rootUser,
					},
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}
	userInfo := serviceaccount.UserInfo(build.Namespace, build.Spec.ServiceAccount, "")
	attrs := admission.NewAttributesRecord(pod, "Pod", pod.Namespace, pod.Name, "pods", "", admission.Create, userInfo)
	err := bs.AdmissionControl.Admit(attrs)
	if err != nil {
		glog.V(2).Infof("Admit for root user returned error: %v", err)
	}
	return err == nil
}
