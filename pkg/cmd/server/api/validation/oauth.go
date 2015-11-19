package validation

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	"k8s.io/kubernetes/pkg/util/fielderrors"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/auth/authenticator/redirector"
	"github.com/openshift/origin/pkg/auth/server/login"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	"github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/user/api/validation"
)

func ValidateOAuthConfig(config *api.OAuthConfig) ValidationResults {
	validationResults := ValidationResults{}

	if config.MasterCA == nil {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("masterCA", config.MasterCA, "a filename or empty string is required"))
	} else if len(*config.MasterCA) > 0 {
		validationResults.AddErrors(ValidateFile(*config.MasterCA, "masterCA")...)
	}

	if len(config.MasterURL) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("masterURL"))
	} else if _, urlErrs := ValidateURL(config.MasterURL, "masterURL"); len(urlErrs) > 0 {
		validationResults.AddErrors(urlErrs...)
	}

	if _, urlErrs := ValidateURL(config.MasterPublicURL, "masterPublicURL"); len(urlErrs) > 0 {
		validationResults.AddErrors(urlErrs...)
	}

	if len(config.AssetPublicURL) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("assetPublicURL"))
	}

	if config.SessionConfig != nil {
		validationResults.AddErrors(ValidateSessionConfig(config.SessionConfig).Prefix("sessionConfig")...)
	}

	validationResults.AddErrors(ValidateGrantConfig(config.GrantConfig).Prefix("grantConfig")...)

	providerNames := sets.NewString()
	redirectingIdentityProviders := []string{}

	challengeIssuingIdentityProviders := []string{}
	challengeRedirectingIdentityProviders := []string{}

	for i, identityProvider := range config.IdentityProviders {
		if identityProvider.UseAsLogin {
			redirectingIdentityProviders = append(redirectingIdentityProviders, identityProvider.Name)

			if api.IsPasswordAuthenticator(identityProvider) {
				if config.SessionConfig == nil {
					validationResults.AddErrors(fielderrors.NewFieldInvalid("sessionConfig", config, "sessionConfig is required if a password identity provider is used for browser based login"))
				}
			}
		}

		if identityProvider.UseAsChallenger {
			// RequestHeaderIdentityProvider is special, it can only react to challenge clients by redirecting them
			// Make sure we don't have more than a single redirector, and don't have a mix of challenge issuers and redirectors
			if _, isRequestHeader := identityProvider.Provider.Object.(*api.RequestHeaderIdentityProvider); isRequestHeader {
				challengeRedirectingIdentityProviders = append(challengeRedirectingIdentityProviders, identityProvider.Name)
			} else {
				challengeIssuingIdentityProviders = append(challengeIssuingIdentityProviders, identityProvider.Name)
			}
		}

		validationResults.Append(ValidateIdentityProvider(identityProvider).Prefix(fmt.Sprintf("identityProvider[%d]", i)))

		if len(identityProvider.Name) > 0 {
			if providerNames.Has(identityProvider.Name) {
				validationResults.AddErrors(fielderrors.NewFieldInvalid(fmt.Sprintf("identityProvider[%d].name", i), identityProvider.Name, "must have a unique name"))
			}
			providerNames.Insert(identityProvider.Name)
		}
	}

	if len(redirectingIdentityProviders) > 1 {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("identityProviders", "login", fmt.Sprintf("only one identity provider can support login for a browser, found: %v", strings.Join(redirectingIdentityProviders, ", "))))
	}
	if len(challengeRedirectingIdentityProviders) > 1 {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("identityProviders", "challenge", fmt.Sprintf("only one identity provider can redirect clients requesting an authentication challenge, found: %v", strings.Join(challengeRedirectingIdentityProviders, ", "))))
	}
	if len(challengeRedirectingIdentityProviders) > 0 && len(challengeIssuingIdentityProviders) > 0 {
		validationResults.AddErrors(
			fielderrors.NewFieldInvalid("identityProviders", "challenge", fmt.Sprintf(
				"cannot mix providers that redirect clients requesting auth challenges (%s) with providers issuing challenges to those clients (%s)",
				strings.Join(challengeRedirectingIdentityProviders, ", "),
				strings.Join(challengeIssuingIdentityProviders, ", "),
			)))
	}

	if config.Templates != nil && len(config.Templates.Login) > 0 {
		content, err := ioutil.ReadFile(config.Templates.Login)
		if err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("templates.login", config.Templates.Login, "could not read file"))
		} else {
			for _, err = range login.ValidateLoginTemplate(content) {
				validationResults.AddErrors(fielderrors.NewFieldInvalid("templates.login", config.Templates.Login, err.Error()))
			}
		}
	}

	return validationResults
}

