package fake

import (
	v1 "github.com/openshift/origin/pkg/image/clientset/release_v3_6/typed/image/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeImageV1 struct {
	*testing.Fake
}

func (c *FakeImageV1) Images() v1.ImageResourceInterface {
	return &FakeImages{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeImageV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
