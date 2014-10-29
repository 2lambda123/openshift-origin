package v1beta1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// A deployment represents a single configuration of a pod deployed into the cluster, and may
// represent both a current deployment or a historical deployment.
type Deployment struct {
	api.TypeMeta       `json:",inline" yaml:",inline"`
	Labels             map[string]string              `json:"labels,omitempty" yaml:"labels,omitempty"`
	Strategy           *DeploymentStrategy            `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	ControllerTemplate api.ReplicationControllerState `json:"controllerTemplate,omitempty" yaml:"controllerTemplate,omitempty"`
	Status             DeploymentStatus               `json:"status,omitempty" yaml:"status,omitempty"`
}

// A DeploymentList is a collection of deployments.
type DeploymentList struct {
	api.TypeMeta `json:",inline" yaml:",inline"`
	Items        []Deployment `json:"items,omitempty" yaml:"items,omitempty"`
}

// DeploymentStatus decribes the possible states a Deployment can be in.
type DeploymentStatus string

const (
	// DeploymentStatusNew means the deployment has been accepted but not yet acted upon.
	DeploymentStatusNew DeploymentStatus = "New"
	// DeploymentStatusPending means the deployment been handed over to a deployment strategy,
	// but the strategy has not yet declared the deployment to be running.
	DeploymentStatusPending DeploymentStatus = "Pending"
	// DeploymentStatusRunning means the deployment strategy has reported the deployment as
	// being in-progress.
	DeploymentStatusRunning DeploymentStatus = "Running"
	// DeploymentStatusComplete means the deployment finished without an error.
	DeploymentStatusComplete DeploymentStatus = "Complete"
	// DeploymentStatusFailed means the deployment finished with an error.
	DeploymentStatusFailed DeploymentStatus = "Failed"
)

// DeploymentConfigLabel is the key of a Deployment label whose value is the ID of a DeploymentConfig
// on which the Deployment is based.
const DeploymentConfigLabel = "deploymentConfig"

// DeploymentStrategy describes how to perform a deployment.
type DeploymentStrategy struct {
	// CustomPod represents the parameters for the CustomPod strategy.
	CustomPod *CustomPodDeploymentStrategy `json:"customPod,omitempty" yaml:"customPod,omitempty"`
}

// CustomPodDeploymentStrategy represents parameters for the CustomPod strategy.
type CustomPodDeploymentStrategy struct {
	// Image specifies a Docker image which can carry out a deployment.
	Image string `json:"image,omitempty" yaml:"image,omitempty"`
	// Environment holds the environment which will be given to the container for Image.
	Environment []api.EnvVar `json:"environment,omitempty" yaml:"environment,omitempty"`
}

// DeploymentConfig represents a configuration for a single deployment of a replication controller:
// what the template is for the deployment, how new deployments are triggered, what the desired
// deployment state is.
type DeploymentConfig struct {
	api.TypeMeta `json:",inline" yaml:",inline"`
	Labels       map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	// Triggers determine how updates to a DeploymentConfig result in new deployments. If no triggers
	// are defined, a new deployment can only occur as a result of an explicit client update to the
	// DeploymentConfig with a new LatestVersion.
	Triggers []DeploymentTriggerPolicy `json:"triggers,omitempty" yaml:"triggers,omitempty"`
	// Template represents a desired deployment state and how to deploy it.
	Template DeploymentTemplate `json:"template,omitempty" yaml:"template,omitempty"`
	// LatestVersion is used to determine whether the current deployment associated with a DeploymentConfig
	// is out of sync.
	LatestVersion int `json:"latestVersion,omitempty" yaml:"latestVersion,omitempty"`
}

// A DeploymentConfigList is a collection of deployment configs.
type DeploymentConfigList struct {
	api.TypeMeta `json:",inline" yaml:",inline"`
	Items        []DeploymentConfig `json:"items,omitempty" yaml:"items,omitempty"`
}

// DeploymentTemplate contains all the necessary information to create a Deployment from a
// DeploymentStrategy.
type DeploymentTemplate struct {
	Strategy           *DeploymentStrategy            `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	ControllerTemplate api.ReplicationControllerState `json:"controllerTemplate,omitempty" yaml:"controllerTemplate,omitempty"`
}

// DeploymentTriggerPolicy describes a policy for a single trigger that results in a new Deployment.
type DeploymentTriggerPolicy struct {
	Type DeploymentTriggerType `json:"type,omitempty" yaml:"type,omitempty"`
	// ImageChangeParams represents the parameters for the ImageChange trigger.
	ImageChangeParams *DeploymentTriggerImageChangeParams `json:"imageChangeParams,omitempty" yaml:"imageChangeParams,omitempty"`
}

// DeploymentTriggerImageChangeParams represents the parameters to the ImageChange trigger.
type DeploymentTriggerImageChangeParams struct {
	// Automatic means that the detection of a new tag value should result in a new deployment.
	Automatic bool `json:"automatic,omitempty" yaml:"automatic,omitempty"`
	// ContainerNames is used to restrict tag updates to the specified set of container names in a pod.
	ContainerNames []string `json:"containerNames,omitempty" yaml:"containerNames,omitempty"`
	// RepositoryName is the identifier for a Docker image repository to watch for changes.
	RepositoryName string `json:"repositoryName,omitempty" yaml:"repositoryName,omitempty"`
	// Tag is the name of an image repository tag to watch for changes.
	Tag string `json:"tag,omitempty" yaml:"tag,omitempty"`
}

// DeploymentTriggerType refers to a specific DeploymentTriggerPolicy implementation.
type DeploymentTriggerType string

const (
	// DeploymentTriggerManual is a placeholder implementation which does nothing.
	DeploymentTriggerManual DeploymentTriggerType = "Manual"
	// DeploymentTriggerOnImageChange will create new deployments in response to updated tags from
	// a Docker image repository.
	DeploymentTriggerOnImageChange DeploymentTriggerType = "ImageChange"
	// DeploymentTriggerOnConfigChange will create new deployments in response to changes to
	// the ControllerTemplate of a DeploymentConfig.
	DeploymentTriggerOnConfigChange DeploymentTriggerType = "ConfigChange"
)
