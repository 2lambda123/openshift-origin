package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/apis/webconsole/v1helpers"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
	apiregistrationclientv1beta1 "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1beta1"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/golang/glog"
	webconsoleconfigv1 "github.com/openshift/api/webconsole/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceapply"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourcemerge"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceread"
	webconsolev1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/apis/webconsole/v1"
	webconsoleclientv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/generated/clientset/versioned/typed/webconsole/v1"
)

type WebConsoleOperator struct {
	operatorConfigClient webconsoleclientv1.OpenShiftWebConsoleConfigsGetter

	appsv1Client      appsclientv1.AppsV1Interface
	corev1Client      coreclientv1.CoreV1Interface
	apiServicesClient apiregistrationclientv1beta1.APIServicesGetter

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewWebConsoleOperator(
	operatorConfigClient webconsoleclientv1.OpenShiftWebConsoleConfigsGetter,
	appsv1Client appsclientv1.AppsV1Interface,
	corev1Client coreclientv1.CoreV1Interface,
) *WebConsoleOperator {
	c := &WebConsoleOperator{
		operatorConfigClient: operatorConfigClient,
		appsv1Client:         appsv1Client,
		corev1Client:         corev1Client,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "WebConsoleOperator"),
	}

	return c
}

func (c WebConsoleOperator) sync() error {
	operatorConfig, err := c.operatorConfigClient.OpenShiftWebConsoleConfigs().Get("instance", metav1.GetOptions{})
	if err != nil {
		return err
	}
	switch operatorConfig.Spec.ManagementState {
	case webconsolev1.Unmanaged:
		return nil

	case webconsolev1.Disabled:
		// TODO probably need to watch until the NS is really gone
		if err := c.corev1Client.Namespaces().Delete("openshift-web-console", nil); err != nil && !apierrors.IsNotFound(err) {
			utilruntime.HandleError(err)
			if _, updateErr := c.operatorConfigClient.OpenShiftWebConsoleConfigs().Update(operatorConfig); updateErr != nil {
				utilruntime.HandleError(updateErr)
			}
			return err
		}
		operatorConfig.Status.Version = ""
		operatorConfig.Status.VersionAvailability = []webconsolev1.WebConsoleVersionAvailablity{}
		if _, err := c.operatorConfigClient.OpenShiftWebConsoleConfigs().Update(operatorConfig); err != nil {
			return err
		}
		return nil
	}

	// TODO use semver
	isFirst := len(operatorConfig.Status.Version) == 0
	isSame := operatorConfig.Spec.Version == operatorConfig.Status.Version
	is10_0 := is10_0Version(operatorConfig.Status.Version)
	wants10_0 := is10_0Version(operatorConfig.Spec.Version)
	wants10_1 := operatorConfig.Spec.Version == "3.10.1"

	errors := []error{}
	switch {
	case wants10_0 && (isSame || isFirst):
		operatorConfig, errors = c.sync10_0(operatorConfig)
		v1helpers.FilterAvailability(&operatorConfig.Status.VersionAvailability, "3.10.0")
		operatorConfig.Status.Task = "sync-3.10.0"

	case wants10_1 && (isSame || isFirst):
		operatorConfig, errors = c.sync10_1(operatorConfig)
		v1helpers.FilterAvailability(&operatorConfig.Status.VersionAvailability, "3.10.1")
		operatorConfig.Status.Task = "sync-3.10.1"

	case wants10_1 && is10_0:
		operatorConfig, errors = c.migrate10_0_to_10_1(operatorConfig)
		operatorConfig.Status.Task = "migration-3.10.0-to-3.10.1"

	default:
		operatorConfig.Status.Task = "unrecognized"
		if _, err := c.operatorConfigClient.OpenShiftWebConsoleConfigs().Update(operatorConfig); err != nil {
			utilruntime.HandleError(err)
		}

		return fmt.Errorf("unrecognized state")
	}

	v1helpers.FilterAvailability(&operatorConfig.Status.VersionAvailability, "3.10.0", "3.10.1")
	if _, err := c.operatorConfigClient.OpenShiftWebConsoleConfigs().Update(operatorConfig); err != nil {
		errors = append(errors, err)
	}

	// given the VersionAvailability and the status.Version, we can compute availability
	availableCondition := webconsolev1.OpenShiftOperatorCondition{
		Type:   webconsolev1.OperatorStatusTypeAvailable,
		Status: webconsolev1.ConditionUnknown,
	}
	for _, versionAvailability := range operatorConfig.Status.VersionAvailability {
		if versionAvailability.Version == operatorConfig.Status.Version {
			if versionAvailability.AvailableReplicas > 0 {
				availableCondition.Status = webconsolev1.ConditionTrue
			} else {
				availableCondition.Status = webconsolev1.ConditionFalse
			}
			break
		}
	}
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, availableCondition)

	failingCondition := webconsolev1.OpenShiftOperatorCondition{
		Type:   webconsolev1.OperatorStatusTypeFailing,
		Status: webconsolev1.ConditionFalse,
	}
	for _, versionAvailability := range operatorConfig.Status.VersionAvailability {
		if len(versionAvailability.Errors) > 0 {
			availableCondition.Status = webconsolev1.ConditionTrue
			if len(availableCondition.Message) == 0 {
				availableCondition.Message = strings.Join(versionAvailability.Errors, "\n")
			} else {
				availableCondition.Message = availableCondition.Message + "\n" + strings.Join(versionAvailability.Errors, "\n")
			}
		}
	}
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, failingCondition)

	return utilerrors.NewAggregate(errors)
}

func (c WebConsoleOperator) ensureNamespace() (bool, error) {
	required := resourceread.ReadNamespaceOrDie([]byte(nsYaml))
	return resourceapply.ApplyNamespace(c.corev1Client, required)
}

func ensureWebConsoleConfig(options webconsolev1.OpenShiftWebConsoleConfigSpec) (*webconsoleconfigv1.WebConsoleConfiguration, error) {
	mergedConfig := &webconsoleconfigv1.WebConsoleConfiguration{}
	defaultConfig, err := readWebConsoleConfiguration(defaultConfig)
	if err != nil {
		return nil, err
	}
	ensureWebConsoleConfiguration(resourcemerge.BoolPtr(false), mergedConfig, *defaultConfig)
	ensureWebConsoleConfiguration(resourcemerge.BoolPtr(false), mergedConfig, options.WebConsoleConfig)

	return mergedConfig, nil
}

func readWebConsoleConfiguration(objBytes string) (*webconsoleconfigv1.WebConsoleConfiguration, error) {
	defaultConfigObj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(webconsoleconfigv1.SchemeGroupVersion), []byte(objBytes))
	if err != nil {
		return nil, err
	}
	ret, ok := defaultConfigObj.(*webconsoleconfigv1.WebConsoleConfiguration)
	if !ok {
		return nil, fmt.Errorf("expected *webconsoleconfigv1.WebConsoleConfiguration, got %T", defaultConfigObj)
	}

	return ret, nil
}

// Run starts the webconsole and blocks until stopCh is closed.
func (c *WebConsoleOperator) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting WebConsoleOperator")
	defer glog.Infof("Shutting down WebConsoleOperator")

	// TODO remove.  This kicks us until we wire correctly against a watch
	go wait.Until(func() {
		c.queue.Add("key")
	}, 10*time.Second, stopCh)

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *WebConsoleOperator) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *WebConsoleOperator) processNextWorkItem() bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	err := c.sync()
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}
