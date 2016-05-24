// +build !ignore_autogenerated

// This file was autogenerated by conversion-gen. Do not edit it manually!

package v1

import (
	sdn_api "github.com/openshift/origin/pkg/sdn/api"
	api "k8s.io/kubernetes/pkg/api"
	conversion "k8s.io/kubernetes/pkg/conversion"
)

func init() {
	if err := api.Scheme.AddGeneratedConversionFuncs(
		Convert_v1_ClusterNetwork_To_api_ClusterNetwork,
		Convert_api_ClusterNetwork_To_v1_ClusterNetwork,
		Convert_v1_ClusterNetworkList_To_api_ClusterNetworkList,
		Convert_api_ClusterNetworkList_To_v1_ClusterNetworkList,
		Convert_v1_HostSubnet_To_api_HostSubnet,
		Convert_api_HostSubnet_To_v1_HostSubnet,
		Convert_v1_HostSubnetList_To_api_HostSubnetList,
		Convert_api_HostSubnetList_To_v1_HostSubnetList,
		Convert_v1_NetNamespace_To_api_NetNamespace,
		Convert_api_NetNamespace_To_v1_NetNamespace,
		Convert_v1_NetNamespaceList_To_api_NetNamespaceList,
		Convert_api_NetNamespaceList_To_v1_NetNamespaceList,
	); err != nil {
		// if one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

func autoConvert_v1_ClusterNetwork_To_api_ClusterNetwork(in *ClusterNetwork, out *sdn_api.ClusterNetwork, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	// TODO: Inefficient conversion - can we improve it?
	if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
		return err
	}
	out.Network = in.Network
	out.HostSubnetLength = in.HostSubnetLength
	out.ServiceNetwork = in.ServiceNetwork
	return nil
}

func Convert_v1_ClusterNetwork_To_api_ClusterNetwork(in *ClusterNetwork, out *sdn_api.ClusterNetwork, s conversion.Scope) error {
	return autoConvert_v1_ClusterNetwork_To_api_ClusterNetwork(in, out, s)
}

func autoConvert_api_ClusterNetwork_To_v1_ClusterNetwork(in *sdn_api.ClusterNetwork, out *ClusterNetwork, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	// TODO: Inefficient conversion - can we improve it?
	if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
		return err
	}
	out.Network = in.Network
	out.HostSubnetLength = in.HostSubnetLength
	out.ServiceNetwork = in.ServiceNetwork
	return nil
}

func Convert_api_ClusterNetwork_To_v1_ClusterNetwork(in *sdn_api.ClusterNetwork, out *ClusterNetwork, s conversion.Scope) error {
	return autoConvert_api_ClusterNetwork_To_v1_ClusterNetwork(in, out, s)
}

