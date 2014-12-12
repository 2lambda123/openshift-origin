package client

import (
	"errors"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImagesNamespacer has methods to work with Image resources in a namespace
type ImagesNamespacer interface {
	Images(namespace string) ImageInterface
}

// ImageInterface exposes methods on Image resources.
type ImageInterface interface {
	List(label, field labels.Selector) (*imageapi.ImageList, error)
	Get(name string) (*imageapi.Image, error)
	Create(image *imageapi.Image) (*imageapi.Image, error)
}

// images implements ImagesNamespacer interface
type images struct {
	r  *Client
	ns string
}

// newImages returns an images
func newImages(c *Client, namespace string) ImageInterface {
	return &images{
		r:  c,
		ns: namespace,
	}
}

// List returns a list of images that match the label and field selectors.
func (c *images) List(label, field labels.Selector) (result *imageapi.ImageList, err error) {
	result = &imageapi.ImageList{}
	err = c.r.Get().
		Namespace(c.ns).
		Path("images").
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Do().
		Into(result)
	return
}

// Get returns information about a particular image and error if one occurs.
func (c *images) Get(name string) (result *imageapi.Image, err error) {
	if len(name) == 0 {
		return nil, errors.New("name is required parameter to Get")
	}

	result = &imageapi.Image{}
	err = c.r.Get().Namespace(c.ns).Path("images").Path(name).Do().Into(result)
	return
}

// Create creates a new image. Returns the server's representation of the image and error if one occurs.
func (c *images) Create(image *imageapi.Image) (result *imageapi.Image, err error) {
	result = &imageapi.Image{}
	err = c.r.Post().Namespace(c.ns).Path("images").Body(image).Do().Into(result)
	return
}
