package prompt

import (
	"fmt"
	"reflect"

	oauthhandlers "github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/server/grant"
	"github.com/openshift/origin/pkg/cmd/auth"
	oauthetcd "github.com/openshift/origin/pkg/oauth/registry/etcd"
)

func init() {
	auth.RegisterInstantiator(newInstantiator())
}

type instantiator struct {
	ownedReturnType reflect.Type
	ownedConfigType string
}

func newInstantiator() *instantiator {
	return &instantiator{reflect.TypeOf((*oauthhandlers.GrantHandler)(nil)).Elem(), "prompt"}
}

func (a *instantiator) Owns(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo) bool {
	return (resultingType == a.ownedReturnType) && (elementConfigInfo.AuthElementConfigType == a.ownedConfigType)
}
func (a *instantiator) IsValid(elementConfigInfo auth.AuthElementConfigInfo, authConfigInfo auth.AuthConfigInfo) error {
	return nil
}
func (a *instantiator) Instantiate(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo, authConfig auth.AuthConfig, envInfo *auth.EnvInfo) (interface{}, error) {
	if !a.Owns(resultingType, elementConfigInfo) {
		return nil, fmt.Errorf("%v does not own %v", a, elementConfigInfo)
	}

	oauthEtcd := oauthetcd.New(envInfo.EtcdHelper)
	grantServer := grant.NewGrant(auth.GetCSRF(), authConfig.GetAuthorizeRequestAuthenticator(), grant.DefaultFormRenderer, oauthEtcd, oauthEtcd)
	grantServer.Install(envInfo.Mux, auth.OpenShiftApprovePrefix)
	return oauthhandlers.NewRedirectGrant(auth.OpenShiftApprovePrefix), nil
}
