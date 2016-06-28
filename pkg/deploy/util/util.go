package util

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	kdeplutil "k8s.io/kubernetes/pkg/util/deployment"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/util/namer"
)

// LatestDeploymentNameForConfig returns a stable identifier for config based on its version.
func LatestDeploymentNameForConfig(config *deployapi.DeploymentConfig) string {
	return fmt.Sprintf("%s-%d", config.Name, config.Status.LatestVersion)
}

// LatestDeploymentInfo returns info about the latest deployment for a config,
// or nil if there is no latest deployment. The latest deployment is not
// always the same as the active deployment.
func LatestDeploymentInfo(config *deployapi.DeploymentConfig, deployments []api.ReplicationController) (bool, *api.ReplicationController) {
	if config.Status.LatestVersion == 0 || len(deployments) == 0 {
		return false, nil
	}
	sort.Sort(ByLatestVersionDesc(deployments))
	candidate := &deployments[0]
	return DeploymentVersionFor(candidate) == config.Status.LatestVersion, candidate
}

// ActiveDeployment returns the latest complete deployment, or nil if there is
// no such deployment. The active deployment is not always the same as the
// latest deployment.
func ActiveDeployment(config *deployapi.DeploymentConfig, deployments []api.ReplicationController) *api.ReplicationController {
	sort.Sort(ByLatestVersionDesc(deployments))
	var activeDeployment *api.ReplicationController
	for _, deployment := range deployments {
		if DeploymentStatusFor(&deployment) == deployapi.DeploymentStatusComplete {
			activeDeployment = &deployment
			break
		}
	}
	return activeDeployment
}

// DeployerPodSuffix is the suffix added to pods created from a deployment
const DeployerPodSuffix = "deploy"

// DeployerPodNameForDeployment returns the name of a pod for a given deployment
func DeployerPodNameForDeployment(deployment string) string {
	return namer.GetPodName(deployment, DeployerPodSuffix)
}

// LabelForDeployment builds a string identifier for a Deployment.
func LabelForDeployment(deployment *api.ReplicationController) string {
	return fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name)
}

// LabelForDeploymentConfig builds a string identifier for a DeploymentConfig.
func LabelForDeploymentConfig(config *deployapi.DeploymentConfig) string {
	return fmt.Sprintf("%s/%s", config.Namespace, config.Name)
}

// DeploymentNameForConfigVersion returns the name of the version-th deployment
// for the config that has the provided name
func DeploymentNameForConfigVersion(name string, version int64) string {
	return fmt.Sprintf("%s-%d", name, version)
}

// ConfigSelector returns a label Selector which can be used to find all
// deployments for a DeploymentConfig.
//
// TODO: Using the annotation constant for now since the value is correct
// but we could consider adding a new constant to the public types.
func ConfigSelector(name string) labels.Selector {
	return labels.Set{deployapi.DeploymentConfigAnnotation: name}.AsSelector()
}

// DeployerPodSelector returns a label Selector which can be used to find all
// deployer pods associated with a deployment with name.
func DeployerPodSelector(name string) labels.Selector {
	return labels.Set{deployapi.DeployerPodForDeploymentLabel: name}.AsSelector()
}

// AnyDeployerPodSelector returns a label Selector which can be used to find
// all deployer pods across all deployments, including hook and custom
// deployer pods.
func AnyDeployerPodSelector() labels.Selector {
	sel, _ := labels.Parse(deployapi.DeployerPodForDeploymentLabel)
	return sel
}

// HasChangeTrigger returns whether the provided deployment configuration has
// a config change trigger or not
func HasChangeTrigger(config *deployapi.DeploymentConfig) bool {
	for _, trigger := range config.Spec.Triggers {
		if trigger.Type == deployapi.DeploymentTriggerOnConfigChange {
			return true
		}
	}
	return false
}

func DeploymentConfigDeepCopy(dc *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	objCopy, err := api.Scheme.DeepCopy(dc)
	if err != nil {
		return nil, err
	}
	copied, ok := objCopy.(*deployapi.DeploymentConfig)
	if !ok {
		return nil, fmt.Errorf("expected DeploymentConfig, got %#v", objCopy)
	}
	return copied, nil
}

// DecodeDeploymentConfig decodes a DeploymentConfig from controller using codec. An error is returned
// if the controller doesn't contain an encoded config.
func DecodeDeploymentConfig(controller *api.ReplicationController, decoder runtime.Decoder) (*deployapi.DeploymentConfig, error) {
	encodedConfig := []byte(EncodedDeploymentConfigFor(controller))
	decoded, err := runtime.Decode(decoder, encodedConfig)
	if err == nil {
		if config, ok := decoded.(*deployapi.DeploymentConfig); ok {
			return config, nil
		}
		return nil, fmt.Errorf("decoded object from controller is not a DeploymentConfig")
	}
	return nil, fmt.Errorf("failed to decode DeploymentConfig from controller: %v", err)
}

