/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"fmt"
	"reflect"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/conversion"
)

func addConversionFuncs() {
	// Add non-generated conversion functions
	err := api.Scheme.AddConversionFuncs(
		convert_api_PodSpec_To_v1_PodSpec,
		convert_api_ReplicationControllerSpec_To_v1_ReplicationControllerSpec,
		convert_api_ServiceSpec_To_v1_ServiceSpec,
		convert_api_VolumeSource_To_v1_VolumeSource,
		convert_v1_PodSpec_To_api_PodSpec,
		convert_v1_ReplicationControllerSpec_To_api_ReplicationControllerSpec,
		convert_v1_ServiceSpec_To_api_ServiceSpec,
		convert_v1_VolumeSource_To_api_VolumeSource,
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}

	// Add field conversion funcs.
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "Pod",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name",
				"metadata.namespace",
				"metadata.labels",
				"metadata.annotations",
				"status.phase",
				"status.podIP",
				"spec.nodeName":
				return label, value, nil
				// This is for backwards compatibility with old v1 clients which send spec.host
			case "spec.host":
				return "spec.nodeName", value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "Node",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name":
				return label, value, nil
			case "spec.unschedulable":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "ReplicationController",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name",
				"status.replicas":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "Event",
		func(label, value string) (string, string, error) {
			switch label {
			case "involvedObject.kind",
				"involvedObject.namespace",
				"involvedObject.name",
				"involvedObject.uid",
				"involvedObject.apiVersion",
				"involvedObject.resourceVersion",
				"involvedObject.fieldPath",
				"reason",
				"source":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "Namespace",
		func(label, value string) (string, string, error) {
			switch label {
			case "status.phase":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "Secret",
		func(label, value string) (string, string, error) {
			switch label {
			case "type":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "ServiceAccount",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
	err = api.Scheme.AddFieldLabelConversionFunc("v1", "Endpoints",
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

func convert_api_ReplicationControllerSpec_To_v1_ReplicationControllerSpec(in *api.ReplicationControllerSpec, out *ReplicationControllerSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ReplicationControllerSpec))(in)
	}
	out.Replicas = new(int)
	*out.Replicas = in.Replicas
	if in.Selector != nil {
		out.Selector = make(map[string]string)
		for key, val := range in.Selector {
			out.Selector[key] = val
		}
	} else {
		out.Selector = nil
	}
	//if in.TemplateRef != nil {
	//	out.TemplateRef = new(ObjectReference)
	//	if err := convert_api_ObjectReference_To_v1_ObjectReference(in.TemplateRef, out.TemplateRef, s); err != nil {
	//		return err
	//	}
	//} else {
	//	out.TemplateRef = nil
	//}
	if in.Template != nil {
		out.Template = new(PodTemplateSpec)
		if err := convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(in.Template, out.Template, s); err != nil {
			return err
		}
	} else {
		out.Template = nil
	}
	return nil
}

func convert_v1_ReplicationControllerSpec_To_api_ReplicationControllerSpec(in *ReplicationControllerSpec, out *api.ReplicationControllerSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*ReplicationControllerSpec))(in)
	}
	out.Replicas = *in.Replicas
	if in.Selector != nil {
		out.Selector = make(map[string]string)
		for key, val := range in.Selector {
			out.Selector[key] = val
		}
	} else {
		out.Selector = nil
	}
	//if in.TemplateRef != nil {
	//	out.TemplateRef = new(api.ObjectReference)
	//	if err := convert_v1_ObjectReference_To_api_ObjectReference(in.TemplateRef, out.TemplateRef, s); err != nil {
	//		return err
	//	}
	//} else {
	//	out.TemplateRef = nil
	//}
	if in.Template != nil {
		out.Template = new(api.PodTemplateSpec)
		if err := convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(in.Template, out.Template, s); err != nil {
			return err
		}
	} else {
		out.Template = nil
	}
	return nil
}

// The following two PodSpec conversions are done here to support ServiceAccount
// as an alias for ServiceAccountName.
func convert_api_PodSpec_To_v1_PodSpec(in *api.PodSpec, out *PodSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PodSpec))(in)
	}
	if in.Volumes != nil {
		out.Volumes = make([]Volume, len(in.Volumes))
		for i := range in.Volumes {
			if err := convert_api_Volume_To_v1_Volume(&in.Volumes[i], &out.Volumes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Volumes = nil
	}
	if in.Containers != nil {
		out.Containers = make([]Container, len(in.Containers))
		for i := range in.Containers {
			if err := convert_api_Container_To_v1_Container(&in.Containers[i], &out.Containers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Containers = nil
	}
	out.RestartPolicy = RestartPolicy(in.RestartPolicy)
	if in.TerminationGracePeriodSeconds != nil {
		out.TerminationGracePeriodSeconds = new(int64)
		*out.TerminationGracePeriodSeconds = *in.TerminationGracePeriodSeconds
	} else {
		out.TerminationGracePeriodSeconds = nil
	}
	if in.ActiveDeadlineSeconds != nil {
		out.ActiveDeadlineSeconds = new(int64)
		*out.ActiveDeadlineSeconds = *in.ActiveDeadlineSeconds
	} else {
		out.ActiveDeadlineSeconds = nil
	}
	out.DNSPolicy = DNSPolicy(in.DNSPolicy)
	if in.NodeSelector != nil {
		out.NodeSelector = make(map[string]string)
		for key, val := range in.NodeSelector {
			out.NodeSelector[key] = val
		}
	} else {
		out.NodeSelector = nil
	}
	out.ServiceAccountName = in.ServiceAccountName
	// DeprecatedServiceAccount is an alias for ServiceAccountName.
	out.DeprecatedServiceAccount = in.ServiceAccountName
	out.NodeName = in.NodeName
	if in.SecurityContext != nil {
		out.SecurityContext = new(PodSecurityContext)
		if err := convert_api_PodSecurityContext_To_v1_PodSecurityContext(in.SecurityContext, out.SecurityContext, s); err != nil {
			return err
		}

		out.HostPID = in.SecurityContext.HostPID
		out.HostNetwork = in.SecurityContext.HostNetwork
		out.HostIPC = in.SecurityContext.HostIPC
	}
	if in.ImagePullSecrets != nil {
		out.ImagePullSecrets = make([]LocalObjectReference, len(in.ImagePullSecrets))
		for i := range in.ImagePullSecrets {
			if err := convert_api_LocalObjectReference_To_v1_LocalObjectReference(&in.ImagePullSecrets[i], &out.ImagePullSecrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.ImagePullSecrets = nil
	}

	// carry conversion
	out.DeprecatedHost = in.NodeName

	return nil
}

func convert_v1_PodSpec_To_api_PodSpec(in *PodSpec, out *api.PodSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*PodSpec))(in)
	}
	if in.Volumes != nil {
		out.Volumes = make([]api.Volume, len(in.Volumes))
		for i := range in.Volumes {
			if err := convert_v1_Volume_To_api_Volume(&in.Volumes[i], &out.Volumes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Volumes = nil
	}
	if in.Containers != nil {
		out.Containers = make([]api.Container, len(in.Containers))
		for i := range in.Containers {
			if err := convert_v1_Container_To_api_Container(&in.Containers[i], &out.Containers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Containers = nil
	}
	out.RestartPolicy = api.RestartPolicy(in.RestartPolicy)
	if in.TerminationGracePeriodSeconds != nil {
		out.TerminationGracePeriodSeconds = new(int64)
		*out.TerminationGracePeriodSeconds = *in.TerminationGracePeriodSeconds
	} else {
		out.TerminationGracePeriodSeconds = nil
	}
	if in.ActiveDeadlineSeconds != nil {
		out.ActiveDeadlineSeconds = new(int64)
		*out.ActiveDeadlineSeconds = *in.ActiveDeadlineSeconds
	} else {
		out.ActiveDeadlineSeconds = nil
	}
	out.DNSPolicy = api.DNSPolicy(in.DNSPolicy)
	if in.NodeSelector != nil {
		out.NodeSelector = make(map[string]string)
		for key, val := range in.NodeSelector {
			out.NodeSelector[key] = val
		}
	} else {
		out.NodeSelector = nil
	}
	// We support DeprecatedServiceAccount as an alias for ServiceAccountName.
	// If both are specified, ServiceAccountName (the new field) wins.
	out.ServiceAccountName = in.ServiceAccountName
	if in.ServiceAccountName == "" {
		out.ServiceAccountName = in.DeprecatedServiceAccount
	}
	out.NodeName = in.NodeName
	if in.SecurityContext != nil {
		out.SecurityContext = new(api.PodSecurityContext)
		if err := convert_v1_PodSecurityContext_To_api_PodSecurityContext(in.SecurityContext, out.SecurityContext, s); err != nil {
			return err
		}
	}
	if out.SecurityContext == nil {
		out.SecurityContext = new(api.PodSecurityContext)
	}
	out.SecurityContext.HostNetwork = in.HostNetwork
	out.SecurityContext.HostPID = in.HostPID
	out.SecurityContext.HostIPC = in.HostIPC

	// carry conversion
	if in.NodeName == "" {
		out.NodeName = in.DeprecatedHost
	}

	out.HostNetwork = in.HostNetwork
	if in.ImagePullSecrets != nil {
		out.ImagePullSecrets = make([]api.LocalObjectReference, len(in.ImagePullSecrets))
		for i := range in.ImagePullSecrets {
			if err := convert_v1_LocalObjectReference_To_api_LocalObjectReference(&in.ImagePullSecrets[i], &out.ImagePullSecrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.ImagePullSecrets = nil
	}
	return nil
}

func convert_api_ServiceSpec_To_v1_ServiceSpec(in *api.ServiceSpec, out *ServiceSpec, s conversion.Scope) error {
	if err := autoconvert_api_ServiceSpec_To_v1_ServiceSpec(in, out, s); err != nil {
		return err
	}
	// Publish both externalIPs and deprecatedPublicIPs fields in v1.
	for _, ip := range in.ExternalIPs {
		out.DeprecatedPublicIPs = append(out.DeprecatedPublicIPs, ip)
	}
	return nil
}

// This will convert our internal represantation of VolumeSource to its v1 representation
// Used for keeping backwards compatibility for the Metadata field
func convert_api_VolumeSource_To_v1_VolumeSource(in *api.VolumeSource, out *VolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.VolumeSource))(in)
	}

	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}

	if in.DownwardAPI != nil {
		out.Metadata = new(MetadataVolumeSource)
		if err := convert_api_DownwardAPIVolumeSource_To_v1_MetadataVolumeSource(in.DownwardAPI, out.Metadata, s); err != nil {
			return err
		}
	}
	return nil
}

// downward -> metadata (api -> v1)
func convert_api_DownwardAPIVolumeSource_To_v1_MetadataVolumeSource(in *api.DownwardAPIVolumeSource, out *MetadataVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.DownwardAPIVolumeSource))(in)
	}
	if in.Items != nil {
		out.Items = make([]MetadataFile, len(in.Items))
		for i := range in.Items {
			if err := convert_api_DownwardAPIVolumeFile_To_v1_MetadataFile(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	}
	return nil
}

func convert_v1_ServiceSpec_To_api_ServiceSpec(in *ServiceSpec, out *api.ServiceSpec, s conversion.Scope) error {
	if err := autoconvert_v1_ServiceSpec_To_api_ServiceSpec(in, out, s); err != nil {
		return err
	}
	// Prefer the legacy deprecatedPublicIPs field, if provided.
	if len(in.DeprecatedPublicIPs) > 0 {
		out.ExternalIPs = nil
		for _, ip := range in.DeprecatedPublicIPs {
			out.ExternalIPs = append(out.ExternalIPs, ip)
		}
	}
	return nil
}

func convert_api_DownwardAPIVolumeFile_To_v1_MetadataFile(in *api.DownwardAPIVolumeFile, out *MetadataFile, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.DownwardAPIVolumeFile))(in)
	}
	out.Name = in.Path
	if err := convert_api_ObjectFieldSelector_To_v1_ObjectFieldSelector(&in.FieldRef, &out.FieldRef, s); err != nil {
		return err
	}
	return nil
}

