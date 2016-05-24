package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/registry/hostsubnet"
)

// rest implements a RESTStorage for sdn against etcd
type REST struct {
	registry.Store
}

const etcdPrefix = "/registry/sdnsubnets"

// NewREST returns a RESTStorage object that will work against subnets
func NewREST(opts generic.RESTOptions) *REST {
	newListFunc := func() runtime.Object { return &api.HostSubnetList{} }
	storageInterface := opts.Decorator(opts.Storage, 100, &api.HostSubnetList{}, etcdPrefix, hostsubnet.Strategy, newListFunc)

	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.HostSubnet{} },
		NewListFunc: newListFunc,
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return registry.NoNamespaceKeyFunc(ctx, etcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.HostSubnet).Host, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return hostsubnet.Matcher(label, field)
		},
		QualifiedResource: api.Resource("hostsubnets"),

		Storage: storageInterface,
	}

	store.CreateStrategy = hostsubnet.Strategy
	store.UpdateStrategy = hostsubnet.Strategy

	return &REST{*store}
}
