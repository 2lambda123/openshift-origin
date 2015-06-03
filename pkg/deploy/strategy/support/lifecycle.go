package support

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	namer "github.com/openshift/origin/pkg/util/namer"
)

// HookExecutor executes a deployment lifecycle hook.
type HookExecutor struct {
	// PodClient provides access to pods.
	PodClient HookExecutorPodClient
}

// Execute executes hook in the context of deployment. The label is used to
// distinguish the kind of hook (e.g. pre, post).
func (e *HookExecutor) Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
	var err error
	switch {
	case hook.ExecNewPod != nil:
		err = e.executeExecNewPod(hook, deployment, label)
	}

	if err == nil {
		return nil
	}

	// Retry failures are treated the same as Abort.
	switch hook.FailurePolicy {
	case deployapi.LifecycleHookFailurePolicyAbort, deployapi.LifecycleHookFailurePolicyRetry:
		return fmt.Errorf("Hook failed, aborting: %s", err)
	case deployapi.LifecycleHookFailurePolicyIgnore:
		glog.Infof("Hook failed, ignoring: %s", err)
		return nil
	default:
		return err
	}
}

// executeExecNewPod executes a ExecNewPod hook by creating a new pod based on
// the hook parameters and deployment. The pod is then synchronously watched
// until the pod completes, and if the pod failed, an error is returned.
//
// The hook pod inherits the following from the container the hook refers to:
//
//   * Environment (hook keys take precedence)
//   * Working directory
//   * Resources
func (e *HookExecutor) executeExecNewPod(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
	// Build a pod spec from the hook config and deployment
	podSpec, err := makeHookPod(hook, deployment, label)
	if err != nil {
		return err
	}

	// Try to create the pod.
	pod, err := e.PodClient.CreatePod(deployment.Namespace, podSpec)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return fmt.Errorf("couldn't create lifecycle pod for %s: %v", deployutil.LabelForDeployment(deployment), err)
		}
	} else {
		glog.V(0).Infof("Created lifecycle pod %s for deployment %s", pod.Name, deployutil.LabelForDeployment(deployment))
	}

	// Wait for the pod to finish.
	nextPod := e.PodClient.PodWatch(pod.Namespace, pod.Name, pod.ResourceVersion)
	glog.V(0).Infof("Waiting for hook pod %s/%s to complete", pod.Namespace, pod.Name)
	for {
		pod := nextPod()
		switch pod.Status.Phase {
		case kapi.PodSucceeded:
			return nil
		case kapi.PodFailed:
			return fmt.Errorf(pod.Status.Message)
		}
	}
}

// makeHookPod makes a pod spec from a hook and deployment.
func makeHookPod(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) (*kapi.Pod, error) {
	exec := hook.ExecNewPod
	var baseContainer *kapi.Container
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == exec.ContainerName {
			baseContainer = &container
			break
		}
	}
	if baseContainer == nil {
		return nil, fmt.Errorf("no container named '%s' found in deployment template", exec.ContainerName)
	}

	// Build a merged environment; hook environment takes precedence over base
	// container environment
	envMap := map[string]string{}
	mergedEnv := []kapi.EnvVar{}
	for _, env := range baseContainer.Env {
		envMap[env.Name] = env.Value
	}
	for _, env := range exec.Env {
		envMap[env.Name] = env.Value
	}
	for k, v := range envMap {
		mergedEnv = append(mergedEnv, kapi.EnvVar{Name: k, Value: v})
	}

	// Inherit resources from the base container
	resources := kapi.ResourceRequirements{}
	if err := kapi.Scheme.Convert(&baseContainer.Resources, &resources); err != nil {
		return nil, fmt.Errorf("couldn't clone ResourceRequirements: %v", err)
	}

	// Assigning to a variable since its address is required
	maxDeploymentDurationSeconds := deployapi.MaxDeploymentDurationSeconds

	// Let the kubelet manage retries if requested
	restartPolicy := kapi.RestartPolicyNever
	if hook.FailurePolicy == deployapi.LifecycleHookFailurePolicyRetry {
		restartPolicy = kapi.RestartPolicyOnFailure
	}

	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: namer.GetPodName(deployment.Name, label),
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: deployment.Name,
			},
			Labels: map[string]string{
				deployapi.DeployerPodForDeploymentLabel: deployment.Name,
			},
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:       "lifecycle",
					Image:      baseContainer.Image,
					Command:    exec.Command,
					WorkingDir: baseContainer.WorkingDir,
					Env:        mergedEnv,
					Resources:  resources,
				},
			},
			ActiveDeadlineSeconds: &maxDeploymentDurationSeconds,
			RestartPolicy:         restartPolicy,
		},
	}

	return pod, nil
}

// HookExecutorPodClient abstracts access to pods.
type HookExecutorPodClient interface {
	CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	PodWatch(namespace, name, resourceVersion string) func() *kapi.Pod
}

// HookExecutorPodClientImpl is a pluggable HookExecutorPodClient.
type HookExecutorPodClientImpl struct {
	CreatePodFunc func(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	PodWatchFunc  func(namespace, name, resourceVersion string) func() *kapi.Pod
}

func (i *HookExecutorPodClientImpl) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return i.CreatePodFunc(namespace, pod)
}

func (i *HookExecutorPodClientImpl) PodWatch(namespace, name, resourceVersion string) func() *kapi.Pod {
	return i.PodWatchFunc(namespace, name, resourceVersion)
}

// NewPodWatch creates a pod watching function which is backed by a
// FIFO/reflector pair. This avoids managing watches directly.
func NewPodWatch(client kclient.Interface, namespace, name, resourceVersion string) func() *kapi.Pod {
	fieldSelector, _ := fields.ParseSelector("metadata.name=" + name)
	podLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return client.Pods(namespace).List(labels.Everything(), fieldSelector)
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return client.Pods(namespace).Watch(labels.Everything(), fieldSelector, resourceVersion)
		},
	}
	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(podLW, &kapi.Pod{}, queue, 1*time.Minute).Run()

	return func() *kapi.Pod {
		obj := queue.Pop()
		return obj.(*kapi.Pod)
	}
}
