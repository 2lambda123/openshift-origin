package servicebroker

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	restclient "k8s.io/client-go/rest"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/openservicebroker/api"
	extrouteclientset "github.com/openshift/origin/pkg/route/generated/clientset/typed/route/v1"
	"github.com/openshift/origin/pkg/serviceaccounts"
	templateinformer "github.com/openshift/origin/pkg/template/generated/informers/internalversion/template/internalversion"
	templateclientset "github.com/openshift/origin/pkg/template/generated/internalclientset"
	internalversiontemplate "github.com/openshift/origin/pkg/template/generated/internalclientset/typed/template/internalversion"
	templatelister "github.com/openshift/origin/pkg/template/generated/listers/template/internalversion"
)

// Broker represents the template service broker.  It implements
// openservicebroker/api.Broker.
type Broker struct {
	kc                 kclientset.Interface
	templateclient     internalversiontemplate.TemplateInterface
	extkc              kclientsetexternal.Interface
	extrouteclient     extrouteclientset.RouteV1Interface
	lister             templatelister.TemplateLister
	templateNamespaces map[string]struct{}
	ready              chan struct{}
}

var _ api.Broker = &Broker{}

// NewBroker returns a new instance of the template service broker.  While
// built into origin, its initialisation is asynchronous.  This is because it is
// part of the API server, but requires the API server to be up to get its
// service account token.
func NewBroker(privrestconfig restclient.Config, privkc kclientset.Interface, infraNamespace string, informer templateinformer.TemplateInformer, namespaces []string) *Broker {
	templateNamespaces := map[string]struct{}{}
	for _, namespace := range namespaces {
		templateNamespaces[namespace] = struct{}{}
	}

	b := &Broker{
		lister:             informer.Lister(),
		templateNamespaces: templateNamespaces,
		ready:              make(chan struct{}),
	}

	go func() {
		// the broker is initialised asynchronously because fetching the service
		// account token requires the main API server to be running.

		glog.Infof("Template service broker: waiting for authentication token")

		restconfig, _, kc, extkc, err := serviceaccounts.Clients(
			privrestconfig,
			&serviceaccounts.ClientLookupTokenRetriever{Client: privkc},
			infraNamespace,
			bootstrappolicy.InfraTemplateServiceBrokerServiceAccountName,
		)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Template service broker: failed to initialize clients: %v", err))
			return
		}

		extrouteclientset, err := extrouteclientset.NewForConfig(restconfig)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Template service broker: failed to initialize route clientset: %v", err))
			return
		}

		templateclientset, err := templateclientset.NewForConfig(restconfig)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Template service broker: failed to initialize template clientset: %v", err))
			return
		}

		b.kc = kc
		b.extkc = extkc
		b.extrouteclient = extrouteclientset
		b.templateclient = templateclientset.Template()

		glog.Infof("Template service broker: waiting for informer sync")

		for !informer.Informer().HasSynced() {
			time.Sleep(100 * time.Millisecond)
		}

		glog.Infof("Template service broker: ready; reading namespaces %v", namespaces)

		close(b.ready)
	}()

	return b
}

// WaitForReady is called on each incoming API request via a server filter.  It
// is intended to be a quick check that the broker is initialized (which should
// itself be a fast one-off start-up event).
func (b *Broker) WaitForReady() error {
	// delay up to 10 seconds if not ready (unlikely), before returning a
	// "try again" response.
	timer := time.NewTimer(5 * time.Minute)
	defer timer.Stop()

	select {
	case <-b.ready:
		return nil
	case <-timer.C:
		return errors.New("timeout waiting for broker to be ready")
	}
}
