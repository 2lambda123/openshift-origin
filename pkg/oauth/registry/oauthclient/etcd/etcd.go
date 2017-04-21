package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for oauth clients against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against oauth clients
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		Copier:            kapi.Scheme,
		NewFunc:           func() runtime.Object { return &api.OAuthClient{} },
		NewListFunc:       func() runtime.Object { return &api.OAuthClientList{} },
		PredicateFunc:     oauthclient.Matcher,
		QualifiedResource: api.Resource("oauthclients"),

		CreateStrategy: oauthclient.Strategy,
		UpdateStrategy: oauthclient.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: oauthclient.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
