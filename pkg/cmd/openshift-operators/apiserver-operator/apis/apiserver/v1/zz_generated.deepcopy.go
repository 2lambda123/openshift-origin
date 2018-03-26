// +build !ignore_autogenerated

// This file was autogenerated by deepcopy-gen. Do not edit it manually!

package v1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerConfig) DeepCopyInto(out *APIServerConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerConfig.
func (in *APIServerConfig) DeepCopy() *APIServerConfig {
	if in == nil {
		return nil
	}
	out := new(APIServerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenShiftAPIServerConfig) DeepCopyInto(out *OpenShiftAPIServerConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenShiftAPIServerConfig.
func (in *OpenShiftAPIServerConfig) DeepCopy() *OpenShiftAPIServerConfig {
	if in == nil {
		return nil
	}
	out := new(OpenShiftAPIServerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenShiftAPIServerConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenShiftAPIServerConfigList) DeepCopyInto(out *OpenShiftAPIServerConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OpenShiftAPIServerConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenShiftAPIServerConfigList.
func (in *OpenShiftAPIServerConfigList) DeepCopy() *OpenShiftAPIServerConfigList {
	if in == nil {
		return nil
	}
	out := new(OpenShiftAPIServerConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OpenShiftAPIServerConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenShiftAPIServerConfigSpec) DeepCopyInto(out *OpenShiftAPIServerConfigSpec) {
	*out = *in
	out.APIServerConfig = in.APIServerConfig
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenShiftAPIServerConfigSpec.
func (in *OpenShiftAPIServerConfigSpec) DeepCopy() *OpenShiftAPIServerConfigSpec {
	if in == nil {
		return nil
	}
	out := new(OpenShiftAPIServerConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenShiftAPIServerConfigStatus) DeepCopyInto(out *OpenShiftAPIServerConfigStatus) {
	*out = *in
	if in.LastUnsuccessfulRunErrors != nil {
		in, out := &in.LastUnsuccessfulRunErrors, &out.LastUnsuccessfulRunErrors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenShiftAPIServerConfigStatus.
func (in *OpenShiftAPIServerConfigStatus) DeepCopy() *OpenShiftAPIServerConfigStatus {
	if in == nil {
		return nil
	}
	out := new(OpenShiftAPIServerConfigStatus)
	in.DeepCopyInto(out)
	return out
}
