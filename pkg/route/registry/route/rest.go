package route

import (
	"fmt"
	"strings"

	"code.google.com/p/go-uuid/uuid"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/route/api/allocator"
	"github.com/openshift/origin/pkg/route/api/validation"
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	registry Registry
}

func NewREST(registry Registry) *REST {
	return &REST{
		registry: registry,
	}
}

func (rs *REST) New() runtime.Object {
	return &api.Route{}
}

func (*REST) NewList() runtime.Object {
	return &api.Route{}
}

// List obtains a list of Routes that match selector.
func (rs *REST) List(ctx kapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	list, err := rs.registry.ListRoutes(ctx, selector)
	if err != nil {
		return nil, err
	}
	return list, err
}

// Get obtains the route specified by its id.
func (rs *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	route, err := rs.registry.GetRoute(ctx, id)
	if err != nil {
		return nil, err
	}
	return route, err
}

// Delete asynchronously deletes the Route specified by its id.
func (rs *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	_, err := rs.registry.GetRoute(ctx, id)
	if err != nil {
		return nil, err
	}
	return &kapi.Status{Status: kapi.StatusSuccess}, rs.registry.DeleteRoute(ctx, id)
}

// Create registers a given new Route instance to rs.registry.
func (rs *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	route, ok := obj.(*api.Route)
	if !ok {
		return nil, fmt.Errorf("not a route: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &route.ObjectMeta) {
		return nil, errors.NewConflict("route", route.Namespace, fmt.Errorf("Route.Namespace does not match the provided context"))
	}

	//  shards will be eventually allocated via a separate controller.
	shard := allocator.Allocate(route)

	if len(route.Host) == 0 {
		route.Host = allocator.Generate(route, shard)
	}

	if errs := validation.ValidateRoute(route); len(errs) > 0 {
		return nil, errors.NewInvalid("route", route.Name, errs)
	}
	if len(route.Name) == 0 {
		route.Name = uuid.NewUUID().String()
	}

	kapi.FillObjectMetaSystemFields(ctx, &route.ObjectMeta)

	escapeNewLines(route.TLS)

	err := rs.registry.CreateRoute(ctx, route)
	if err != nil {
		return nil, err
	}
	return rs.registry.GetRoute(ctx, route.Name)
}

// Update replaces a given Route instance with an existing instance in rs.registry.
func (rs *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	route, ok := obj.(*api.Route)
	if !ok {
		return nil, false, fmt.Errorf("not a route: %#v", obj)
	}
	if len(route.Name) == 0 {
		return nil, false, fmt.Errorf("name is unspecified: %#v", route)
	}
	if !kapi.ValidNamespace(ctx, &route.ObjectMeta) {
		return nil, false, errors.NewConflict("route", route.Namespace, fmt.Errorf("Route.Namespace does not match the provided context"))
	}

	if errs := validation.ValidateRoute(route); len(errs) > 0 {
		return nil, false, errors.NewInvalid("route", route.Name, errs)
	}

	escapeNewLines(route.TLS)

	err := rs.registry.UpdateRoute(ctx, route)
	if err != nil {
		return nil, false, err
	}
	out, err := rs.registry.GetRoute(ctx, route.Name)
	return out, false, err
}

// Watch returns Routes events via a watch.Interface.
// It implements apiserver.ResourceWatcher.
func (rs *REST) Watch(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	return rs.registry.WatchRoutes(ctx, label, field, resourceVersion)
}

// escapeNewLines replaces json escaped new lines with actual line breaks
// certs in json must be single line strings, a new line in json is represented by \\n.  This utility will replace
// a json escaped newline with a real line break which is required for the cert to function properly
func escapeNewLines(tls *api.TLSConfig) {
	if tls != nil {
		if len(tls.Certificate) > 0 {
			tls.Certificate = strings.Replace(tls.Certificate, "\\n", "\n", -1)
		}

		if len(tls.Key) > 0 {
			tls.Key = strings.Replace(tls.Key, "\\n", "\n", -1)
		}

		if len(tls.CACertificate) > 0 {
			tls.CACertificate = strings.Replace(tls.CACertificate, "\\n", "\n", -1)
		}

		if len(tls.DestinationCACertificate) > 0 {
			tls.DestinationCACertificate = strings.Replace(tls.DestinationCACertificate, "\\n", "\n", -1)
		}
	}
}
