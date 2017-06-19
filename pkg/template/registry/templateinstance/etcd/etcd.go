package etcd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	kapirest "k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	templateapi "github.com/openshift/origin/pkg/template/api"
	rest "github.com/openshift/origin/pkg/template/registry/templateinstance"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST implements a RESTStorage for templateinstances against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against templateinstances.
func NewREST(optsGetter restoptions.Getter, kc kclientset.Interface) (*REST, *StatusREST, error) {
	strategy := rest.NewStrategy(kc)

	store := &registry.Store{
		Copier:            kapi.Scheme,
		NewFunc:           func() runtime.Object { return &templateapi.TemplateInstance{} },
		NewListFunc:       func() runtime.Object { return &templateapi.TemplateInstanceList{} },
		PredicateFunc:     rest.Matcher,
		QualifiedResource: templateapi.Resource("templateinstances"),

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: rest.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, nil, err
	}

	statusStore := *store
	statusStore.UpdateStrategy = rest.StatusStrategy

	return &REST{store}, &StatusREST{&statusStore}, nil
}

// StatusREST implements the REST endpoint for changing the status of a templateInstance.
type StatusREST struct {
	store *registry.Store
}

// StatusREST implements Patcher
var _ = kapirest.Patcher(&StatusREST{})

// New creates a new templateInstance resource
func (r *StatusREST) New() runtime.Object {
	return &templateapi.TemplateInstance{}
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx request.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx request.Context, name string, objInfo kapirest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo)
}
