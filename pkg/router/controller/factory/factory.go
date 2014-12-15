package factory

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
	"github.com/openshift/origin/pkg/router/controller"
)

type RouterControllerFactory struct {
	KClient  kclient.Interface
	OSClient osclient.Interface
}

func (factory *RouterControllerFactory) Create(plugin router.Plugin) *controller.RouterController {
	routeEventQueue := oscache.NewEventQueue()
	cache.NewReflector(&routeLW{factory.OSClient}, &routeapi.Route{}, routeEventQueue).Run()

	endpointsEventQueue := oscache.NewEventQueue()
	cache.NewReflector(&endpointsLW{factory.KClient}, &kapi.Endpoints{}, endpointsEventQueue).Run()

	return &controller.RouterController{
		Plugin: plugin,
		NextEndpoints: func() (watch.EventType, *kapi.Endpoints) {
			eventType, obj := endpointsEventQueue.Pop()
			return eventType, obj.(*kapi.Endpoints)
		},
		NextRoute: func() (watch.EventType, *routeapi.Route) {
			eventType, obj := routeEventQueue.Pop()
			return eventType, obj.(*routeapi.Route)
		},
	}
}

type routeLW struct {
	client osclient.Interface
}

func (lw *routeLW) List() (runtime.Object, error) {
	return lw.client.Routes(kapi.NamespaceAll).List(labels.Everything(), labels.Everything())
}

func (lw *routeLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.Routes(kapi.NamespaceAll).Watch(labels.Everything(), labels.Everything(), resourceVersion)
}

type endpointsLW struct {
	client kclient.Interface
}

func (lw *endpointsLW) List() (runtime.Object, error) {
	return lw.client.Endpoints(kapi.NamespaceAll).List(labels.Everything())
}

func (lw *endpointsLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.Endpoints(kapi.NamespaceAll).Watch(labels.Everything(), labels.Everything(), resourceVersion)
}
