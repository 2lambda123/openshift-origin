package deploymentconfig

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// TestHandle_initialOk ensures that an initial config (version 0) doesn't result
// in a new deployment.
func TestHandle_initialOk(t *testing.T) {
	controller := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call with deployment %v", deployment)
				return nil, nil
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				t.Fatalf("unexpected call to list deployments")
				return nil, nil
			},
		},
		recorder: &record.FakeRecorder{},
	}

	err := controller.Handle(deploytest.OkDeploymentConfig(0))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHandle_updateOk ensures that an updated config (version >0) results in
// a new deployment with the appropriate replica count based on a variety of
// existing prior deployments.
func TestHandle_updateOk(t *testing.T) {
	var (
		config              *deployapi.DeploymentConfig
		deployed            *kapi.ReplicationController
		existingDeployments *kapi.ReplicationControllerList
	)

	controller := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				deployed = deployment
				return deployment, nil
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return existingDeployments, nil
			},
		},
		recorder: &record.FakeRecorder{},
	}

	type existing struct {
		version  int
		replicas int
	}

	type scenario struct {
		version          int
		expectedReplicas int
		existing         []existing
	}

	scenarios := []scenario{
		// No existing deployments
		{1, 1, []existing{}},
		// A single existing deployment
		{2, 1, []existing{{1, 1}}},
		// An active and deactivated existing deployment
		{3, 2, []existing{{2, 2}, {1, 0}}},
		// An active and deactivated existing deployment with weird ordering
		{4, 3, []existing{{1, 0}, {2, 0}, {3, 3}}},
	}

	for _, scenario := range scenarios {
		deployed = nil
		config = deploytest.OkDeploymentConfig(scenario.version)
		existingDeployments = &kapi.ReplicationControllerList{}
		for _, e := range scenario.existing {
			d, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(e.version), api.Codec)
			d.Spec.Replicas = e.replicas
			d.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)
			existingDeployments.Items = append(existingDeployments.Items, *d)
		}
		err := controller.Handle(config)

		if deployed == nil {
			t.Fatalf("expected a deployment")
		}

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if e, a := strconv.Itoa(scenario.expectedReplicas), deployed.Annotations[deployapi.DesiredReplicasAnnotation]; e != a {
			t.Errorf("expected desired replicas %s, got %s", e, a)
		}
	}
}

// TestHandle_nonfatalLookupError ensures that an API failure to look up the
// existing deployment for an updated config results in a nonfatal error.
func TestHandle_nonfatalLookupError(t *testing.T) {
	configController := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call with deployment %v", deployment)
				return nil, nil
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return nil, kerrors.NewInternalError(fmt.Errorf("fatal test error"))
			},
		},
	}

	err := configController.Handle(deploytest.OkDeploymentConfig(1))
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, isFatal := err.(fatalError); isFatal {
		t.Fatalf("expected a retryable error, got a fatal error: %v", err)
	}
}

// TestHandle_configAlreadyDeployed ensures that an attempt to create a
// deployment for an updated config for which the deployment was already
// created results in a no-op.
func TestHandle_configAlreadyDeployed(t *testing.T) {
	deploymentConfig := deploytest.OkDeploymentConfig(0)

	controller := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call to to create deployment: %v", deployment)
				return nil, nil
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				existingDeployments := []kapi.ReplicationController{}
				deployment, _ := deployutil.MakeDeployment(deploymentConfig, kapi.Codec)
				existingDeployments = append(existingDeployments, *deployment)
				return &kapi.ReplicationControllerList{Items: existingDeployments}, nil
			},
		},
	}

	err := controller.Handle(deploymentConfig)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHandle_nonfatalCreateError ensures that a failed API attempt to create
// a new deployment for an updated config results in a nonfatal error.
func TestHandle_nonfatalCreateError(t *testing.T) {
	configController := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewInternalError(fmt.Errorf("test error"))
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return &kapi.ReplicationControllerList{}, nil
			},
		},
		recorder: &record.FakeRecorder{},
	}

	err := configController.Handle(deploytest.OkDeploymentConfig(1))
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, isFatal := err.(fatalError); isFatal {
		t.Fatalf("expected a nonfatal error, got a fatal error: %v", err)
	}
}

// TestHandle_fatalError ensures that in internal (not API) failure to make a
// deployment from an updated config results in a fatal error.
func TestHandle_fatalError(t *testing.T) {
	configController := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return nil, fmt.Errorf("couldn't make deployment")
		},
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call to create")
				return nil, kerrors.NewInternalError(fmt.Errorf("test error"))
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return &kapi.ReplicationControllerList{}, nil
			},
		},
	}

	err := configController.Handle(deploytest.OkDeploymentConfig(1))
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, isFatal := err.(fatalError); !isFatal {
		t.Fatalf("expected a fatal error, got: %v", err)
	}
}

// TestHandle_existingDeployments ensures that an attempt to create a
// new deployment for a config that has existing deployments succeeds of fails
// depending upon the state of the existing deployments
func TestHandle_existingDeployments(t *testing.T) {
	var (
		config              *deployapi.DeploymentConfig
		deployed            *kapi.ReplicationController
		existingDeployments *kapi.ReplicationControllerList
	)

	controller := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				deployed = deployment
				return deployment, nil
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return existingDeployments, nil
			},
		},
		recorder: &record.FakeRecorder{},
	}

	type existing struct {
		version int
		status  deployapi.DeploymentStatus
	}

	type scenario struct {
		version   int
		existing  []existing
		errorType reflect.Type
	}

	transientErrorType := reflect.TypeOf(transientError(""))
	scenarios := []scenario{
		// No existing deployments
		{1, []existing{}, nil},
		// A single existing completed deployment
		{2, []existing{{1, deployapi.DeploymentStatusComplete}}, nil},
		// A single existing failed deployment
		{2, []existing{{1, deployapi.DeploymentStatusFailed}}, nil},
		// Multiple existing completed/failed deployments
		{3, []existing{{2, deployapi.DeploymentStatusFailed}, {1, deployapi.DeploymentStatusComplete}}, nil},

		// A single existing deployment in the default state
		{2, []existing{{1, ""}}, transientErrorType},
		// A single existing new deployment
		{2, []existing{{1, deployapi.DeploymentStatusNew}}, transientErrorType},
		// A single existing pending deployment
		{2, []existing{{1, deployapi.DeploymentStatusPending}}, transientErrorType},
		// A single existing running deployment
		{2, []existing{{1, deployapi.DeploymentStatusRunning}}, transientErrorType},
		// Multiple existing deployments with one in new/pending/running
		{4, []existing{{3, deployapi.DeploymentStatusRunning}, {2, deployapi.DeploymentStatusComplete}, {1, deployapi.DeploymentStatusFailed}}, transientErrorType},
	}

	for _, scenario := range scenarios {
		deployed = nil
		config = deploytest.OkDeploymentConfig(scenario.version)
		existingDeployments = &kapi.ReplicationControllerList{}
		for _, e := range scenario.existing {
			d, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(e.version), api.Codec)
			if e.status != "" {
				d.Annotations[deployapi.DeploymentStatusAnnotation] = string(e.status)
			}
			existingDeployments.Items = append(existingDeployments.Items, *d)
		}
		err := controller.Handle(config)

		if scenario.errorType == nil {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if deployed == nil {
				t.Fatalf("expected a deployment")
			}
		} else {
			if err == nil {
				t.Fatalf("expected error")
			}
			if reflect.TypeOf(err) != scenario.errorType {
				t.Fatalf("error expected: %s, got: %s", scenario.errorType, reflect.TypeOf(err))
			}
		}
	}
}
