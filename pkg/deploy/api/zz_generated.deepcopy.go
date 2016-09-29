// +build !ignore_autogenerated_openshift

// This file was autogenerated by deepcopy-gen. Do not edit it manually!

package api

import (
	image_api "github.com/openshift/origin/pkg/image/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	conversion "k8s.io/kubernetes/pkg/conversion"
	runtime "k8s.io/kubernetes/pkg/runtime"
	reflect "reflect"
)

func init() {
	SchemeBuilder.Register(RegisterDeepCopies)
}

// RegisterDeepCopies adds deep-copy functions to the given scheme. Public
// to allow building arbitrary schemes.
func RegisterDeepCopies(scheme *runtime.Scheme) error {
	return scheme.AddGeneratedDeepCopyFuncs(
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_CustomDeploymentStrategyParams, InType: reflect.TypeOf(&CustomDeploymentStrategyParams{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentCause, InType: reflect.TypeOf(&DeploymentCause{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentCauseImageTrigger, InType: reflect.TypeOf(&DeploymentCauseImageTrigger{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentCondition, InType: reflect.TypeOf(&DeploymentCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentConfig, InType: reflect.TypeOf(&DeploymentConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentConfigList, InType: reflect.TypeOf(&DeploymentConfigList{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentConfigRollback, InType: reflect.TypeOf(&DeploymentConfigRollback{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentConfigRollbackSpec, InType: reflect.TypeOf(&DeploymentConfigRollbackSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentConfigSpec, InType: reflect.TypeOf(&DeploymentConfigSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentConfigStatus, InType: reflect.TypeOf(&DeploymentConfigStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentDetails, InType: reflect.TypeOf(&DeploymentDetails{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentLog, InType: reflect.TypeOf(&DeploymentLog{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentLogOptions, InType: reflect.TypeOf(&DeploymentLogOptions{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentStrategy, InType: reflect.TypeOf(&DeploymentStrategy{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentTriggerImageChangeParams, InType: reflect.TypeOf(&DeploymentTriggerImageChangeParams{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DeploymentTriggerPolicy, InType: reflect.TypeOf(&DeploymentTriggerPolicy{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ExecNewPodHook, InType: reflect.TypeOf(&ExecNewPodHook{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_LifecycleHook, InType: reflect.TypeOf(&LifecycleHook{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_RecreateDeploymentStrategyParams, InType: reflect.TypeOf(&RecreateDeploymentStrategyParams{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_RollingDeploymentStrategyParams, InType: reflect.TypeOf(&RollingDeploymentStrategyParams{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_TagImageHook, InType: reflect.TypeOf(&TagImageHook{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_TemplateImage, InType: reflect.TypeOf(&TemplateImage{})},
	)
}

func DeepCopy_api_CustomDeploymentStrategyParams(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*CustomDeploymentStrategyParams)
		out := out.(*CustomDeploymentStrategyParams)
		out.Image = in.Image
		if in.Environment != nil {
			in, out := &in.Environment, &out.Environment
			*out = make([]pkg_api.EnvVar, len(*in))
			for i := range *in {
				if err := pkg_api.DeepCopy_api_EnvVar(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		} else {
			out.Environment = nil
		}
		if in.Command != nil {
			in, out := &in.Command, &out.Command
			*out = make([]string, len(*in))
			copy(*out, *in)
		} else {
			out.Command = nil
		}
		return nil
	}
}

func DeepCopy_api_DeploymentCause(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentCause)
		out := out.(*DeploymentCause)
		out.Type = in.Type
		if in.ImageTrigger != nil {
			in, out := &in.ImageTrigger, &out.ImageTrigger
			*out = new(DeploymentCauseImageTrigger)
			**out = **in
		} else {
			out.ImageTrigger = nil
		}
		return nil
	}
}

func DeepCopy_api_DeploymentCauseImageTrigger(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentCauseImageTrigger)
		out := out.(*DeploymentCauseImageTrigger)
		out.From = in.From
		return nil
	}
}

func DeepCopy_api_DeploymentCondition(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentCondition)
		out := out.(*DeploymentCondition)
		out.Type = in.Type
		out.Status = in.Status
		out.LastTransitionTime = in.LastTransitionTime.DeepCopy()
		out.Reason = in.Reason
		out.Message = in.Message
		return nil
	}
}

func DeepCopy_api_DeploymentConfig(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentConfig)
		out := out.(*DeploymentConfig)
		out.TypeMeta = in.TypeMeta
		if err := pkg_api.DeepCopy_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, c); err != nil {
			return err
		}
		if err := DeepCopy_api_DeploymentConfigSpec(&in.Spec, &out.Spec, c); err != nil {
			return err
		}
		if err := DeepCopy_api_DeploymentConfigStatus(&in.Status, &out.Status, c); err != nil {
			return err
		}
		return nil
	}
}

func DeepCopy_api_DeploymentConfigList(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentConfigList)
		out := out.(*DeploymentConfigList)
		out.TypeMeta = in.TypeMeta
		out.ListMeta = in.ListMeta
		if in.Items != nil {
			in, out := &in.Items, &out.Items
			*out = make([]DeploymentConfig, len(*in))
			for i := range *in {
				if err := DeepCopy_api_DeploymentConfig(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		} else {
			out.Items = nil
		}
		return nil
	}
}

func DeepCopy_api_DeploymentConfigRollback(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentConfigRollback)
		out := out.(*DeploymentConfigRollback)
		out.TypeMeta = in.TypeMeta
		out.Name = in.Name
		if in.UpdatedAnnotations != nil {
			in, out := &in.UpdatedAnnotations, &out.UpdatedAnnotations
			*out = make(map[string]string)
			for key, val := range *in {
				(*out)[key] = val
			}
		} else {
			out.UpdatedAnnotations = nil
		}
		out.Spec = in.Spec
		return nil
	}
}

func DeepCopy_api_DeploymentConfigRollbackSpec(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentConfigRollbackSpec)
		out := out.(*DeploymentConfigRollbackSpec)
		out.From = in.From
		out.Revision = in.Revision
		out.IncludeTriggers = in.IncludeTriggers
		out.IncludeTemplate = in.IncludeTemplate
		out.IncludeReplicationMeta = in.IncludeReplicationMeta
		out.IncludeStrategy = in.IncludeStrategy
		return nil
	}
}

func DeepCopy_api_DeploymentConfigSpec(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentConfigSpec)
		out := out.(*DeploymentConfigSpec)
		if err := DeepCopy_api_DeploymentStrategy(&in.Strategy, &out.Strategy, c); err != nil {
			return err
		}
		out.MinReadySeconds = in.MinReadySeconds
		if in.Triggers != nil {
			in, out := &in.Triggers, &out.Triggers
			*out = make([]DeploymentTriggerPolicy, len(*in))
			for i := range *in {
				if err := DeepCopy_api_DeploymentTriggerPolicy(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		} else {
			out.Triggers = nil
		}
		out.Replicas = in.Replicas
		if in.RevisionHistoryLimit != nil {
			in, out := &in.RevisionHistoryLimit, &out.RevisionHistoryLimit
			*out = new(int32)
			**out = **in
		} else {
			out.RevisionHistoryLimit = nil
		}
		out.Test = in.Test
		out.Paused = in.Paused
		if in.Selector != nil {
			in, out := &in.Selector, &out.Selector
			*out = make(map[string]string)
			for key, val := range *in {
				(*out)[key] = val
			}
		} else {
			out.Selector = nil
		}
		if in.Template != nil {
			in, out := &in.Template, &out.Template
			*out = new(pkg_api.PodTemplateSpec)
			if err := pkg_api.DeepCopy_api_PodTemplateSpec(*in, *out, c); err != nil {
				return err
			}
		} else {
			out.Template = nil
		}
		return nil
	}
}

func DeepCopy_api_DeploymentConfigStatus(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentConfigStatus)
		out := out.(*DeploymentConfigStatus)
		out.LatestVersion = in.LatestVersion
		out.ObservedGeneration = in.ObservedGeneration
		out.Replicas = in.Replicas
		out.UpdatedReplicas = in.UpdatedReplicas
		out.AvailableReplicas = in.AvailableReplicas
		out.UnavailableReplicas = in.UnavailableReplicas
		if in.Details != nil {
			in, out := &in.Details, &out.Details
			*out = new(DeploymentDetails)
			if err := DeepCopy_api_DeploymentDetails(*in, *out, c); err != nil {
				return err
			}
		} else {
			out.Details = nil
		}
		if in.Conditions != nil {
			in, out := &in.Conditions, &out.Conditions
			*out = make([]DeploymentCondition, len(*in))
			for i := range *in {
				if err := DeepCopy_api_DeploymentCondition(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		} else {
			out.Conditions = nil
		}
		return nil
	}
}

func DeepCopy_api_DeploymentDetails(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentDetails)
		out := out.(*DeploymentDetails)
		out.Message = in.Message
		if in.Causes != nil {
			in, out := &in.Causes, &out.Causes
			*out = make([]DeploymentCause, len(*in))
			for i := range *in {
				if err := DeepCopy_api_DeploymentCause(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		} else {
			out.Causes = nil
		}
		return nil
	}
}

func DeepCopy_api_DeploymentLog(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentLog)
		out := out.(*DeploymentLog)
		out.TypeMeta = in.TypeMeta
		return nil
	}
}

func DeepCopy_api_DeploymentLogOptions(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentLogOptions)
		out := out.(*DeploymentLogOptions)
		out.TypeMeta = in.TypeMeta
		out.Container = in.Container
		out.Follow = in.Follow
		out.Previous = in.Previous
		if in.SinceSeconds != nil {
			in, out := &in.SinceSeconds, &out.SinceSeconds
			*out = new(int64)
			**out = **in
		} else {
			out.SinceSeconds = nil
		}
		if in.SinceTime != nil {
			in, out := &in.SinceTime, &out.SinceTime
			*out = new(unversioned.Time)
			**out = (*in).DeepCopy()
		} else {
			out.SinceTime = nil
		}
		out.Timestamps = in.Timestamps
		if in.TailLines != nil {
			in, out := &in.TailLines, &out.TailLines
			*out = new(int64)
			**out = **in
		} else {
			out.TailLines = nil
		}
		if in.LimitBytes != nil {
			in, out := &in.LimitBytes, &out.LimitBytes
			*out = new(int64)
			**out = **in
		} else {
			out.LimitBytes = nil
		}
		out.NoWait = in.NoWait
		if in.Version != nil {
			in, out := &in.Version, &out.Version
			*out = new(int64)
			**out = **in
		} else {
			out.Version = nil
		}
		return nil
	}
}

func DeepCopy_api_DeploymentStrategy(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentStrategy)
		out := out.(*DeploymentStrategy)
		out.Type = in.Type
		if in.RecreateParams != nil {
			in, out := &in.RecreateParams, &out.RecreateParams
			*out = new(RecreateDeploymentStrategyParams)
			if err := DeepCopy_api_RecreateDeploymentStrategyParams(*in, *out, c); err != nil {
				return err
			}
		} else {
			out.RecreateParams = nil
		}
		if in.RollingParams != nil {
			in, out := &in.RollingParams, &out.RollingParams
			*out = new(RollingDeploymentStrategyParams)
			if err := DeepCopy_api_RollingDeploymentStrategyParams(*in, *out, c); err != nil {
				return err
			}
		} else {
			out.RollingParams = nil
		}
		if in.CustomParams != nil {
			in, out := &in.CustomParams, &out.CustomParams
			*out = new(CustomDeploymentStrategyParams)
			if err := DeepCopy_api_CustomDeploymentStrategyParams(*in, *out, c); err != nil {
				return err
			}
		} else {
			out.CustomParams = nil
		}
		if err := pkg_api.DeepCopy_api_ResourceRequirements(&in.Resources, &out.Resources, c); err != nil {
			return err
		}
		if in.Labels != nil {
			in, out := &in.Labels, &out.Labels
			*out = make(map[string]string)
			for key, val := range *in {
				(*out)[key] = val
			}
		} else {
			out.Labels = nil
		}
		if in.Annotations != nil {
			in, out := &in.Annotations, &out.Annotations
			*out = make(map[string]string)
			for key, val := range *in {
				(*out)[key] = val
			}
		} else {
			out.Annotations = nil
		}
		return nil
	}
}

func DeepCopy_api_DeploymentTriggerImageChangeParams(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentTriggerImageChangeParams)
		out := out.(*DeploymentTriggerImageChangeParams)
		out.Automatic = in.Automatic
		if in.ContainerNames != nil {
			in, out := &in.ContainerNames, &out.ContainerNames
			*out = make([]string, len(*in))
			copy(*out, *in)
		} else {
			out.ContainerNames = nil
		}
		out.From = in.From
		out.LastTriggeredImage = in.LastTriggeredImage
		return nil
	}
}

func DeepCopy_api_DeploymentTriggerPolicy(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DeploymentTriggerPolicy)
		out := out.(*DeploymentTriggerPolicy)
		out.Type = in.Type
		if in.ImageChangeParams != nil {
			in, out := &in.ImageChangeParams, &out.ImageChangeParams
			*out = new(DeploymentTriggerImageChangeParams)
			if err := DeepCopy_api_DeploymentTriggerImageChangeParams(*in, *out, c); err != nil {
				return err
			}
		} else {
			out.ImageChangeParams = nil
		}
		return nil
	}
}

func DeepCopy_api_ExecNewPodHook(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ExecNewPodHook)
		out := out.(*ExecNewPodHook)
		if in.Command != nil {
			in, out := &in.Command, &out.Command
			*out = make([]string, len(*in))
			copy(*out, *in)
		} else {
			out.Command = nil
		}
		if in.Env != nil {
			in, out := &in.Env, &out.Env
			*out = make([]pkg_api.EnvVar, len(*in))
			for i := range *in {
				if err := pkg_api.DeepCopy_api_EnvVar(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		} else {
			out.Env = nil
		}
		out.ContainerName = in.ContainerName
		if in.Volumes != nil {
			in, out := &in.Volumes, &out.Volumes
			*out = make([]string, len(*in))
			copy(*out, *in)
		} else {
			out.Volumes = nil
		}
		return nil
	}
}

func DeepCopy_api_LifecycleHook(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*LifecycleHook)
		out := out.(*LifecycleHook)
		out.FailurePolicy = in.FailurePolicy
		if in.ExecNewPod != nil {
			in, out := &in.ExecNewPod, &out.ExecNewPod
			*out = new(ExecNewPodHook)
			if err := DeepCopy_api_ExecNewPodHook(*in, *out, c); err != nil {
				return err
			}
		} else {
			out.ExecNewPod = nil
		}
		if in.TagImages != nil {
			in, out := &in.TagImages, &out.TagImages
			*out = make([]TagImageHook, len(*in))
			for i := range *in {
				(*out)[i] = (*in)[i]
			}
		} else {
			out.TagImages = nil
		}
		return nil
	}
}

func DeepCopy_api_RecreateDeploymentStrategyParams(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*RecreateDeploymentStrategyParams)
		out := out.(*RecreateDeploymentStrategyParams)
		if in.TimeoutSeconds != nil {
			in, out := &in.TimeoutSeconds, &out.TimeoutSeconds
			*out = new(int64)
			**out = **in
		} else {
			out.TimeoutSeconds = nil
		}
		if in.Pre != nil {
			in, out := &in.Pre, &out.Pre
			*out = new(LifecycleHook)
			if err := DeepCopy_api_LifecycleHook(*in, *out, c); err != nil {
				return err
			}
		} else {
			out.Pre = nil
		}
		if in.Mid != nil {
			in, out := &in.Mid, &out.Mid
			*out = new(LifecycleHook)
			if err := DeepCopy_api_LifecycleHook(*in, *out, c); err != nil {
				return err
			}
		} else {
			out.Mid = nil
		}
		if in.Post != nil {
			in, out := &in.Post, &out.Post
			*out = new(LifecycleHook)
			if err := DeepCopy_api_LifecycleHook(*in, *out, c); err != nil {
				return err
			}
		} else {
			out.Post = nil
		}
		return nil
	}
}

func DeepCopy_api_RollingDeploymentStrategyParams(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*RollingDeploymentStrategyParams)
		out := out.(*RollingDeploymentStrategyParams)
		if in.UpdatePeriodSeconds != nil {
			in, out := &in.UpdatePeriodSeconds, &out.UpdatePeriodSeconds
			*out = new(int64)
			**out = **in
		} else {
			out.UpdatePeriodSeconds = nil
		}
		if in.IntervalSeconds != nil {
			in, out := &in.IntervalSeconds, &out.IntervalSeconds
			*out = new(int64)
			**out = **in
		} else {
			out.IntervalSeconds = nil
		}
		if in.TimeoutSeconds != nil {
			in, out := &in.TimeoutSeconds, &out.TimeoutSeconds
			*out = new(int64)
			**out = **in
		} else {
			out.TimeoutSeconds = nil
		}
		out.MaxUnavailable = in.MaxUnavailable
		out.MaxSurge = in.MaxSurge
		if in.Pre != nil {
			in, out := &in.Pre, &out.Pre
			*out = new(LifecycleHook)
			if err := DeepCopy_api_LifecycleHook(*in, *out, c); err != nil {
				return err
			}
		} else {
			out.Pre = nil
		}
		if in.Post != nil {
			in, out := &in.Post, &out.Post
			*out = new(LifecycleHook)
			if err := DeepCopy_api_LifecycleHook(*in, *out, c); err != nil {
				return err
			}
		} else {
			out.Post = nil
		}
		return nil
	}
}

func DeepCopy_api_TagImageHook(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*TagImageHook)
		out := out.(*TagImageHook)
		out.ContainerName = in.ContainerName
		out.To = in.To
		return nil
	}
}

func DeepCopy_api_TemplateImage(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*TemplateImage)
		out := out.(*TemplateImage)
		out.Image = in.Image
		if in.Ref != nil {
			in, out := &in.Ref, &out.Ref
			*out = new(image_api.DockerImageReference)
			**out = **in
		} else {
			out.Ref = nil
		}
		if in.From != nil {
			in, out := &in.From, &out.From
			*out = new(pkg_api.ObjectReference)
			**out = **in
		} else {
			out.From = nil
		}
		if in.Container != nil {
			in, out := &in.Container, &out.Container
			*out = new(pkg_api.Container)
			if err := pkg_api.DeepCopy_api_Container(*in, *out, c); err != nil {
				return err
			}
		} else {
			out.Container = nil
		}
		return nil
	}
}
