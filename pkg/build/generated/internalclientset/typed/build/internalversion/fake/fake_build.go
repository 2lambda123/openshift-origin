package fake

import (
	api "github.com/openshift/origin/pkg/build/api"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeBuilds implements BuildResourceInterface
type FakeBuilds struct {
	Fake *FakeBuild
	ns   string
}

var buildsResource = schema.GroupVersionResource{Group: "build.openshift.io", Version: "", Resource: "builds"}

func (c *FakeBuilds) Create(build *api.Build) (result *api.Build, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(buildsResource, c.ns, build), &api.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Build), err
}

func (c *FakeBuilds) Update(build *api.Build) (result *api.Build, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(buildsResource, c.ns, build), &api.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Build), err
}

func (c *FakeBuilds) UpdateStatus(build *api.Build) (*api.Build, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(buildsResource, "status", c.ns, build), &api.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Build), err
}

func (c *FakeBuilds) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(buildsResource, c.ns, name), &api.Build{})

	return err
}

func (c *FakeBuilds) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(buildsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.BuildList{})
	return err
}

func (c *FakeBuilds) Get(name string, options v1.GetOptions) (result *api.Build, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(buildsResource, c.ns, name), &api.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Build), err
}

func (c *FakeBuilds) List(opts v1.ListOptions) (result *api.BuildList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(buildsResource, c.ns, opts), &api.BuildList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.BuildList{}
	for _, item := range obj.(*api.BuildList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested builds.
func (c *FakeBuilds) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(buildsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched build.
func (c *FakeBuilds) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.Build, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(buildsResource, c.ns, name, data, subresources...), &api.Build{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Build), err
}
