// +build !ignore_autogenerated_openshift

// This file was autogenerated by conversion-gen. Do not edit it manually!

package v1

import (
	unsafe "unsafe"

	api "github.com/openshift/origin/pkg/user/api"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	api_v1 "k8s.io/kubernetes/pkg/api/v1"
)

func init() {
	SchemeBuilder.Register(RegisterConversions)
}

// RegisterConversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterConversions(scheme *runtime.Scheme) error {
	return scheme.AddGeneratedConversionFuncs(
		Convert_v1_Group_To_api_Group,
		Convert_api_Group_To_v1_Group,
		Convert_v1_GroupList_To_api_GroupList,
		Convert_api_GroupList_To_v1_GroupList,
		Convert_v1_Identity_To_api_Identity,
		Convert_api_Identity_To_v1_Identity,
		Convert_v1_IdentityList_To_api_IdentityList,
		Convert_api_IdentityList_To_v1_IdentityList,
		Convert_v1_User_To_api_User,
		Convert_api_User_To_v1_User,
		Convert_v1_UserIdentityMapping_To_api_UserIdentityMapping,
		Convert_api_UserIdentityMapping_To_v1_UserIdentityMapping,
		Convert_v1_UserList_To_api_UserList,
		Convert_api_UserList_To_v1_UserList,
	)
}

func autoConvert_v1_Group_To_api_Group(in *Group, out *api.Group, s conversion.Scope) error {
	if err := api_v1.Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Users = *(*[]string)(unsafe.Pointer(&in.Users))
	return nil
}

func Convert_v1_Group_To_api_Group(in *Group, out *api.Group, s conversion.Scope) error {
	return autoConvert_v1_Group_To_api_Group(in, out, s)
}

func autoConvert_api_Group_To_v1_Group(in *api.Group, out *Group, s conversion.Scope) error {
	if err := api_v1.Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Users = *(*OptionalNames)(unsafe.Pointer(&in.Users))
	return nil
}

func Convert_api_Group_To_v1_Group(in *api.Group, out *Group, s conversion.Scope) error {
	return autoConvert_api_Group_To_v1_Group(in, out, s)
}

