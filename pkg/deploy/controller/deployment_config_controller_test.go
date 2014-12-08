package controller

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

func TestHandleNewDeploymentConfig(t *testing.T) {
	controller := &DeploymentConfigController{
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(namespace, name string) (*deployapi.Deployment, error) {
				t.Fatalf("unexpected call with name %s", name)
				return nil, nil
			},
			CreateDeploymentFunc: func(namespace string, deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				t.Fatalf("unexpected call with deployment %v", deployment)
				return nil, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			deploymentConfig := manualDeploymentConfig()
			deploymentConfig.LatestVersion = 0
			return deploymentConfig
		},
	}

	controller.HandleDeploymentConfig()
}

func TestHandleInitialDeployment(t *testing.T) {
	deploymentConfig := manualDeploymentConfig()
	deploymentConfig.LatestVersion = 1

	var deployed *deployapi.Deployment

	controller := &DeploymentConfigController{
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(namespace, name string) (*deployapi.Deployment, error) {
				return nil, kerrors.NewNotFound("deployment", name)
			},
			CreateDeploymentFunc: func(namespace string, deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				deployed = deployment
				return deployment, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return deploymentConfig
		},
	}

	controller.HandleDeploymentConfig()

	if deployed == nil {
		t.Fatalf("expected a deployment")
	}

	if e, a := deploymentConfig.Name, deployed.Annotations[deployapi.DeploymentConfigAnnotation]; e != a {
		t.Fatalf("expected deployment with deploymentConfig annotation %s, got %s", e, a)
	}
}

func TestHandleConfigChangeNoPodTemplateDiff(t *testing.T) {
	controller := &DeploymentConfigController{
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(namespace, name string) (*deployapi.Deployment, error) {
				return matchingDeployment(), nil
			},
			CreateDeploymentFunc: func(namespace string, deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				t.Fatalf("unexpected call to to create deployment: %v", deployment)
				return nil, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			deploymentConfig := manualDeploymentConfig()
			deploymentConfig.LatestVersion = 0
			return deploymentConfig
		},
	}

	controller.HandleDeploymentConfig()
}

func TestHandleConfigChangeWithPodTemplateDiff(t *testing.T) {
	deploymentConfig := manualDeploymentConfig()
	deploymentConfig.LatestVersion = 2
	deploymentConfig.Template.ControllerTemplate.Template.Labels["foo"] = "bar"

	var deployed *deployapi.Deployment

	controller := &DeploymentConfigController{
		DeploymentInterface: &testDeploymentInterface{
			GetDeploymentFunc: func(namespace, name string) (*deployapi.Deployment, error) {
				return nil, kerrors.NewNotFound("deployment", name)
			},
			CreateDeploymentFunc: func(namespace string, deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
				deployed = deployment
				return deployment, nil
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return deploymentConfig
		},
	}

	controller.HandleDeploymentConfig()

	if deployed == nil {
		t.Fatalf("expected a deployment")
	}

	if e, a := deploymentConfig.Name, deployed.Annotations[deployapi.DeploymentConfigAnnotation]; e != a {
		t.Fatalf("expected deployment annotated with deploymentConfig %s, got %s", e, a)
	}
}

type testDeploymentInterface struct {
	GetDeploymentFunc    func(namespace, name string) (*deployapi.Deployment, error)
	CreateDeploymentFunc func(namespace string, deployment *deployapi.Deployment) (*deployapi.Deployment, error)
}

func (i *testDeploymentInterface) GetDeployment(namespace, name string) (*deployapi.Deployment, error) {
	return i.GetDeploymentFunc(namespace, name)
}

func (i *testDeploymentInterface) CreateDeployment(namespace string, deployment *deployapi.Deployment) (*deployapi.Deployment, error) {
	return i.CreateDeploymentFunc(namespace, deployment)
}

func manualDeploymentConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "manual-deploy-config"},
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerManual,
			},
		},
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
			},
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Replicas: 1,
				Selector: map[string]string{
					"name": "test-pod",
				},
				Template: &kapi.PodTemplateSpec{
					ObjectMeta: kapi.ObjectMeta{
						Labels: map[string]string{
							"name": "test-pod",
						},
					},
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name:  "container-1",
								Image: "registry:8080/openshift/test-image:ref-1",
							},
						},
					},
				},
			},
		},
	}
}

func matchingDeployment() *deployapi.Deployment {
	return &deployapi.Deployment{
		ObjectMeta: kapi.ObjectMeta{Name: "manual-deploy-config-1"},
		Status:     deployapi.DeploymentStatusNew,
		Strategy: deployapi.DeploymentStrategy{
			Type: deployapi.DeploymentStrategyTypeRecreate,
		},
		ControllerTemplate: kapi.ReplicationControllerSpec{
			Replicas: 1,
			Selector: map[string]string{
				"name": "test-pod",
			},
			Template: &kapi.PodTemplateSpec{
				ObjectMeta: kapi.ObjectMeta{
					Labels: map[string]string{
						"name": "test-pod",
					},
				},
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name:  "container-1",
							Image: "registry:8080/openshift/test-image:ref-1",
						},
					},
				},
			},
		},
	}
}
