package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/intstr"
	kvalidation "k8s.io/kubernetes/pkg/util/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imageval "github.com/openshift/origin/pkg/image/api/validation"
)

func ValidateDeploymentConfig(config *deployapi.DeploymentConfig) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&config.ObjectMeta, true, validation.NameIsDNSSubdomain, field.NewPath("metadata"))

	// TODO: Refactor to validate spec and status separately
	for i := range config.Spec.Triggers {
		allErrs = append(allErrs, validateTrigger(&config.Spec.Triggers[i], field.NewPath("spec", "triggers").Index(i))...)
	}

	var spec *kapi.PodSpec
	if config.Spec.Template != nil {
		spec = &config.Spec.Template.Spec
	}
	specPath := field.NewPath("spec")
	allErrs = append(allErrs, validateDeploymentStrategy(&config.Spec.Strategy, spec, field.NewPath("spec", "strategy"))...)
	if config.Spec.Template == nil {
		allErrs = append(allErrs, field.Required(specPath.Child("template"), ""))
	} else {
		allErrs = append(allErrs, validation.ValidatePodTemplateSpec(config.Spec.Template, specPath.Child("template"))...)
	}
	if config.Status.LatestVersion < 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("status", "latestVersion"), config.Status.LatestVersion, "latestVersion cannot be negative"))
	}
	if config.Spec.Replicas < 0 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("replicas"), config.Spec.Replicas, "replicas cannot be negative"))
	}
	if len(config.Spec.Selector) == 0 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("selector"), config.Spec.Selector, "selector cannot be empty"))
	}
	return allErrs
}

func ValidateDeploymentConfigUpdate(newConfig *deployapi.DeploymentConfig, oldConfig *deployapi.DeploymentConfig) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&newConfig.ObjectMeta, &oldConfig.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateDeploymentConfig(newConfig)...)
	statusPath := field.NewPath("status")
	if newConfig.Status.LatestVersion < oldConfig.Status.LatestVersion {
		allErrs = append(allErrs, field.Invalid(statusPath.Child("latestVersion"), newConfig.Status.LatestVersion, "latestVersion cannot be decremented"))
	} else if newConfig.Status.LatestVersion > (oldConfig.Status.LatestVersion + 1) {
		allErrs = append(allErrs, field.Invalid(statusPath.Child("latestVersion"), newConfig.Status.LatestVersion, "latestVersion can only be incremented by 1"))
	}
	return allErrs
}

func ValidateDeploymentConfigRollback(rollback *deployapi.DeploymentConfigRollback) field.ErrorList {
	result := field.ErrorList{}

	fromPath := field.NewPath("spec", "from")
	if len(rollback.Spec.From.Name) == 0 {
		result = append(result, field.Required(fromPath.Child("name"), ""))
	}

	if len(rollback.Spec.From.Kind) == 0 {
		rollback.Spec.From.Kind = "ReplicationController"
	}

	if rollback.Spec.From.Kind != "ReplicationController" {
		result = append(result, field.Invalid(fromPath.Child("kind"), rollback.Spec.From.Kind, "the kind of the rollback target must be 'ReplicationController'"))
	}

	return result
}

func validateDeploymentStrategy(strategy *deployapi.DeploymentStrategy, pod *kapi.PodSpec, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if len(strategy.Type) == 0 {
		errs = append(errs, field.Required(fldPath.Child("type"), ""))
	}

	switch strategy.Type {
	case deployapi.DeploymentStrategyTypeRecreate:
		if strategy.RecreateParams != nil {
			errs = append(errs, validateRecreateParams(strategy.RecreateParams, pod, fldPath.Child("recreateParams"))...)
		}
	case deployapi.DeploymentStrategyTypeRolling:
		if strategy.RollingParams == nil {
			errs = append(errs, field.Required(fldPath.Child("rollingParams"), ""))
		} else {
			errs = append(errs, validateRollingParams(strategy.RollingParams, pod, fldPath.Child("rollingParams"))...)
		}
	case deployapi.DeploymentStrategyTypeCustom:
		if strategy.CustomParams == nil {
			errs = append(errs, field.Required(fldPath.Child("customParams"), ""))
		} else {
			errs = append(errs, validateCustomParams(strategy.CustomParams, fldPath.Child("customParams"))...)
		}
	}

	if strategy.Labels != nil {
		errs = append(errs, validation.ValidateLabels(strategy.Labels, fldPath.Child("labels"))...)
	}
	if strategy.Annotations != nil {
		errs = append(errs, validation.ValidateAnnotations(strategy.Annotations, fldPath.Child("annotations"))...)
	}

	// TODO: validate resource requirements (prereq: https://github.com/kubernetes/kubernetes/pull/7059)

	return errs
}