var validMappingMethods = sets.NewString(
	string(identitymapper.MappingMethodLookup),
	string(identitymapper.MappingMethodClaim),
	string(identitymapper.MappingMethodAdd),
	string(identitymapper.MappingMethodGenerate),
)

func ValidateIdentityProvider(identityProvider api.IdentityProvider) ValidationResults {
	validationResults := ValidationResults{}

	if len(identityProvider.Name) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("name"))
	}
	if ok, err := validation.ValidateIdentityProviderName(identityProvider.Name); !ok {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("name", identityProvider.Name, err))
	}

	if len(identityProvider.MappingMethod) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("mappingMethod"))
	} else if !validMappingMethods.Has(identityProvider.MappingMethod) {
		validationResults.AddErrors(fielderrors.NewFieldValueNotSupported("mappingMethod", identityProvider.MappingMethod, validMappingMethods.List()))
	}

	if !api.IsIdentityProviderType(identityProvider.Provider) {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("provider", identityProvider.Provider, fmt.Sprintf("%v is invalid in this context", identityProvider.Provider)))
	} else {
		switch provider := identityProvider.Provider.Object.(type) {
		case (*api.RequestHeaderIdentityProvider):
			validationResults.Append(ValidateRequestHeaderIdentityProvider(provider, identityProvider))

		case (*api.BasicAuthPasswordIdentityProvider):
			validationResults.AddErrors(ValidateRemoteConnectionInfo(provider.RemoteConnectionInfo).Prefix("provider")...)

		case (*api.HTPasswdPasswordIdentityProvider):
			validationResults.AddErrors(ValidateFile(provider.File, "provider.file")...)

		case (*api.LDAPPasswordIdentityProvider):
			validationResults.Append(ValidateLDAPIdentityProvider(provider))

		case (*api.KeystonePasswordIdentityProvider):
			validationResults.Append(ValidateKeystoneIdentityProvider(provider, identityProvider).Prefix("provider"))

		case (*api.GitHubIdentityProvider):
			validationResults.AddErrors(ValidateOAuthIdentityProvider(provider.ClientID, provider.ClientSecret, identityProvider.UseAsChallenger)...)

		case (*api.GoogleIdentityProvider):
			validationResults.AddErrors(ValidateOAuthIdentityProvider(provider.ClientID, provider.ClientSecret, identityProvider.UseAsChallenger)...)

		case (*api.OpenIDIdentityProvider):
			validationResults.AddErrors(ValidateOpenIDIdentityProvider(provider, identityProvider)...)

		}
	}

	return validationResults
}

func ValidateLDAPIdentityProvider(provider *api.LDAPPasswordIdentityProvider) ValidationResults {
	validationResults := ValidateLDAPClientConfig(provider.URL, provider.BindDN, provider.BindPassword, provider.CA, provider.Insecure).Prefix("provider")

	// At least one attribute to use as the user id is required
	if len(provider.Attributes.ID) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("provider.attributes.id", "[]", "at least one id attribute is required (LDAP standard identity attribute is 'dn')"))
	}

	return validationResults
}

