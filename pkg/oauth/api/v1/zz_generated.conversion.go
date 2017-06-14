// +build !ignore_autogenerated_openshift

// This file was autogenerated by conversion-gen. Do not edit it manually!

package v1

import (
	api "github.com/openshift/origin/pkg/oauth/api"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	unsafe "unsafe"
)

func init() {
	SchemeBuilder.Register(RegisterConversions)
}

// RegisterConversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterConversions(scheme *runtime.Scheme) error {
	return scheme.AddGeneratedConversionFuncs(
		Convert_v1_ClusterRoleScopeRestriction_To_api_ClusterRoleScopeRestriction,
		Convert_api_ClusterRoleScopeRestriction_To_v1_ClusterRoleScopeRestriction,
		Convert_v1_OAuthAccessToken_To_api_OAuthAccessToken,
		Convert_api_OAuthAccessToken_To_v1_OAuthAccessToken,
		Convert_v1_OAuthAccessTokenList_To_api_OAuthAccessTokenList,
		Convert_api_OAuthAccessTokenList_To_v1_OAuthAccessTokenList,
		Convert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken,
		Convert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken,
		Convert_v1_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList,
		Convert_api_OAuthAuthorizeTokenList_To_v1_OAuthAuthorizeTokenList,
		Convert_v1_OAuthClient_To_api_OAuthClient,
		Convert_api_OAuthClient_To_v1_OAuthClient,
		Convert_v1_OAuthClientAuthorization_To_api_OAuthClientAuthorization,
		Convert_api_OAuthClientAuthorization_To_v1_OAuthClientAuthorization,
		Convert_v1_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList,
		Convert_api_OAuthClientAuthorizationList_To_v1_OAuthClientAuthorizationList,
		Convert_v1_OAuthClientList_To_api_OAuthClientList,
		Convert_api_OAuthClientList_To_v1_OAuthClientList,
		Convert_v1_OAuthRedirectReference_To_api_OAuthRedirectReference,
		Convert_api_OAuthRedirectReference_To_v1_OAuthRedirectReference,
		Convert_v1_RedirectReference_To_api_RedirectReference,
		Convert_api_RedirectReference_To_v1_RedirectReference,
		Convert_v1_ScopeRestriction_To_api_ScopeRestriction,
		Convert_api_ScopeRestriction_To_v1_ScopeRestriction,
	)
}

func autoConvert_v1_ClusterRoleScopeRestriction_To_api_ClusterRoleScopeRestriction(in *ClusterRoleScopeRestriction, out *api.ClusterRoleScopeRestriction, s conversion.Scope) error {
	out.RoleNames = *(*[]string)(unsafe.Pointer(&in.RoleNames))
	out.Namespaces = *(*[]string)(unsafe.Pointer(&in.Namespaces))
	out.AllowEscalation = in.AllowEscalation
	return nil
}

func Convert_v1_ClusterRoleScopeRestriction_To_api_ClusterRoleScopeRestriction(in *ClusterRoleScopeRestriction, out *api.ClusterRoleScopeRestriction, s conversion.Scope) error {
	return autoConvert_v1_ClusterRoleScopeRestriction_To_api_ClusterRoleScopeRestriction(in, out, s)
}

func autoConvert_api_ClusterRoleScopeRestriction_To_v1_ClusterRoleScopeRestriction(in *api.ClusterRoleScopeRestriction, out *ClusterRoleScopeRestriction, s conversion.Scope) error {
	out.RoleNames = *(*[]string)(unsafe.Pointer(&in.RoleNames))
	out.Namespaces = *(*[]string)(unsafe.Pointer(&in.Namespaces))
	out.AllowEscalation = in.AllowEscalation
	return nil
}

func Convert_api_ClusterRoleScopeRestriction_To_v1_ClusterRoleScopeRestriction(in *api.ClusterRoleScopeRestriction, out *ClusterRoleScopeRestriction, s conversion.Scope) error {
	return autoConvert_api_ClusterRoleScopeRestriction_To_v1_ClusterRoleScopeRestriction(in, out, s)
}

func autoConvert_v1_OAuthAccessToken_To_api_OAuthAccessToken(in *OAuthAccessToken, out *api.OAuthAccessToken, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.ClientName = in.ClientName
	out.ExpiresIn = in.ExpiresIn
	out.Scopes = *(*[]string)(unsafe.Pointer(&in.Scopes))
	out.RedirectURI = in.RedirectURI
	out.UserName = in.UserName
	out.UserUID = in.UserUID
	out.AuthorizeToken = in.AuthorizeToken
	out.RefreshToken = in.RefreshToken
	return nil
}

