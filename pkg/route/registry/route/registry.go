package route

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/route/api"
)

// Registry is an interface for things that know how to store Routes.
type Registry interface {
	// ListRoutes obtains list of routes that match a selector.
	ListRoutes(ctx kapi.Context, labelSelector labels.Selector, fieldSelector labels.Selector) (*api.RouteList, error)
	// GetRoute retrieves a specific route.
	GetRoute(ctx kapi.Context, routeID string) (*api.Route, error)
	// CreateRoute creates a new route.
	CreateRoute(ctx kapi.Context, route *api.Route) error
	// UpdateRoute updates a route.
	UpdateRoute(ctx kapi.Context, route *api.Route) error
	// DeleteRoute deletes a route.
	DeleteRoute(ctx kapi.Context, routeID string) error
	// WatchRoutes watches for new/modified/deleted routes.
	WatchRoutes(ctx kapi.Context, labels, fields labels.Selector, resourceVersion string) (watch.Interface, error)
}
