package internalversion

import (
	"github.com/openshift/origin/pkg/authorization/clientset/internalclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type AuthorizationInterface interface {
	RESTClient() rest.Interface
	PoliciesGetter
}

// AuthorizationClient is used to interact with features provided by the authorization.openshift.io group.
type AuthorizationClient struct {
	restClient rest.Interface
}

func (c *AuthorizationClient) Policies(namespace string) PolicyInterface {
	return newPolicies(c, namespace)
}

// NewForConfig creates a new AuthorizationClient for the given config.
func NewForConfig(c *rest.Config) (*AuthorizationClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &AuthorizationClient{client}, nil
}

// NewForConfigOrDie creates a new AuthorizationClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *AuthorizationClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new AuthorizationClient for the given RESTClient.
func New(c rest.Interface) *AuthorizationClient {
	return &AuthorizationClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("authorization.openshift.io")
	if err != nil {
		return err
	}

	config.APIPath = "/apis"
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	if config.GroupVersion == nil || config.GroupVersion.Group != g.GroupVersion.Group {
		gv := g.GroupVersion
		config.GroupVersion = &gv
	}
	config.NegotiatedSerializer = scheme.Codecs

	if config.QPS == 0 {
		config.QPS = 5
	}
	if config.Burst == 0 {
		config.Burst = 10
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *AuthorizationClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
