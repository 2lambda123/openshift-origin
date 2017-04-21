package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for authorize tokens against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against authorize tokens
func NewREST(optsGetter restoptions.Getter, clientGetter oauthclient.Getter) (*REST, error) {
	strategy := oauthauthorizetoken.NewStrategy(clientGetter)
	store := &registry.Store{
		Copier:            kapi.Scheme,
		NewFunc:           func() runtime.Object { return &api.OAuthAuthorizeToken{} },
		NewListFunc:       func() runtime.Object { return &api.OAuthAuthorizeTokenList{} },
		PredicateFunc:     oauthauthorizetoken.Matcher,
		QualifiedResource: api.Resource("oauthauthorizetokens"),

		TTLFunc: func(obj runtime.Object, existing uint64, update bool) (uint64, error) {
			token := obj.(*api.OAuthAuthorizeToken)
			expires := uint64(token.ExpiresIn)
			return expires, nil
		},

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: oauthauthorizetoken.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}
	return &REST{store}, nil
}
