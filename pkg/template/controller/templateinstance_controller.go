package controller

import (
	"errors"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kerrs "k8s.io/apimachinery/pkg/util/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/apis/authorization"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/utils/clock"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/openshift/origin/pkg/authorization/util"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templateinternalclient "github.com/openshift/origin/pkg/template/client/internalversion"
	"github.com/openshift/origin/pkg/template/generated/informers/internalversion/template/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
	templatelister "github.com/openshift/origin/pkg/template/generated/listers/template/internalversion"
)

const readinessTimeout = time.Hour

// TemplateInstanceController watches for new TemplateInstance objects and
// instantiates the template contained within, using parameters read from a
// linked Secret object.  The TemplateInstanceController instantiates objects
// using its own service account, first verifying that the requester also has
// permissions to instantiate.
type TemplateInstanceController struct {
	// TODO replace this with use of a codec built against the dynamic client
	// (discuss w/ deads what this means)
	dynamicRestMapper meta.RESTMapper
	dynamicClient     dynamic.Interface
	templateClient    templateclient.Interface

	// FIXME: Remove then cient when the build configs are able to report the
	//				status of the last build.
	buildClient buildclient.Interface

	kc kclientsetinternal.Interface

	lister   templatelister.TemplateInstanceLister
	informer cache.SharedIndexInformer

	queue workqueue.RateLimitingInterface

	readinessLimiter workqueue.RateLimiter

	clock clock.Clock
}

// NewTemplateInstanceController returns a new TemplateInstanceController.
func NewTemplateInstanceController(dynamicRestMapper meta.RESTMapper, dynamicClient dynamic.Interface, kc kclientsetinternal.Interface, buildClient buildclient.Interface, templateClient templateclient.Interface, informer internalversion.TemplateInstanceInformer) *TemplateInstanceController {
	c := &TemplateInstanceController{
		dynamicRestMapper: dynamicRestMapper,
		dynamicClient:     dynamicClient,
		kc:                kc,
		templateClient:    templateClient,
		buildClient:       buildClient,
		lister:            informer.Lister(),
		informer:          informer.Informer(),
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "openshift_template_instance_controller"),
		readinessLimiter:  workqueue.NewItemFastSlowRateLimiter(5*time.Second, 20*time.Second, 200),
		clock:             clock.RealClock{},
	}

	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueue(obj.(*templateapi.TemplateInstance))
		},
		UpdateFunc: func(_, obj interface{}) {
			c.enqueue(obj.(*templateapi.TemplateInstance))
		},
		DeleteFunc: func(obj interface{}) {
		},
	})

	prometheus.MustRegister(c)

	return c
}

// getTemplateInstance returns the TemplateInstance from the shared informer,
// given its key (dequeued from c.queue).
func (c *TemplateInstanceController) getTemplateInstance(key string) (*templateapi.TemplateInstance, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	return c.lister.TemplateInstances(namespace).Get(name)
}

