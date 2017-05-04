package internalversion

import (
	api "github.com/openshift/origin/pkg/oauth/api"
	scheme "github.com/openshift/origin/pkg/oauth/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// OAuthClientsGetter has a method to return a OAuthClientInterface.
// A group's client should implement this interface.
type OAuthClientsGetter interface {
	OAuthClients(namespace string) OAuthClientInterface
}

// OAuthClientInterface has methods to work with OAuthClient resources.
type OAuthClientInterface interface {
	Create(*api.OAuthClient) (*api.OAuthClient, error)
	Update(*api.OAuthClient) (*api.OAuthClient, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*api.OAuthClient, error)
	List(opts v1.ListOptions) (*api.OAuthClientList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.OAuthClient, err error)
	OAuthClientExpansion
}

// oAuthClients implements OAuthClientInterface
type oAuthClients struct {
	client rest.Interface
	ns     string
}

// newOAuthClients returns a OAuthClients
func newOAuthClients(c *OauthClient, namespace string) *oAuthClients {
	return &oAuthClients{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a oAuthClient and creates it.  Returns the server's representation of the oAuthClient, and an error, if there is any.
func (c *oAuthClients) Create(oAuthClient *api.OAuthClient) (result *api.OAuthClient, err error) {
	result = &api.OAuthClient{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("oauthclients").
		Body(oAuthClient).
		Do().
		Into(result)
	return
}

// Update takes the representation of a oAuthClient and updates it. Returns the server's representation of the oAuthClient, and an error, if there is any.
func (c *oAuthClients) Update(oAuthClient *api.OAuthClient) (result *api.OAuthClient, err error) {
	result = &api.OAuthClient{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("oauthclients").
		Name(oAuthClient.Name).
		Body(oAuthClient).
		Do().
		Into(result)
	return
}

// Delete takes name of the oAuthClient and deletes it. Returns an error if one occurs.
func (c *oAuthClients) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("oauthclients").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *oAuthClients) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("oauthclients").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the oAuthClient, and returns the corresponding oAuthClient object, and an error if there is any.
func (c *oAuthClients) Get(name string, options v1.GetOptions) (result *api.OAuthClient, err error) {
	result = &api.OAuthClient{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("oauthclients").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of OAuthClients that match those selectors.
func (c *oAuthClients) List(opts v1.ListOptions) (result *api.OAuthClientList, err error) {
	result = &api.OAuthClientList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("oauthclients").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested oAuthClients.
func (c *oAuthClients) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("oauthclients").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Patch applies the patch and returns the patched oAuthClient.
func (c *oAuthClients) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.OAuthClient, err error) {
	result = &api.OAuthClient{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("oauthclients").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
