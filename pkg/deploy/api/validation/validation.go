package validation

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// TODO: These tests validate the ReplicationControllerState in a Deployment or DeploymentConfig.
//       The upstream validation API isn't factored currently to allow this; we'll make a PR to
//       upstream and fix when it goes in.

func ValidateDeployment(deployment *deployapi.Deployment) errors.ErrorList {
	result := validateDeploymentStrategy(deployment.Strategy).Prefix("strategy")
	controllerStateErrors := validation.ValidateReplicationControllerState(&deployment.ControllerTemplate)
	result = append(result, controllerStateErrors.Prefix("controllerTemplate")...)

	return result
}

func validateDeploymentStrategy(strategy *deployapi.DeploymentStrategy) errors.ErrorList {
	result := errors.ErrorList{}
	if strategy.CustomPod != nil {
		result = append(result, validateCustomPodStrategy(strategy.CustomPod).Prefix("customPod")...)
	}

	return result
}

func validateCustomPodStrategy(customPod *deployapi.CustomPodDeploymentStrategy) errors.ErrorList {
	result := errors.ErrorList{}

	if len(customPod.Image) == 0 {
		result = append(result, errors.NewFieldRequired("image", ""))
	}

	return result
}

func validateTrigger(trigger *deployapi.DeploymentTriggerPolicy) errors.ErrorList {
	result := errors.ErrorList{}

	if len(trigger.Type) == 0 {
		result = append(result, errors.NewFieldRequired("type", ""))
	}

	if trigger.Type == deployapi.DeploymentTriggerOnImageChange {
		if trigger.ImageChangeParams == nil {
			result = append(result, errors.NewFieldRequired("imageChangeParams", nil))
		} else {
			result = append(result, validateImageChangeParams(trigger.ImageChangeParams).Prefix("imageChangeParams")...)
		}
	}

	return result
}

func validateImageChangeParams(params *deployapi.DeploymentTriggerImageChangeParams) errors.ErrorList {
	result := errors.ErrorList{}

	if len(params.RepositoryName) == 0 {
		result = append(result, errors.NewFieldRequired("repositoryName", ""))
	}

	if len(params.ContainerNames) == 0 {
		result = append(result, errors.NewFieldRequired("containerNames", ""))
	}

	return result
}

func ValidateDeploymentConfig(config *deployapi.DeploymentConfig) errors.ErrorList {
	result := errors.ErrorList{}

	for i, _ := range config.Triggers {
		result = append(result, validateTrigger(&config.Triggers[i]).PrefixIndex(i).Prefix("triggers")...)
	}

	result = append(result, validateDeploymentStrategy(config.Template.Strategy).Prefix("template.strategy")...)
	controllerStateErrors := validation.ValidateReplicationControllerState(&config.Template.ControllerTemplate)
	result = append(result, controllerStateErrors.Prefix("template.controllerTemplate")...)

	return result
}
