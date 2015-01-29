package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// PolicyBindingsNamespacer has methods to work with PolicyBinding resources in a namespace
type PolicyBindingsNamespacer interface {
	PolicyBindings(namespace string) PolicyBindingInterface
}

// PolicyBindingInterface exposes methods on PolicyBinding resources.
type PolicyBindingInterface interface {
	List(label, field labels.Selector) (*authorizationapi.PolicyBindingList, error)
	Get(name string) (*authorizationapi.PolicyBinding, error)
	Create(policyBinding *authorizationapi.PolicyBinding) (*authorizationapi.PolicyBinding, error)
	Delete(name string) error
}

// policyBindings implements PolicyBindingsNamespacer interface
type policyBindings struct {
	r  *Client
	ns string
}

// newPolicyBindings returns a policyBindings
func newPolicyBindings(c *Client, namespace string) *policyBindings {
	return &policyBindings{
		r:  c,
		ns: namespace,
	}
}

// List returns a list of policyBindings that match the label and field selectors.
func (c *policyBindings) List(label, field labels.Selector) (result *authorizationapi.PolicyBindingList, err error) {
	result = &authorizationapi.PolicyBindingList{}
	err = c.r.Get().Namespace(c.ns).Resource("policyBindings").SelectorParam("labels", label).SelectorParam("fields", field).Do().Into(result)
	return
}

// Get returns information about a particular policyBinding and error if one occurs.
func (c *policyBindings) Get(name string) (result *authorizationapi.PolicyBinding, err error) {
	result = &authorizationapi.PolicyBinding{}
	err = c.r.Get().Namespace(c.ns).Resource("policyBindings").Name(name).Do().Into(result)
	return
}

// Create creates new policyBinding. Returns the server's representation of the policyBinding and error if one occurs.
func (c *policyBindings) Create(policyBinding *authorizationapi.PolicyBinding) (result *authorizationapi.PolicyBinding, err error) {
	result = &authorizationapi.PolicyBinding{}
	err = c.r.Post().Namespace(c.ns).Resource("policyBindings").Body(policyBinding).Do().Into(result)
	return
}

// Delete deletes a policyBinding, returns error if one occurs.
func (c *policyBindings) Delete(name string) (err error) {
	err = c.r.Delete().Namespace(c.ns).Resource("policyBindings").Name(name).Do().Error()
	return
}
