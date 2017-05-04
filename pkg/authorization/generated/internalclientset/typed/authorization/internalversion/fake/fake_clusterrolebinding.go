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

// FakeClusterRoleBindings implements ClusterRoleBindingInterface
type FakeClusterRoleBindings struct {
	Fake *FakeAuthorization
}

var clusterrolebindingsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "clusterrolebindings"}

func (c *FakeClusterRoleBindings) Create(clusterRoleBinding *api.ClusterRoleBinding) (result *api.ClusterRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterrolebindingsResource, clusterRoleBinding), &api.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) Update(clusterRoleBinding *api.ClusterRoleBinding) (result *api.ClusterRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterrolebindingsResource, clusterRoleBinding), &api.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterrolebindingsResource, name), &api.ClusterRoleBinding{})
	return err
}

func (c *FakeClusterRoleBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterrolebindingsResource, listOptions)

	_, err := c.Fake.Invokes(action, &api.ClusterRoleBindingList{})
	return err
}

func (c *FakeClusterRoleBindings) Get(name string, options v1.GetOptions) (result *api.ClusterRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterrolebindingsResource, name), &api.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) List(opts v1.ListOptions) (result *api.ClusterRoleBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterrolebindingsResource, opts), &api.ClusterRoleBindingList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.ClusterRoleBindingList{}
	for _, item := range obj.(*api.ClusterRoleBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterRoleBindings.
func (c *FakeClusterRoleBindings) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterrolebindingsResource, opts))
}

// Patch applies the patch and returns the patched clusterRoleBinding.
func (c *FakeClusterRoleBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.ClusterRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterrolebindingsResource, name, data, subresources...), &api.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ClusterRoleBinding), err
}
