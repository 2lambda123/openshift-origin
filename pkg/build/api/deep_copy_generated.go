// +build !ignore_autogenerated_openshift

// This file was autogenerated by deepcopy-gen. Do not edit it manually!

package api

import (
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	conversion "k8s.io/kubernetes/pkg/conversion"
)

func init() {
	if err := api.Scheme.AddGeneratedDeepCopyFuncs(
		DeepCopy_api_BinaryBuildRequestOptions,
		DeepCopy_api_BinaryBuildSource,
		DeepCopy_api_Build,
		DeepCopy_api_BuildConfig,
		DeepCopy_api_BuildConfigList,
		DeepCopy_api_BuildConfigSpec,
		DeepCopy_api_BuildConfigStatus,
		DeepCopy_api_BuildList,
		DeepCopy_api_BuildLog,
		DeepCopy_api_BuildLogOptions,
		DeepCopy_api_BuildOutput,
		DeepCopy_api_BuildPostCommitSpec,
		DeepCopy_api_BuildRequest,
		DeepCopy_api_BuildSource,
		DeepCopy_api_BuildSpec,
		DeepCopy_api_BuildStatus,
		DeepCopy_api_BuildStrategy,
		DeepCopy_api_BuildTriggerCause,
		DeepCopy_api_BuildTriggerPolicy,
		DeepCopy_api_CommonSpec,
		DeepCopy_api_CustomBuildStrategy,
		DeepCopy_api_DockerBuildStrategy,
		DeepCopy_api_GenericWebHookCause,
		DeepCopy_api_GenericWebHookEvent,
		DeepCopy_api_GitBuildSource,
		DeepCopy_api_GitHubWebHookCause,
		DeepCopy_api_GitInfo,
		DeepCopy_api_GitRefInfo,
		DeepCopy_api_GitSourceRevision,
		DeepCopy_api_ImageChangeCause,
		DeepCopy_api_ImageChangeTrigger,
		DeepCopy_api_ImageSource,
		DeepCopy_api_ImageSourcePath,
		DeepCopy_api_JenkinsPipelineBuildStrategy,
		DeepCopy_api_SecretBuildSource,
		DeepCopy_api_SecretSpec,
		DeepCopy_api_SourceBuildStrategy,
		DeepCopy_api_SourceControlUser,
		DeepCopy_api_SourceRevision,
		DeepCopy_api_WebHookTrigger,
	); err != nil {
		// if one of the deep copy functions is malformed, detect it immediately.
		panic(err)
	}
}

func DeepCopy_api_BinaryBuildRequestOptions(in BinaryBuildRequestOptions, out *BinaryBuildRequestOptions, c *conversion.Cloner) error {
	if err := unversioned.DeepCopy_unversioned_TypeMeta(in.TypeMeta, &out.TypeMeta, c); err != nil {
		return err
	}
	if err := api.DeepCopy_api_ObjectMeta(in.ObjectMeta, &out.ObjectMeta, c); err != nil {
		return err
	}
	out.AsFile = in.AsFile
	out.Commit = in.Commit
	out.Message = in.Message
	out.AuthorName = in.AuthorName
	out.AuthorEmail = in.AuthorEmail
	out.CommitterName = in.CommitterName
	out.CommitterEmail = in.CommitterEmail
	return nil
}

func DeepCopy_api_BinaryBuildSource(in BinaryBuildSource, out *BinaryBuildSource, c *conversion.Cloner) error {
	out.AsFile = in.AsFile
	return nil
}

