package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/registry/identity"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST implements a RESTStorage for identites against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against identites
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		Copier:            kapi.Scheme,
		NewFunc:           func() runtime.Object { return &api.Identity{} },
		NewListFunc:       func() runtime.Object { return &api.IdentityList{} },
		PredicateFunc:     identity.Matcher,
		QualifiedResource: api.Resource("identities"),

		CreateStrategy: identity.Strategy,
		UpdateStrategy: identity.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: identity.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
