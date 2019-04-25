package oauth

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kauthenticator "k8s.io/apiserver/pkg/authentication/authenticator"
	kuser "k8s.io/apiserver/pkg/authentication/user"

	userapi "github.com/openshift/api/user/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/password/bootstrap"
)

type bootstrapAuthenticator struct {
	tokens    oauthclient.OAuthAccessTokenInterface
	getter    bootstrap.BootstrapUserDataGetter
	validator OAuthTokenValidator
}

func NewBootstrapAuthenticator(tokens oauthclient.OAuthAccessTokenInterface, getter bootstrap.BootstrapUserDataGetter, validators ...OAuthTokenValidator) kauthenticator.Token {
	return &bootstrapAuthenticator{
		tokens:    tokens,
		getter:    getter,
		validator: OAuthTokenValidators(validators),
	}
}

func (a *bootstrapAuthenticator) AuthenticateToken(ctx context.Context, name string) (*kauthenticator.Response, bool, error) {
	token, err := a.tokens.Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("oauth token get error %s", config.Sdump(err, token))
	}

	if token.UserName != bootstrap.BootstrapUser {
		return nil, false, nil
	}

	data, ok, err := a.getter.Get()
	if err != nil || !ok {
		return nil, ok, err
	}

	// this allows us to reuse existing validators
	// since the uid is based on the secret, if the secret changes, all
	// tokens issued for the bootstrap user before that change stop working
	fakeUser := &userapi.User{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID(data.UID),
		},
	}

	if err := a.validator.Validate(token, fakeUser); err != nil {
		return nil, false, err
	}

	// we explicitly do not set UID as we do not want to leak any derivative of the password
	return &kauthenticator.Response{
		User: &kuser.DefaultInfo{
			Name: bootstrap.BootstrapUser,
			// we cannot use SystemPrivilegedGroup because it cannot be properly scoped.
			// see openshift/origin#18922 and how loopback connections are handled upstream via AuthorizeClientBearerToken.
			// api aggregation with delegated authorization makes this impossible to control, see WithAlwaysAllowGroups.
			// an openshift specific cluster role binding binds ClusterAdminGroup to the cluster role cluster-admin.
			// thus this group is authorized to do everything via RBAC.
			// this does make the bootstrap user susceptible to anything that causes the RBAC authorizer to fail.
			// this is a safe trade-off because scopes must always be evaluated before RBAC for them to work at all.
			// a failure in that logic means scopes are broken instead of a specific failure related to the bootstrap user.
			// if this becomes a problem in the future, we could generate a custom extra value based on the secret content
			// and store it in BootstrapUserData, similar to how UID is calculated.  this extra value would then be wired
			// to a custom authorizer that allows all actions.  the problem with such an approach is that since we do not
			// allow remote authorizers in OpenShift, the BootstrapUserDataGetter logic would have to be shared between the
			// the kube api server and osin instead of being an implementation detail hidden inside of osin.  currently the
			// only shared code is the value of the BootstrapUser constant (since it is special cased in validation).
			Groups: []string{bootstrappolicy.ClusterAdminGroup},
			Extra: map[string][]string{
				// this user still needs scopes because it can be used in OAuth flows (unlike cert based users)
				authorizationapi.ScopesKey: token.Scopes,
			},
		},
	}, true, nil
}
