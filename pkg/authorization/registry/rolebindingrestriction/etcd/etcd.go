package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/rolebindingrestriction"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against nodes.
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		Copier:            kapi.Scheme,
		NewFunc:           func() runtime.Object { return &authorizationapi.RoleBindingRestriction{} },
		NewListFunc:       func() runtime.Object { return &authorizationapi.RoleBindingRestrictionList{} },
		QualifiedResource: authorizationapi.Resource("rolebindingrestrictions"),
		PredicateFunc:     rolebindingrestriction.Matcher,

		CreateStrategy: rolebindingrestriction.Strategy,
		UpdateStrategy: rolebindingrestriction.Strategy,
		DeleteStrategy: rolebindingrestriction.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: rolebindingrestriction.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{Store: store}, nil
}