func Convert_v1_OAuthAccessToken_To_api_OAuthAccessToken(in *OAuthAccessToken, out *api.OAuthAccessToken, s conversion.Scope) error {
	return autoConvert_v1_OAuthAccessToken_To_api_OAuthAccessToken(in, out, s)
}

func autoConvert_api_OAuthAccessToken_To_v1_OAuthAccessToken(in *api.OAuthAccessToken, out *OAuthAccessToken, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.ClientName = in.ClientName
	out.ExpiresIn = in.ExpiresIn
	out.Scopes = *(*[]string)(unsafe.Pointer(&in.Scopes))
	out.RedirectURI = in.RedirectURI
	out.UserName = in.UserName
	out.UserUID = in.UserUID
	out.AuthorizeToken = in.AuthorizeToken
	out.RefreshToken = in.RefreshToken
	return nil
}

func Convert_api_OAuthAccessToken_To_v1_OAuthAccessToken(in *api.OAuthAccessToken, out *OAuthAccessToken, s conversion.Scope) error {
	return autoConvert_api_OAuthAccessToken_To_v1_OAuthAccessToken(in, out, s)
}

func autoConvert_v1_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in *OAuthAccessTokenList, out *api.OAuthAccessTokenList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	out.Items = *(*[]api.OAuthAccessToken)(unsafe.Pointer(&in.Items))
	return nil
}

func Convert_v1_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in *OAuthAccessTokenList, out *api.OAuthAccessTokenList, s conversion.Scope) error {
	return autoConvert_v1_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in, out, s)
}

func autoConvert_api_OAuthAccessTokenList_To_v1_OAuthAccessTokenList(in *api.OAuthAccessTokenList, out *OAuthAccessTokenList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	out.Items = *(*[]OAuthAccessToken)(unsafe.Pointer(&in.Items))
	return nil
}

func Convert_api_OAuthAccessTokenList_To_v1_OAuthAccessTokenList(in *api.OAuthAccessTokenList, out *OAuthAccessTokenList, s conversion.Scope) error {
	return autoConvert_api_OAuthAccessTokenList_To_v1_OAuthAccessTokenList(in, out, s)
}

func autoConvert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in *OAuthAuthorizeToken, out *api.OAuthAuthorizeToken, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.ClientName = in.ClientName
	out.ExpiresIn = in.ExpiresIn
	out.Scopes = *(*[]string)(unsafe.Pointer(&in.Scopes))
	out.RedirectURI = in.RedirectURI
	out.State = in.State
	out.UserName = in.UserName
	out.UserUID = in.UserUID
	out.CodeChallenge = in.CodeChallenge
	out.CodeChallengeMethod = in.CodeChallengeMethod
	return nil
}

func Convert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in *OAuthAuthorizeToken, out *api.OAuthAuthorizeToken, s conversion.Scope) error {
	return autoConvert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in, out, s)
}

func autoConvert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken(in *api.OAuthAuthorizeToken, out *OAuthAuthorizeToken, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.ClientName = in.ClientName
	out.ExpiresIn = in.ExpiresIn
	out.Scopes = *(*[]string)(unsafe.Pointer(&in.Scopes))
	out.RedirectURI = in.RedirectURI
	out.State = in.State
	out.UserName = in.UserName
	out.UserUID = in.UserUID
	out.CodeChallenge = in.CodeChallenge
	out.CodeChallengeMethod = in.CodeChallengeMethod
	return nil
}

func Convert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken(in *api.OAuthAuthorizeToken, out *OAuthAuthorizeToken, s conversion.Scope) error {
	return autoConvert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken(in, out, s)
}

func autoConvert_v1_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in *OAuthAuthorizeTokenList, out *api.OAuthAuthorizeTokenList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	out.Items = *(*[]api.OAuthAuthorizeToken)(unsafe.Pointer(&in.Items))
	return nil
}

func Convert_v1_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in *OAuthAuthorizeTokenList, out *api.OAuthAuthorizeTokenList, s conversion.Scope) error {
	return autoConvert_v1_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in, out, s)
}

func autoConvert_api_OAuthAuthorizeTokenList_To_v1_OAuthAuthorizeTokenList(in *api.OAuthAuthorizeTokenList, out *OAuthAuthorizeTokenList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	out.Items = *(*[]OAuthAuthorizeToken)(unsafe.Pointer(&in.Items))
	return nil
}

