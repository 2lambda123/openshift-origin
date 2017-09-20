package v1

import (
	v1 "github.com/openshift/origin/pkg/user/apis/user/v1"
	scheme "github.com/openshift/origin/pkg/user/generated/clientset/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// IdentitiesGetter has a method to return a IdentityInterface.
// A group's client should implement this interface.
type IdentitiesGetter interface {
	Identities() IdentityInterface
}

// IdentityInterface has methods to work with Identity resources.
type IdentityInterface interface {
	Create(*v1.Identity) (*v1.Identity, error)
	Update(*v1.Identity) (*v1.Identity, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.Identity, error)
	List(opts meta_v1.ListOptions) (*v1.IdentityList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Identity, err error)
	IdentityExpansion
}

// identities implements IdentityInterface
type identities struct {
	client rest.Interface
}

// newIdentities returns a Identities
func newIdentities(c *UserV1Client) *identities {
	return &identities{
		client: c.RESTClient(),
	}
}

// Get takes name of the identity, and returns the corresponding identity object, and an error if there is any.
func (c *identities) Get(name string, options meta_v1.GetOptions) (result *v1.Identity, err error) {
	result = &v1.Identity{}
	err = c.client.Get().
		Resource("identities").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Identities that match those selectors.
func (c *identities) List(opts meta_v1.ListOptions) (result *v1.IdentityList, err error) {
	result = &v1.IdentityList{}
	err = c.client.Get().
		Resource("identities").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested identities.
func (c *identities) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("identities").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a identity and creates it.  Returns the server's representation of the identity, and an error, if there is any.
func (c *identities) Create(identity *v1.Identity) (result *v1.Identity, err error) {
	result = &v1.Identity{}
	err = c.client.Post().
		Resource("identities").
		Body(identity).
		Do().
		Into(result)
	return
}

// Update takes the representation of a identity and updates it. Returns the server's representation of the identity, and an error, if there is any.
func (c *identities) Update(identity *v1.Identity) (result *v1.Identity, err error) {
	result = &v1.Identity{}
	err = c.client.Put().
		Resource("identities").
		Name(identity.Name).
		Body(identity).
		Do().
		Into(result)
	return
}

// Delete takes name of the identity and deletes it. Returns an error if one occurs.
func (c *identities) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("identities").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *identities) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Resource("identities").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched identity.
func (c *identities) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Identity, err error) {
	result = &v1.Identity{}
	err = c.client.Patch(pt).
		Resource("identities").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
