package controllers

import (
	"time"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion/core/internalversion"
)

// DockercfgTokenDeletedControllerOptions contains options for the DockercfgTokenDeletedController
type DockercfgTokenDeletedControllerOptions struct {
	// Resync is the time.Duration at which to fully re-list secrets.
	// If zero, re-list will be delayed as long as possible
	Resync time.Duration
}

// NewDockercfgTokenDeletedController returns a new *DockercfgTokenDeletedController.
func NewDockercfgTokenDeletedController(cl kclientset.Interface, secretInformer kinternalinformers.SecretInformer, options DockercfgTokenDeletedControllerOptions) *DockercfgTokenDeletedController {
	e := &DockercfgTokenDeletedController{
		client:           cl,
		secretController: secretInformer.Informer().GetController(),
	}
	secretInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			DeleteFunc: e.secretDeleted,
		},
		options.Resync,
	)
	return e
}

// The DockercfgTokenDeletedController watches for service account tokens to be deleted.
// On delete, it removes the associated dockercfg secret if it exists.
type DockercfgTokenDeletedController struct {
	stopChan chan struct{}

	client kclientset.Interface

	secretController cache.Controller
}

// Runs controller loops and returns immediately
func (e *DockercfgTokenDeletedController) Run() {
	if e.stopChan == nil {
		e.stopChan = make(chan struct{})
		go e.secretController.Run(e.stopChan)
	}
}

// Stop gracefully shuts down this controller
func (e *DockercfgTokenDeletedController) Stop() {
	if e.stopChan != nil {
		close(e.stopChan)
		e.stopChan = nil
	}
}

// secretDeleted reacts to a token secret being deleted by looking for a corresponding dockercfg secret and deleting it if it exists
func (e *DockercfgTokenDeletedController) secretDeleted(obj interface{}) {
	tokenSecret, ok := obj.(*api.Secret)
	if !ok {
		return
	}
	if tokenSecret.Type != api.SecretTypeServiceAccountToken {
		return
	}

	dockercfgSecrets, err := e.findDockercfgSecrets(tokenSecret)
	if err != nil {
		glog.Error(err)
		return
	}
	if len(dockercfgSecrets) == 0 {
		return
	}

	// remove the reference token secrets
	for _, dockercfgSecret := range dockercfgSecrets {
		if err := e.client.Core().Secrets(dockercfgSecret.Namespace).Delete(dockercfgSecret.Name, nil); (err != nil) && !apierrors.IsNotFound(err) {
			utilruntime.HandleError(err)
		}
	}
}

// findDockercfgSecret checks all the secrets in the namespace to see if the token secret has any existing dockercfg secrets that reference it
func (e *DockercfgTokenDeletedController) findDockercfgSecrets(tokenSecret *api.Secret) ([]*api.Secret, error) {
	dockercfgSecrets := []*api.Secret{}

	options := metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector(api.SecretTypeField, string(api.SecretTypeDockercfg)).String()}
	potentialSecrets, err := e.client.Core().Secrets(tokenSecret.Namespace).List(options)
	if err != nil {
		return nil, err
	}

	for i, currSecret := range potentialSecrets.Items {
		if currSecret.Annotations[ServiceAccountTokenSecretNameKey] == tokenSecret.Name {
			dockercfgSecrets = append(dockercfgSecrets, &potentialSecrets.Items[i])
		}
	}

	return dockercfgSecrets, nil
}