func DeepCopy_api_Build(in Build, out *Build, c *conversion.Cloner) error {
	if err := unversioned.DeepCopy_unversioned_TypeMeta(in.TypeMeta, &out.TypeMeta, c); err != nil {
		return err
	}
	if err := api.DeepCopy_api_ObjectMeta(in.ObjectMeta, &out.ObjectMeta, c); err != nil {
		return err
	}
	if err := DeepCopy_api_BuildSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := DeepCopy_api_BuildStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func DeepCopy_api_BuildConfig(in BuildConfig, out *BuildConfig, c *conversion.Cloner) error {
	if err := unversioned.DeepCopy_unversioned_TypeMeta(in.TypeMeta, &out.TypeMeta, c); err != nil {
		return err
	}
	if err := api.DeepCopy_api_ObjectMeta(in.ObjectMeta, &out.ObjectMeta, c); err != nil {
		return err
	}
	if err := DeepCopy_api_BuildConfigSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := DeepCopy_api_BuildConfigStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func DeepCopy_api_BuildConfigList(in BuildConfigList, out *BuildConfigList, c *conversion.Cloner) error {
	if err := unversioned.DeepCopy_unversioned_TypeMeta(in.TypeMeta, &out.TypeMeta, c); err != nil {
		return err
	}
	if err := unversioned.DeepCopy_unversioned_ListMeta(in.ListMeta, &out.ListMeta, c); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := in.Items, &out.Items
		*out = make([]BuildConfig, len(in))
		for i := range in {
			if err := DeepCopy_api_BuildConfig(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func DeepCopy_api_BuildConfigSpec(in BuildConfigSpec, out *BuildConfigSpec, c *conversion.Cloner) error {
	if in.Triggers != nil {
		in, out := in.Triggers, &out.Triggers
		*out = make([]BuildTriggerPolicy, len(in))
		for i := range in {
			if err := DeepCopy_api_BuildTriggerPolicy(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	out.RunPolicy = in.RunPolicy
	if err := DeepCopy_api_CommonSpec(in.CommonSpec, &out.CommonSpec, c); err != nil {
		return err
	}
	return nil
}

func DeepCopy_api_BuildConfigStatus(in BuildConfigStatus, out *BuildConfigStatus, c *conversion.Cloner) error {
	out.LastVersion = in.LastVersion
	return nil
}

func DeepCopy_api_BuildList(in BuildList, out *BuildList, c *conversion.Cloner) error {
	if err := unversioned.DeepCopy_unversioned_TypeMeta(in.TypeMeta, &out.TypeMeta, c); err != nil {
		return err
	}
	if err := unversioned.DeepCopy_unversioned_ListMeta(in.ListMeta, &out.ListMeta, c); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := in.Items, &out.Items
		*out = make([]Build, len(in))
		for i := range in {
			if err := DeepCopy_api_Build(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func DeepCopy_api_BuildLog(in BuildLog, out *BuildLog, c *conversion.Cloner) error {
	if err := unversioned.DeepCopy_unversioned_TypeMeta(in.TypeMeta, &out.TypeMeta, c); err != nil {
		return err
	}
	return nil
}

func DeepCopy_api_BuildLogOptions(in BuildLogOptions, out *BuildLogOptions, c *conversion.Cloner) error {
	if err := unversioned.DeepCopy_unversioned_TypeMeta(in.TypeMeta, &out.TypeMeta, c); err != nil {
		return err
	}
	out.Container = in.Container
	out.Follow = in.Follow
	out.Previous = in.Previous
	if in.SinceSeconds != nil {
		in, out := in.SinceSeconds, &out.SinceSeconds
		*out = new(int64)
		**out = *in
	} else {
		out.SinceSeconds = nil
	}
	if in.SinceTime != nil {
		in, out := in.SinceTime, &out.SinceTime
		*out = new(unversioned.Time)
		if err := unversioned.DeepCopy_unversioned_Time(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.SinceTime = nil
	}
	out.Timestamps = in.Timestamps
	if in.TailLines != nil {
		in, out := in.TailLines, &out.TailLines
		*out = new(int64)
		**out = *in
	} else {
		out.TailLines = nil
	}
	if in.LimitBytes != nil {
		in, out := in.LimitBytes, &out.LimitBytes
		*out = new(int64)
		**out = *in
	} else {
		out.LimitBytes = nil
	}
	out.NoWait = in.NoWait
	if in.Version != nil {
		in, out := in.Version, &out.Version
		*out = new(int64)
		**out = *in
	} else {
		out.Version = nil
	}
	return nil
}

func DeepCopy_api_BuildOutput(in BuildOutput, out *BuildOutput, c *conversion.Cloner) error {
	if in.To != nil {
		in, out := in.To, &out.To
		*out = new(api.ObjectReference)
		if err := api.DeepCopy_api_ObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.To = nil
	}
	if in.PushSecret != nil {
		in, out := in.PushSecret, &out.PushSecret
		*out = new(api.LocalObjectReference)
		if err := api.DeepCopy_api_LocalObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.PushSecret = nil
	}
	return nil
}

func DeepCopy_api_BuildPostCommitSpec(in BuildPostCommitSpec, out *BuildPostCommitSpec, c *conversion.Cloner) error {
	if in.Command != nil {
		in, out := in.Command, &out.Command
		*out = make([]string, len(in))
		copy(*out, in)
	} else {
		out.Command = nil
	}
	if in.Args != nil {
		in, out := in.Args, &out.Args
		*out = make([]string, len(in))
		copy(*out, in)
	} else {
		out.Args = nil
	}
	out.Script = in.Script
	return nil
}

func DeepCopy_api_BuildRequest(in BuildRequest, out *BuildRequest, c *conversion.Cloner) error {
	if err := unversioned.DeepCopy_unversioned_TypeMeta(in.TypeMeta, &out.TypeMeta, c); err != nil {
		return err
	}
	if err := api.DeepCopy_api_ObjectMeta(in.ObjectMeta, &out.ObjectMeta, c); err != nil {
		return err
	}
	if in.Revision != nil {
		in, out := in.Revision, &out.Revision
		*out = new(SourceRevision)
		if err := DeepCopy_api_SourceRevision(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if in.TriggeredByImage != nil {
		in, out := in.TriggeredByImage, &out.TriggeredByImage
		*out = new(api.ObjectReference)
		if err := api.DeepCopy_api_ObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.TriggeredByImage = nil
	}
	if in.From != nil {
		in, out := in.From, &out.From
		*out = new(api.ObjectReference)
		if err := api.DeepCopy_api_ObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	if in.Binary != nil {
		in, out := in.Binary, &out.Binary
		*out = new(BinaryBuildSource)
		if err := DeepCopy_api_BinaryBuildSource(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.Binary = nil
	}
	if in.LastVersion != nil {
		in, out := in.LastVersion, &out.LastVersion
		*out = new(int64)
		**out = *in
	} else {
		out.LastVersion = nil
	}
	if in.Env != nil {
		in, out := in.Env, &out.Env
		*out = make([]api.EnvVar, len(in))
		for i := range in {
			if err := api.DeepCopy_api_EnvVar(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	if in.TriggeredBy != nil {
		in, out := in.TriggeredBy, &out.TriggeredBy
		*out = make([]BuildTriggerCause, len(in))
		for i := range in {
			if err := DeepCopy_api_BuildTriggerCause(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.TriggeredBy = nil
	}
	return nil
}

func DeepCopy_api_BuildSource(in BuildSource, out *BuildSource, c *conversion.Cloner) error {
	if in.Binary != nil {
		in, out := in.Binary, &out.Binary
		*out = new(BinaryBuildSource)
		if err := DeepCopy_api_BinaryBuildSource(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.Binary = nil
	}
	if in.Dockerfile != nil {
		in, out := in.Dockerfile, &out.Dockerfile
		*out = new(string)
		**out = *in
	} else {
		out.Dockerfile = nil
	}
	if in.Git != nil {
		in, out := in.Git, &out.Git
		*out = new(GitBuildSource)
		if err := DeepCopy_api_GitBuildSource(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	if in.Images != nil {
		in, out := in.Images, &out.Images
		*out = make([]ImageSource, len(in))
		for i := range in {
			if err := DeepCopy_api_ImageSource(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Images = nil
	}
	out.ContextDir = in.ContextDir
	if in.SourceSecret != nil {
		in, out := in.SourceSecret, &out.SourceSecret
		*out = new(api.LocalObjectReference)
		if err := api.DeepCopy_api_LocalObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.SourceSecret = nil
	}
	if in.Secrets != nil {
		in, out := in.Secrets, &out.Secrets
		*out = make([]SecretBuildSource, len(in))
		for i := range in {
			if err := DeepCopy_api_SecretBuildSource(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	return nil
}

func DeepCopy_api_BuildSpec(in BuildSpec, out *BuildSpec, c *conversion.Cloner) error {
	if err := DeepCopy_api_CommonSpec(in.CommonSpec, &out.CommonSpec, c); err != nil {
		return err
	}
	if in.TriggeredBy != nil {
		in, out := in.TriggeredBy, &out.TriggeredBy
		*out = make([]BuildTriggerCause, len(in))
		for i := range in {
			if err := DeepCopy_api_BuildTriggerCause(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.TriggeredBy = nil
	}
	return nil
}

func DeepCopy_api_BuildStatus(in BuildStatus, out *BuildStatus, c *conversion.Cloner) error {
	out.Phase = in.Phase
	out.Cancelled = in.Cancelled
	out.Reason = in.Reason
	out.Message = in.Message
	if in.StartTimestamp != nil {
		in, out := in.StartTimestamp, &out.StartTimestamp
		*out = new(unversioned.Time)
		if err := unversioned.DeepCopy_unversioned_Time(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.StartTimestamp = nil
	}
	if in.CompletionTimestamp != nil {
		in, out := in.CompletionTimestamp, &out.CompletionTimestamp
		*out = new(unversioned.Time)
		if err := unversioned.DeepCopy_unversioned_Time(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.CompletionTimestamp = nil
	}
	out.Duration = in.Duration
	out.OutputDockerImageReference = in.OutputDockerImageReference
	if in.Config != nil {
		in, out := in.Config, &out.Config
		*out = new(api.ObjectReference)
		if err := api.DeepCopy_api_ObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.Config = nil
	}
	return nil
}

func DeepCopy_api_BuildStrategy(in BuildStrategy, out *BuildStrategy, c *conversion.Cloner) error {
	if in.DockerStrategy != nil {
		in, out := in.DockerStrategy, &out.DockerStrategy
		*out = new(DockerBuildStrategy)
		if err := DeepCopy_api_DockerBuildStrategy(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.DockerStrategy = nil
	}
	if in.SourceStrategy != nil {
		in, out := in.SourceStrategy, &out.SourceStrategy
		*out = new(SourceBuildStrategy)
		if err := DeepCopy_api_SourceBuildStrategy(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.SourceStrategy = nil
	}
	if in.CustomStrategy != nil {
		in, out := in.CustomStrategy, &out.CustomStrategy
		*out = new(CustomBuildStrategy)
		if err := DeepCopy_api_CustomBuildStrategy(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.CustomStrategy = nil
	}
	if in.JenkinsPipelineStrategy != nil {
		in, out := in.JenkinsPipelineStrategy, &out.JenkinsPipelineStrategy
		*out = new(JenkinsPipelineBuildStrategy)
		if err := DeepCopy_api_JenkinsPipelineBuildStrategy(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.JenkinsPipelineStrategy = nil
	}
	return nil
}

func DeepCopy_api_BuildTriggerCause(in BuildTriggerCause, out *BuildTriggerCause, c *conversion.Cloner) error {
	out.Message = in.Message
	if in.GenericWebHook != nil {
		in, out := in.GenericWebHook, &out.GenericWebHook
		*out = new(GenericWebHookCause)
		if err := DeepCopy_api_GenericWebHookCause(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.GenericWebHook = nil
	}
	if in.GitHubWebHook != nil {
		in, out := in.GitHubWebHook, &out.GitHubWebHook
		*out = new(GitHubWebHookCause)
		if err := DeepCopy_api_GitHubWebHookCause(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.GitHubWebHook = nil
	}
	if in.ImageChangeBuild != nil {
		in, out := in.ImageChangeBuild, &out.ImageChangeBuild
		*out = new(ImageChangeCause)
		if err := DeepCopy_api_ImageChangeCause(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.ImageChangeBuild = nil
	}
	return nil
}

func DeepCopy_api_BuildTriggerPolicy(in BuildTriggerPolicy, out *BuildTriggerPolicy, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.GitHubWebHook != nil {
		in, out := in.GitHubWebHook, &out.GitHubWebHook
		*out = new(WebHookTrigger)
		if err := DeepCopy_api_WebHookTrigger(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.GitHubWebHook = nil
	}
	if in.GenericWebHook != nil {
		in, out := in.GenericWebHook, &out.GenericWebHook
		*out = new(WebHookTrigger)
		if err := DeepCopy_api_WebHookTrigger(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.GenericWebHook = nil
	}
	if in.ImageChange != nil {
		in, out := in.ImageChange, &out.ImageChange
		*out = new(ImageChangeTrigger)
		if err := DeepCopy_api_ImageChangeTrigger(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.ImageChange = nil
	}
	return nil
}

func DeepCopy_api_CommonSpec(in CommonSpec, out *CommonSpec, c *conversion.Cloner) error {
	out.ServiceAccount = in.ServiceAccount
	if err := DeepCopy_api_BuildSource(in.Source, &out.Source, c); err != nil {
		return err
	}
	if in.Revision != nil {
		in, out := in.Revision, &out.Revision
		*out = new(SourceRevision)
		if err := DeepCopy_api_SourceRevision(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if err := DeepCopy_api_BuildStrategy(in.Strategy, &out.Strategy, c); err != nil {
		return err
	}
	if err := DeepCopy_api_BuildOutput(in.Output, &out.Output, c); err != nil {
		return err
	}
	if err := api.DeepCopy_api_ResourceRequirements(in.Resources, &out.Resources, c); err != nil {
		return err
	}
	if err := DeepCopy_api_BuildPostCommitSpec(in.PostCommit, &out.PostCommit, c); err != nil {
		return err
	}
	if in.CompletionDeadlineSeconds != nil {
		in, out := in.CompletionDeadlineSeconds, &out.CompletionDeadlineSeconds
		*out = new(int64)
		**out = *in
	} else {
		out.CompletionDeadlineSeconds = nil
	}
	return nil
}

func DeepCopy_api_CustomBuildStrategy(in CustomBuildStrategy, out *CustomBuildStrategy, c *conversion.Cloner) error {
	if err := api.DeepCopy_api_ObjectReference(in.From, &out.From, c); err != nil {
		return err
	}
	if in.PullSecret != nil {
		in, out := in.PullSecret, &out.PullSecret
		*out = new(api.LocalObjectReference)
		if err := api.DeepCopy_api_LocalObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		in, out := in.Env, &out.Env
		*out = make([]api.EnvVar, len(in))
		for i := range in {
			if err := api.DeepCopy_api_EnvVar(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.ExposeDockerSocket = in.ExposeDockerSocket
	out.ForcePull = in.ForcePull
	if in.Secrets != nil {
		in, out := in.Secrets, &out.Secrets
		*out = make([]SecretSpec, len(in))
		for i := range in {
			if err := DeepCopy_api_SecretSpec(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	out.BuildAPIVersion = in.BuildAPIVersion
	return nil
}

func DeepCopy_api_DockerBuildStrategy(in DockerBuildStrategy, out *DockerBuildStrategy, c *conversion.Cloner) error {
	if in.From != nil {
		in, out := in.From, &out.From
		*out = new(api.ObjectReference)
		if err := api.DeepCopy_api_ObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	if in.PullSecret != nil {
		in, out := in.PullSecret, &out.PullSecret
		*out = new(api.LocalObjectReference)
		if err := api.DeepCopy_api_LocalObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	out.NoCache = in.NoCache
	if in.Env != nil {
		in, out := in.Env, &out.Env
		*out = make([]api.EnvVar, len(in))
		for i := range in {
			if err := api.DeepCopy_api_EnvVar(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.ForcePull = in.ForcePull
	out.DockerfilePath = in.DockerfilePath
	return nil
}

func DeepCopy_api_GenericWebHookCause(in GenericWebHookCause, out *GenericWebHookCause, c *conversion.Cloner) error {
	if in.Revision != nil {
		in, out := in.Revision, &out.Revision
		*out = new(SourceRevision)
		if err := DeepCopy_api_SourceRevision(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	out.Secret = in.Secret
	return nil
}

func DeepCopy_api_GenericWebHookEvent(in GenericWebHookEvent, out *GenericWebHookEvent, c *conversion.Cloner) error {
	if in.Git != nil {
		in, out := in.Git, &out.Git
		*out = new(GitInfo)
		if err := DeepCopy_api_GitInfo(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	if in.Env != nil {
		in, out := in.Env, &out.Env
		*out = make([]api.EnvVar, len(in))
		for i := range in {
			if err := api.DeepCopy_api_EnvVar(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	return nil
}

func DeepCopy_api_GitBuildSource(in GitBuildSource, out *GitBuildSource, c *conversion.Cloner) error {
	out.URI = in.URI
	out.Ref = in.Ref
	if in.HTTPProxy != nil {
		in, out := in.HTTPProxy, &out.HTTPProxy
		*out = new(string)
		**out = *in
	} else {
		out.HTTPProxy = nil
	}
	if in.HTTPSProxy != nil {
		in, out := in.HTTPSProxy, &out.HTTPSProxy
		*out = new(string)
		**out = *in
	} else {
		out.HTTPSProxy = nil
	}
	return nil
}

func DeepCopy_api_GitHubWebHookCause(in GitHubWebHookCause, out *GitHubWebHookCause, c *conversion.Cloner) error {
	if in.Revision != nil {
		in, out := in.Revision, &out.Revision
		*out = new(SourceRevision)
		if err := DeepCopy_api_SourceRevision(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	out.Secret = in.Secret
	return nil
}

func DeepCopy_api_GitInfo(in GitInfo, out *GitInfo, c *conversion.Cloner) error {
	if err := DeepCopy_api_GitBuildSource(in.GitBuildSource, &out.GitBuildSource, c); err != nil {
		return err
	}
	if err := DeepCopy_api_GitSourceRevision(in.GitSourceRevision, &out.GitSourceRevision, c); err != nil {
		return err
	}
	if in.Refs != nil {
		in, out := in.Refs, &out.Refs
		*out = make([]GitRefInfo, len(in))
		for i := range in {
			if err := DeepCopy_api_GitRefInfo(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Refs = nil
	}
	return nil
}

func DeepCopy_api_GitRefInfo(in GitRefInfo, out *GitRefInfo, c *conversion.Cloner) error {
	if err := DeepCopy_api_GitBuildSource(in.GitBuildSource, &out.GitBuildSource, c); err != nil {
		return err
	}
	if err := DeepCopy_api_GitSourceRevision(in.GitSourceRevision, &out.GitSourceRevision, c); err != nil {
		return err
	}
	return nil
}

func DeepCopy_api_GitSourceRevision(in GitSourceRevision, out *GitSourceRevision, c *conversion.Cloner) error {
	out.Commit = in.Commit
	if err := DeepCopy_api_SourceControlUser(in.Author, &out.Author, c); err != nil {
		return err
	}
	if err := DeepCopy_api_SourceControlUser(in.Committer, &out.Committer, c); err != nil {
		return err
	}
	out.Message = in.Message
	return nil
}

func DeepCopy_api_ImageChangeCause(in ImageChangeCause, out *ImageChangeCause, c *conversion.Cloner) error {
	out.ImageID = in.ImageID
	if in.FromRef != nil {
		in, out := in.FromRef, &out.FromRef
		*out = new(api.ObjectReference)
		if err := api.DeepCopy_api_ObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.FromRef = nil
	}
	return nil
}

func DeepCopy_api_ImageChangeTrigger(in ImageChangeTrigger, out *ImageChangeTrigger, c *conversion.Cloner) error {
	out.LastTriggeredImageID = in.LastTriggeredImageID
	if in.From != nil {
		in, out := in.From, &out.From
		*out = new(api.ObjectReference)
		if err := api.DeepCopy_api_ObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	return nil
}

func DeepCopy_api_ImageSource(in ImageSource, out *ImageSource, c *conversion.Cloner) error {
	if err := api.DeepCopy_api_ObjectReference(in.From, &out.From, c); err != nil {
		return err
	}
	if in.Paths != nil {
		in, out := in.Paths, &out.Paths
		*out = make([]ImageSourcePath, len(in))
		for i := range in {
			if err := DeepCopy_api_ImageSourcePath(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Paths = nil
	}
	if in.PullSecret != nil {
		in, out := in.PullSecret, &out.PullSecret
		*out = new(api.LocalObjectReference)
		if err := api.DeepCopy_api_LocalObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	return nil
}

func DeepCopy_api_ImageSourcePath(in ImageSourcePath, out *ImageSourcePath, c *conversion.Cloner) error {
	out.SourcePath = in.SourcePath
	out.DestinationDir = in.DestinationDir
	return nil
}

func DeepCopy_api_JenkinsPipelineBuildStrategy(in JenkinsPipelineBuildStrategy, out *JenkinsPipelineBuildStrategy, c *conversion.Cloner) error {
	out.JenkinsfilePath = in.JenkinsfilePath
	out.Jenkinsfile = in.Jenkinsfile
	return nil
}

func DeepCopy_api_SecretBuildSource(in SecretBuildSource, out *SecretBuildSource, c *conversion.Cloner) error {
	if err := api.DeepCopy_api_LocalObjectReference(in.Secret, &out.Secret, c); err != nil {
		return err
	}
	out.DestinationDir = in.DestinationDir
	return nil
}

func DeepCopy_api_SecretSpec(in SecretSpec, out *SecretSpec, c *conversion.Cloner) error {
	if err := api.DeepCopy_api_LocalObjectReference(in.SecretSource, &out.SecretSource, c); err != nil {
		return err
	}
	out.MountPath = in.MountPath
	return nil
}

func DeepCopy_api_SourceBuildStrategy(in SourceBuildStrategy, out *SourceBuildStrategy, c *conversion.Cloner) error {
	if err := api.DeepCopy_api_ObjectReference(in.From, &out.From, c); err != nil {
		return err
	}
	if in.PullSecret != nil {
		in, out := in.PullSecret, &out.PullSecret
		*out = new(api.LocalObjectReference)
		if err := api.DeepCopy_api_LocalObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		in, out := in.Env, &out.Env
		*out = make([]api.EnvVar, len(in))
		for i := range in {
			if err := api.DeepCopy_api_EnvVar(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.Scripts = in.Scripts
	out.Incremental = in.Incremental
	out.ForcePull = in.ForcePull
	if in.RuntimeImage != nil {
		in, out := in.RuntimeImage, &out.RuntimeImage
		*out = new(api.ObjectReference)
		if err := api.DeepCopy_api_ObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.RuntimeImage = nil
	}
	if in.RuntimeArtifacts != nil {
		in, out := in.RuntimeArtifacts, &out.RuntimeArtifacts
		*out = make([]ImageSourcePath, len(in))
		for i := range in {
			if err := DeepCopy_api_ImageSourcePath(in[i], &(*out)[i], c); err != nil {
				return err
			}
		}
	} else {
		out.RuntimeArtifacts = nil
	}
	if in.RuntimePullSecret != nil {
		in, out := in.RuntimePullSecret, &out.RuntimePullSecret
		*out = new(api.LocalObjectReference)
		if err := api.DeepCopy_api_LocalObjectReference(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.RuntimePullSecret = nil
	}
	return nil
}

func DeepCopy_api_SourceControlUser(in SourceControlUser, out *SourceControlUser, c *conversion.Cloner) error {
	out.Name = in.Name
	out.Email = in.Email
	return nil
}

func DeepCopy_api_SourceRevision(in SourceRevision, out *SourceRevision, c *conversion.Cloner) error {
	if in.Git != nil {
		in, out := in.Git, &out.Git
		*out = new(GitSourceRevision)
		if err := DeepCopy_api_GitSourceRevision(*in, *out, c); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	return nil
}

func DeepCopy_api_WebHookTrigger(in WebHookTrigger, out *WebHookTrigger, c *conversion.Cloner) error {
	out.Secret = in.Secret
	out.AllowEnv = in.AllowEnv
	return nil
}
