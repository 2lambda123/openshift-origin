package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against ClusterPolicy.
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		Copier:            kapi.Scheme,
		NewFunc:           func() runtime.Object { return &authorizationapi.ClusterPolicy{} },
		NewListFunc:       func() runtime.Object { return &authorizationapi.ClusterPolicyList{} },
		QualifiedResource: authorizationapi.Resource("clusterpolicies"),
		PredicateFunc:     clusterpolicy.Matcher,

		CreateStrategy: clusterpolicy.Strategy,
		UpdateStrategy: clusterpolicy.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: clusterpolicy.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