func validateCustomParams(params *deployapi.CustomDeploymentStrategyParams, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if len(params.Image) == 0 {
		errs = append(errs, field.Required(fldPath.Child("image"), ""))
	}

	return errs
}

func validateRecreateParams(params *deployapi.RecreateDeploymentStrategyParams, pod *kapi.PodSpec, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if params.TimeoutSeconds != nil && *params.TimeoutSeconds < 1 {
		errs = append(errs, field.Invalid(fldPath.Child("timeoutSeconds"), *params.TimeoutSeconds, "must be >0"))
	}

	if params.Pre != nil {
		errs = append(errs, validateLifecycleHook(params.Pre, pod, fldPath.Child("pre"))...)
	}
	if params.Mid != nil {
		errs = append(errs, validateLifecycleHook(params.Mid, pod, fldPath.Child("mid"))...)
	}
	if params.Post != nil {
		errs = append(errs, validateLifecycleHook(params.Post, pod, fldPath.Child("post"))...)
	}

	return errs
}

func validateLifecycleHook(hook *deployapi.LifecycleHook, pod *kapi.PodSpec, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if len(hook.FailurePolicy) == 0 {
		errs = append(errs, field.Required(fldPath.Child("failurePolicy"), ""))
	}

	if *hook.ProgressDeadlineSeconds > 0 && hook.FailurePolicy != deployapi.LifecycleHookFailurePolicyRetry {
		errs = append(errs, field.Invalid(fldPath, "<hook>", "progressDeadlineSeconds can be used only with the 'Retry' failurePolicy"))
	}

	if *hook.ProgressDeadlineSeconds < 0 {
		errs = append(errs, field.Invalid(fldPath, "<hook>", "progressDeadlineSeconds cannot be negative number"))
	}

	if *hook.ProgressDeadlineSeconds >= deployapi.MaxDeploymentDurationSeconds {
		errs = append(errs, field.Invalid(fldPath, "<hook>", "progressDeadlineSeconds must be lower than %ds", deployapi.MaxDeploymentDurationSeconds))
	}

	switch {
	case hook.ExecNewPod != nil && len(hook.TagImages) > 0:
		errs = append(errs, field.Invalid(fldPath, "<hook>", "only one of 'execNewPod' of 'tagImages' may be specified"))
	case hook.ExecNewPod != nil:
		errs = append(errs, validateExecNewPod(hook.ExecNewPod, fldPath.Child("execNewPod"))...)
	case len(hook.TagImages) > 0:
		for i, image := range hook.TagImages {
			if len(image.ContainerName) == 0 {
				errs = append(errs, field.Required(fldPath.Child("tagImages").Index(i).Child("containerName"), "a containerName is required"))
			} else {
				if _, err := deployapi.TemplateImageForContainer(pod, deployapi.IgnoreTriggers, image.ContainerName); err != nil {
					errs = append(errs, field.Invalid(fldPath.Child("tagImages").Index(i).Child("containerName"), image.ContainerName, err.Error()))
				}
			}
			if image.To.Kind != "ImageStreamTag" {
				errs = append(errs, field.Invalid(fldPath.Child("tagImages").Index(i).Child("to", "kind"), image.To.Kind, "Must be 'ImageStreamTag'"))
			}
			if len(image.To.Name) == 0 {
				errs = append(errs, field.Required(fldPath.Child("tagImages").Index(i).Child("to", "name"), "a destination tag name is required"))
			}
		}
	default:
		errs = append(errs, field.Invalid(fldPath, "<empty>", "One of execNewPod or tagImages must be specified"))
	}

	return errs
}