func autoConvert_v1_GroupList_To_api_GroupList(in *GroupList, out *api.GroupList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]api.Group, len(*in))
		for i := range *in {
			if err := Convert_v1_Group_To_api_Group(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_GroupList_To_api_GroupList(in *GroupList, out *api.GroupList, s conversion.Scope) error {
	return autoConvert_v1_GroupList_To_api_GroupList(in, out, s)
}

func autoConvert_api_GroupList_To_v1_GroupList(in *api.GroupList, out *GroupList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Group, len(*in))
		for i := range *in {
			if err := Convert_api_Group_To_v1_Group(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_GroupList_To_v1_GroupList(in *api.GroupList, out *GroupList, s conversion.Scope) error {
	return autoConvert_api_GroupList_To_v1_GroupList(in, out, s)
}

func autoConvert_v1_Identity_To_api_Identity(in *Identity, out *api.Identity, s conversion.Scope) error {
	if err := api_v1.Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ProviderName = in.ProviderName
	out.ProviderUserName = in.ProviderUserName
	if err := api_v1.Convert_v1_ObjectReference_To_api_ObjectReference(&in.User, &out.User, s); err != nil {
		return err
	}
	out.Extra = *(*map[string]string)(unsafe.Pointer(&in.Extra))
	return nil
}

func Convert_v1_Identity_To_api_Identity(in *Identity, out *api.Identity, s conversion.Scope) error {
	return autoConvert_v1_Identity_To_api_Identity(in, out, s)
}

func autoConvert_api_Identity_To_v1_Identity(in *api.Identity, out *Identity, s conversion.Scope) error {
	if err := api_v1.Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ProviderName = in.ProviderName
	out.ProviderUserName = in.ProviderUserName
	if err := api_v1.Convert_api_ObjectReference_To_v1_ObjectReference(&in.User, &out.User, s); err != nil {
		return err
	}
	out.Extra = *(*map[string]string)(unsafe.Pointer(&in.Extra))
	return nil
}

func Convert_api_Identity_To_v1_Identity(in *api.Identity, out *Identity, s conversion.Scope) error {
	return autoConvert_api_Identity_To_v1_Identity(in, out, s)
}

func autoConvert_v1_IdentityList_To_api_IdentityList(in *IdentityList, out *api.IdentityList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]api.Identity, len(*in))
		for i := range *in {
			if err := Convert_v1_Identity_To_api_Identity(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_IdentityList_To_api_IdentityList(in *IdentityList, out *api.IdentityList, s conversion.Scope) error {
	return autoConvert_v1_IdentityList_To_api_IdentityList(in, out, s)
}

func autoConvert_api_IdentityList_To_v1_IdentityList(in *api.IdentityList, out *IdentityList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Identity, len(*in))
		for i := range *in {
			if err := Convert_api_Identity_To_v1_Identity(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_IdentityList_To_v1_IdentityList(in *api.IdentityList, out *IdentityList, s conversion.Scope) error {
	return autoConvert_api_IdentityList_To_v1_IdentityList(in, out, s)
}

func autoConvert_v1_User_To_api_User(in *User, out *api.User, s conversion.Scope) error {
	if err := api_v1.Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.FullName = in.FullName
	out.Identities = *(*[]string)(unsafe.Pointer(&in.Identities))
	out.Groups = *(*[]string)(unsafe.Pointer(&in.Groups))
	return nil
}

func Convert_v1_User_To_api_User(in *User, out *api.User, s conversion.Scope) error {
	return autoConvert_v1_User_To_api_User(in, out, s)
}

func autoConvert_api_User_To_v1_User(in *api.User, out *User, s conversion.Scope) error {
	if err := api_v1.Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.FullName = in.FullName
	out.Identities = *(*[]string)(unsafe.Pointer(&in.Identities))
	out.Groups = *(*[]string)(unsafe.Pointer(&in.Groups))
	return nil
}

func Convert_api_User_To_v1_User(in *api.User, out *User, s conversion.Scope) error {
	return autoConvert_api_User_To_v1_User(in, out, s)
}

func autoConvert_v1_UserIdentityMapping_To_api_UserIdentityMapping(in *UserIdentityMapping, out *api.UserIdentityMapping, s conversion.Scope) error {
	if err := api_v1.Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api_v1.Convert_v1_ObjectReference_To_api_ObjectReference(&in.Identity, &out.Identity, s); err != nil {
		return err
	}
	if err := api_v1.Convert_v1_ObjectReference_To_api_ObjectReference(&in.User, &out.User, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_UserIdentityMapping_To_api_UserIdentityMapping(in *UserIdentityMapping, out *api.UserIdentityMapping, s conversion.Scope) error {
	return autoConvert_v1_UserIdentityMapping_To_api_UserIdentityMapping(in, out, s)
}

func autoConvert_api_UserIdentityMapping_To_v1_UserIdentityMapping(in *api.UserIdentityMapping, out *UserIdentityMapping, s conversion.Scope) error {
	if err := api_v1.Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api_v1.Convert_api_ObjectReference_To_v1_ObjectReference(&in.Identity, &out.Identity, s); err != nil {
		return err
	}
	if err := api_v1.Convert_api_ObjectReference_To_v1_ObjectReference(&in.User, &out.User, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_UserIdentityMapping_To_v1_UserIdentityMapping(in *api.UserIdentityMapping, out *UserIdentityMapping, s conversion.Scope) error {
	return autoConvert_api_UserIdentityMapping_To_v1_UserIdentityMapping(in, out, s)
}

func autoConvert_v1_UserList_To_api_UserList(in *UserList, out *api.UserList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]api.User, len(*in))
		for i := range *in {
			if err := Convert_v1_User_To_api_User(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_UserList_To_api_UserList(in *UserList, out *api.UserList, s conversion.Scope) error {
	return autoConvert_v1_UserList_To_api_UserList(in, out, s)
}

func autoConvert_api_UserList_To_v1_UserList(in *api.UserList, out *UserList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]User, len(*in))
		for i := range *in {
			if err := Convert_api_User_To_v1_User(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_UserList_To_v1_UserList(in *api.UserList, out *UserList, s conversion.Scope) error {
	return autoConvert_api_UserList_To_v1_UserList(in, out, s)
}
