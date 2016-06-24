package generictrigger

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcontroller "k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"

	"github.com/golang/glog"
	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const (
	// We must avoid creating processing deployment configs until the deployment config and image
	// stream stores have synced. If it hasn't synced, to avoid a hot loop, we'll wait this long
	// between checks.
	StoreSyncedPollPeriod = 100 * time.Millisecond
)

// NewDeploymentTriggerController returns a new DeploymentTriggerController.
func NewDeploymentTriggerController(dcInformer, streamInformer framework.SharedIndexInformer, oc osclient.Interface, kc kclient.Interface, codec runtime.Codec) *DeploymentTriggerController {
	c := &DeploymentTriggerController{
		dn: oc,
		rn: kc,

		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		codec: codec,
	}

	c.dcStore.Indexer = dcInformer.GetIndexer()
	dcInformer.AddEventHandler(framework.ResourceEventHandlerFuncs{
		AddFunc:    c.addDeploymentConfig,
		UpdateFunc: c.updateDeploymentConfig,
	})
	c.dcStoreSynced = dcInformer.HasSynced

	streamInformer.AddEventHandler(framework.ResourceEventHandlerFuncs{
		AddFunc:    c.addImageStream,
		UpdateFunc: c.updateImageStream,
	})

	return c
}

// Run begins watching and syncing.
func (c *DeploymentTriggerController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	// Wait for the dc store to sync before starting any work in this controller.
	ready := make(chan struct{})
	go c.waitForSyncedStore(ready, stopCh)
	select {
	case <-ready:
	case <-stopCh:
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	<-stopCh
	glog.Infof("Shutting down deployment trigger controller")
	c.queue.ShutDown()
}

func (c *DeploymentTriggerController) waitForSyncedStore(ready chan<- struct{}, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	for !c.dcStoreSynced() {
		glog.V(4).Infof("Waiting for the deployment config cache to sync before starting the trigger controller workers")
		select {
		case <-time.After(StoreSyncedPollPeriod):
		case <-stopCh:
			return
		}
	}
	close(ready)
}

func (c *DeploymentTriggerController) addDeploymentConfig(obj interface{}) {
	dc := obj.(*deployapi.DeploymentConfig)
	c.enqueueDeploymentConfig(dc)
}

func (c *DeploymentTriggerController) updateDeploymentConfig(old, cur interface{}) {
	dc := cur.(*deployapi.DeploymentConfig)
	c.enqueueDeploymentConfig(dc)
}

// addImageStream enqueues the deployment configs that point to the new image stream.
func (c *DeploymentTriggerController) addImageStream(obj interface{}) {
	stream := obj.(*imageapi.ImageStream)
	glog.V(4).Infof("Image stream %q added.", stream.Name)
	dcList, err := c.dcStore.GetConfigsForImageStream(stream)
	if err != nil {
		return
	}
	for _, dc := range dcList {
		c.enqueueDeploymentConfig(dc)
	}
}

// updateImageStream enqueues the deployment configs that point to the updated image stream.
func (c *DeploymentTriggerController) updateImageStream(old, cur interface{}) {
	// A periodic relist will send update events for all known streams.
	if kapi.Semantic.DeepEqual(old, cur) {
		return
	}

	stream := cur.(*imageapi.ImageStream)
	glog.V(4).Infof("Image stream %q updated.", stream.Name)
	dcList, err := c.dcStore.GetConfigsForImageStream(stream)
	if err != nil {
		return
	}
	for _, dc := range dcList {
		c.enqueueDeploymentConfig(dc)
	}
}

func (c *DeploymentTriggerController) enqueueDeploymentConfig(dc *deployapi.DeploymentConfig) {
	key, err := kcontroller.KeyFunc(dc)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", dc, err)
		return
	}
	c.queue.Add(key)
}

func (c *DeploymentTriggerController) worker() {
	for {
		if quit := c.work(); quit {
			return
		}
	}
}

func (c *DeploymentTriggerController) work() bool {
	key, quit := c.queue.Get()
	if quit {
		return true
	}
	defer c.queue.Done(key)

	dc, err := c.getByKey(key.(string))
	if err != nil {
		glog.Error(err.Error())
	}

	if dc == nil {
		return false
	}

	if err := c.Handle(dc); err != nil {
		utilruntime.HandleError(err)

		if c.queue.NumRequeues(key) < 2 {
			c.queue.AddRateLimited(key)
			return false
		}
	}
	c.queue.Forget(key)
	return false
}

func (c *DeploymentTriggerController) getByKey(key string) (*deployapi.DeploymentConfig, error) {
	obj, exists, err := c.dcStore.Indexer.GetByKey(key)
	if err != nil {
		glog.Infof("Unable to retrieve deployment config %q from store: %v", key, err)
		c.queue.Add(key)
		return nil, err
	}
	if !exists {
		glog.Infof("Deployment config %q has been deleted", key)
		return nil, nil
	}

	return obj.(*deployapi.DeploymentConfig), nil
}
