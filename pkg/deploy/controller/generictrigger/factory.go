package generictrigger

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/record"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/watch"

	osclient "github.com/openshift/origin/pkg/client"
	controller "github.com/openshift/origin/pkg/controller"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// DeploymentTriggerControllerFactory can create a DeploymentTriggerController that watches all DeploymentConfigs.
type DeploymentTriggerControllerFactory struct {
	// Client is an OpenShift client.
	Client osclient.Interface
	// KubeClient is a Kubernetes client.
	KubeClient kclient.Interface
	// Codec is used for encoding/decoding.
	Codec runtime.Codec
}

// Create creates a DeploymentTriggerController.
func (factory *DeploymentTriggerControllerFactory) Create() controller.RunnableController {
	deploymentConfigLW := &cache.ListWatch{
		ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
			return factory.Client.DeploymentConfigs(kapi.NamespaceAll).List(options)
		},
		WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
			return factory.Client.DeploymentConfigs(kapi.NamespaceAll).Watch(options)
		},
	}
	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentConfigLW, &deployapi.DeploymentConfig{}, queue, 2*time.Minute).Run()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(factory.KubeClient.Events(""))

	triggerController := NewDeploymentTriggerController(factory.Client, factory.KubeClient, factory.Codec)

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, retries controller.Retry) bool {
				utilruntime.HandleError(err)
				if _, isFatal := err.(fatalError); isFatal {
					return false
				}
				if retries.Count > 0 {
					return false
				}
				return true
			},
			flowcontrol.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			config := obj.(*deployapi.DeploymentConfig)
			return triggerController.Handle(config)
		},
	}
}
