package etcd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	kapirest "k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"

	"github.com/openshift/origin/pkg/route"
	"github.com/openshift/origin/pkg/route/api"
	rest "github.com/openshift/origin/pkg/route/registry/route"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against routes.
func NewREST(optsGetter restoptions.Getter, allocator route.RouteAllocator) (*REST, *StatusREST, error) {
	strategy := rest.NewStrategy(allocator)

	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.Route{} },
		NewListFunc:       func() runtime.Object { return &api.RouteList{} },
		PredicateFunc:     rest.Matcher,
		QualifiedResource: api.Resource("routes"),

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
	}

	// TODO this will be uncommented after 1.6 rebase:
	// options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: rest.GetAttrs}
	// if err := store.CompleteWithOptions(options); err != nil {
	if err := restoptions.ApplyOptions(optsGetter, store, storage.NoTriggerPublisher); err != nil {
		return nil, nil, err
	}

	statusStore := *store
	statusStore.UpdateStrategy = rest.StatusStrategy

	return &REST{store}, &StatusREST{&statusStore}, nil
}

// StatusREST implements the REST endpoint for changing the status of a route.
type StatusREST struct {
	store *registry.Store
}

// StatusREST implements Patcher
var _ = kapirest.Patcher(&StatusREST{})

// New creates a new route resource
func (r *StatusREST) New() runtime.Object {
	return &api.Route{}
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx apirequest.Context, name string, objInfo kapirest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo)
}
