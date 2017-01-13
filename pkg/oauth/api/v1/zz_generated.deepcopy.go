// +build !ignore_autogenerated_openshift

// This file was autogenerated by deepcopy-gen. Do not edit it manually!

package v1

import (
	api_v1 "k8s.io/kubernetes/pkg/api/v1"
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
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_ClusterRoleScopeRestriction, InType: reflect.TypeOf(&ClusterRoleScopeRestriction{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_OAuthAccessToken, InType: reflect.TypeOf(&OAuthAccessToken{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_OAuthAccessTokenList, InType: reflect.TypeOf(&OAuthAccessTokenList{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_OAuthAuthorizeToken, InType: reflect.TypeOf(&OAuthAuthorizeToken{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_OAuthAuthorizeTokenList, InType: reflect.TypeOf(&OAuthAuthorizeTokenList{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_OAuthClient, InType: reflect.TypeOf(&OAuthClient{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_OAuthClientAuthorization, InType: reflect.TypeOf(&OAuthClientAuthorization{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_OAuthClientAuthorizationList, InType: reflect.TypeOf(&OAuthClientAuthorizationList{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_OAuthClientList, InType: reflect.TypeOf(&OAuthClientList{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_OAuthRedirectReference, InType: reflect.TypeOf(&OAuthRedirectReference{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_RedirectReference, InType: reflect.TypeOf(&RedirectReference{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_ScopeRestriction, InType: reflect.TypeOf(&ScopeRestriction{})},
	)
}

func DeepCopy_v1_ClusterRoleScopeRestriction(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ClusterRoleScopeRestriction)
		out := out.(*ClusterRoleScopeRestriction)
		if in.RoleNames != nil {
			in, out := &in.RoleNames, &out.RoleNames
			*out = make([]string, len(*in))
			copy(*out, *in)
		} else {
			out.RoleNames = nil
		}
		if in.Namespaces != nil {
			in, out := &in.Namespaces, &out.Namespaces
			*out = make([]string, len(*in))
			copy(*out, *in)
		} else {
			out.Namespaces = nil
		}
		out.AllowEscalation = in.AllowEscalation
		return nil
	}
}

func DeepCopy_v1_OAuthAccessToken(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*OAuthAccessToken)
		out := out.(*OAuthAccessToken)
		out.TypeMeta = in.TypeMeta
		if err := api_v1.DeepCopy_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, c); err != nil {
			return err
		}
		out.ClientName = in.ClientName
		out.ExpiresIn = in.ExpiresIn
		if in.Scopes != nil {
			in, out := &in.Scopes, &out.Scopes
			*out = make([]string, len(*in))
			copy(*out, *in)
		} else {
			out.Scopes = nil
		}
		out.RedirectURI = in.RedirectURI
		out.UserName = in.UserName
		out.UserUID = in.UserUID
		out.AuthorizeToken = in.AuthorizeToken
		out.RefreshToken = in.RefreshToken
		out.Token = in.Token
		out.Salt = in.Salt
		out.SaltedHash = in.SaltedHash
		return nil
	}
}

func DeepCopy_v1_OAuthAccessTokenList(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*OAuthAccessTokenList)
		out := out.(*OAuthAccessTokenList)
		out.TypeMeta = in.TypeMeta
		out.ListMeta = in.ListMeta
		if in.Items != nil {
			in, out := &in.Items, &out.Items
			*out = make([]OAuthAccessToken, len(*in))
			for i := range *in {
				if err := DeepCopy_v1_OAuthAccessToken(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		} else {
			out.Items = nil
		}
		return nil
	}
}

func DeepCopy_v1_OAuthAuthorizeToken(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*OAuthAuthorizeToken)
		out := out.(*OAuthAuthorizeToken)
		out.TypeMeta = in.TypeMeta
		if err := api_v1.DeepCopy_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, c); err != nil {
			return err
		}
		out.ClientName = in.ClientName
		out.ExpiresIn = in.ExpiresIn
		if in.Scopes != nil {
			in, out := &in.Scopes, &out.Scopes
			*out = make([]string, len(*in))
			copy(*out, *in)
		} else {
			out.Scopes = nil
		}
		out.RedirectURI = in.RedirectURI
		out.State = in.State
		out.UserName = in.UserName
		out.UserUID = in.UserUID
		out.CodeChallenge = in.CodeChallenge
		out.CodeChallengeMethod = in.CodeChallengeMethod
		out.Token = in.Token
		out.Salt = in.Salt
		out.SaltedHash = in.SaltedHash
		return nil
	}
}

func DeepCopy_v1_OAuthAuthorizeTokenList(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*OAuthAuthorizeTokenList)
		out := out.(*OAuthAuthorizeTokenList)
		out.TypeMeta = in.TypeMeta
		out.ListMeta = in.ListMeta
		if in.Items != nil {
			in, out := &in.Items, &out.Items
			*out = make([]OAuthAuthorizeToken, len(*in))
			for i := range *in {
				if err := DeepCopy_v1_OAuthAuthorizeToken(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		} else {
			out.Items = nil
		}
		return nil
	}
}

func DeepCopy_v1_OAuthClient(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*OAuthClient)
		out := out.(*OAuthClient)
		out.TypeMeta = in.TypeMeta
		if err := api_v1.DeepCopy_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, c); err != nil {
			return err
		}
		out.Secret = in.Secret
		if in.AdditionalSecrets != nil {
			in, out := &in.AdditionalSecrets, &out.AdditionalSecrets
			*out = make([]string, len(*in))
			copy(*out, *in)
		} else {
			out.AdditionalSecrets = nil
		}
		out.RespondWithChallenges = in.RespondWithChallenges
		if in.RedirectURIs != nil {
			in, out := &in.RedirectURIs, &out.RedirectURIs
			*out = make([]string, len(*in))
			copy(*out, *in)
		} else {
			out.RedirectURIs = nil
		}
		out.GrantMethod = in.GrantMethod
		if in.ScopeRestrictions != nil {
			in, out := &in.ScopeRestrictions, &out.ScopeRestrictions
			*out = make([]ScopeRestriction, len(*in))
			for i := range *in {
				if err := DeepCopy_v1_ScopeRestriction(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		} else {
			out.ScopeRestrictions = nil
		}
		return nil
	}
}

func DeepCopy_v1_OAuthClientAuthorization(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*OAuthClientAuthorization)
		out := out.(*OAuthClientAuthorization)
		out.TypeMeta = in.TypeMeta
		if err := api_v1.DeepCopy_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, c); err != nil {
			return err
		}
		out.ClientName = in.ClientName
		out.UserName = in.UserName
		out.UserUID = in.UserUID
		if in.Scopes != nil {
			in, out := &in.Scopes, &out.Scopes
			*out = make([]string, len(*in))
			copy(*out, *in)
		} else {
			out.Scopes = nil
		}
		return nil
	}
}

func DeepCopy_v1_OAuthClientAuthorizationList(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*OAuthClientAuthorizationList)
		out := out.(*OAuthClientAuthorizationList)
		out.TypeMeta = in.TypeMeta
		out.ListMeta = in.ListMeta
		if in.Items != nil {
			in, out := &in.Items, &out.Items
			*out = make([]OAuthClientAuthorization, len(*in))
			for i := range *in {
				if err := DeepCopy_v1_OAuthClientAuthorization(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		} else {
			out.Items = nil
		}
		return nil
	}
}

func DeepCopy_v1_OAuthClientList(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*OAuthClientList)
		out := out.(*OAuthClientList)
		out.TypeMeta = in.TypeMeta
		out.ListMeta = in.ListMeta
		if in.Items != nil {
			in, out := &in.Items, &out.Items
			*out = make([]OAuthClient, len(*in))
			for i := range *in {
				if err := DeepCopy_v1_OAuthClient(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		} else {
			out.Items = nil
		}
		return nil
	}
}

func DeepCopy_v1_OAuthRedirectReference(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*OAuthRedirectReference)
		out := out.(*OAuthRedirectReference)
		out.TypeMeta = in.TypeMeta
		if err := api_v1.DeepCopy_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, c); err != nil {
			return err
		}
		out.Reference = in.Reference
		return nil
	}
}

func DeepCopy_v1_RedirectReference(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*RedirectReference)
		out := out.(*RedirectReference)
		out.Group = in.Group
		out.Kind = in.Kind
		out.Name = in.Name
		return nil
	}
}

func DeepCopy_v1_ScopeRestriction(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ScopeRestriction)
		out := out.(*ScopeRestriction)
		if in.ExactValues != nil {
			in, out := &in.ExactValues, &out.ExactValues
			*out = make([]string, len(*in))
			copy(*out, *in)
		} else {
			out.ExactValues = nil
		}
		if in.ClusterRole != nil {
			in, out := &in.ClusterRole, &out.ClusterRole
			*out = new(ClusterRoleScopeRestriction)
			if err := DeepCopy_v1_ClusterRoleScopeRestriction(*in, *out, c); err != nil {
				return err
			}
		} else {
			out.ClusterRole = nil
		}
		return nil
	}
}