func Convert_api_OAuthAuthorizeTokenList_To_v1_OAuthAuthorizeTokenList(in *api.OAuthAuthorizeTokenList, out *OAuthAuthorizeTokenList, s conversion.Scope) error {
	return autoConvert_api_OAuthAuthorizeTokenList_To_v1_OAuthAuthorizeTokenList(in, out, s)
}

func autoConvert_v1_OAuthClient_To_api_OAuthClient(in *OAuthClient, out *api.OAuthClient, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Secret = in.Secret
	out.AdditionalSecrets = *(*[]string)(unsafe.Pointer(&in.AdditionalSecrets))
	out.RespondWithChallenges = in.RespondWithChallenges
	out.RedirectURIs = *(*[]string)(unsafe.Pointer(&in.RedirectURIs))
	out.GrantMethod = api.GrantHandlerType(in.GrantMethod)
	out.ScopeRestrictions = *(*[]api.ScopeRestriction)(unsafe.Pointer(&in.ScopeRestrictions))
	return nil
}

func Convert_v1_OAuthClient_To_api_OAuthClient(in *OAuthClient, out *api.OAuthClient, s conversion.Scope) error {
	return autoConvert_v1_OAuthClient_To_api_OAuthClient(in, out, s)
}

func autoConvert_api_OAuthClient_To_v1_OAuthClient(in *api.OAuthClient, out *OAuthClient, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Secret = in.Secret
	out.AdditionalSecrets = *(*[]string)(unsafe.Pointer(&in.AdditionalSecrets))
	out.RespondWithChallenges = in.RespondWithChallenges
	out.RedirectURIs = *(*[]string)(unsafe.Pointer(&in.RedirectURIs))
	out.GrantMethod = GrantHandlerType(in.GrantMethod)
	out.ScopeRestrictions = *(*[]ScopeRestriction)(unsafe.Pointer(&in.ScopeRestrictions))
	return nil
}

func Convert_api_OAuthClient_To_v1_OAuthClient(in *api.OAuthClient, out *OAuthClient, s conversion.Scope) error {
	return autoConvert_api_OAuthClient_To_v1_OAuthClient(in, out, s)
}

func autoConvert_v1_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in *OAuthClientAuthorization, out *api.OAuthClientAuthorization, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.ClientName = in.ClientName
	out.UserName = in.UserName
	out.UserUID = in.UserUID
	out.Scopes = *(*[]string)(unsafe.Pointer(&in.Scopes))
	return nil
}

func Convert_v1_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in *OAuthClientAuthorization, out *api.OAuthClientAuthorization, s conversion.Scope) error {
	return autoConvert_v1_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in, out, s)
}

func autoConvert_api_OAuthClientAuthorization_To_v1_OAuthClientAuthorization(in *api.OAuthClientAuthorization, out *OAuthClientAuthorization, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.ClientName = in.ClientName
	out.UserName = in.UserName
	out.UserUID = in.UserUID
	out.Scopes = *(*[]string)(unsafe.Pointer(&in.Scopes))
	return nil
}

func Convert_api_OAuthClientAuthorization_To_v1_OAuthClientAuthorization(in *api.OAuthClientAuthorization, out *OAuthClientAuthorization, s conversion.Scope) error {
	return autoConvert_api_OAuthClientAuthorization_To_v1_OAuthClientAuthorization(in, out, s)
}

func autoConvert_v1_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in *OAuthClientAuthorizationList, out *api.OAuthClientAuthorizationList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	out.Items = *(*[]api.OAuthClientAuthorization)(unsafe.Pointer(&in.Items))
	return nil
}

func Convert_v1_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in *OAuthClientAuthorizationList, out *api.OAuthClientAuthorizationList, s conversion.Scope) error {
	return autoConvert_v1_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in, out, s)
}

func autoConvert_api_OAuthClientAuthorizationList_To_v1_OAuthClientAuthorizationList(in *api.OAuthClientAuthorizationList, out *OAuthClientAuthorizationList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	out.Items = *(*[]OAuthClientAuthorization)(unsafe.Pointer(&in.Items))
	return nil
}

func Convert_api_OAuthClientAuthorizationList_To_v1_OAuthClientAuthorizationList(in *api.OAuthClientAuthorizationList, out *OAuthClientAuthorizationList, s conversion.Scope) error {
	return autoConvert_api_OAuthClientAuthorizationList_To_v1_OAuthClientAuthorizationList(in, out, s)
}

