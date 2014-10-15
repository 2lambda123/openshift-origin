package deploy

import (
	"fmt"
	"strings"
	"testing"
	"time"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/registry/test"
)

func TestListDeploymentsError(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	mockRegistry.Err = fmt.Errorf("test error")

	storage := REST{
		registry: mockRegistry,
	}

	deployments, err := storage.List(nil, nil, nil)
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRegistry.Err, err)
	}

	if deployments != nil {
		t.Errorf("Unexpected non-nil deployments list: %#v", deployments)
	}
}

func TestListDeploymentsEmptyList(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	mockRegistry.Deployments = &api.DeploymentList{
		Items: []api.Deployment{},
	}

	storage := REST{
		registry: mockRegistry,
	}

	deployments, err := storage.List(nil, labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	if len(deployments.(*api.DeploymentList).Items) != 0 {
		t.Errorf("Unexpected non-zero deployments list: %#v", deployments)
	}
}

func TestListDeploymentsPopulatedList(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	mockRegistry.Deployments = &api.DeploymentList{
		Items: []api.Deployment{
			{
				JSONBase: kubeapi.JSONBase{
					ID: "foo",
				},
			},
			{
				JSONBase: kubeapi.JSONBase{
					ID: "bar",
				},
			},
		},
	}

	storage := REST{
		registry: mockRegistry,
	}

	list, err := storage.List(nil, labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	deployments := list.(*api.DeploymentList)

	if e, a := 2, len(deployments.Items); e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestCreateDeploymentBadObject(t *testing.T) {
	storage := REST{}

	channel, err := storage.Create(nil, &api.DeploymentList{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "not a deployment") == -1 {
		t.Errorf("Expected 'not a deployment' error, got '%v'", err.Error())
	}
}

func okStrategy() api.DeploymentStrategy {
	return api.DeploymentStrategy{
		Type:      "customPod",
		CustomPod: okCustomPod(),
	}
}

func okCustomPod() *api.CustomPodDeploymentStrategy {
	return &api.CustomPodDeploymentStrategy{
		Image: "openshift/kube-deploy",
	}
}

func okControllerTemplate() kubeapi.ReplicationControllerState {
	return kubeapi.ReplicationControllerState{
		ReplicaSelector: okSelector(),
		PodTemplate:     okPodTemplate(),
	}
}

func okSelector() map[string]string {
	return map[string]string{"a": "b"}
}

func okPodTemplate() kubeapi.PodTemplate {
	return kubeapi.PodTemplate{
		DesiredState: kubeapi.PodState{
			Manifest: kubeapi.ContainerManifest{
				Version: "v1beta1",
			},
		},
		Labels: okSelector(),
	}
}

func TestCreateRegistrySaveError(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	mockRegistry.Err = fmt.Errorf("test error")
	storage := REST{registry: mockRegistry}

	channel, err := storage.Create(nil, &api.Deployment{
		JSONBase:           kubeapi.JSONBase{ID: "foo"},
		Strategy:           okStrategy(),
		ControllerTemplate: okControllerTemplate(),
	})
	if channel == nil {
		t.Errorf("Expected nil channel, got %v", channel)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	select {
	case result := <-channel:
		status, ok := result.(*kubeapi.Status)
		if !ok {
			t.Errorf("Expected status type, got: %#v", result)
		}
		if status.Status != "failure" || status.Message != "foo" {
			t.Errorf("Expected failure status, got %#V", status)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
	default:
	}
}

func TestCreateDeploymentOK(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	storage := REST{registry: mockRegistry}

	channel, err := storage.Create(nil, &api.Deployment{
		JSONBase:           kubeapi.JSONBase{ID: "foo"},
		Strategy:           okStrategy(),
		ControllerTemplate: okControllerTemplate(),
	})
	if channel == nil {
		t.Errorf("Expected nil channel, got %v", channel)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	select {
	case result := <-channel:
		deployment, ok := result.(*api.Deployment)
		if !ok {
			t.Errorf("Expected deployment type, got: %#v", result)
		}
		if deployment.ID != "foo" {
			t.Errorf("Unexpected deployment: %#v", deployment)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
	default:
	}
}

func TestGetDeploymentError(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	mockRegistry.Err = fmt.Errorf("bad")
	storage := REST{registry: mockRegistry}

	deployment, err := storage.Get(nil, "foo")
	if deployment != nil {
		t.Errorf("Unexpected non-nil deployment: %#v", deployment)
	}
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, got %#v", mockRegistry.Err, err)
	}
}

func TestGetDeploymentOK(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	mockRegistry.Deployment = &api.Deployment{
		JSONBase: kubeapi.JSONBase{ID: "foo"},
	}
	storage := REST{registry: mockRegistry}

	deployment, err := storage.Get(nil, "foo")
	if deployment == nil {
		t.Error("Unexpected nil deployment")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error", err)
	}
	if deployment.(*api.Deployment).ID != "foo" {
		t.Errorf("Unexpected deployment: %#v", deployment)
	}
}

func TestUpdateDeploymentBadObject(t *testing.T) {
	storage := REST{}

	channel, err := storage.Update(nil, &api.DeploymentConfig{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "not a deployment:") == -1 {
		t.Errorf("Expected 'not a deployment' error, got %v", err)
	}
}

func TestUpdateDeploymentMissingID(t *testing.T) {
	storage := REST{}

	channel, err := storage.Update(nil, &api.Deployment{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "id is unspecified:") == -1 {
		t.Errorf("Expected 'id is unspecified' error, got %v", err)
	}
}

func TestUpdateRegistryErrorSaving(t *testing.T) {
	mockRepositoryRegistry := test.NewDeploymentRegistry()
	mockRepositoryRegistry.Err = fmt.Errorf("foo")
	storage := REST{registry: mockRepositoryRegistry}

	channel, err := storage.Update(nil, &api.Deployment{
		JSONBase: kubeapi.JSONBase{ID: "bar"},
	})
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	result := <-channel
	status, ok := result.(*kubeapi.Status)
	if !ok {
		t.Errorf("Expected status, got %#v", result)
	}
	if status.Status != kubeapi.StatusFailure || status.Message != "foo" {
		t.Errorf("Expected status=failure, message=foo, got %#v", status)
	}
}

func TestUpdateDeploymentOK(t *testing.T) {
	mockRepositoryRegistry := test.NewDeploymentRegistry()
	storage := REST{registry: mockRepositoryRegistry}

	channel, err := storage.Update(nil, &api.Deployment{
		JSONBase: kubeapi.JSONBase{ID: "bar"},
	})
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	result := <-channel
	repo, ok := result.(*api.Deployment)
	if !ok {
		t.Errorf("Expected Deployment, got %#v", result)
	}
	if repo.ID != "bar" {
		t.Errorf("Unexpected repo returned: %#v", repo)
	}
}

func TestDeleteDeployment(t *testing.T) {
	mockRegistry := test.NewDeploymentRegistry()
	storage := REST{registry: mockRegistry}
	channel, err := storage.Delete(nil, "foo")
	if channel == nil {
		t.Error("Unexpected nil channel")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	select {
	case result := <-channel:
		status, ok := result.(*kubeapi.Status)
		if !ok {
			t.Errorf("Expected status type, got: %#v", result)
		}
		if status.Status != kubeapi.StatusSuccess {
			t.Errorf("Expected status=success, got: %#v", status)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
	default:
	}
}