// This will convert the v1 representation of VolumeSource to our internal representation
// Used for keeping backwards compatibility for the Metadata field
func convert_v1_VolumeSource_To_api_VolumeSource(in *VolumeSource, out *api.VolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*VolumeSource))(in)
	}

	if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
		return err
	}

	if in.Metadata != nil {
		out.DownwardAPI = new(api.DownwardAPIVolumeSource)
		if err := convert_v1_MetadataVolumeSource_To_api_DownwardAPIVolumeSource(in.Metadata, out.DownwardAPI, s); err != nil {
			return err
		}
	}
	return nil
}

func convert_api_PodSecurityContext_To_v1_PodSecurityContext(in *api.PodSecurityContext, out *PodSecurityContext, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PodSecurityContext))(in)
	}
	return nil
}

// metadata -> downward (v1 -> api)
func convert_v1_MetadataVolumeSource_To_api_DownwardAPIVolumeSource(in *MetadataVolumeSource, out *api.DownwardAPIVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*MetadataVolumeSource))(in)
	}
	if in.Items != nil {
		out.Items = make([]api.DownwardAPIVolumeFile, len(in.Items))
		for i := range in.Items {
			if err := convert_v1_MetadataFile_To_api_DownwardAPIVolumeFile(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	}
	return nil
}

func convert_v1_PodSecurityContext_To_api_PodSecurityContext(in *PodSecurityContext, out *api.PodSecurityContext, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*PodSecurityContext))(in)
	}
	return nil
}

func convert_v1_MetadataFile_To_api_DownwardAPIVolumeFile(in *MetadataFile, out *api.DownwardAPIVolumeFile, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*MetadataFile))(in)
	}
	out.Path = in.Name
	if err := convert_v1_ObjectFieldSelector_To_api_ObjectFieldSelector(&in.FieldRef, &out.FieldRef, s); err != nil {
		return err
	}
	return nil
}