func autoConvert_v1_OAuthClientList_To_api_OAuthClientList(in *OAuthClientList, out *api.OAuthClientList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	out.Items = *(*[]api.OAuthClient)(unsafe.Pointer(&in.Items))
	return nil
}

func Convert_v1_OAuthClientList_To_api_OAuthClientList(in *OAuthClientList, out *api.OAuthClientList, s conversion.Scope) error {
	return autoConvert_v1_OAuthClientList_To_api_OAuthClientList(in, out, s)
}

func autoConvert_api_OAuthClientList_To_v1_OAuthClientList(in *api.OAuthClientList, out *OAuthClientList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	out.Items = *(*[]OAuthClient)(unsafe.Pointer(&in.Items))
	return nil
}

func Convert_api_OAuthClientList_To_v1_OAuthClientList(in *api.OAuthClientList, out *OAuthClientList, s conversion.Scope) error {
	return autoConvert_api_OAuthClientList_To_v1_OAuthClientList(in, out, s)
}

func autoConvert_v1_OAuthRedirectReference_To_api_OAuthRedirectReference(in *OAuthRedirectReference, out *api.OAuthRedirectReference, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_v1_RedirectReference_To_api_RedirectReference(&in.Reference, &out.Reference, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_OAuthRedirectReference_To_api_OAuthRedirectReference(in *OAuthRedirectReference, out *api.OAuthRedirectReference, s conversion.Scope) error {
	return autoConvert_v1_OAuthRedirectReference_To_api_OAuthRedirectReference(in, out, s)
}

func autoConvert_api_OAuthRedirectReference_To_v1_OAuthRedirectReference(in *api.OAuthRedirectReference, out *OAuthRedirectReference, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_api_RedirectReference_To_v1_RedirectReference(&in.Reference, &out.Reference, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_OAuthRedirectReference_To_v1_OAuthRedirectReference(in *api.OAuthRedirectReference, out *OAuthRedirectReference, s conversion.Scope) error {
	return autoConvert_api_OAuthRedirectReference_To_v1_OAuthRedirectReference(in, out, s)
}

func autoConvert_v1_RedirectReference_To_api_RedirectReference(in *RedirectReference, out *api.RedirectReference, s conversion.Scope) error {
	out.Group = in.Group
	out.Kind = in.Kind
	out.Name = in.Name
	return nil
}

func Convert_v1_RedirectReference_To_api_RedirectReference(in *RedirectReference, out *api.RedirectReference, s conversion.Scope) error {
	return autoConvert_v1_RedirectReference_To_api_RedirectReference(in, out, s)
}

func autoConvert_api_RedirectReference_To_v1_RedirectReference(in *api.RedirectReference, out *RedirectReference, s conversion.Scope) error {
	out.Group = in.Group
	out.Kind = in.Kind
	out.Name = in.Name
	return nil
}

func Convert_api_RedirectReference_To_v1_RedirectReference(in *api.RedirectReference, out *RedirectReference, s conversion.Scope) error {
	return autoConvert_api_RedirectReference_To_v1_RedirectReference(in, out, s)
}

func autoConvert_v1_ScopeRestriction_To_api_ScopeRestriction(in *ScopeRestriction, out *api.ScopeRestriction, s conversion.Scope) error {
	out.ExactValues = *(*[]string)(unsafe.Pointer(&in.ExactValues))
	out.ClusterRole = (*api.ClusterRoleScopeRestriction)(unsafe.Pointer(in.ClusterRole))
	return nil
}

func Convert_v1_ScopeRestriction_To_api_ScopeRestriction(in *ScopeRestriction, out *api.ScopeRestriction, s conversion.Scope) error {
	return autoConvert_v1_ScopeRestriction_To_api_ScopeRestriction(in, out, s)
}

func autoConvert_api_ScopeRestriction_To_v1_ScopeRestriction(in *api.ScopeRestriction, out *ScopeRestriction, s conversion.Scope) error {
	out.ExactValues = *(*[]string)(unsafe.Pointer(&in.ExactValues))
	out.ClusterRole = (*ClusterRoleScopeRestriction)(unsafe.Pointer(in.ClusterRole))
	return nil
}

func Convert_api_ScopeRestriction_To_v1_ScopeRestriction(in *api.ScopeRestriction, out *ScopeRestriction, s conversion.Scope) error {
	return autoConvert_api_ScopeRestriction_To_v1_ScopeRestriction(in, out, s)
}