func validateExecNewPod(hook *deployapi.ExecNewPodHook, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if len(hook.Command) == 0 {
		errs = append(errs, field.Required(fldPath.Child("command"), ""))
	}

	if len(hook.ContainerName) == 0 {
		errs = append(errs, field.Required(fldPath.Child("containerName"), ""))
	}

	if len(hook.Env) > 0 {
		errs = append(errs, validateEnv(hook.Env, fldPath.Child("env"))...)
	}

	errs = append(errs, validateHookVolumes(hook.Volumes, fldPath.Child("volumes"))...)

	return errs
}

func validateEnv(vars []kapi.EnvVar, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, ev := range vars {
		vErrs := field.ErrorList{}
		idxPath := fldPath.Child("name").Index(i)
		if len(ev.Name) == 0 {
			vErrs = append(vErrs, field.Required(idxPath, ""))
		}
		if !kvalidation.IsCIdentifier(ev.Name) {
			vErrs = append(vErrs, field.Invalid(idxPath, ev.Name, "must match regex "+kvalidation.CIdentifierFmt))
		}
		allErrs = append(allErrs, vErrs...)
	}
	return allErrs
}

func validateHookVolumes(volumes []string, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	for i, vol := range volumes {
		vErrs := field.ErrorList{}
		if len(vol) == 0 {
			vErrs = append(vErrs, field.Invalid(fldPath.Index(i), "", "must not be empty"))
		}
		errs = append(errs, vErrs...)
	}
	return errs
}

func validateRollingParams(params *deployapi.RollingDeploymentStrategyParams, pod *kapi.PodSpec, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if params.IntervalSeconds != nil && *params.IntervalSeconds < 1 {
		errs = append(errs, field.Invalid(fldPath.Child("intervalSeconds"), *params.IntervalSeconds, "must be >0"))
	}

	if params.UpdatePeriodSeconds != nil && *params.UpdatePeriodSeconds < 1 {
		errs = append(errs, field.Invalid(fldPath.Child("updatePeriodSeconds"), *params.UpdatePeriodSeconds, "must be >0"))
	}

	if params.TimeoutSeconds != nil && *params.TimeoutSeconds < 1 {
		errs = append(errs, field.Invalid(fldPath.Child("timeoutSeconds"), *params.TimeoutSeconds, "must be >0"))
	}

	if params.UpdatePercent != nil {
		p := *params.UpdatePercent
		if p == 0 || p < -100 || p > 100 {
			errs = append(errs, field.Invalid(fldPath.Child("updatePercent"), *params.UpdatePercent, "must be between 1 and 100 or between -1 and -100 (inclusive)"))
		}
	}
	// Most of this is lifted from the upstream experimental deployments API. We
	// can't reuse it directly yet, but no use reinventing the logic, so copy-
	// pasted and adapted here.
	errs = append(errs, ValidatePositiveIntOrPercent(params.MaxUnavailable, fldPath.Child("maxUnavailable"))...)
	errs = append(errs, ValidatePositiveIntOrPercent(params.MaxSurge, fldPath.Child("maxSurge"))...)
	if getIntOrPercentValue(params.MaxUnavailable) == 0 && getIntOrPercentValue(params.MaxSurge) == 0 {
		// Both MaxSurge and MaxUnavailable cannot be zero.
		errs = append(errs, field.Invalid(fldPath.Child("maxUnavailable"), params.MaxUnavailable, "cannot be 0 when maxSurge is 0 as well"))
	}
	// Validate that MaxUnavailable is not more than 100%.
	errs = append(errs, IsNotMoreThan100Percent(params.MaxUnavailable, fldPath.Child("maxUnavailable"))...)

	if params.Pre != nil {
		errs = append(errs, validateLifecycleHook(params.Pre, pod, fldPath.Child("pre"))...)
	}
	if params.Post != nil {
		errs = append(errs, validateLifecycleHook(params.Post, pod, fldPath.Child("post"))...)
	}

	return errs
}

func validateTrigger(trigger *deployapi.DeploymentTriggerPolicy, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if len(trigger.Type) == 0 {
		errs = append(errs, field.Required(fldPath.Child("type"), ""))
	}

	if trigger.Type == deployapi.DeploymentTriggerOnImageChange {
		if trigger.ImageChangeParams == nil {
			errs = append(errs, field.Required(fldPath.Child("imageChangeParams"), ""))
		} else {
			errs = append(errs, validateImageChangeParams(trigger.ImageChangeParams, fldPath.Child("imageChangeParams"))...)
		}
	}

	return errs
}

