package controller

import (
	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// CustomPodDeploymentController implements the DeploymentStrategyTypeCustomPod deployment strategy.
// Its behavior is to delegate the deployment logic to a pod. The status of the resulting Deployment
// will follow the status of the corresponding pod.
type CustomPodDeploymentController struct {
	DeploymentInterface dcDeploymentInterface
	PodInterface        dcPodInterface
	Environment         []kapi.EnvVar
	NextDeployment      func() *deployapi.Deployment
	NextPod             func() *kapi.Pod
	DeploymentStore     cache.Store
}

type dcDeploymentInterface interface {
	UpdateDeployment(ctx kapi.Context, deployment *deployapi.Deployment) (*deployapi.Deployment, error)
}

type dcPodInterface interface {
	CreatePod(ctx kapi.Context, pod *kapi.Pod) (*kapi.Pod, error)
	DeletePod(ctx kapi.Context, id string) error
}

// Run begins watching and synchronizing deployment states.
func (dc *CustomPodDeploymentController) Run() {
	go util.Forever(func() { dc.HandleDeployment() }, 0)
	go util.Forever(func() { dc.HandlePod() }, 0)
}

// Invokes the appropriate handler for the current state of the given deployment.
func (dc *CustomPodDeploymentController) HandleDeployment() error {
	deployment := dc.NextDeployment()

	if deployment.Strategy == nil || deployment.Strategy.CustomPod == nil {
		glog.V(4).Infof("Dropping deployment %s due to incompatible strategy type %s", deployment.ID, deployment.Strategy)
		return nil
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), deployment.Namespace)
	glog.V(4).Infof("Synchronizing deployment id: %v status: %v resourceVersion: %v",
		deployment.ID, deployment.Status, deployment.ResourceVersion)

	if deployment.Status != deployapi.DeploymentStatusNew {
		glog.V(4).Infof("Dropping deployment %v", deployment.ID)
		return nil
	}

	deploymentPod := dc.makeDeploymentPod(deployment)
	glog.V(2).Infof("Attempting to create deployment pod: %+v", deploymentPod)
	if pod, err := dc.PodInterface.CreatePod(kapi.NewContext(), deploymentPod); err != nil {
		glog.V(2).Infof("Received error creating pod: %v", err)
		deployment.Status = deployapi.DeploymentStatusFailed
	} else {
		glog.V(4).Infof("Successfully created pod %+v", pod)
		deployment.Status = deployapi.DeploymentStatusPending
	}

	return dc.saveDeployment(ctx, deployment)
}

func (dc *CustomPodDeploymentController) HandlePod() error {
	pod := dc.NextPod()
	ctx := kapi.WithNamespace(kapi.NewContext(), pod.Namespace)
	glog.V(2).Infof("Synchronizing pod id: %v status: %v", pod.ID, pod.CurrentState.Status)

	// assumption: filter prevents this label from not being present
	id := pod.Labels["deployment"]
	obj, exists := dc.DeploymentStore.Get(id)
	if !exists {
		return kerrors.NewNotFound("Deployment", id)
	}
	deployment := obj.(*deployapi.Deployment)

	if deployment.Status == deployapi.DeploymentStatusComplete || deployment.Status == deployapi.DeploymentStatusFailed {
		return nil
	}
	currentDeploymentStatus := deployment.Status

	switch pod.CurrentState.Status {
	case kapi.PodRunning:
		deployment.Status = deployapi.DeploymentStatusRunning
	case kapi.PodTerminated:
		deployment.Status = dc.inspectTerminatedDeploymentPod(deployment, pod)
	}

	if currentDeploymentStatus != deployment.Status {
		return dc.saveDeployment(ctx, deployment)
	}

	return nil
}

func deploymentPodID(deployment *deployapi.Deployment) string {
	return "deploy-" + deployment.ID
}

func (dc *CustomPodDeploymentController) inspectTerminatedDeploymentPod(deployment *deployapi.Deployment, pod *kapi.Pod) deployapi.DeploymentStatus {
	nextStatus := deployment.Status
	if pod.CurrentState.Status != kapi.PodTerminated {
		glog.V(2).Infof("The deployment has not yet finished. Pod status is %s. Continuing", pod.CurrentState.Status)
		return nextStatus
	}

	nextStatus = deployapi.DeploymentStatusComplete
	for _, info := range pod.CurrentState.Info {
		if info.State.Termination != nil && info.State.Termination.ExitCode != 0 {
			nextStatus = deployapi.DeploymentStatusFailed
		}
	}

	if nextStatus == deployapi.DeploymentStatusComplete {
		podID := deploymentPodID(deployment)
		glog.V(2).Infof("Removing deployment pod for ID %v", podID)
		dc.PodInterface.DeletePod(kapi.NewContext(), podID)
	}

	glog.V(4).Infof("The deployment pod has finished. Setting deployment state to %s", deployment.Status)
	return nextStatus
}

func (dc *CustomPodDeploymentController) saveDeployment(ctx kapi.Context, deployment *deployapi.Deployment) error {
	glog.V(4).Infof("Saving deployment %v status: %v", deployment.ID, deployment.Status)
	_, err := dc.DeploymentInterface.UpdateDeployment(ctx, deployment)
	if err != nil {
		glog.V(2).Infof("Received error while saving deployment %v: %v", deployment.ID, err)
	}
	return err
}

func (dc *CustomPodDeploymentController) makeDeploymentPod(deployment *deployapi.Deployment) *kapi.Pod {
	podID := deploymentPodID(deployment)

	envVars := deployment.Strategy.CustomPod.Environment
	envVars = append(envVars, kapi.EnvVar{Name: "KUBERNETES_DEPLOYMENT_ID", Value: deployment.ID})
	for _, env := range dc.Environment {
		envVars = append(envVars, env)
	}

	return &kapi.Pod{
		TypeMeta: kapi.TypeMeta{
			ID: podID,
		},
		DesiredState: kapi.PodState{
			Manifest: kapi.ContainerManifest{
				Version: "v1beta1",
				Containers: []kapi.Container{
					{
						Name:  "deployment",
						Image: deployment.Strategy.CustomPod.Image,
						Env:   envVars,
					},
				},
				RestartPolicy: kapi.RestartPolicy{
					Never: &kapi.RestartPolicyNever{},
				},
			},
		},
		Labels: map[string]string{
			"deployment": deployment.ID,
		},
	}
}