// sync is the actual controller worker function.
func (c *TemplateInstanceController) sync(key string) error {
	templateInstanceOriginal, err := c.getTemplateInstance(key)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if templateInstanceOriginal.HasCondition(templateapi.TemplateInstanceReady, kapi.ConditionTrue) ||
		templateInstanceOriginal.HasCondition(templateapi.TemplateInstanceInstantiateFailure, kapi.ConditionTrue) {
		return nil
	}

	glog.V(4).Infof("TemplateInstance controller: syncing %s", key)

	templateInstanceCopy := templateInstanceOriginal.DeepCopy()

	if len(templateInstanceCopy.Status.Objects) != len(templateInstanceCopy.Spec.Template.Objects) {
		err = c.instantiate(templateInstanceCopy)
		if err != nil {
			glog.V(4).Infof("TemplateInstance controller: instantiate %s returned %v", key, err)

			templateInstanceCopy.SetCondition(templateapi.TemplateInstanceCondition{
				Type:    templateapi.TemplateInstanceInstantiateFailure,
				Status:  kapi.ConditionTrue,
				Reason:  "Failed",
				Message: formatError(err),
			})
			templateInstanceCompleted.WithLabelValues(string(templateapi.TemplateInstanceInstantiateFailure)).Inc()
		}
	}

	if !templateInstanceCopy.HasCondition(templateapi.TemplateInstanceInstantiateFailure, kapi.ConditionTrue) {
		ready, err := c.checkReadiness(templateInstanceCopy)
		if err != nil && !kerrors.IsTimeout(err) {
			// NB: kerrors.IsTimeout() is true in the case of an API server
			// timeout, not the timeout caused by readinessTimeout expiring.
			glog.V(4).Infof("TemplateInstance controller: checkReadiness %s returned %v", key, err)

			templateInstanceCopy.SetCondition(templateapi.TemplateInstanceCondition{
				Type:    templateapi.TemplateInstanceInstantiateFailure,
				Status:  kapi.ConditionTrue,
				Reason:  "Failed",
				Message: formatError(err),
			})
			templateInstanceCopy.SetCondition(templateapi.TemplateInstanceCondition{
				Type:    templateapi.TemplateInstanceReady,
				Status:  kapi.ConditionFalse,
				Reason:  "Failed",
				Message: "See InstantiateFailure condition for error message",
			})
			templateInstanceCompleted.WithLabelValues(string(templateapi.TemplateInstanceInstantiateFailure)).Inc()

		} else if ready {
			templateInstanceCopy.SetCondition(templateapi.TemplateInstanceCondition{
				Type:   templateapi.TemplateInstanceReady,
				Status: kapi.ConditionTrue,
				Reason: "Created",
			})
			templateInstanceCompleted.WithLabelValues(string(templateapi.TemplateInstanceReady)).Inc()

		} else {
			templateInstanceCopy.SetCondition(templateapi.TemplateInstanceCondition{
				Type:    templateapi.TemplateInstanceReady,
				Status:  kapi.ConditionFalse,
				Reason:  "Waiting",
				Message: "Waiting for instantiated objects to report ready",
			})
		}
	}

	_, err = c.templateClient.Template().TemplateInstances(templateInstanceCopy.Namespace).UpdateStatus(templateInstanceCopy)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("TemplateInstance status update failed: %v", err))
		return err
	}

	if !templateInstanceCopy.HasCondition(templateapi.TemplateInstanceReady, kapi.ConditionTrue) &&
		!templateInstanceCopy.HasCondition(templateapi.TemplateInstanceInstantiateFailure, kapi.ConditionTrue) {
		c.enqueueAfter(templateInstanceCopy, c.readinessLimiter.When(key))
	} else {
		c.readinessLimiter.Forget(key)
	}

	return nil
}

func (c *TemplateInstanceController) checkReadiness(templateInstance *templateapi.TemplateInstance) (bool, error) {
	if c.clock.Now().After(templateInstance.CreationTimestamp.Add(readinessTimeout)) {
		return false, fmt.Errorf("Timeout")
	}

	u := &user.DefaultInfo{Name: templateInstance.Spec.Requester.Username}

	for _, object := range templateInstance.Status.Objects {
		if !CanCheckReadiness(object.Ref) {
			continue
		}

		mapping, err := c.dynamicRestMapper.RESTMapping(object.Ref.GroupVersionKind().GroupKind())
		if err != nil {
			return false, err
		}

		if err = util.Authorize(c.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
			Namespace: object.Ref.Namespace,
			Verb:      "get",
			Group:     mapping.Resource.Group,
			Resource:  mapping.Resource.Resource,
			Name:      object.Ref.Name,
		}); err != nil {
			return false, err
		}

		obj, err := c.dynamicClient.Resource(mapping.Resource).Namespace(object.Ref.Namespace).Get(object.Ref.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if obj.GetUID() != object.Ref.UID {
			return false, kerrors.NewNotFound(mapping.Resource.GroupResource(), object.Ref.Name)
		}

		if strings.ToLower(obj.GetAnnotations()[templateapi.WaitForReadyAnnotation]) != "true" {
			continue
		}

		ready, failed, err := CheckReadiness(c.buildClient, object.Ref, obj)
		if err != nil {
			return false, err
		}
		if failed {
			return false, fmt.Errorf("Readiness failed on %s %s/%s", object.Ref.Kind, object.Ref.Namespace, object.Ref.Name)
		}
		if !ready {
			return false, nil
		}
	}

	return true, nil
}