// RemoteConnection fields validated separately -- this is for keystone-specific validation
func ValidateKeystoneIdentityProvider(provider *api.KeystonePasswordIdentityProvider, identityProvider api.IdentityProvider) ValidationResults {
	validationResults := ValidationResults{}
	validationResults.AddErrors(ValidateRemoteConnectionInfo(provider.RemoteConnectionInfo)...)

	providerURL, err := url.Parse(provider.RemoteConnectionInfo.URL)
	if err == nil {
		if providerURL.Scheme != "https" {
			validationResults.AddWarnings(fielderrors.NewFieldInvalid("url", provider.RemoteConnectionInfo.URL, "Auth URL should be secure and start with https"))
		}
	}
	if len(provider.DomainName) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("domainName"))
	}

	return validationResults
}

func ValidateRequestHeaderIdentityProvider(provider *api.RequestHeaderIdentityProvider, identityProvider api.IdentityProvider) ValidationResults {
	validationResults := ValidationResults{}

	if len(provider.ClientCA) > 0 {
		validationResults.AddErrors(ValidateFile(provider.ClientCA, "provider.clientCA")...)
	}
	if len(provider.Headers) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("provider.headers"))
	}
	if identityProvider.UseAsChallenger && len(provider.ChallengeURL) == 0 {
		err := fielderrors.NewFieldRequired("provider.challengeURL")
		err.Detail = "challengeURL is required if challenge=true"
		validationResults.AddErrors(err)
	}
	if identityProvider.UseAsLogin && len(provider.LoginURL) == 0 {
		err := fielderrors.NewFieldRequired("provider.loginURL")
		err.Detail = "loginURL is required if login=true"
		validationResults.AddErrors(err)
	}

	if len(provider.ChallengeURL) > 0 {
		url, urlErrs := ValidateURL(provider.ChallengeURL, "provider.challengeURL")
		validationResults.AddErrors(urlErrs...)
		if len(urlErrs) == 0 && !strings.Contains(url.RawQuery, redirector.URLToken) && !strings.Contains(url.RawQuery, redirector.QueryToken) {
			validationResults.AddWarnings(
				fielderrors.NewFieldInvalid(
					"provider.challengeURL",
					provider.ChallengeURL,
					fmt.Sprintf("query does not include %q or %q, redirect will not preserve original authorize parameters", redirector.URLToken, redirector.QueryToken),
				),
			)
		}
	}
	if len(provider.LoginURL) > 0 {
		url, urlErrs := ValidateURL(provider.LoginURL, "provider.loginURL")
		validationResults.AddErrors(urlErrs...)
		if len(urlErrs) == 0 && !strings.Contains(url.RawQuery, redirector.URLToken) && !strings.Contains(url.RawQuery, redirector.QueryToken) {
			validationResults.AddWarnings(
				fielderrors.NewFieldInvalid(
					"provider.loginURL",
					provider.LoginURL,
					fmt.Sprintf("query does not include %q or %q, redirect will not preserve original authorize parameters", redirector.URLToken, redirector.QueryToken),
				),
			)
		}
	}

	// Warn if it looks like they expect direct requests to the OAuth endpoints, and have not secured the header checking with a client certificate check
	if len(provider.ClientCA) == 0 && (len(provider.ChallengeURL) > 0 || len(provider.LoginURL) > 0) {
		validationResults.AddWarnings(fielderrors.NewFieldInvalid("provider.clientCA", "", "if no clientCA is set, no request verification is done, and any request directly against the OAuth server can impersonate any identity from this provider"))
	}

	return validationResults
}

func ValidateOAuthIdentityProvider(clientID, clientSecret string, challenge bool) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(clientID) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("provider.clientID"))
	}
	if len(clientSecret) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("provider.clientSecret"))
	}
	if challenge {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("challenge", challenge, "oauth providers cannot be used for challenges"))
	}

	return allErrs
}

