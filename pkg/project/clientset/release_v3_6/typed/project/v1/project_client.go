package v1

import (
	v1 "github.com/openshift/origin/pkg/project/api/v1"
	"github.com/openshift/origin/pkg/project/clientset/release_v3_6/scheme"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	rest "k8s.io/client-go/rest"
)

type ProjectV1Interface interface {
	RESTClient() rest.Interface
	ProjectsGetter
}

// ProjectV1Client is used to interact with features provided by the project.openshift.io group.
type ProjectV1Client struct {
	restClient rest.Interface
}

func (c *ProjectV1Client) Projects() ProjectResourceInterface {
	return newProjects(c)
}

// NewForConfig creates a new ProjectV1Client for the given config.
func NewForConfig(c *rest.Config) (*ProjectV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &ProjectV1Client{client}, nil
}

// NewForConfigOrDie creates a new ProjectV1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *ProjectV1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new ProjectV1Client for the given RESTClient.
func New(c rest.Interface) *ProjectV1Client {
	return &ProjectV1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *ProjectV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
