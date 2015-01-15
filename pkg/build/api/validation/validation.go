package validation

import (
	"net/url"

	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// ValidateBuild tests required fields for a Build.
func ValidateBuild(build *buildapi.Build) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(build.Name) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("name", build.Name))
	}
	allErrs = append(allErrs, validateBuildParameters(&build.Parameters).Prefix("parameters")...)
	return allErrs
}

// ValidateBuildConfig tests required fields for a Build.
func ValidateBuildConfig(config *buildapi.BuildConfig) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(config.Name) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("name", config.Name))
	}
	for i := range config.Triggers {
		allErrs = append(allErrs, validateTrigger(&config.Triggers[i]).PrefixIndex(i).Prefix("triggers")...)
	}
	allErrs = append(allErrs, validateBuildParameters(&config.Parameters).Prefix("parameters")...)
	return allErrs
}

func validateBuildParameters(params *buildapi.BuildParameters) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	isCustomBuild := params.Strategy.Type == buildapi.CustomBuildStrategyType
	// Validate 'source' and 'output' for all build types except Custom build
	// where they are optional and validated only if present.
	if !isCustomBuild || (isCustomBuild && len(params.Source.Type) != 0) {
		allErrs = append(allErrs, validateSource(&params.Source).Prefix("source")...)

		if params.Revision != nil {
			allErrs = append(allErrs, validateRevision(params.Revision).Prefix("revision")...)
		}
	}

	if !isCustomBuild || (isCustomBuild && len(params.Output.ImageTag) != 0) {
		allErrs = append(allErrs, validateOutput(&params.Output).Prefix("output")...)
	}

	allErrs = append(allErrs, validateStrategy(&params.Strategy).Prefix("strategy")...)

	return allErrs
}

func validateSource(input *buildapi.BuildSource) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if input.Type != buildapi.BuildSourceGit {
		allErrs = append(allErrs, errs.NewFieldRequired("type", buildapi.BuildSourceGit))
	}
	if input.Git == nil {
		allErrs = append(allErrs, errs.NewFieldRequired("git", input.Git))
	} else {
		allErrs = append(allErrs, validateGitSource(input.Git).Prefix("git")...)
	}
	return allErrs
}

func validateGitSource(git *buildapi.GitBuildSource) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(git.URI) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("uri", git.URI))
	} else if !isValidURL(git.URI) {
		allErrs = append(allErrs, errs.NewFieldInvalid("uri", git.URI, "uri is not a valid url"))
	}
	return allErrs
}

func validateRevision(revision *buildapi.SourceRevision) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(revision.Type) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("type", revision.Type))
	}
	// TODO: validate other stuff
	return allErrs
}

func validateStrategy(strategy *buildapi.BuildStrategy) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	if len(strategy.Type) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("type", strategy.Type))
	}

	switch strategy.Type {
	case buildapi.STIBuildStrategyType:
		if strategy.STIStrategy == nil {
			allErrs = append(allErrs, errs.NewFieldRequired("stiStrategy", strategy.STIStrategy))
		} else {
			allErrs = append(allErrs, validateSTIStrategy(strategy.STIStrategy).Prefix("stiStrategy")...)
		}
	case buildapi.DockerBuildStrategyType:
		// DockerStrategy is currently optional, initialize it to a default state if it's not set.
		if strategy.DockerStrategy == nil {
			strategy.DockerStrategy = &buildapi.DockerBuildStrategy{}
		}
	case buildapi.CustomBuildStrategyType:
		if strategy.CustomStrategy == nil {
			allErrs = append(allErrs, errs.NewFieldRequired("customStrategy", strategy.CustomStrategy))
		} else {
			// CustomBuildStrategy requires 'image' to be specified in JSON
			if len(strategy.CustomStrategy.Image) == 0 {
				allErrs = append(allErrs, errs.NewFieldRequired("image", strategy.CustomStrategy.Image))
			}
		}
	default:
		allErrs = append(allErrs, errs.NewFieldInvalid("type", strategy.Type, "type is not in the enumerated list"))
	}

	return allErrs
}

func validateSTIStrategy(strategy *buildapi.STIBuildStrategy) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(strategy.Image) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("image", strategy.Image))
	}
	return allErrs
}

func validateOutput(output *buildapi.BuildOutput) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(output.ImageTag) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("imageTag", output.ImageTag))
	}
	return allErrs
}

func validateTrigger(trigger *buildapi.BuildTriggerPolicy) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(trigger.Type) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("type", ""))
		return allErrs
	}

	// Ensure that only parameters for the trigger's type are present
	triggerPresence := map[buildapi.BuildTriggerType]bool{
		buildapi.GithubWebHookBuildTriggerType:  trigger.GithubWebHook != nil,
		buildapi.GenericWebHookBuildTriggerType: trigger.GenericWebHook != nil,
	}
	allErrs = append(allErrs, validateTriggerPresence(triggerPresence, trigger.Type)...)

	// Validate each trigger type
	switch trigger.Type {
	case buildapi.GithubWebHookBuildTriggerType:
		if trigger.GithubWebHook == nil {
			allErrs = append(allErrs, errs.NewFieldRequired("github", nil))
		} else {
			allErrs = append(allErrs, validateWebHook(trigger.GithubWebHook).Prefix("github")...)
		}
	case buildapi.GenericWebHookBuildTriggerType:
		if trigger.GenericWebHook == nil {
			allErrs = append(allErrs, errs.NewFieldRequired("generic", nil))
		} else {
			allErrs = append(allErrs, validateWebHook(trigger.GenericWebHook).Prefix("generic")...)
		}
	case buildapi.ImageChangeBuildTriggerType:
		if trigger.ImageChange == nil {
			allErrs = append(allErrs, errs.NewFieldRequired("imageChange", nil))
		} else {
			allErrs = append(allErrs, validateImageChange(trigger.ImageChange).Prefix("imageChange")...)
		}
	}
	return allErrs
}

func validateTriggerPresence(params map[buildapi.BuildTriggerType]bool, t buildapi.BuildTriggerType) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	for triggerType, present := range params {
		if triggerType != t && present {
			allErrs = append(allErrs, errs.NewFieldInvalid(string(triggerType), "", "triggerType wasn't found"))
		}
	}
	return allErrs
}

func validateImageChange(imageChange *buildapi.ImageChangeTrigger) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(imageChange.Image) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("image", ""))
	}
	if imageChange.ImageRepositoryRef == nil {
		allErrs = append(allErrs, errs.NewFieldRequired("imageRepositoryRef", ""))
	} else if len(imageChange.ImageRepositoryRef.Name) == 0 {
		nestedErrs := errs.ValidationErrorList{errs.NewFieldRequired("name", "")}
		nestedErrs.Prefix("imageRepositoryRef")
		allErrs = append(allErrs, nestedErrs...)
	}
	return allErrs
}

func validateWebHook(webHook *buildapi.WebHookTrigger) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(webHook.Secret) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("secret", ""))
	}
	return allErrs
}

func isValidURL(uri string) bool {
	_, err := url.Parse(uri)
	return err == nil
}
