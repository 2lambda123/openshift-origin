// +build !ignore_autogenerated

// This file was autogenerated by deepcopy-gen. Do not edit it manually!

package v1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
	unsafe "unsafe"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterNetwork) DeepCopyInto(out *ClusterNetwork) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.ClusterNetworks != nil {
		in, out := &in.ClusterNetworks, &out.ClusterNetworks
		*out = make([]ClusterNetworkEntry, len(*in))
		copy(*out, *in)
	}
	if in.VXLANPort != nil {
		in, out := &in.VXLANPort, &out.VXLANPort
		if *in == nil {
			*out = nil
		} else {
			*out = new(uint32)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterNetwork.
func (in *ClusterNetwork) DeepCopy() *ClusterNetwork {
	if in == nil {
		return nil
	}
	out := new(ClusterNetwork)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterNetwork) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterNetworkEntry) DeepCopyInto(out *ClusterNetworkEntry) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterNetworkEntry.
func (in *ClusterNetworkEntry) DeepCopy() *ClusterNetworkEntry {
	if in == nil {
		return nil
	}
	out := new(ClusterNetworkEntry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterNetworkList) DeepCopyInto(out *ClusterNetworkList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterNetwork, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterNetworkList.
func (in *ClusterNetworkList) DeepCopy() *ClusterNetworkList {
	if in == nil {
		return nil
	}
	out := new(ClusterNetworkList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterNetworkList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EgressNetworkPolicy) DeepCopyInto(out *EgressNetworkPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EgressNetworkPolicy.
func (in *EgressNetworkPolicy) DeepCopy() *EgressNetworkPolicy {
	if in == nil {
		return nil
	}
	out := new(EgressNetworkPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *EgressNetworkPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EgressNetworkPolicyList) DeepCopyInto(out *EgressNetworkPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]EgressNetworkPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EgressNetworkPolicyList.
func (in *EgressNetworkPolicyList) DeepCopy() *EgressNetworkPolicyList {
	if in == nil {
		return nil
	}
	out := new(EgressNetworkPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *EgressNetworkPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EgressNetworkPolicyPeer) DeepCopyInto(out *EgressNetworkPolicyPeer) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EgressNetworkPolicyPeer.
func (in *EgressNetworkPolicyPeer) DeepCopy() *EgressNetworkPolicyPeer {
	if in == nil {
		return nil
	}
	out := new(EgressNetworkPolicyPeer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EgressNetworkPolicyRule) DeepCopyInto(out *EgressNetworkPolicyRule) {
	*out = *in
	out.To = in.To
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EgressNetworkPolicyRule.
func (in *EgressNetworkPolicyRule) DeepCopy() *EgressNetworkPolicyRule {
	if in == nil {
		return nil
	}
	out := new(EgressNetworkPolicyRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EgressNetworkPolicyRuleType) DeepCopyInto(out *EgressNetworkPolicyRuleType) {
	{
		in := (*string)(unsafe.Pointer(in))
		out := (*string)(unsafe.Pointer(out))
		*out = *in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EgressNetworkPolicyRuleType.
func (in *EgressNetworkPolicyRuleType) DeepCopy() *EgressNetworkPolicyRuleType {
	if in == nil {
		return nil
	}
	out := new(EgressNetworkPolicyRuleType)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EgressNetworkPolicySpec) DeepCopyInto(out *EgressNetworkPolicySpec) {
	*out = *in
	if in.Egress != nil {
		in, out := &in.Egress, &out.Egress
		*out = make([]EgressNetworkPolicyRule, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EgressNetworkPolicySpec.
func (in *EgressNetworkPolicySpec) DeepCopy() *EgressNetworkPolicySpec {
	if in == nil {
		return nil
	}
	out := new(EgressNetworkPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HostSubnet) DeepCopyInto(out *HostSubnet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.EgressIPs != nil {
		in, out := &in.EgressIPs, &out.EgressIPs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HostSubnet.
func (in *HostSubnet) DeepCopy() *HostSubnet {
	if in == nil {
		return nil
	}
	out := new(HostSubnet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *HostSubnet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HostSubnetList) DeepCopyInto(out *HostSubnetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]HostSubnet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HostSubnetList.
func (in *HostSubnetList) DeepCopy() *HostSubnetList {
	if in == nil {
		return nil
	}
	out := new(HostSubnetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *HostSubnetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetNamespace) DeepCopyInto(out *NetNamespace) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.EgressIPs != nil {
		in, out := &in.EgressIPs, &out.EgressIPs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetNamespace.
func (in *NetNamespace) DeepCopy() *NetNamespace {
	if in == nil {
		return nil
	}
	out := new(NetNamespace)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NetNamespace) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetNamespaceList) DeepCopyInto(out *NetNamespaceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NetNamespace, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetNamespaceList.
func (in *NetNamespaceList) DeepCopy() *NetNamespaceList {
	if in == nil {
		return nil
	}
	out := new(NetNamespaceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NetNamespaceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}
