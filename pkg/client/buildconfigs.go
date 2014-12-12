package client

import (
	"errors"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// BuildConfigsNamespacer has methods to work with BuildConfig resources in a namespace
type BuildConfigsNamespacer interface {
	BuildConfigs(namespace string) BuildConfigInterface
}

// BuildConfigInterface exposes methods on BuildConfig resources
type BuildConfigInterface interface {
	List(label, field labels.Selector) (*buildapi.BuildConfigList, error)
	Get(name string) (*buildapi.BuildConfig, error)
	Create(config *buildapi.BuildConfig) (*buildapi.BuildConfig, error)
	Update(config *buildapi.BuildConfig) (*buildapi.BuildConfig, error)
	Delete(name string) error
	Watch(label, field labels.Selector, resourceVersion string) (watch.Interface, error)
}

// buildConfigs implements BuildConfigsNamespacer interface
type buildConfigs struct {
	r  *Client
	ns string
}

// newBuildConfigs returns a buildConfigs
func newBuildConfigs(c *Client, namespace string) *buildConfigs {
	return &buildConfigs{
		r:  c,
		ns: namespace,
	}
}

// List returns a list of buildconfigs that match the label and field selectors.
func (c *buildConfigs) List(label, field labels.Selector) (result *buildapi.BuildConfigList, err error) {
	result = &buildapi.BuildConfigList{}
	err = c.r.Get().
		Namespace(c.ns).
		Path("buildConfigs").
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Do().
		Into(result)
	return
}

// Get returns information about a particular buildconfig and error if one occurs.
func (c *buildConfigs) Get(name string) (result *buildapi.BuildConfig, err error) {
	if len(name) == 0 {
		return nil, errors.New("name is required parameter to Get")
	}

	result = &buildapi.BuildConfig{}
	err = c.r.Get().Namespace(c.ns).Path("buildConfigs").Path(name).Do().Into(result)
	return
}

// Create creates a new buildconfig. Returns the server's representation of the buildconfig and error if one occurs.
func (c *buildConfigs) Create(build *buildapi.BuildConfig) (result *buildapi.BuildConfig, err error) {
	result = &buildapi.BuildConfig{}
	err = c.r.Post().Namespace(c.ns).Path("buildConfigs").Body(build).Do().Into(result)
	return
}

// Update updates the buildconfig on server. Returns the server's representation of the buildconfig and error if one occurs.
func (c *buildConfigs) Update(build *buildapi.BuildConfig) (result *buildapi.BuildConfig, err error) {
	result = &buildapi.BuildConfig{}
	err = c.r.Put().Namespace(c.ns).Path("buildConfigs").Path(build.Name).Body(build).Do().Into(result)
	return
}

// Delete deletes a BuildConfig, returns error if one occurs.
func (c *buildConfigs) Delete(name string) error {
	return c.r.Delete().Namespace(c.ns).Path("buildConfigs").Path(name).Do().Error()
}

// Watch returns a watch.Interface that watches the requested buildConfigs.
func (c *buildConfigs) Watch(label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	return c.r.Get().
		Namespace(c.ns).
		Path("watch").
		Path("buildConfigs").
		Param("resourceVersion", resourceVersion).
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Watch()
}