// Run runs the controller until stopCh is closed, with as many workers as
// specified.
func (c *TemplateInstanceController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		return
	}

	glog.V(2).Infof("Starting TemplateInstance controller")

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

// runWorker repeatedly calls processNextWorkItem until the latter wants to
// exit.
func (c *TemplateInstanceController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem reads from the queue and calls the sync worker function.
// It returns false only when the queue is closed.
func (c *TemplateInstanceController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.sync(key.(string))
	if err == nil { // for example, success, or the TemplateInstance has gone away
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("TemplateInstance %v failed with: %v", key, err))
	c.queue.AddRateLimited(key) // avoid hot looping

	return true
}

// enqueue adds a TemplateInstance to c.queue.  This function is called on the
// shared informer goroutine.
func (c *TemplateInstanceController) enqueue(templateInstance *templateapi.TemplateInstance) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(templateInstance)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", templateInstance, err))
		return
	}

	c.queue.Add(key)
}

// enqueueAfter adds a TemplateInstance to c.queue after a duration.
func (c *TemplateInstanceController) enqueueAfter(templateInstance *templateapi.TemplateInstance, duration time.Duration) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(templateInstance)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", templateInstance, err))
		return
	}

	c.queue.AddAfter(key, duration)
}

// instantiate instantiates the objects contained in a TemplateInstance.  Any
// parameters for instantiation are contained in the Secret linked to the
// TemplateInstance.
func (c *TemplateInstanceController) instantiate(templateInstance *templateapi.TemplateInstance) error {
	if templateInstance.Spec.Requester == nil || templateInstance.Spec.Requester.Username == "" {
		return fmt.Errorf("spec.requester.username not set")
	}

	extra := map[string][]string{}
	for k, v := range templateInstance.Spec.Requester.Extra {
		extra[k] = []string(v)
	}

	u := &user.DefaultInfo{
		Name:   templateInstance.Spec.Requester.Username,
		UID:    templateInstance.Spec.Requester.UID,
		Groups: templateInstance.Spec.Requester.Groups,
		Extra:  extra,
	}

	var secret *kapi.Secret
	if templateInstance.Spec.Secret != nil {
		if err := util.Authorize(c.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
			Namespace: templateInstance.Namespace,
			Verb:      "get",
			Group:     kapi.GroupName,
			Resource:  "secrets",
			Name:      templateInstance.Spec.Secret.Name,
		}); err != nil {
			return err
		}

		s, err := c.kc.Core().Secrets(templateInstance.Namespace).Get(templateInstance.Spec.Secret.Name, metav1.GetOptions{})
		secret = s
		if err != nil {
			return err
		}
	}

	templatePtr := &templateInstance.Spec.Template
	template := templatePtr.DeepCopy()

	if secret != nil {
		for i, param := range template.Parameters {
			if value, ok := secret.Data[param.Name]; ok {
				template.Parameters[i].Value = string(value)
				template.Parameters[i].Generate = ""
			}
		}
	}

	if err := util.Authorize(c.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
		Namespace: templateInstance.Namespace,
		Verb:      "create",
		Group:     templateapi.GroupName,
		Resource:  "templateconfigs",
		Name:      template.Name,
	}); err != nil {
		return err
	}

	glog.V(4).Infof("TemplateInstance controller: creating TemplateConfig for %s/%s", templateInstance.Namespace, templateInstance.Name)

	tc := templateinternalclient.NewTemplateProcessorClient(c.templateClient.Template().RESTClient(), templateInstance.Namespace)
	template, err := tc.Process(template)
	if err != nil {
		return err
	}

	errs := runtime.DecodeList(template.Objects, unstructured.UnstructuredJSONScheme)
	if len(errs) > 0 {
		return kerrs.NewAggregate(errs)
	}

	for _, obj := range template.Objects {
		meta, err := meta.Accessor(obj)
		if err != nil {
			return err
		}
		labels := meta.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[templateapi.TemplateInstanceOwner] = string(templateInstance.UID)
		meta.SetLabels(labels)
	}

	// First, do all the SARs to ensure the requester actually has permissions
	// to create.
	glog.V(4).Infof("TemplateInstance controller: running SARs for %s/%s", templateInstance.Namespace, templateInstance.Name)
	allErrors := []error{}
	for _, currObjUncast := range template.Objects {
		currObj := currObjUncast.(*unstructured.Unstructured)
		restMapping, mappingErr := c.dynamicRestMapper.RESTMapping(currObj.GroupVersionKind().GroupKind(), currObj.GroupVersionKind().Version)
		if mappingErr != nil {
			allErrors = append(allErrors, mappingErr)
			continue
		}

		namespace := templateInstance.Namespace
		if len(currObj.GetNamespace()) > 0 {
			namespace = currObj.GetNamespace()
		}
		if namespace == "" {
			allErrors = append(allErrors, errors.New("namespace was empty"))
			continue
		}

		if err := util.Authorize(c.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
			Namespace: namespace,
			Verb:      "create",
			Group:     restMapping.Resource.Group,
			Resource:  restMapping.Resource.Resource,
			Name:      currObj.GetName(),
		}); err != nil {
			allErrors = append(allErrors, err)
			continue
		}
	}
	if len(allErrors) > 0 {
		return utilerrors.NewAggregate(allErrors)
	}

	// Second, create the objects, being tolerant if they already exist and are
	// labelled as having previously been created by us.
	glog.V(4).Infof("TemplateInstance controller: creating objects for %s/%s", templateInstance.Namespace, templateInstance.Name)
	templateInstance.Status.Objects = nil
	allErrors = []error{}
	for _, currObjUncast := range template.Objects {
		currObj := currObjUncast.(*unstructured.Unstructured)
		restMapping, mappingErr := c.dynamicRestMapper.RESTMapping(currObj.GroupVersionKind().GroupKind(), currObj.GroupVersionKind().Version)
		if mappingErr != nil {
			allErrors = append(allErrors, mappingErr)
			continue
		}

		namespace := templateInstance.Namespace
		if len(currObj.GetNamespace()) > 0 {
			namespace = currObj.GetNamespace()
		}
		if namespace == "" {
			allErrors = append(allErrors, errors.New("namespace was empty"))
			continue
		}

		createObj, createErr := c.dynamicClient.Resource(restMapping.Resource).Namespace(namespace).Create(currObj)
		if kerrors.IsAlreadyExists(createErr) {
			freshGottenObj, getErr := c.dynamicClient.Resource(restMapping.Resource).Namespace(namespace).Get(currObj.GetName(), metav1.GetOptions{})
			if getErr != nil {
				allErrors = append(allErrors, getErr)
				continue
			}

			owner, ok := freshGottenObj.GetLabels()[templateapi.TemplateInstanceOwner]
			// if the labels match, it's already our object so pretend we created
			// it successfully.
			if ok && owner == string(templateInstance.UID) {
				createObj, createErr = freshGottenObj, nil
			} else {
				allErrors = append(allErrors, createErr)
				continue
			}
		}
		if createErr != nil {
			allErrors = append(allErrors, mappingErr)
			continue
		}

		templateInstance.Status.Objects = append(templateInstance.Status.Objects,
			templateapi.TemplateInstanceObject{
				Ref: kapi.ObjectReference{
					Kind:       restMapping.GroupVersionKind.Kind,
					Namespace:  namespace,
					Name:       createObj.GetName(),
					UID:        createObj.GetUID(),
					APIVersion: restMapping.GroupVersionKind.GroupVersion().String(),
				},
			},
		)
	}

	// unconditionally add finalizer to the templateinstance because it should always have one.
	// TODO perhaps this should be done in a strategy long term.
	hasFinalizer := false
	for _, v := range templateInstance.Finalizers {
		if v == templateapi.TemplateInstanceFinalizer {
			hasFinalizer = true
			break
		}
	}
	if !hasFinalizer {
		templateInstance.Finalizers = append(templateInstance.Finalizers, templateapi.TemplateInstanceFinalizer)
	}
	if len(allErrors) > 0 {
		return utilerrors.NewAggregate(allErrors)
	}

	return nil
}

// formatError returns err.Error(), unless err is an Aggregate, in which case it
// "\n"-separates the contained errors.
func formatError(err error) string {
	if err, ok := err.(kerrs.Aggregate); ok {
		var errorStrings []string
		for _, err := range err.Errors() {
			errorStrings = append(errorStrings, err.Error())
		}
		return strings.Join(errorStrings, "\n")
	}

	return err.Error()
}