// EncodeDeploymentConfig encodes config as a string using codec.
func EncodeDeploymentConfig(config *deployapi.DeploymentConfig, codec runtime.Codec) (string, error) {
	if bytes, err := runtime.Encode(codec, config); err == nil {
		return string(bytes[:]), nil
	} else {
		return "", err
	}
}

// MakeDeployment creates a deployment represented as a ReplicationController and based on the given
// DeploymentConfig. The controller replica count will be zero.
func MakeDeployment(config *deployapi.DeploymentConfig, codec runtime.Codec) (*api.ReplicationController, error) {
	var err error
	var encodedConfig string

	if encodedConfig, err = EncodeDeploymentConfig(config, codec); err != nil {
		return nil, err
	}

	deploymentName := LatestDeploymentNameForConfig(config)

	podSpec := api.PodSpec{}
	if err := api.Scheme.Convert(&config.Spec.Template.Spec, &podSpec); err != nil {
		return nil, fmt.Errorf("couldn't clone podSpec: %v", err)
	}

	controllerLabels := make(labels.Set)
	for k, v := range config.Labels {
		controllerLabels[k] = v
	}
	// Correlate the deployment with the config.
	// TODO: Using the annotation constant for now since the value is correct
	// but we could consider adding a new constant to the public types.
	controllerLabels[deployapi.DeploymentConfigAnnotation] = config.Name

	// Ensure that pods created by this deployment controller can be safely associated back
	// to the controller, and that multiple deployment controllers for the same config don't
	// manipulate each others' pods.
	selector := map[string]string{}
	for k, v := range config.Spec.Selector {
		selector[k] = v
	}
	selector[deployapi.DeploymentConfigLabel] = config.Name
	selector[deployapi.DeploymentLabel] = deploymentName

	podLabels := make(labels.Set)
	for k, v := range config.Spec.Template.Labels {
		podLabels[k] = v
	}
	podLabels[deployapi.DeploymentConfigLabel] = config.Name
	podLabels[deployapi.DeploymentLabel] = deploymentName

	podAnnotations := make(labels.Set)
	for k, v := range config.Spec.Template.Annotations {
		podAnnotations[k] = v
	}
	podAnnotations[deployapi.DeploymentAnnotation] = deploymentName
	podAnnotations[deployapi.DeploymentConfigAnnotation] = config.Name
	podAnnotations[deployapi.DeploymentVersionAnnotation] = strconv.FormatInt(config.Status.LatestVersion, 10)

	deployment := &api.ReplicationController{
		ObjectMeta: api.ObjectMeta{
			Name: deploymentName,
			Annotations: map[string]string{
				deployapi.DeploymentConfigAnnotation:        config.Name,
				deployapi.DeploymentStatusAnnotation:        string(deployapi.DeploymentStatusNew),
				deployapi.DeploymentEncodedConfigAnnotation: encodedConfig,
				deployapi.DeploymentVersionAnnotation:       strconv.FormatInt(config.Status.LatestVersion, 10),
				// This is the target replica count for the new deployment.
				deployapi.DesiredReplicasAnnotation:    strconv.Itoa(int(config.Spec.Replicas)),
				deployapi.DeploymentReplicasAnnotation: strconv.Itoa(0),
			},
			Labels: controllerLabels,
		},
		Spec: api.ReplicationControllerSpec{
			// The deployment should be inactive initially
			Replicas: 0,
			Selector: selector,
			Template: &api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels:      podLabels,
					Annotations: podAnnotations,
				},
				Spec: podSpec,
			},
		},
	}
	if value, ok := config.Annotations[deployapi.DeploymentIgnorePodAnnotation]; ok {
		deployment.Annotations[deployapi.DeploymentIgnorePodAnnotation] = value
	}

	return deployment, nil
}

// GetReplicaCountForDeployments returns the sum of all replicas for the
// given deployments.
func GetReplicaCountForDeployments(deployments []api.ReplicationController) int32 {
	totalReplicaCount := int32(0)
	for _, deployment := range deployments {
		totalReplicaCount += deployment.Spec.Replicas
	}
	return totalReplicaCount
}

// GetStatusReplicaCountForDeployments returns the sum of the replicas reported in the
// status of the given deployments.
func GetStatusReplicaCountForDeployments(deployments []api.ReplicationController) int32 {
	totalReplicaCount := int32(0)
	for _, deployment := range deployments {
		totalReplicaCount += deployment.Status.Replicas
	}
	return totalReplicaCount
}

// GetAvailablePods returns all the available pods from the provided pod list.
func GetAvailablePods(pods []api.Pod, minReadySeconds int32) int32 {
	available := int32(0)
	for i := range pods {
		pod := pods[i]
		if kdeplutil.IsPodAvailable(&pod, minReadySeconds) {
			available++
		}
	}
	return available
}

