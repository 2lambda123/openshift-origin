package recreate

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	stratsupport "github.com/openshift/origin/pkg/deploy/strategy/support"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// RecreateDeploymentStrategy is a simple strategy appropriate as a default.
// Its behavior is to increase the replica count of the new deployment to 1,
// and to decrease the replica count of previous deployments to zero.
//
// A failure to disable any existing deployments will be considered a
// deployment failure.
type RecreateDeploymentStrategy struct {
	// client is used to interact with ReplicatonControllers.
	client replicationControllerClient
	// codec is used to decode DeploymentConfigs contained in deployments.
	codec runtime.Codec
	// hookExecutor can execute a lifecycle hook.
	hookExecutor hookExecutor

	retryTimeout time.Duration
	retryPeriod  time.Duration
}

// NewRecreateDeploymentStrategy makes a RecreateDeploymentStrategy backed by
// a real HookExecutor and client.
func NewRecreateDeploymentStrategy(client kclient.Interface, codec runtime.Codec) *RecreateDeploymentStrategy {
	return &RecreateDeploymentStrategy{
		client: &realReplicationControllerClient{client},
		codec:  codec,
		hookExecutor: &stratsupport.HookExecutor{
			PodClient: &stratsupport.HookExecutorPodClientImpl{
				CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
					return client.Pods(namespace).Create(pod)
				},
				WatchPodFunc: func(namespace, name string) (watch.Interface, error) {
					return newPodWatch(client, namespace, name, 5*time.Second), nil
				},
			},
		},
		retryTimeout: 10 * time.Second,
		retryPeriod:  1 * time.Second,
	}
}

// Deploy makes deployment active and disables oldDeployments.
func (s *RecreateDeploymentStrategy) Deploy(deployment *kapi.ReplicationController, lastDeployment *kapi.ObjectReference, oldDeployments []kapi.ObjectReference) error {
	var err error
	var deploymentConfig *deployapi.DeploymentConfig

	if deploymentConfig, err = deployutil.DecodeDeploymentConfig(deployment, s.codec); err != nil {
		return fmt.Errorf("Couldn't decode DeploymentConfig from deployment %s: %v", deployment.Name, err)
	}

	// Execute any pre-hook.
	if deploymentConfig.Template.Strategy.RecreateParams != nil {
		preHook := deploymentConfig.Template.Strategy.RecreateParams.Pre
		if preHook != nil {
		preHookLoop:
			for {
				err := s.hookExecutor.Execute(preHook, deployment)
				if err == nil {
					glog.Info("Pre hook finished successfully")
					break
				}
				switch preHook.FailurePolicy {
				case deployapi.LifecycleHookFailurePolicyAbort:
					return fmt.Errorf("Pre hook failed, aborting: %s", err)
				case deployapi.LifecycleHookFailurePolicyIgnore:
					glog.Infof("Pre hook failed, ignoring: %s", err)
					break preHookLoop
				case deployapi.LifecycleHookFailurePolicyRetry:
					glog.Infof("Pre hook failed, retrying: %s", err)
					time.Sleep(s.retryPeriod)
				}
			}
		}
	}

	// Scale up the new deployment. If a last deployment exists, preserve the
	// exiting replica count; otherwise, use what's defined on the template.
	desired := deploymentConfig.Template.ControllerTemplate.Replicas
	if lastDeployment != nil {
		if oldDeployment, err := s.client.getReplicationController(lastDeployment.Namespace, lastDeployment.Name); err != nil {
			glog.Errorf("Couldn't get deployment %s/%s: %v", lastDeployment.Namespace, lastDeployment.Name, err)
		} else {
			desired = oldDeployment.Spec.Replicas
			glog.Infof("Preserving replica count %d from last deployment %s/%s", desired, lastDeployment.Namespace, lastDeployment.Name)
		}
	}
	if err = s.updateReplicas(deployment.Namespace, deployment.Name, desired); err != nil {
		return err
	}

	// Disable any old deployments.
	glog.Infof("Found %d prior deployments to disable", len(oldDeployments))
	allProcessed := true
	for _, oldDeployment := range oldDeployments {
		if err = s.updateReplicas(oldDeployment.Namespace, oldDeployment.Name, 0); err != nil {
			glog.Errorf("%v", err)
			allProcessed = false
		}
	}

	// Execute any post-hook.
	if deploymentConfig.Template.Strategy.RecreateParams != nil {
		postHook := deploymentConfig.Template.Strategy.RecreateParams.Post
		if postHook != nil {
		postHookLoop:
			for {
				err := s.hookExecutor.Execute(postHook, deployment)
				if err == nil {
					glog.Info("Post hook finished successfully")
					break
				}
				switch postHook.FailurePolicy {
				case deployapi.LifecycleHookFailurePolicyIgnore, deployapi.LifecycleHookFailurePolicyAbort:
					// Abort isn't supported here, so treat it like ignore.
					glog.Infof("Post hook failed, ignoring: %s", err)
					break postHookLoop
				case deployapi.LifecycleHookFailurePolicyRetry:
					glog.Infof("Post hook failed, retrying: %s", err)
					time.Sleep(s.retryPeriod)
				}
			}
		}
	}

	if !allProcessed {
		return fmt.Errorf("Failed to disable all prior deployments for new deployment %s", deployment.Name)
	}

	glog.Infof("Deployment %s successfully made active", deployment.Name)
	return nil
}

