package controller

import (
	"github.com/golang/glog"

	kubecontroller "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/controller"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/controller/shared"
)

type ControllerContext struct {
	KubeControllerContext kubecontroller.ControllerContext

	// ClientBuilder will provide a client for this controller to use
	ClientBuilder ControllerClientBuilder

	DeprecatedOpenshiftInformers shared.InformerFactory

	// Stop is the stop channel
	Stop <-chan struct{}
}

// TODO wire this up to something that handles the names.  The logic is available upstream, we just have to wire to it
func (c ControllerContext) IsControllerEnabled(name string) bool {
	return true
}

type ControllerClientBuilder interface {
	controller.ControllerClientBuilder
	KubeInternalClient(name string) (kclientsetinternal.Interface, error)
	KubeInternalClientOrDie(name string) kclientsetinternal.Interface
	DeprecatedOpenshiftClient(name string) (osclient.Interface, error)
	DeprecatedOpenshiftClientOrDie(name string) osclient.Interface
}

// InitFunc is used to launch a particular controller.  It may run additional "should I activate checks".
// Any error returned will cause the controller process to `Fatal`
// The bool indicates whether the controller was enabled.
type InitFunc func(ctx ControllerContext) (bool, error)

type OpenshiftControllerClientBuilder struct {
	controller.ControllerClientBuilder
}

func (b OpenshiftControllerClientBuilder) KubeInternalClient(name string) (kclientsetinternal.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return kclientsetinternal.NewForConfig(clientConfig)
}

func (b OpenshiftControllerClientBuilder) KubeInternalClientOrDie(name string) kclientsetinternal.Interface {
	client, err := b.KubeInternalClient(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}

func (b OpenshiftControllerClientBuilder) DeprecatedOpenshiftClient(name string) (osclient.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return osclient.New(clientConfig)
}

func (b OpenshiftControllerClientBuilder) DeprecatedOpenshiftClientOrDie(name string) osclient.Interface {
	client, err := b.DeprecatedOpenshiftClient(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}
