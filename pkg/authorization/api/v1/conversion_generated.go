// +build !ignore_autogenerated_openshift

// This file was autogenerated by conversion-gen. Do not edit it manually!

package v1

import (
	authorization_api "github.com/openshift/origin/pkg/authorization/api"
	api "k8s.io/kubernetes/pkg/api"
	api_v1 "k8s.io/kubernetes/pkg/api/v1"
	conversion "k8s.io/kubernetes/pkg/conversion"
	runtime "k8s.io/kubernetes/pkg/runtime"
)

func init() {
	if err := api.Scheme.AddGeneratedConversionFuncs(
		Convert_v1_Action_To_api_Action,
		Convert_api_Action_To_v1_Action,
		Convert_v1_ClusterPolicy_To_api_ClusterPolicy,
		Convert_api_ClusterPolicy_To_v1_ClusterPolicy,
		Convert_v1_ClusterPolicyBinding_To_api_ClusterPolicyBinding,
		Convert_api_ClusterPolicyBinding_To_v1_ClusterPolicyBinding,
		Convert_v1_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList,
		Convert_api_ClusterPolicyBindingList_To_v1_ClusterPolicyBindingList,
		Convert_v1_ClusterPolicyList_To_api_ClusterPolicyList,
		Convert_api_ClusterPolicyList_To_v1_ClusterPolicyList,
		Convert_v1_ClusterRole_To_api_ClusterRole,
		Convert_api_ClusterRole_To_v1_ClusterRole,
		Convert_v1_ClusterRoleBinding_To_api_ClusterRoleBinding,
		Convert_api_ClusterRoleBinding_To_v1_ClusterRoleBinding,
		Convert_v1_ClusterRoleBindingList_To_api_ClusterRoleBindingList,
		Convert_api_ClusterRoleBindingList_To_v1_ClusterRoleBindingList,
		Convert_v1_ClusterRoleList_To_api_ClusterRoleList,
		Convert_api_ClusterRoleList_To_v1_ClusterRoleList,
		Convert_v1_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview,
		Convert_api_IsPersonalSubjectAccessReview_To_v1_IsPersonalSubjectAccessReview,
		Convert_v1_LocalResourceAccessReview_To_api_LocalResourceAccessReview,
		Convert_api_LocalResourceAccessReview_To_v1_LocalResourceAccessReview,
		Convert_v1_LocalSubjectAccessReview_To_api_LocalSubjectAccessReview,
		Convert_api_LocalSubjectAccessReview_To_v1_LocalSubjectAccessReview,
		Convert_v1_Policy_To_api_Policy,
		Convert_api_Policy_To_v1_Policy,
		Convert_v1_PolicyBinding_To_api_PolicyBinding,
		Convert_api_PolicyBinding_To_v1_PolicyBinding,
		Convert_v1_PolicyBindingList_To_api_PolicyBindingList,
		Convert_api_PolicyBindingList_To_v1_PolicyBindingList,
		Convert_v1_PolicyList_To_api_PolicyList,
		Convert_api_PolicyList_To_v1_PolicyList,
		Convert_v1_PolicyRule_To_api_PolicyRule,
		Convert_api_PolicyRule_To_v1_PolicyRule,
		Convert_v1_ResourceAccessReview_To_api_ResourceAccessReview,
		Convert_api_ResourceAccessReview_To_v1_ResourceAccessReview,
		Convert_v1_ResourceAccessReviewResponse_To_api_ResourceAccessReviewResponse,
		Convert_api_ResourceAccessReviewResponse_To_v1_ResourceAccessReviewResponse,
		Convert_v1_Role_To_api_Role,
		Convert_api_Role_To_v1_Role,
		Convert_v1_RoleBinding_To_api_RoleBinding,
		Convert_api_RoleBinding_To_v1_RoleBinding,
		Convert_v1_RoleBindingList_To_api_RoleBindingList,
		Convert_api_RoleBindingList_To_v1_RoleBindingList,
		Convert_v1_RoleList_To_api_RoleList,
		Convert_api_RoleList_To_v1_RoleList,
		Convert_v1_SelfSubjectRulesReview_To_api_SelfSubjectRulesReview,
		Convert_api_SelfSubjectRulesReview_To_v1_SelfSubjectRulesReview,
		Convert_v1_SelfSubjectRulesReviewSpec_To_api_SelfSubjectRulesReviewSpec,
		Convert_api_SelfSubjectRulesReviewSpec_To_v1_SelfSubjectRulesReviewSpec,
		Convert_v1_SubjectAccessReview_To_api_SubjectAccessReview,
		Convert_api_SubjectAccessReview_To_v1_SubjectAccessReview,
		Convert_v1_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse,
		Convert_api_SubjectAccessReviewResponse_To_v1_SubjectAccessReviewResponse,
		Convert_v1_SubjectRulesReviewStatus_To_api_SubjectRulesReviewStatus,
		Convert_api_SubjectRulesReviewStatus_To_v1_SubjectRulesReviewStatus,
	); err != nil {
		// if one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

func autoConvert_v1_Action_To_api_Action(in *Action, out *authorization_api.Action, s conversion.Scope) error {
	out.Namespace = in.Namespace
	out.Verb = in.Verb
	out.Group = in.Group
	out.Version = in.Version
	out.Resource = in.Resource
	out.ResourceName = in.ResourceName
	if err := runtime.Convert_runtime_RawExtension_To_runtime_Object(&in.Content, &out.Content, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_Action_To_api_Action(in *Action, out *authorization_api.Action, s conversion.Scope) error {
	return autoConvert_v1_Action_To_api_Action(in, out, s)
}

func autoConvert_api_Action_To_v1_Action(in *authorization_api.Action, out *Action, s conversion.Scope) error {
	out.Namespace = in.Namespace
	out.Verb = in.Verb
	out.Group = in.Group
	out.Version = in.Version
	out.Resource = in.Resource
	out.ResourceName = in.ResourceName
	if err := runtime.Convert_runtime_Object_To_runtime_RawExtension(&in.Content, &out.Content, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_Action_To_v1_Action(in *authorization_api.Action, out *Action, s conversion.Scope) error {
	return autoConvert_api_Action_To_v1_Action(in, out, s)
}

func autoConvert_v1_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in *ClusterPolicyBindingList, out *authorization_api.ClusterPolicyBindingList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]authorization_api.ClusterPolicyBinding, len(*in))
		for i := range *in {
			if err := Convert_v1_ClusterPolicyBinding_To_api_ClusterPolicyBinding(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in *ClusterPolicyBindingList, out *authorization_api.ClusterPolicyBindingList, s conversion.Scope) error {
	return autoConvert_v1_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in, out, s)
}

func autoConvert_api_ClusterPolicyBindingList_To_v1_ClusterPolicyBindingList(in *authorization_api.ClusterPolicyBindingList, out *ClusterPolicyBindingList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterPolicyBinding, len(*in))
		for i := range *in {
			if err := Convert_api_ClusterPolicyBinding_To_v1_ClusterPolicyBinding(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterPolicyBindingList_To_v1_ClusterPolicyBindingList(in *authorization_api.ClusterPolicyBindingList, out *ClusterPolicyBindingList, s conversion.Scope) error {
	return autoConvert_api_ClusterPolicyBindingList_To_v1_ClusterPolicyBindingList(in, out, s)
}

func autoConvert_v1_ClusterPolicyList_To_api_ClusterPolicyList(in *ClusterPolicyList, out *authorization_api.ClusterPolicyList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]authorization_api.ClusterPolicy, len(*in))
		for i := range *in {
			if err := Convert_v1_ClusterPolicy_To_api_ClusterPolicy(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ClusterPolicyList_To_api_ClusterPolicyList(in *ClusterPolicyList, out *authorization_api.ClusterPolicyList, s conversion.Scope) error {
	return autoConvert_v1_ClusterPolicyList_To_api_ClusterPolicyList(in, out, s)
}

func autoConvert_api_ClusterPolicyList_To_v1_ClusterPolicyList(in *authorization_api.ClusterPolicyList, out *ClusterPolicyList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterPolicy, len(*in))
		for i := range *in {
			if err := Convert_api_ClusterPolicy_To_v1_ClusterPolicy(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterPolicyList_To_v1_ClusterPolicyList(in *authorization_api.ClusterPolicyList, out *ClusterPolicyList, s conversion.Scope) error {
	return autoConvert_api_ClusterPolicyList_To_v1_ClusterPolicyList(in, out, s)
}

func autoConvert_v1_ClusterRole_To_api_ClusterRole(in *ClusterRole, out *authorization_api.ClusterRole, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api_v1.Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]authorization_api.PolicyRule, len(*in))
		for i := range *in {
			if err := Convert_v1_PolicyRule_To_api_PolicyRule(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func Convert_v1_ClusterRole_To_api_ClusterRole(in *ClusterRole, out *authorization_api.ClusterRole, s conversion.Scope) error {
	return autoConvert_v1_ClusterRole_To_api_ClusterRole(in, out, s)
}

func autoConvert_api_ClusterRole_To_v1_ClusterRole(in *authorization_api.ClusterRole, out *ClusterRole, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api_v1.Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]PolicyRule, len(*in))
		for i := range *in {
			if err := Convert_api_PolicyRule_To_v1_PolicyRule(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func Convert_api_ClusterRole_To_v1_ClusterRole(in *authorization_api.ClusterRole, out *ClusterRole, s conversion.Scope) error {
	return autoConvert_api_ClusterRole_To_v1_ClusterRole(in, out, s)
}

func autoConvert_api_ClusterRoleBinding_To_v1_ClusterRoleBinding(in *authorization_api.ClusterRoleBinding, out *ClusterRoleBinding, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api_v1.Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Subjects != nil {
		in, out := &in.Subjects, &out.Subjects
		*out = make([]api_v1.ObjectReference, len(*in))
		for i := range *in {
			if err := api_v1.Convert_api_ObjectReference_To_v1_ObjectReference(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := api_v1.Convert_api_ObjectReference_To_v1_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in *ClusterRoleBindingList, out *authorization_api.ClusterRoleBindingList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]authorization_api.ClusterRoleBinding, len(*in))
		for i := range *in {
			if err := Convert_v1_ClusterRoleBinding_To_api_ClusterRoleBinding(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in *ClusterRoleBindingList, out *authorization_api.ClusterRoleBindingList, s conversion.Scope) error {
	return autoConvert_v1_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in, out, s)
}

func autoConvert_api_ClusterRoleBindingList_To_v1_ClusterRoleBindingList(in *authorization_api.ClusterRoleBindingList, out *ClusterRoleBindingList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterRoleBinding, len(*in))
		for i := range *in {
			if err := Convert_api_ClusterRoleBinding_To_v1_ClusterRoleBinding(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterRoleBindingList_To_v1_ClusterRoleBindingList(in *authorization_api.ClusterRoleBindingList, out *ClusterRoleBindingList, s conversion.Scope) error {
	return autoConvert_api_ClusterRoleBindingList_To_v1_ClusterRoleBindingList(in, out, s)
}

func autoConvert_v1_ClusterRoleList_To_api_ClusterRoleList(in *ClusterRoleList, out *authorization_api.ClusterRoleList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]authorization_api.ClusterRole, len(*in))
		for i := range *in {
			if err := Convert_v1_ClusterRole_To_api_ClusterRole(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ClusterRoleList_To_api_ClusterRoleList(in *ClusterRoleList, out *authorization_api.ClusterRoleList, s conversion.Scope) error {
	return autoConvert_v1_ClusterRoleList_To_api_ClusterRoleList(in, out, s)
}

func autoConvert_api_ClusterRoleList_To_v1_ClusterRoleList(in *authorization_api.ClusterRoleList, out *ClusterRoleList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterRole, len(*in))
		for i := range *in {
			if err := Convert_api_ClusterRole_To_v1_ClusterRole(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterRoleList_To_v1_ClusterRoleList(in *authorization_api.ClusterRoleList, out *ClusterRoleList, s conversion.Scope) error {
	return autoConvert_api_ClusterRoleList_To_v1_ClusterRoleList(in, out, s)
}

func autoConvert_v1_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in *IsPersonalSubjectAccessReview, out *authorization_api.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in *IsPersonalSubjectAccessReview, out *authorization_api.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	return autoConvert_v1_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in, out, s)
}

func autoConvert_api_IsPersonalSubjectAccessReview_To_v1_IsPersonalSubjectAccessReview(in *authorization_api.IsPersonalSubjectAccessReview, out *IsPersonalSubjectAccessReview, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_IsPersonalSubjectAccessReview_To_v1_IsPersonalSubjectAccessReview(in *authorization_api.IsPersonalSubjectAccessReview, out *IsPersonalSubjectAccessReview, s conversion.Scope) error {
	return autoConvert_api_IsPersonalSubjectAccessReview_To_v1_IsPersonalSubjectAccessReview(in, out, s)
}

func autoConvert_v1_LocalResourceAccessReview_To_api_LocalResourceAccessReview(in *LocalResourceAccessReview, out *authorization_api.LocalResourceAccessReview, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := Convert_v1_Action_To_api_Action(&in.Action, &out.Action, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_LocalResourceAccessReview_To_api_LocalResourceAccessReview(in *LocalResourceAccessReview, out *authorization_api.LocalResourceAccessReview, s conversion.Scope) error {
	return autoConvert_v1_LocalResourceAccessReview_To_api_LocalResourceAccessReview(in, out, s)
}

func autoConvert_api_LocalResourceAccessReview_To_v1_LocalResourceAccessReview(in *authorization_api.LocalResourceAccessReview, out *LocalResourceAccessReview, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := Convert_api_Action_To_v1_Action(&in.Action, &out.Action, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_LocalResourceAccessReview_To_v1_LocalResourceAccessReview(in *authorization_api.LocalResourceAccessReview, out *LocalResourceAccessReview, s conversion.Scope) error {
	return autoConvert_api_LocalResourceAccessReview_To_v1_LocalResourceAccessReview(in, out, s)
}

func autoConvert_v1_PolicyBindingList_To_api_PolicyBindingList(in *PolicyBindingList, out *authorization_api.PolicyBindingList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]authorization_api.PolicyBinding, len(*in))
		for i := range *in {
			if err := Convert_v1_PolicyBinding_To_api_PolicyBinding(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_PolicyBindingList_To_api_PolicyBindingList(in *PolicyBindingList, out *authorization_api.PolicyBindingList, s conversion.Scope) error {
	return autoConvert_v1_PolicyBindingList_To_api_PolicyBindingList(in, out, s)
}

func autoConvert_api_PolicyBindingList_To_v1_PolicyBindingList(in *authorization_api.PolicyBindingList, out *PolicyBindingList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PolicyBinding, len(*in))
		for i := range *in {
			if err := Convert_api_PolicyBinding_To_v1_PolicyBinding(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_PolicyBindingList_To_v1_PolicyBindingList(in *authorization_api.PolicyBindingList, out *PolicyBindingList, s conversion.Scope) error {
	return autoConvert_api_PolicyBindingList_To_v1_PolicyBindingList(in, out, s)
}

func autoConvert_v1_PolicyList_To_api_PolicyList(in *PolicyList, out *authorization_api.PolicyList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]authorization_api.Policy, len(*in))
		for i := range *in {
			if err := Convert_v1_Policy_To_api_Policy(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_PolicyList_To_api_PolicyList(in *PolicyList, out *authorization_api.PolicyList, s conversion.Scope) error {
	return autoConvert_v1_PolicyList_To_api_PolicyList(in, out, s)
}

func autoConvert_api_PolicyList_To_v1_PolicyList(in *authorization_api.PolicyList, out *PolicyList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Policy, len(*in))
		for i := range *in {
			if err := Convert_api_Policy_To_v1_Policy(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_PolicyList_To_v1_PolicyList(in *authorization_api.PolicyList, out *PolicyList, s conversion.Scope) error {
	return autoConvert_api_PolicyList_To_v1_PolicyList(in, out, s)
}

func autoConvert_v1_ResourceAccessReview_To_api_ResourceAccessReview(in *ResourceAccessReview, out *authorization_api.ResourceAccessReview, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := Convert_v1_Action_To_api_Action(&in.Action, &out.Action, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_ResourceAccessReview_To_api_ResourceAccessReview(in *ResourceAccessReview, out *authorization_api.ResourceAccessReview, s conversion.Scope) error {
	return autoConvert_v1_ResourceAccessReview_To_api_ResourceAccessReview(in, out, s)
}

func autoConvert_api_ResourceAccessReview_To_v1_ResourceAccessReview(in *authorization_api.ResourceAccessReview, out *ResourceAccessReview, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := Convert_api_Action_To_v1_Action(&in.Action, &out.Action, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_ResourceAccessReview_To_v1_ResourceAccessReview(in *authorization_api.ResourceAccessReview, out *ResourceAccessReview, s conversion.Scope) error {
	return autoConvert_api_ResourceAccessReview_To_v1_ResourceAccessReview(in, out, s)
}

func autoConvert_v1_Role_To_api_Role(in *Role, out *authorization_api.Role, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api_v1.Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]authorization_api.PolicyRule, len(*in))
		for i := range *in {
			if err := Convert_v1_PolicyRule_To_api_PolicyRule(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func Convert_v1_Role_To_api_Role(in *Role, out *authorization_api.Role, s conversion.Scope) error {
	return autoConvert_v1_Role_To_api_Role(in, out, s)
}

func autoConvert_api_Role_To_v1_Role(in *authorization_api.Role, out *Role, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api_v1.Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]PolicyRule, len(*in))
		for i := range *in {
			if err := Convert_api_PolicyRule_To_v1_PolicyRule(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func Convert_api_Role_To_v1_Role(in *authorization_api.Role, out *Role, s conversion.Scope) error {
	return autoConvert_api_Role_To_v1_Role(in, out, s)
}

func autoConvert_api_RoleBinding_To_v1_RoleBinding(in *authorization_api.RoleBinding, out *RoleBinding, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api_v1.Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Subjects != nil {
		in, out := &in.Subjects, &out.Subjects
		*out = make([]api_v1.ObjectReference, len(*in))
		for i := range *in {
			if err := api_v1.Convert_api_ObjectReference_To_v1_ObjectReference(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := api_v1.Convert_api_ObjectReference_To_v1_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1_RoleBindingList_To_api_RoleBindingList(in *RoleBindingList, out *authorization_api.RoleBindingList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]authorization_api.RoleBinding, len(*in))
		for i := range *in {
			if err := Convert_v1_RoleBinding_To_api_RoleBinding(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_RoleBindingList_To_api_RoleBindingList(in *RoleBindingList, out *authorization_api.RoleBindingList, s conversion.Scope) error {
	return autoConvert_v1_RoleBindingList_To_api_RoleBindingList(in, out, s)
}

func autoConvert_api_RoleBindingList_To_v1_RoleBindingList(in *authorization_api.RoleBindingList, out *RoleBindingList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]RoleBinding, len(*in))
		for i := range *in {
			if err := Convert_api_RoleBinding_To_v1_RoleBinding(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_RoleBindingList_To_v1_RoleBindingList(in *authorization_api.RoleBindingList, out *RoleBindingList, s conversion.Scope) error {
	return autoConvert_api_RoleBindingList_To_v1_RoleBindingList(in, out, s)
}

func autoConvert_v1_RoleList_To_api_RoleList(in *RoleList, out *authorization_api.RoleList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]authorization_api.Role, len(*in))
		for i := range *in {
			if err := Convert_v1_Role_To_api_Role(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_RoleList_To_api_RoleList(in *RoleList, out *authorization_api.RoleList, s conversion.Scope) error {
	return autoConvert_v1_RoleList_To_api_RoleList(in, out, s)
}

func autoConvert_api_RoleList_To_v1_RoleList(in *authorization_api.RoleList, out *RoleList, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Role, len(*in))
		for i := range *in {
			if err := Convert_api_Role_To_v1_Role(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_RoleList_To_v1_RoleList(in *authorization_api.RoleList, out *RoleList, s conversion.Scope) error {
	return autoConvert_api_RoleList_To_v1_RoleList(in, out, s)
}

func autoConvert_v1_SelfSubjectRulesReview_To_api_SelfSubjectRulesReview(in *SelfSubjectRulesReview, out *authorization_api.SelfSubjectRulesReview, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := Convert_v1_SelfSubjectRulesReviewSpec_To_api_SelfSubjectRulesReviewSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1_SubjectRulesReviewStatus_To_api_SubjectRulesReviewStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_SelfSubjectRulesReview_To_api_SelfSubjectRulesReview(in *SelfSubjectRulesReview, out *authorization_api.SelfSubjectRulesReview, s conversion.Scope) error {
	return autoConvert_v1_SelfSubjectRulesReview_To_api_SelfSubjectRulesReview(in, out, s)
}

func autoConvert_api_SelfSubjectRulesReview_To_v1_SelfSubjectRulesReview(in *authorization_api.SelfSubjectRulesReview, out *SelfSubjectRulesReview, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	if err := Convert_api_SelfSubjectRulesReviewSpec_To_v1_SelfSubjectRulesReviewSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_api_SubjectRulesReviewStatus_To_v1_SubjectRulesReviewStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_SelfSubjectRulesReview_To_v1_SelfSubjectRulesReview(in *authorization_api.SelfSubjectRulesReview, out *SelfSubjectRulesReview, s conversion.Scope) error {
	return autoConvert_api_SelfSubjectRulesReview_To_v1_SelfSubjectRulesReview(in, out, s)
}

func autoConvert_v1_SelfSubjectRulesReviewSpec_To_api_SelfSubjectRulesReviewSpec(in *SelfSubjectRulesReviewSpec, out *authorization_api.SelfSubjectRulesReviewSpec, s conversion.Scope) error {
	out.Scopes = in.Scopes
	return nil
}

func Convert_v1_SelfSubjectRulesReviewSpec_To_api_SelfSubjectRulesReviewSpec(in *SelfSubjectRulesReviewSpec, out *authorization_api.SelfSubjectRulesReviewSpec, s conversion.Scope) error {
	return autoConvert_v1_SelfSubjectRulesReviewSpec_To_api_SelfSubjectRulesReviewSpec(in, out, s)
}

func autoConvert_api_SelfSubjectRulesReviewSpec_To_v1_SelfSubjectRulesReviewSpec(in *authorization_api.SelfSubjectRulesReviewSpec, out *SelfSubjectRulesReviewSpec, s conversion.Scope) error {
	out.Scopes = in.Scopes
	return nil
}

func Convert_api_SelfSubjectRulesReviewSpec_To_v1_SelfSubjectRulesReviewSpec(in *authorization_api.SelfSubjectRulesReviewSpec, out *SelfSubjectRulesReviewSpec, s conversion.Scope) error {
	return autoConvert_api_SelfSubjectRulesReviewSpec_To_v1_SelfSubjectRulesReviewSpec(in, out, s)
}

func autoConvert_v1_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in *SubjectAccessReviewResponse, out *authorization_api.SubjectAccessReviewResponse, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	out.Namespace = in.Namespace
	out.Allowed = in.Allowed
	out.Reason = in.Reason
	return nil
}

func Convert_v1_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in *SubjectAccessReviewResponse, out *authorization_api.SubjectAccessReviewResponse, s conversion.Scope) error {
	return autoConvert_v1_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in, out, s)
}

func autoConvert_api_SubjectAccessReviewResponse_To_v1_SubjectAccessReviewResponse(in *authorization_api.SubjectAccessReviewResponse, out *SubjectAccessReviewResponse, s conversion.Scope) error {
	if err := api.Convert_unversioned_TypeMeta_To_unversioned_TypeMeta(&in.TypeMeta, &out.TypeMeta, s); err != nil {
		return err
	}
	out.Namespace = in.Namespace
	out.Allowed = in.Allowed
	out.Reason = in.Reason
	return nil
}

func Convert_api_SubjectAccessReviewResponse_To_v1_SubjectAccessReviewResponse(in *authorization_api.SubjectAccessReviewResponse, out *SubjectAccessReviewResponse, s conversion.Scope) error {
	return autoConvert_api_SubjectAccessReviewResponse_To_v1_SubjectAccessReviewResponse(in, out, s)
}

func autoConvert_v1_SubjectRulesReviewStatus_To_api_SubjectRulesReviewStatus(in *SubjectRulesReviewStatus, out *authorization_api.SubjectRulesReviewStatus, s conversion.Scope) error {
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]authorization_api.PolicyRule, len(*in))
		for i := range *in {
			if err := Convert_v1_PolicyRule_To_api_PolicyRule(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	out.EvaluationError = in.EvaluationError
	return nil
}

func Convert_v1_SubjectRulesReviewStatus_To_api_SubjectRulesReviewStatus(in *SubjectRulesReviewStatus, out *authorization_api.SubjectRulesReviewStatus, s conversion.Scope) error {
	return autoConvert_v1_SubjectRulesReviewStatus_To_api_SubjectRulesReviewStatus(in, out, s)
}

func autoConvert_api_SubjectRulesReviewStatus_To_v1_SubjectRulesReviewStatus(in *authorization_api.SubjectRulesReviewStatus, out *SubjectRulesReviewStatus, s conversion.Scope) error {
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]PolicyRule, len(*in))
		for i := range *in {
			if err := Convert_api_PolicyRule_To_v1_PolicyRule(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	out.EvaluationError = in.EvaluationError
	return nil
}

func Convert_api_SubjectRulesReviewStatus_To_v1_SubjectRulesReviewStatus(in *authorization_api.SubjectRulesReviewStatus, out *SubjectRulesReviewStatus, s conversion.Scope) error {
	return autoConvert_api_SubjectRulesReviewStatus_To_v1_SubjectRulesReviewStatus(in, out, s)
}