// updateReplicas attempts to set the given deployment's replicaCount using retry logic.
func (s *RecreateDeploymentStrategy) updateReplicas(namespace, name string, replicaCount int) error {
	var err error
	var deployment *kapi.ReplicationController

	timeout := time.After(s.retryTimeout)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("Couldn't successfully update deployment %s/%s replica count to %d (timeout exceeded)", namespace, name, replicaCount)
		default:
			if deployment, err = s.client.getReplicationController(namespace, name); err != nil {
				glog.Errorf("Couldn't get deployment %s/%s: %v", namespace, name, err)
			} else {
				deployment.Spec.Replicas = replicaCount
				glog.Infof("Updating deployment %s/%s replica count to %d", namespace, name, replicaCount)
				if _, err = s.client.updateReplicationController(namespace, deployment); err == nil {
					return nil
				}
				// For conflict errors, retry immediately
				if kerrors.IsConflict(err) {
					continue
				}
				glog.Errorf("Error updating deployment %s/%s replica count to %d: %v", namespace, name, replicaCount, err)
			}

			time.Sleep(s.retryPeriod)
		}
	}
}

// replicationControllerClient provides access to ReplicationControllers.
type replicationControllerClient interface {
	getReplicationController(namespace, name string) (*kapi.ReplicationController, error)
	updateReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

// realReplicationControllerClient is a replicationControllerClient which uses
// a Kube client.
type realReplicationControllerClient struct {
	client kclient.Interface
}

func (r *realReplicationControllerClient) getReplicationController(namespace string, name string) (*kapi.ReplicationController, error) {
	return r.client.ReplicationControllers(namespace).Get(name)
}

func (r *realReplicationControllerClient) updateReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return r.client.ReplicationControllers(namespace).Update(ctrl)
}

// hookExecutor knows how to execute a deployment lifecycle hook.
type hookExecutor interface {
	Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error
}

// hookExecutorImpl is a pluggable hookExecutor.
type hookExecutorImpl struct {
	executeFunc func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error
}

func (i *hookExecutorImpl) Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController) error {
	return i.executeFunc(hook, deployment)
}

// podWatch provides watch semantics for a pod backed by a poller, since
// events aren't generated for pod status updates.
type podWatch struct {
	result chan watch.Event
	stop   chan bool
}

// newPodWatch makes a new podWatch.
func newPodWatch(client kclient.Interface, namespace, name string, period time.Duration) *podWatch {
	pods := make(chan watch.Event)
	stop := make(chan bool)
	tick := time.NewTicker(period)
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				pod, err := client.Pods(namespace).Get(name)
				if err != nil {
					pods <- watch.Event{
						Type: watch.Error,
						Object: &kapi.Status{
							Status:  "Failure",
							Message: fmt.Sprintf("couldn't get pod %s/%s: %s", namespace, name, err),
						},
					}
					continue
				}
				pods <- watch.Event{
					Type:   watch.Modified,
					Object: pod,
				}
			}
		}
	}()

	return &podWatch{
		result: pods,
		stop:   stop,
	}
}

func (w *podWatch) Stop() {
	w.stop <- true
}

func (w *podWatch) ResultChan() <-chan watch.Event {
	return w.result
}
