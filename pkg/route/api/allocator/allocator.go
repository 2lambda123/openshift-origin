package allocator

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	routeapi "github.com/openshift/origin/pkg/route/api"
)

// This should be something we get from config but we would still need a
// default name if nothing's passed. The first pass uses the default name.
// Would be better if we could use "v3.openshift.app", someone bought that!
const defaultDNSSuffix = "v3.openshift.com"

// Generate a host name for a route - using the service name,
// namespace (if provided) and the router shard dns suffix.
func Generate(route *routeapi.Route, shard *routeapi.RouterShard) string {
	name := route.ServiceName
	if len(name) == 0 {
		name = uuid.NewUUID().String()
	}

	if len(route.Namespace) <= 0 {
		return fmt.Sprintf("%s.%s", name, shard.DNSSuffix)
	}

	return fmt.Sprintf("%s-%s.%s", name, route.Namespace, shard.DNSSuffix)
}

// Allocate a router shard for the given route.
// Placeholder for now ... returns the "global" router shard.
func Allocate(route *routeapi.Route) *routeapi.RouterShard {
	return &routeapi.RouterShard{ShardName: "global", DNSSuffix: defaultDNSSuffix}
}
