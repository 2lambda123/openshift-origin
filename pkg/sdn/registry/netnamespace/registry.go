package netnamespace

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/sdn/api"
)

// Registry is an interface implemented by things that know how to store sdn objects.
type Registry interface {
	// ListNetNamespaces obtains a list of NetNamespaces
	ListNetNamespaces(ctx kapi.Context, label labels.Selector, field fields.Selector) (*api.NetNamespaceList, error)
	// GetNetNamespace returns a specific NetNamespace
	GetNetNamespace(ctx kapi.Context, name string) (*api.NetNamespace, error)
	// CreateNetNamespace creates a NetNamespace
	CreateNetNamespace(ctx kapi.Context, nn *api.NetNamespace) (*api.NetNamespace, error)
	// UpdateNetNamespace updates a NetNamespace
	UpdateNetNamespace(ctx kapi.Context, nn *api.NetNamespace) (*api.NetNamespace, error)
	// DeleteNetNamespace deletes a NetNamespace
	DeleteNetNamespace(ctx kapi.Context, name string) error
	// WatchNetNamespaces watches NetNamespaces
	WatchNetNamespaces(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
}

// storage puts strong typing around storage calls
type storage struct {
	rest.StandardStorage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s rest.StandardStorage) Registry {
	return &storage{s}
}

func (s *storage) ListNetNamespaces(ctx kapi.Context, label labels.Selector, field fields.Selector) (*api.NetNamespaceList, error) {
	obj, err := s.List(ctx, label, field)
	if err != nil {
		return nil, err
	}
	return obj.(*api.NetNamespaceList), nil
}

func (s *storage) GetNetNamespace(ctx kapi.Context, name string) (*api.NetNamespace, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*api.NetNamespace), nil
}

func (s *storage) CreateNetNamespace(ctx kapi.Context, nn *api.NetNamespace) (*api.NetNamespace, error) {
	obj, err := s.Create(ctx, nn)
	if err != nil {
		return nil, err
	}
	return obj.(*api.NetNamespace), nil
}

func (s *storage) UpdateNetNamespace(ctx kapi.Context, nn *api.NetNamespace) (*api.NetNamespace, error) {
	obj, _, err := s.Update(ctx, nn)
	if err != nil {
		return nil, err
	}
	return obj.(*api.NetNamespace), nil
}

func (s *storage) DeleteNetNamespace(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	if err != nil {
		return err
	}
	return nil
}

func (s *storage) WatchNetNamespaces(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return s.Watch(ctx, label, field, resourceVersion)
}
