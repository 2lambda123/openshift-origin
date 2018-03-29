package v1

import (
	v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/apis/webconsole/v1"
	scheme "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/generated/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// OpenShiftWebConsoleConfigsGetter has a method to return a OpenShiftWebConsoleConfigInterface.
// A group's client should implement this interface.
type OpenShiftWebConsoleConfigsGetter interface {
	OpenShiftWebConsoleConfigs() OpenShiftWebConsoleConfigInterface
}

// OpenShiftWebConsoleConfigInterface has methods to work with OpenShiftWebConsoleConfig resources.
type OpenShiftWebConsoleConfigInterface interface {
	Create(*v1.OpenShiftWebConsoleConfig) (*v1.OpenShiftWebConsoleConfig, error)
	Update(*v1.OpenShiftWebConsoleConfig) (*v1.OpenShiftWebConsoleConfig, error)
	UpdateStatus(*v1.OpenShiftWebConsoleConfig) (*v1.OpenShiftWebConsoleConfig, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.OpenShiftWebConsoleConfig, error)
	List(opts meta_v1.ListOptions) (*v1.OpenShiftWebConsoleConfigList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.OpenShiftWebConsoleConfig, err error)
	OpenShiftWebConsoleConfigExpansion
}

// openShiftWebConsoleConfigs implements OpenShiftWebConsoleConfigInterface
type openShiftWebConsoleConfigs struct {
	client rest.Interface
}

// newOpenShiftWebConsoleConfigs returns a OpenShiftWebConsoleConfigs
func newOpenShiftWebConsoleConfigs(c *WebconsoleV1Client) *openShiftWebConsoleConfigs {
	return &openShiftWebConsoleConfigs{
		client: c.RESTClient(),
	}
}

// Get takes name of the openShiftWebConsoleConfig, and returns the corresponding openShiftWebConsoleConfig object, and an error if there is any.
func (c *openShiftWebConsoleConfigs) Get(name string, options meta_v1.GetOptions) (result *v1.OpenShiftWebConsoleConfig, err error) {
	result = &v1.OpenShiftWebConsoleConfig{}
	err = c.client.Get().
		Resource("openshiftwebconsoleconfigs").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of OpenShiftWebConsoleConfigs that match those selectors.
func (c *openShiftWebConsoleConfigs) List(opts meta_v1.ListOptions) (result *v1.OpenShiftWebConsoleConfigList, err error) {
	result = &v1.OpenShiftWebConsoleConfigList{}
	err = c.client.Get().
		Resource("openshiftwebconsoleconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested openShiftWebConsoleConfigs.
func (c *openShiftWebConsoleConfigs) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("openshiftwebconsoleconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a openShiftWebConsoleConfig and creates it.  Returns the server's representation of the openShiftWebConsoleConfig, and an error, if there is any.
func (c *openShiftWebConsoleConfigs) Create(openShiftWebConsoleConfig *v1.OpenShiftWebConsoleConfig) (result *v1.OpenShiftWebConsoleConfig, err error) {
	result = &v1.OpenShiftWebConsoleConfig{}
	err = c.client.Post().
		Resource("openshiftwebconsoleconfigs").
		Body(openShiftWebConsoleConfig).
		Do().
		Into(result)
	return
}

// Update takes the representation of a openShiftWebConsoleConfig and updates it. Returns the server's representation of the openShiftWebConsoleConfig, and an error, if there is any.
func (c *openShiftWebConsoleConfigs) Update(openShiftWebConsoleConfig *v1.OpenShiftWebConsoleConfig) (result *v1.OpenShiftWebConsoleConfig, err error) {
	result = &v1.OpenShiftWebConsoleConfig{}
	err = c.client.Put().
		Resource("openshiftwebconsoleconfigs").
		Name(openShiftWebConsoleConfig.Name).
		Body(openShiftWebConsoleConfig).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *openShiftWebConsoleConfigs) UpdateStatus(openShiftWebConsoleConfig *v1.OpenShiftWebConsoleConfig) (result *v1.OpenShiftWebConsoleConfig, err error) {
	result = &v1.OpenShiftWebConsoleConfig{}
	err = c.client.Put().
		Resource("openshiftwebconsoleconfigs").
		Name(openShiftWebConsoleConfig.Name).
		SubResource("status").
		Body(openShiftWebConsoleConfig).
		Do().
		Into(result)
	return
}

// Delete takes name of the openShiftWebConsoleConfig and deletes it. Returns an error if one occurs.
func (c *openShiftWebConsoleConfigs) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("openshiftwebconsoleconfigs").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *openShiftWebConsoleConfigs) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Resource("openshiftwebconsoleconfigs").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched openShiftWebConsoleConfig.
func (c *openShiftWebConsoleConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.OpenShiftWebConsoleConfig, err error) {
	result = &v1.OpenShiftWebConsoleConfig{}
	err = c.client.Patch(pt).
		Resource("openshiftwebconsoleconfigs").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
