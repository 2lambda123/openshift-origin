package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/registry/netnamespace"
)

// rest implements a RESTStorage for sdn against etcd
type REST struct {
	registry.Store
}

const etcdPrefix = "/registry/sdnnetnamespaces"

// NewREST returns a RESTStorage object that will work against netnamespaces
func NewREST(opts generic.RESTOptions) *REST {
	newListFunc := func() runtime.Object { return &api.NetNamespaceList{} }
	storageInterface := opts.Decorator(opts.Storage, 100, &api.NetNamespaceList{}, etcdPrefix, netnamespace.Strategy, newListFunc)

	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.NetNamespace{} },
		NewListFunc: newListFunc,
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return registry.NoNamespaceKeyFunc(ctx, etcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.NetNamespace).NetName, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return netnamespace.Matcher(label, field)
		},
		QualifiedResource: api.Resource("netnamespace"),

		Storage: storageInterface,
	}

	store.CreateStrategy = netnamespace.Strategy
	store.UpdateStrategy = netnamespace.Strategy

	return &REST{*store}
}
