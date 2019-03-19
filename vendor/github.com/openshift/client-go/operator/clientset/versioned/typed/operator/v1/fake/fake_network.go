// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	operator_v1 "github.com/openshift/api/operator/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeNetworks implements NetworkInterface
type FakeNetworks struct {
	Fake *FakeOperatorV1
}

var networksResource = schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1", Resource: "networks"}

var networksKind = schema.GroupVersionKind{Group: "operator.openshift.io", Version: "v1", Kind: "Network"}

// Get takes name of the network, and returns the corresponding network object, and an error if there is any.
func (c *FakeNetworks) Get(name string, options v1.GetOptions) (result *operator_v1.Network, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(networksResource, name), &operator_v1.Network{})
	if obj == nil {
		return nil, err
	}
	return obj.(*operator_v1.Network), err
}

// List takes label and field selectors, and returns the list of Networks that match those selectors.
func (c *FakeNetworks) List(opts v1.ListOptions) (result *operator_v1.NetworkList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(networksResource, networksKind, opts), &operator_v1.NetworkList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &operator_v1.NetworkList{ListMeta: obj.(*operator_v1.NetworkList).ListMeta}
	for _, item := range obj.(*operator_v1.NetworkList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested networks.
func (c *FakeNetworks) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(networksResource, opts))
}

// Create takes the representation of a network and creates it.  Returns the server's representation of the network, and an error, if there is any.
func (c *FakeNetworks) Create(network *operator_v1.Network) (result *operator_v1.Network, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(networksResource, network), &operator_v1.Network{})
	if obj == nil {
		return nil, err
	}
	return obj.(*operator_v1.Network), err
}

// Update takes the representation of a network and updates it. Returns the server's representation of the network, and an error, if there is any.
func (c *FakeNetworks) Update(network *operator_v1.Network) (result *operator_v1.Network, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(networksResource, network), &operator_v1.Network{})
	if obj == nil {
		return nil, err
	}
	return obj.(*operator_v1.Network), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeNetworks) UpdateStatus(network *operator_v1.Network) (*operator_v1.Network, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(networksResource, "status", network), &operator_v1.Network{})
	if obj == nil {
		return nil, err
	}
	return obj.(*operator_v1.Network), err
}

// Delete takes name of the network and deletes it. Returns an error if one occurs.
func (c *FakeNetworks) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(networksResource, name), &operator_v1.Network{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeNetworks) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(networksResource, listOptions)

	_, err := c.Fake.Invokes(action, &operator_v1.NetworkList{})
	return err
}

// Patch applies the patch and returns the patched network.
func (c *FakeNetworks) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *operator_v1.Network, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(networksResource, name, data, subresources...), &operator_v1.Network{})
	if obj == nil {
		return nil, err
	}
	return obj.(*operator_v1.Network), err
}
