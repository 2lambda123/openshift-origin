package lifecycle

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// DeploymentContext informs the manager which Lifecycle point is being handled.
type DeploymentContext string

const (
	// PreDeploymentContext refers to Lifecycle.Pre
	PreDeploymentContext DeploymentContext = "PreDeploymentContext"
	// PostDeploymentContext refers to Lifecycle.Post
	PostDeploymentContext DeploymentContext = "PostDeploymentContext"
)

// Interface provides deployment controllers with a way to execute and track
// lifecycle actions.
//
// This interface abstracts action policy handling; users should assume
// Complete, Failed, and CompleteWithErrors are terminal states, and that any
// request to retry has already been accounted for. Users should not attempt
// to retry actions with status Failed or CompleteWithErrors.
//
// Users should not be concerned with whether a given lifecycle action is
// actually defined on a deployment; calls to execute non-existent actions
// will no-op, and status for non-existent actions will appear to be Complete.
type Interface interface {
	// Execute executes the deployment lifecycle action for the given context.
	// If no action is defined, Execute should return nil.
	Execute(context DeploymentContext, deployment *kapi.ReplicationController) error
	// Status returns the status of the lifecycle action for the deployment. If
	// no action is defined for the given context, Status returns Complete. If
	// the action finished, one of the following will be returned:
	//
	//  1. Complete: If the action succeeded or doesn't exist
	//  2. Failed: If the action failed and the policy is not configured to
	//     ignore failures
	//  3. CompleteWithErrors: If the action failed and the policy is configured
	//     to ignore failures
	//
	// If the status couldn't be determined, an error is returned.
	Status(context DeploymentContext, deployment *kapi.ReplicationController) (deployapi.DeploymentLifecycleStatus, error)
}

// Plugin knows how to execute lifecycle handlers and report their status.
//
// Plugins are expected to report actual status, NOT policy based status.
type Plugin interface {
	// CanHandle should return true if the plugin knows how to execute handler.
	CanHandle(handler *deployapi.Handler) bool
	// Execute executes handler in the given context for deployment.
	Execute(context DeploymentContext, handler *deployapi.Handler, deployment *kapi.ReplicationController, config *deployapi.DeploymentConfig) error
	// Status should report the actual status of the action without taking into
	// account failure policies.
	Status(context DeploymentContext, handler *deployapi.Handler, deployment *kapi.ReplicationController) deployapi.DeploymentLifecycleStatus
}

// LifecycleManager implements a pluggable lifecycle.Interface which handles
// the high level details of lifecyle action execution such as decoding
// DeploymentConfigs and implementing the lifecycle.Interface contract for
// policy based status reporting using the actual status returned from
// plugins.
type LifecycleManager struct {
	// Plugins execute specific handler instances.
	Plugins []Plugin
	// DecodeConfig knows how to decode the deploymentConfig from a deployment's annotations.
	DecodeConfig func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error)
}

var _ Interface = &LifecycleManager{}

// Execute implements Interface.
func (m *LifecycleManager) Execute(context DeploymentContext, deployment *kapi.ReplicationController) error {
	// Decode the config
	config, err := m.DecodeConfig(deployment)
	if err != nil {
		return err
	}

	// If there's no handler, no-op
	handler := handlerFor(context, config)
	if handler == nil {
		return nil
	}

	plugin, err := m.pluginFor(handler)
	if err != nil {
		return err
	}

	return plugin.Execute(context, handler, deployment, config)
}

// Status implements Interface.
func (m *LifecycleManager) Status(context DeploymentContext, deployment *kapi.ReplicationController) (deployapi.DeploymentLifecycleStatus, error) {
	// Decode the config
	config, err := m.DecodeConfig(deployment)
	if err != nil {
		return "", nil
	}

	handler := handlerFor(context, config)
	if handler == nil {
		return deployapi.DeploymentLifecycleStatusComplete, nil
	}

	plugin, err := m.pluginFor(handler)
	if err != nil {
		return "", err
	}

	status := plugin.Status(context, handler, deployment)
	if status == deployapi.DeploymentLifecycleStatusFailed &&
		handler.FailurePolicy == deployapi.HandlerFailurePolicyIgnore {
		status = deployapi.DeploymentLifecycleStatusCompleteWithErrors
	}
	return status, nil
}

// pluginFor finds a plugin which knows how to deal with handler.
func (m *LifecycleManager) pluginFor(handler *deployapi.Handler) (Plugin, error) {
	for _, plugin := range m.Plugins {
		if plugin.CanHandle(handler) {
			return plugin, nil
		}
	}

	return nil, fmt.Errorf("no plugin registered for handler: %#v", handler)
}

// handlerFor finds any handler in config for the given context.
func handlerFor(context DeploymentContext, config *deployapi.DeploymentConfig) *deployapi.Handler {
	if config.Template.Strategy.Lifecycle == nil {
		return nil
	}

	// Find any right handler given the context
	var handler *deployapi.Handler
	switch context {
	case PreDeploymentContext:
		handler = config.Template.Strategy.Lifecycle.Pre
	case PostDeploymentContext:
		handler = config.Template.Strategy.Lifecycle.Post
	}
	return handler
}
