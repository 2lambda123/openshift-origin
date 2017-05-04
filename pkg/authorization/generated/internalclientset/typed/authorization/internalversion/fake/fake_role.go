package fake

import (
	api "github.com/openshift/origin/pkg/authorization/api"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeRoles implements RoleInterface
type FakeRoles struct {
	Fake *FakeAuthorization
	ns   string
}

var rolesResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "roles"}

func (c *FakeRoles) Create(role *api.Role) (result *api.Role, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(rolesResource, c.ns, role), &api.Role{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Role), err
}

func (c *FakeRoles) Update(role *api.Role) (result *api.Role, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(rolesResource, c.ns, role), &api.Role{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Role), err
}

func (c *FakeRoles) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(rolesResource, c.ns, name), &api.Role{})

	return err
}

func (c *FakeRoles) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(rolesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.RoleList{})
	return err
}

func (c *FakeRoles) Get(name string, options v1.GetOptions) (result *api.Role, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(rolesResource, c.ns, name), &api.Role{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Role), err
}

func (c *FakeRoles) List(opts v1.ListOptions) (result *api.RoleList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(rolesResource, c.ns, opts), &api.RoleList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.RoleList{}
	for _, item := range obj.(*api.RoleList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested roles.
func (c *FakeRoles) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(rolesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched role.
func (c *FakeRoles) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.Role, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(rolesResource, c.ns, name, data, subresources...), &api.Role{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Role), err
}
