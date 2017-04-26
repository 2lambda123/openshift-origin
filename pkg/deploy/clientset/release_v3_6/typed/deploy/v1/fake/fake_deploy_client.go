package fake

import (
	v1 "github.com/openshift/origin/pkg/deploy/clientset/release_v3_6/typed/deploy/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeDeployV1 struct {
	*testing.Fake
}

func (c *FakeDeployV1) DeploymentConfigs(namespace string) v1.DeploymentConfigInterface {
	return &FakeDeploymentConfigs{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeDeployV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