func ValidateOpenIDIdentityProvider(provider *api.OpenIDIdentityProvider, identityProvider api.IdentityProvider) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateOAuthIdentityProvider(provider.ClientID, provider.ClientSecret, identityProvider.UseAsChallenger)...)

	// Communication with the Authorization Endpoint MUST utilize TLS
	// http://openid.net/specs/openid-connect-core-1_0.html#AuthorizationEndpoint
	_, urlErrs := ValidateSecureURL(provider.URLs.Authorize, "authorize")
	allErrs = append(allErrs, urlErrs.Prefix("provider.urls")...)

	// Communication with the Token Endpoint MUST utilize TLS
	// http://openid.net/specs/openid-connect-core-1_0.html#TokenEndpoint
	_, urlErrs = ValidateSecureURL(provider.URLs.Token, "token")
	allErrs = append(allErrs, urlErrs.Prefix("provider.urls")...)

	if len(provider.URLs.UserInfo) != 0 {
		// Communication with the UserInfo Endpoint MUST utilize TLS
		// http://openid.net/specs/openid-connect-core-1_0.html#UserInfo
		_, urlErrs = ValidateSecureURL(provider.URLs.UserInfo, "userInfo")
		allErrs = append(allErrs, urlErrs.Prefix("provider.urls")...)
	}

	// At least one claim to use as the user id is required
	if len(provider.Claims.ID) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("provider.claims.id", "[]", "at least one id claim is required (OpenID standard identity claim is 'sub')"))
	}

	if len(provider.CA) != 0 {
		allErrs = append(allErrs, ValidateFile(provider.CA, "provider.ca")...)
	}

	return allErrs
}

func ValidateGrantConfig(config api.GrantConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if !api.ValidGrantHandlerTypes.Has(string(config.Method)) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("grantConfig.method", config.Method, fmt.Sprintf("must be one of: %v", api.ValidGrantHandlerTypes.List())))
	}

	return allErrs
}

func ValidateSessionConfig(config *api.SessionConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	// Validate session secrets file, if specified
	if len(config.SessionSecretsFile) > 0 {
		fileErrs := ValidateFile(config.SessionSecretsFile, "sessionSecretsFile")
		if len(fileErrs) != 0 {
			// Missing file
			allErrs = append(allErrs, fileErrs...)
		} else {
			// Validate file contents
			secrets, err := latest.ReadSessionSecrets(config.SessionSecretsFile)
			if err != nil {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("sessionSecretsFile", config.SessionSecretsFile, fmt.Sprintf("error reading file: %v", err)))
			} else {
				for _, err := range ValidateSessionSecrets(secrets) {
					allErrs = append(allErrs, fielderrors.NewFieldInvalid("sessionSecretsFile", config.SessionSecretsFile, err.Error()))
				}
			}
		}
	}

	if len(config.SessionName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("sessionName"))
	}

	return allErrs
}

func ValidateSessionSecrets(config *api.SessionSecrets) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.Secrets) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("secrets"))
	}

	for i, secret := range config.Secrets {
		switch {
		case len(secret.Authentication) == 0:
			allErrs = append(allErrs, fielderrors.NewFieldRequired(fmt.Sprintf("secrets[%d].authentication", i)))
		case len(secret.Authentication) < 32:
			// Don't output current value in error message... we don't want it logged
			allErrs = append(allErrs,
				fielderrors.NewFieldInvalid(
					fmt.Sprintf("secrets[%d].authentication", i),
					strings.Repeat("*", len(secret.Authentication)),
					"must be at least 32 characters long",
				),
			)
		}

		switch len(secret.Encryption) {
		case 0:
			// Require encryption secrets
			allErrs = append(allErrs, fielderrors.NewFieldRequired(fmt.Sprintf("secrets[%d].encryption", i)))
		case 16, 24, 32:
			// Valid lengths
		default:
			// Don't output current value in error message... we don't want it logged
			allErrs = append(allErrs,
				fielderrors.NewFieldInvalid(
					fmt.Sprintf("secrets[%d].encryption", i),
					strings.Repeat("*", len(secret.Encryption)),
					"must be 16, 24, or 32 characters long",
				),
			)
		}
	}

	return allErrs
}
