package fake

import (
	api "github.com/openshift/origin/pkg/route/api"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeRoutes implements RouteResourceInterface
type FakeRoutes struct {
	Fake *FakeRoute
	ns   string
}

var routesResource = schema.GroupVersionResource{Group: "route.openshift.io", Version: "", Resource: "routes"}

func (c *FakeRoutes) Create(route *api.Route) (result *api.Route, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(routesResource, c.ns, route), &api.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Route), err
}

func (c *FakeRoutes) Update(route *api.Route) (result *api.Route, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(routesResource, c.ns, route), &api.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Route), err
}

func (c *FakeRoutes) UpdateStatus(route *api.Route) (*api.Route, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(routesResource, "status", c.ns, route), &api.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Route), err
}

func (c *FakeRoutes) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(routesResource, c.ns, name), &api.Route{})

	return err
}

func (c *FakeRoutes) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(routesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.RouteList{})
	return err
}

func (c *FakeRoutes) Get(name string, options v1.GetOptions) (result *api.Route, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(routesResource, c.ns, name), &api.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Route), err
}

func (c *FakeRoutes) List(opts v1.ListOptions) (result *api.RouteList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(routesResource, c.ns, opts), &api.RouteList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.RouteList{}
	for _, item := range obj.(*api.RouteList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested routes.
func (c *FakeRoutes) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(routesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched route.
func (c *FakeRoutes) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.Route, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(routesResource, c.ns, name, data, subresources...), &api.Route{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Route), err
}