func DeploymentConfigNameFor(obj runtime.Object) string {
	return annotationFor(obj, deployapi.DeploymentConfigAnnotation)
}

func DeploymentNameFor(obj runtime.Object) string {
	return annotationFor(obj, deployapi.DeploymentAnnotation)
}

func DeployerPodNameFor(obj runtime.Object) string {
	return annotationFor(obj, deployapi.DeploymentPodAnnotation)
}

func DeploymentStatusFor(obj runtime.Object) deployapi.DeploymentStatus {
	return deployapi.DeploymentStatus(annotationFor(obj, deployapi.DeploymentStatusAnnotation))
}

func DeploymentStatusReasonFor(obj runtime.Object) string {
	return annotationFor(obj, deployapi.DeploymentStatusReasonAnnotation)
}

func DeploymentDesiredReplicas(obj runtime.Object) (int32, bool) {
	return int32AnnotationFor(obj, deployapi.DesiredReplicasAnnotation)
}

func DeploymentReplicas(obj runtime.Object) (int32, bool) {
	return int32AnnotationFor(obj, deployapi.DeploymentReplicasAnnotation)
}

func EncodedDeploymentConfigFor(obj runtime.Object) string {
	return annotationFor(obj, deployapi.DeploymentEncodedConfigAnnotation)
}

func DeploymentVersionFor(obj runtime.Object) int64 {
	v, err := strconv.ParseInt(annotationFor(obj, deployapi.DeploymentVersionAnnotation), 10, 64)
	if err != nil {
		return -1
	}
	return v
}

func IsDeploymentCancelled(deployment *api.ReplicationController) bool {
	value := annotationFor(deployment, deployapi.DeploymentCancelledAnnotation)
	return strings.EqualFold(value, deployapi.DeploymentCancelledAnnotationValue)
}

func HasSynced(dc *deployapi.DeploymentConfig) bool {
	return dc.Status.ObservedGeneration >= dc.Generation
}

// IsTerminatedDeployment returns true if the passed deployment has terminated (either
// complete or failed).
func IsTerminatedDeployment(deployment *api.ReplicationController) bool {
	current := DeploymentStatusFor(deployment)
	return current == deployapi.DeploymentStatusComplete || current == deployapi.DeploymentStatusFailed
}

// CanTransitionPhase returns whether it is allowed to go from the current to the next phase.
func CanTransitionPhase(current, next deployapi.DeploymentStatus) bool {
	switch current {
	case deployapi.DeploymentStatusNew:
		switch next {
		case deployapi.DeploymentStatusPending,
			deployapi.DeploymentStatusRunning,
			deployapi.DeploymentStatusFailed,
			deployapi.DeploymentStatusComplete:
			return true
		}
	case deployapi.DeploymentStatusPending:
		switch next {
		case deployapi.DeploymentStatusRunning,
			deployapi.DeploymentStatusFailed,
			deployapi.DeploymentStatusComplete:
			return true
		}
	case deployapi.DeploymentStatusRunning:
		switch next {
		case deployapi.DeploymentStatusFailed, deployapi.DeploymentStatusComplete:
			return true
		}
	}
	return false
}

// annotationFor returns the annotation with key for obj.
func annotationFor(obj runtime.Object, key string) string {
	meta, err := api.ObjectMetaFor(obj)
	if err != nil {
		return ""
	}
	return meta.Annotations[key]
}

func int32AnnotationFor(obj runtime.Object, key string) (int32, bool) {
	s := annotationFor(obj, key)
	if len(s) == 0 {
		return 0, false
	}
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, false
	}
	return int32(i), true
}

// ByLatestVersionAsc sorts deployments by LatestVersion ascending.
type ByLatestVersionAsc []api.ReplicationController

func (d ByLatestVersionAsc) Len() int      { return len(d) }
func (d ByLatestVersionAsc) Swap(i, j int) { d[i], d[j] = d[j], d[i] }
func (d ByLatestVersionAsc) Less(i, j int) bool {
	return DeploymentVersionFor(&d[i]) < DeploymentVersionFor(&d[j])
}

// ByLatestVersionDesc sorts deployments by LatestVersion descending.
type ByLatestVersionDesc []api.ReplicationController

func (d ByLatestVersionDesc) Len() int      { return len(d) }
func (d ByLatestVersionDesc) Swap(i, j int) { d[i], d[j] = d[j], d[i] }
func (d ByLatestVersionDesc) Less(i, j int) bool {
	return DeploymentVersionFor(&d[j]) < DeploymentVersionFor(&d[i])
}

// ByMostRecent sorts deployments by most recently created.
type ByMostRecent []*api.ReplicationController

func (s ByMostRecent) Len() int      { return len(s) }
func (s ByMostRecent) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ByMostRecent) Less(i, j int) bool {
	return !s[i].CreationTimestamp.Before(s[j].CreationTimestamp)
}
