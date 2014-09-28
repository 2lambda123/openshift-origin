package v1beta1

import "github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

func init() {
	runtime.AddKnownTypes("v1beta1",
		AccessToken{},
		AccessTokenList{},
		AuthorizeToken{},
		AuthorizeTokenList{},
		Client{},
		ClientList{},
		ClientAuthorization{},
		ClientAuthorizationList{},
	)
}