func autoConvert_v1_ClusterNetworkList_To_api_ClusterNetworkList(in *ClusterNetworkList, out *sdn_api.ClusterNetworkList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]sdn_api.ClusterNetwork, len(*in))
		for i := range *in {
			if err := Convert_v1_ClusterNetwork_To_api_ClusterNetwork(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ClusterNetworkList_To_api_ClusterNetworkList(in *ClusterNetworkList, out *sdn_api.ClusterNetworkList, s conversion.Scope) error {
	return autoConvert_v1_ClusterNetworkList_To_api_ClusterNetworkList(in, out, s)
}

func autoConvert_api_ClusterNetworkList_To_v1_ClusterNetworkList(in *sdn_api.ClusterNetworkList, out *ClusterNetworkList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterNetwork, len(*in))
		for i := range *in {
			if err := Convert_api_ClusterNetwork_To_v1_ClusterNetwork(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterNetworkList_To_v1_ClusterNetworkList(in *sdn_api.ClusterNetworkList, out *ClusterNetworkList, s conversion.Scope) error {
	return autoConvert_api_ClusterNetworkList_To_v1_ClusterNetworkList(in, out, s)
}

func autoConvert_v1_HostSubnet_To_api_HostSubnet(in *HostSubnet, out *sdn_api.HostSubnet, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	// TODO: Inefficient conversion - can we improve it?
	if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
		return err
	}
	out.Host = in.Host
	out.HostIP = in.HostIP
	out.Subnet = in.Subnet
	return nil
}

func Convert_v1_HostSubnet_To_api_HostSubnet(in *HostSubnet, out *sdn_api.HostSubnet, s conversion.Scope) error {
	return autoConvert_v1_HostSubnet_To_api_HostSubnet(in, out, s)
}

func autoConvert_api_HostSubnet_To_v1_HostSubnet(in *sdn_api.HostSubnet, out *HostSubnet, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	// TODO: Inefficient conversion - can we improve it?
	if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
		return err
	}
	out.Host = in.Host
	out.HostIP = in.HostIP
	out.Subnet = in.Subnet
	return nil
}

func Convert_api_HostSubnet_To_v1_HostSubnet(in *sdn_api.HostSubnet, out *HostSubnet, s conversion.Scope) error {
	return autoConvert_api_HostSubnet_To_v1_HostSubnet(in, out, s)
}

func autoConvert_v1_HostSubnetList_To_api_HostSubnetList(in *HostSubnetList, out *sdn_api.HostSubnetList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]sdn_api.HostSubnet, len(*in))
		for i := range *in {
			if err := Convert_v1_HostSubnet_To_api_HostSubnet(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_HostSubnetList_To_api_HostSubnetList(in *HostSubnetList, out *sdn_api.HostSubnetList, s conversion.Scope) error {
	return autoConvert_v1_HostSubnetList_To_api_HostSubnetList(in, out, s)
}

func autoConvert_api_HostSubnetList_To_v1_HostSubnetList(in *sdn_api.HostSubnetList, out *HostSubnetList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]HostSubnet, len(*in))
		for i := range *in {
			if err := Convert_api_HostSubnet_To_v1_HostSubnet(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_HostSubnetList_To_v1_HostSubnetList(in *sdn_api.HostSubnetList, out *HostSubnetList, s conversion.Scope) error {
	return autoConvert_api_HostSubnetList_To_v1_HostSubnetList(in, out, s)
}

func autoConvert_v1_NetNamespace_To_api_NetNamespace(in *NetNamespace, out *sdn_api.NetNamespace, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	// TODO: Inefficient conversion - can we improve it?
	if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
		return err
	}
	out.NetName = in.NetName
	out.NetID = in.NetID
	return nil
}

func Convert_v1_NetNamespace_To_api_NetNamespace(in *NetNamespace, out *sdn_api.NetNamespace, s conversion.Scope) error {
	return autoConvert_v1_NetNamespace_To_api_NetNamespace(in, out, s)
}

func autoConvert_api_NetNamespace_To_v1_NetNamespace(in *sdn_api.NetNamespace, out *NetNamespace, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	// TODO: Inefficient conversion - can we improve it?
	if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
		return err
	}
	out.NetName = in.NetName
	out.NetID = in.NetID
	return nil
}

func Convert_api_NetNamespace_To_v1_NetNamespace(in *sdn_api.NetNamespace, out *NetNamespace, s conversion.Scope) error {
	return autoConvert_api_NetNamespace_To_v1_NetNamespace(in, out, s)
}

func autoConvert_v1_NetNamespaceList_To_api_NetNamespaceList(in *NetNamespaceList, out *sdn_api.NetNamespaceList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]sdn_api.NetNamespace, len(*in))
		for i := range *in {
			if err := Convert_v1_NetNamespace_To_api_NetNamespace(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_NetNamespaceList_To_api_NetNamespaceList(in *NetNamespaceList, out *sdn_api.NetNamespaceList, s conversion.Scope) error {
	return autoConvert_v1_NetNamespaceList_To_api_NetNamespaceList(in, out, s)
}

func autoConvert_api_NetNamespaceList_To_v1_NetNamespaceList(in *sdn_api.NetNamespaceList, out *NetNamespaceList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NetNamespace, len(*in))
		for i := range *in {
			if err := Convert_api_NetNamespace_To_v1_NetNamespace(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_NetNamespaceList_To_v1_NetNamespaceList(in *sdn_api.NetNamespaceList, out *NetNamespaceList, s conversion.Scope) error {
	return autoConvert_api_NetNamespaceList_To_v1_NetNamespaceList(in, out, s)
}
