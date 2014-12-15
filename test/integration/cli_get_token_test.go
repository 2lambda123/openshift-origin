// +build integration,!no-etcd

package integration

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"

	// for osinserver setup.
	"github.com/openshift/origin/pkg/auth/authenticator/challenger/passwordchallenger"
	"github.com/openshift/origin/pkg/auth/authenticator/password/allowanypassword"
	"github.com/openshift/origin/pkg/auth/authenticator/request/basicauthrequest"
	oauthhandlers "github.com/openshift/origin/pkg/auth/oauth/handlers"
	oauthregistry "github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	oauthetcd "github.com/openshift/origin/pkg/oauth/registry/etcd"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
	"github.com/openshift/origin/pkg/oauth/server/osinserver/registrystorage"
	"github.com/openshift/origin/pkg/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/etcd"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
)

func init() {
	requireEtcd()
}

func TestGetToken(t *testing.T) {
	deleteAllEtcdKeys()

	// setup
	etcdClient := newEtcdClient()
	etcdHelper, _ := master.NewEtcdHelper(etcdClient, klatest.Version)
	oauthEtcd := oauthetcd.New(etcdHelper)
	userRegistry := useretcd.New(etcdHelper, user.NewDefaultUserInitStrategy())
	identityMapper := identitymapper.NewAlwaysCreateUserIdentityToUserMapper("front-proxy-test" /*for now*/, userRegistry)

	authRequestHandler := basicauthrequest.NewBasicAuthAuthentication(allowanypassword.New(identityMapper))
	authHandler := oauthhandlers.NewUnionAuthenticationHandler(
		map[string]oauthhandlers.AuthenticationChallenger{"login": passwordchallenger.NewBasicAuthChallenger("openshift")}, nil, nil)

	storage := registrystorage.New(oauthEtcd, oauthEtcd, oauthEtcd, oauthregistry.NewUserConversion())
	config := osinserver.NewDefaultServerConfig()

	grantChecker := oauthregistry.NewClientAuthorizationGrantChecker(oauthEtcd)
	grantHandler := oauthhandlers.NewAutoGrant(oauthEtcd)

	server := osinserver.New(
		config,
		storage,
		osinserver.AuthorizeHandlers{
			oauthhandlers.NewAuthorizeAuthenticator(
				authRequestHandler,
				authHandler,
				oauthhandlers.EmptyError{},
			),
			oauthhandlers.NewGrantCheck(
				grantChecker,
				grantHandler,
				oauthhandlers.EmptyError{},
			),
		},
		osinserver.AccessHandlers{
			oauthhandlers.NewDenyAccessAuthenticator(),
		},
		osinserver.NewDefaultErrorHandler(),
	)
	mux := http.NewServeMux()
	server.Install(mux, origin.OpenShiftOAuthAPIPrefix)
	oauthServer := httptest.NewServer(http.Handler(mux))
	glog.Infof("oauth server is on %v\n", oauthServer.URL)

	// create the default oauth clients with redirects to our server
	origin.CreateOrUpdateDefaultOAuthClients(oauthServer.URL, oauthEtcd)

	flags := pflag.NewFlagSet("test-flags", pflag.ContinueOnError)
	clientCfg := clientcmd.NewConfig()
	clientCfg.Bind(flags)
	flags.Parse(strings.Split("--master="+oauthServer.URL, " "))

	reader := bytes.NewBufferString("user\npass")
	accessToken, err := tokencmd.RequestToken(clientCfg, reader)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(accessToken) == 0 {
		t.Error("Expected accessToken, but did not get one")
	}

	// lets see if this access token is any good
	token, err := oauthEtcd.GetAccessToken(accessToken)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if token.UserName != "front-proxy-test:user" {
		t.Errorf("Expected token for \"user\", but got: %#v", token)
	}
}
