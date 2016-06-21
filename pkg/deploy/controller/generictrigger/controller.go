package generictrigger

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentTriggerController processes all triggers for a deployment config
// and kicks new deployments whenever possible.
type DeploymentTriggerController struct {
	// dn is used to update deployment configs.
	dn osclient.DeploymentConfigsNamespacer
	// rn is used for getting the latest deployment for a config.
	rn kclient.ReplicationControllersNamespacer
	// codec is used for decoding a config out of a deployment.
	codec runtime.Codec
}

// NewDeploymentTriggerController returns a new DeploymentTriggerController.
func NewDeploymentTriggerController(oc osclient.Interface, kc kclient.Interface, codec runtime.Codec) *DeploymentTriggerController {
	return &DeploymentTriggerController{
		dn:    oc,
		rn:    kc,
		codec: codec,
	}
}

// fatalError is an error which can't be retried.
type fatalError string

func (e fatalError) Error() string {
	return fmt.Sprintf("fatal error handling configuration: %s", string(e))
}

// Handle processes deployment triggers for a deployment config.
func (c *DeploymentTriggerController) Handle(config *deployapi.DeploymentConfig) error {
	if len(config.Spec.Triggers) == 0 || config.Spec.Paused {
		return nil
	}

	// Try to decode this deployment config from the encoded annotation found in
	// its latest deployment.
	decoded, err := c.decodeFromLatest(config)
	if err != nil {
		return err
	}

	canTrigger, causes := canTrigger(config, decoded)

	// Return if we cannot trigger a new deployment.
	if !canTrigger {
		return nil
	}

	return c.update(config, causes)
}

// decodeFromLatest will try to return the decoded version of the current deploymentconfig found
// in the annotations of its latest deployment. If there is no previous deploymentconfig (ie.
// latestVersion == 0), the returned deploymentconfig will be the same.
func (c *DeploymentTriggerController) decodeFromLatest(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	if config.Status.LatestVersion == 0 {
		return config, nil
	}

	latestDeploymentName := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := c.rn.ReplicationControllers(config.Namespace).Get(latestDeploymentName)
	if err != nil {
		// If there's no deployment for the latest config, we have no basis of
		// comparison. It's the responsibility of the deployment config controller
		// to make the deployment for the config, so return early.
		return nil, fmt.Errorf("couldn't retrieve deployment for deployment config %q: %v", deployutil.LabelForDeploymentConfig(config), err)
	}

	latest, err := deployutil.DecodeDeploymentConfig(deployment, c.codec)
	if err != nil {
		return nil, fatalError(err.Error())
	}
	return latest, nil
}

// canTrigger is used by the trigger controller to determine if the provided config can trigger
// a deployment.
//
// Image change triggers are processed first. It is required for all of them to point to images
// that exist. Otherwise, this controller will wait for the images to land and be updated in the
// triggers that point to them by the image change controller.
//
// Config change triggers are processed last. If all images are resolved and an automatic trigger
// was updated, then it should be possible to trigger a new deployment without a config change
// trigger. Otherwise, if a config change trigger exists and the config is not deployed yet or it
// has a podtemplate change, then the controller should trigger a new deployment (assuming all
// image change triggers can trigger).
func canTrigger(config, decoded *deployapi.DeploymentConfig) (bool, []deployapi.DeploymentCause) {
	if decoded == nil {
		// The decoded deployment config will never be nil here but a sanity check
		// never hurts.
		return false, nil
	}
	ictCount, resolved, canTriggerByImageChange := 0, 0, false
	var causes []deployapi.DeploymentCause

	// IMAGE CHANGE TRIGGERS
	for _, t := range config.Spec.Triggers {
		if t.Type != deployapi.DeploymentTriggerOnImageChange {
			continue
		}
		ictCount++

		// If this is the initial deployment then we need to wait for the image change controller
		// to resolve the image inside the pod template.
		lastTriggered := t.ImageChangeParams.LastTriggeredImage
		if len(lastTriggered) == 0 {
			continue
		}
		resolved++

		// Non-automatic triggers should not be able to trigger deployments.
		if !t.ImageChangeParams.Automatic {
			continue
		}

		// We need stronger checks in order to validate that this template
		// change is an image change. Look at the deserialized config's
		// triggers and compare with the present trigger.
		if !triggeredByDifferentImage(*t.ImageChangeParams, *decoded) {
			continue
		}

		canTriggerByImageChange = true
		causes = append(causes, deployapi.DeploymentCause{
			Type: deployapi.DeploymentTriggerOnImageChange,
			ImageTrigger: &deployapi.DeploymentCauseImageTrigger{
				From: kapi.ObjectReference{
					Name:      t.ImageChangeParams.From.Name,
					Namespace: t.ImageChangeParams.From.Namespace,
					Kind:      "ImageStreamTag",
				},
			},
		})
	}

	// We need to wait for all images to resolve before triggering a new deployment.
	if ictCount != resolved {
		return false, nil
	}

	// CONFIG CHANGE TRIGGERS
	canTriggerByConfigChange := false
	// Our deployment config has a config change trigger and no image change has triggered.
	// If an image change had happened, it would be enough to start a new deployment without
	// caring about the config change trigger.
	if deployutil.HasChangeTrigger(config) && !canTriggerByImageChange {
		// This is the initial deployment or the config has a template change. We need to
		// kick a new deployment.
		if config.Status.LatestVersion == 0 || !kapi.Semantic.DeepEqual(config.Spec.Template, decoded.Spec.Template) {
			canTriggerByConfigChange = true
			causes = []deployapi.DeploymentCause{{Type: deployapi.DeploymentTriggerOnConfigChange}}
		}
	}

	return canTriggerByConfigChange || canTriggerByImageChange, causes
}

// triggeredByDifferentImage compares the provided image change parameters with those found in the
// previous deployment config (the one we decoded from the annotations of its latest deployment)
// and returns whether the two deployment configs have been triggered by a different image change.
func triggeredByDifferentImage(ictParams deployapi.DeploymentTriggerImageChangeParams, previous deployapi.DeploymentConfig) bool {
	for _, t := range previous.Spec.Triggers {
		if t.Type != deployapi.DeploymentTriggerOnImageChange {
			continue
		}

		if t.ImageChangeParams.From.Name != ictParams.From.Name &&
			t.ImageChangeParams.From.Namespace != ictParams.From.Namespace {
			continue
		}

		return t.ImageChangeParams.LastTriggeredImage != ictParams.LastTriggeredImage
	}
	return false
}

// update increments the latestVersion of the provided deployment config so the deployment config
// controller can run a new deployment and also updates the details of the deployment config.
func (c *DeploymentTriggerController) update(config *deployapi.DeploymentConfig, causes []deployapi.DeploymentCause) error {
	config.Status.LatestVersion++
	config.Status.Details = new(deployapi.DeploymentDetails)
	config.Status.Details.Causes = causes
	_, err := c.dn.DeploymentConfigs(config.Namespace).UpdateStatus(config)
	return err
}