func validateImageChangeParams(params *deployapi.DeploymentTriggerImageChangeParams, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	fromPath := fldPath.Child("from")
	if len(params.From.Name) == 0 {
		errs = append(errs, field.Required(fromPath, ""))
	} else {
		if params.From.Kind != "ImageStreamTag" {
			errs = append(errs, field.Invalid(fromPath.Child("kind"), params.From.Kind, "kind must be an ImageStreamTag"))
		}
		if err := validateImageStreamTagName(params.From.Name); err != nil {
			errs = append(errs, field.Invalid(fromPath.Child("name"), params.From.Name, err.Error()))
		}
		if len(params.From.Namespace) != 0 && !kvalidation.IsDNS1123Subdomain(params.From.Namespace) {
			errs = append(errs, field.Invalid(fromPath.Child("namespace"), params.From.Namespace, "namespace must be a valid subdomain"))
		}
	}

	if len(params.ContainerNames) == 0 {
		errs = append(errs, field.Required(fldPath.Child("containerNames"), ""))
	}

	return errs
}

func validateImageStreamTagName(istag string) error {
	name, _, ok := imageapi.SplitImageStreamTag(istag)
	if !ok {
		return fmt.Errorf("invalid ImageStreamTag: %s", istag)
	}
	ok, reason := imageval.ValidateImageStreamName(name, false)
	if !ok {
		return errors.New(reason)
	}
	return nil
}

func ValidatePositiveIntOrPercent(intOrPercent intstr.IntOrString, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if intOrPercent.Type == intstr.String {
		if !IsValidPercent(intOrPercent.StrVal) {
			allErrs = append(allErrs, field.Invalid(fldPath, intOrPercent, "value should be int(5) or percentage(5%)"))
		}

	} else if intOrPercent.Type == intstr.Int {
		allErrs = append(allErrs, ValidatePositiveField(int64(intOrPercent.IntVal), fldPath)...)
	}
	return allErrs
}

func getPercentValue(intOrStringValue intstr.IntOrString) (int, bool) {
	if intOrStringValue.Type != intstr.String || !IsValidPercent(intOrStringValue.StrVal) {
		return 0, false
	}
	value, _ := strconv.Atoi(intOrStringValue.StrVal[:len(intOrStringValue.StrVal)-1])
	return value, true
}

func getIntOrPercentValue(intOrStringValue intstr.IntOrString) int {
	value, isPercent := getPercentValue(intOrStringValue)
	if isPercent {
		return value
	}
	return int(intOrStringValue.IntVal)
}

func IsNotMoreThan100Percent(intOrStringValue intstr.IntOrString, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	value, isPercent := getPercentValue(intOrStringValue)
	if !isPercent || value <= 100 {
		return nil
	}
	allErrs = append(allErrs, field.Invalid(fldPath, intOrStringValue, "should not be more than 100%"))
	return allErrs
}

func ValidatePositiveField(value int64, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if value < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, value, isNegativeErrorMsg))
	}
	return allErrs
}

const percentFmt string = "[0-9]+%"

var percentRegexp = regexp.MustCompile("^" + percentFmt + "$")

func IsValidPercent(percent string) bool {
	return percentRegexp.MatchString(percent)
}

const isNegativeErrorMsg string = `must be non-negative`

func ValidateDeploymentLogOptions(opts *deployapi.DeploymentLogOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	// TODO: Replace by validating PodLogOptions via DeploymentLogOptions once it's bundled in
	popts := deployapi.DeploymentToPodLogOptions(opts)
	if errs := validation.ValidatePodLogOptions(popts); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if opts.Version != nil && *opts.Version <= 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("version"), *opts.Version, "deployment version must be greater than 0"))
	}
	if opts.Version != nil && opts.Previous {
		allErrs = append(allErrs, field.Invalid(field.NewPath("previous"), opts.Previous, "cannot use previous when a version is specified"))
	}

	return allErrs
}
