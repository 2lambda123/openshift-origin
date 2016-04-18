package kubernetes

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/emicklei/go-restful"
	"github.com/golang/glog"

	kctrlmgr "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	extv1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/client/typed/dynamic"
	coreunversioned "k8s.io/kubernetes/pkg/client/typed/generated/core/unversioned"
	extensionsunversioned "k8s.io/kubernetes/pkg/client/typed/generated/extensions/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/watch"

	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/daemon"
	endpointcontroller "k8s.io/kubernetes/pkg/controller/endpoint"
	gccontroller "k8s.io/kubernetes/pkg/controller/gc"
	jobcontroller "k8s.io/kubernetes/pkg/controller/job"
	namespacecontroller "k8s.io/kubernetes/pkg/controller/namespace"
	nodecontroller "k8s.io/kubernetes/pkg/controller/node"
	volumeclaimbinder "k8s.io/kubernetes/pkg/controller/persistentvolume"
	podautoscalercontroller "k8s.io/kubernetes/pkg/controller/podautoscaler"
	"k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
	replicationcontroller "k8s.io/kubernetes/pkg/controller/replication"
	kresourcequota "k8s.io/kubernetes/pkg/controller/resourcequota"
	"k8s.io/kubernetes/pkg/master"
	quotainstall "k8s.io/kubernetes/pkg/quota/install"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/io"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/aws_ebs"
	"k8s.io/kubernetes/pkg/volume/cinder"
	"k8s.io/kubernetes/pkg/volume/gce_pd"
	"k8s.io/kubernetes/pkg/volume/host_path"
	"k8s.io/kubernetes/pkg/volume/nfs"
	"k8s.io/kubernetes/plugin/pkg/scheduler"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api/latest"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"

	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

const (
	KubeAPIPrefix                  = "/api"
	KubeAPIPrefixV1                = KubeAPIPrefix + "/v1"
	KubeAPIGroupPrefix             = "/apis"
	KubeAPIExtensionsPrefixV1beta1 = KubeAPIGroupPrefix + "/extensions/v1beta1"
)

// InstallAPI starts a Kubernetes master and registers the supported REST APIs
// into the provided mux, then returns an array of strings indicating what
// endpoints were started (these are format strings that will expect to be sent
// a single string value).
func (c *MasterConfig) InstallAPI(container *restful.Container) ([]string, error) {
	c.Master.RestfulContainer = container
	_, err := master.New(c.Master)
	if err != nil {
		return nil, err
	}

	messages := []string{}
	if configapi.HasKubernetesAPIVersion(c.Options, v1.SchemeGroupVersion) {
		messages = append(messages, fmt.Sprintf("Started Kubernetes API at %%s%s", KubeAPIPrefixV1))
	}

	if configapi.HasKubernetesAPIVersion(c.Options, extv1beta1.SchemeGroupVersion) {
		messages = append(messages, fmt.Sprintf("Started Kubernetes API Extensions at %%s%s", KubeAPIExtensionsPrefixV1beta1))
	}

	return messages, nil
}

// RunNamespaceController starts the Kubernetes Namespace Manager
func (c *MasterConfig) RunNamespaceController(kubeClient internalclientset.Interface, clientPool dynamic.ClientPool) {
	// Find the list of namespaced resources via discovery that the namespace controller must manage
	groupVersionResources, err := namespacecontroller.ServerPreferredNamespacedGroupVersionResources(kubeClient.Discovery())
	if err != nil {
		glog.Fatalf("Failed to get supported resources from server: %v", err)
	}
	namespaceController := namespacecontroller.NewNamespaceController(kubeClient, clientPool, groupVersionResources, c.ControllerManager.NamespaceSyncPeriod.Duration, kapi.FinalizerKubernetes)
	go namespaceController.Run(c.ControllerManager.ConcurrentNamespaceSyncs, utilwait.NeverStop)
}

// RunPersistentVolumeClaimBinder starts the Kubernetes Persistent Volume Claim Binder
func (c *MasterConfig) RunPersistentVolumeClaimBinder(client *client.Client) {
	binder := volumeclaimbinder.NewPersistentVolumeClaimBinder(internalclientset.FromUnversionedClient(client), c.ControllerManager.PVClaimBinderSyncPeriod.Duration)
	binder.Run()
}

func (c *MasterConfig) RunPersistentVolumeProvisioner(client *client.Client) {
	provisioner, err := kctrlmgr.NewVolumeProvisioner(c.CloudProvider, c.ControllerManager.VolumeConfiguration)
	if err != nil {
		// a provisioner was expected but encountered an error
		glog.Fatal(err)
	}

	// not all cloud providers have a provisioner.
	if provisioner != nil {
		allPlugins := []volume.VolumePlugin{}
		allPlugins = append(allPlugins, aws_ebs.ProbeVolumePlugins()...)
		allPlugins = append(allPlugins, gce_pd.ProbeVolumePlugins()...)
		allPlugins = append(allPlugins, cinder.ProbeVolumePlugins()...)
		controllerClient := volumeclaimbinder.NewControllerClient(internalclientset.FromUnversionedClient(client))
		provisionerController, err := volumeclaimbinder.NewPersistentVolumeProvisionerController(
			controllerClient,
			c.ControllerManager.PVClaimBinderSyncPeriod.Duration,
			c.ControllerManager.ClusterName,
			allPlugins,
			provisioner,
			c.CloudProvider,
		)
		if err != nil {
			glog.Fatalf("Could not start Persistent Volume Provisioner: %+v", err)
		}
		provisionerController.Run()
	}
}

func (c *MasterConfig) RunPersistentVolumeClaimRecycler(recyclerImageName string, client *client.Client, namespace string) {
	uid := int64(0)
	defaultScrubPod := volume.NewPersistentVolumeRecyclerPodTemplate()
	defaultScrubPod.Namespace = namespace
	defaultScrubPod.Spec.Containers[0].Image = recyclerImageName
	defaultScrubPod.Spec.Containers[0].Command = []string{"/usr/bin/recycle"}
	defaultScrubPod.Spec.Containers[0].Args = []string{"/scrub"}
	defaultScrubPod.Spec.Containers[0].SecurityContext = &kapi.SecurityContext{RunAsUser: &uid}
	defaultScrubPod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent

	volumeConfig := c.ControllerManager.VolumeConfiguration
	hostPathConfig := volume.VolumeConfig{
		RecyclerMinimumTimeout:   volumeConfig.PersistentVolumeRecyclerConfiguration.MinimumTimeoutHostPath,
		RecyclerTimeoutIncrement: volumeConfig.PersistentVolumeRecyclerConfiguration.IncrementTimeoutHostPath,
		RecyclerPodTemplate:      defaultScrubPod,
	}

	if len(volumeConfig.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathHostPath) != 0 {
		if err := attemptToLoadRecycler(volumeConfig.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathHostPath, &hostPathConfig); err != nil {
			glog.Fatalf("Could not create hostpath recycler pod from file %s: %+v", volumeConfig.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathHostPath, err)
		}
	}
	nfsConfig := volume.VolumeConfig{
		RecyclerMinimumTimeout:   volumeConfig.PersistentVolumeRecyclerConfiguration.MinimumTimeoutNFS,
		RecyclerTimeoutIncrement: volumeConfig.PersistentVolumeRecyclerConfiguration.IncrementTimeoutNFS,
		RecyclerPodTemplate:      defaultScrubPod,
	}

	if len(volumeConfig.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathNFS) != 0 {
		if err := attemptToLoadRecycler(volumeConfig.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathNFS, &nfsConfig); err != nil {
			glog.Fatalf("Could not create NFS recycler pod from file %s: %+v", volumeConfig.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathNFS, err)
		}
	}

	allPlugins := []volume.VolumePlugin{}
	allPlugins = append(allPlugins, host_path.ProbeVolumePlugins(hostPathConfig)...)
	allPlugins = append(allPlugins, nfs.ProbeVolumePlugins(nfsConfig)...)

	// dynamic provisioning allows deletion of volumes as a recycling operation after a claim is deleted
	allPlugins = append(allPlugins, aws_ebs.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, gce_pd.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, cinder.ProbeVolumePlugins()...)

	recycler, err := volumeclaimbinder.NewPersistentVolumeRecycler(
		internalclientset.FromUnversionedClient(client),
		c.ControllerManager.PVClaimBinderSyncPeriod.Duration,
		volumeConfig.PersistentVolumeRecyclerConfiguration.MaximumRetry,
		allPlugins,
		c.CloudProvider,
	)
	if err != nil {
		glog.Fatalf("Could not start Persistent Volume Recycler: %+v", err)
	}
	recycler.Run()
}

// attemptToLoadRecycler tries decoding a pod from a filepath for use as a recycler for a volume.
// If a path is not set as a CLI flag, no load will be attempted and no error returned.
// If a path is set and the pod was successfully loaded, the recycler pod will be set on the config and no error returned.
// Any failed attempt to load the recycler pod will return an error.
// TODO: make this func re-usable upstream and use downstream.  No need to duplicate this function.
func attemptToLoadRecycler(path string, config *volume.VolumeConfig) error {
	glog.V(5).Infof("Attempting to load recycler pod file from %s", path)
	recyclerPod, err := io.LoadPodFromFile(path)
	if err != nil {
		return err
	}
	if len(recyclerPod.Spec.Volumes) != 1 {
		return fmt.Errorf("Recycler pod is expected to have exactly 1 volume to scrub, but found %d", len(recyclerPod.Spec.Volumes))
	}
	config.RecyclerPodTemplate = recyclerPod
	glog.V(5).Infof("Recycler set to %s/%s", config.RecyclerPodTemplate.Namespace, config.RecyclerPodTemplate.Name)
	return nil
}

func (c *MasterConfig) CreateSharedPodInformer(client *client.Client) framework.SharedInformer {
	clientset := internalclientset.FromUnversionedClient(client)

	sharedInformer := framework.NewSharedInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return clientset.Core().Pods(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return clientset.Core().Pods(kapi.NamespaceAll).Watch(options)
			},
		},
		&kapi.Pod{}, kctrlmgr.ResyncPeriod(c.ControllerManager)())

	return sharedInformer
}

// RunReplicationController starts the Kubernetes replication controller sync loop
func (c *MasterConfig) RunReplicationController(sharedInformer framework.SharedInformer, client *client.Client) {
	controllerManager := replicationcontroller.NewReplicationManagerFromSharedInformer(
		sharedInformer,
		internalclientset.FromUnversionedClient(client),
		kctrlmgr.ResyncPeriod(c.ControllerManager),
		replicationcontroller.BurstReplicas,
		c.ControllerManager.LookupCacheSizeForRC,
	)
	go controllerManager.Run(c.ControllerManager.ConcurrentRCSyncs, utilwait.NeverStop)
}

// RunJobController starts the Kubernetes job controller sync loop
func (c *MasterConfig) RunJobController(client *client.Client) {
	controller := jobcontroller.NewJobController(internalclientset.FromUnversionedClient(client), kctrlmgr.ResyncPeriod(c.ControllerManager))
	go controller.Run(c.ControllerManager.ConcurrentJobSyncs, utilwait.NeverStop)
}

// RunHPAController starts the Kubernetes hpa controller sync loop
func (c *MasterConfig) RunHPAController(oc *osclient.Client, kc *client.Client, heapsterNamespace string) {
	clientsetClient := internalclientset.FromUnversionedClient(kc)
	delegatingScaleNamespacer := osclient.NewDelegatingScaleNamespacer(oc, kc)
	podautoscaler := podautoscalercontroller.NewHorizontalController(
		coreunversioned.EventsGetter(clientsetClient),
		extensionsunversioned.ScalesGetter(delegatingScaleNamespacer),
		extensionsunversioned.HorizontalPodAutoscalersGetter(clientsetClient),
		metrics.NewHeapsterMetricsClient(clientsetClient, heapsterNamespace, "https", "heapster", ""),
		c.ControllerManager.HorizontalPodAutoscalerSyncPeriod.Duration,
	)
	go podautoscaler.Run(utilwait.NeverStop)
}

func (c *MasterConfig) RunDaemonSetsController(client *client.Client) {
	controller := daemon.NewDaemonSetsController(
		internalclientset.FromUnversionedClient(client),
		kctrlmgr.ResyncPeriod(c.ControllerManager),
		c.ControllerManager.LookupCacheSizeForDaemonSet,
	)
	go controller.Run(c.ControllerManager.ConcurrentDaemonSetSyncs, utilwait.NeverStop)
}

// RunEndpointController starts the Kubernetes replication controller sync loop
func (c *MasterConfig) RunEndpointController(sharedInformer framework.SharedInformer) {
	endpoints := endpointcontroller.NewEndpointControllerFromSharedInformer(sharedInformer, internalclientset.FromUnversionedClient(c.KubeClient))
	go endpoints.Run(c.ControllerManager.ConcurrentEndpointSyncs, utilwait.NeverStop)
}

// RunScheduler starts the Kubernetes scheduler
func (c *MasterConfig) RunScheduler() {
	config, err := c.createSchedulerConfig()
	if err != nil {
		glog.Fatalf("Unable to start scheduler: %v", err)
	}
	eventcast := record.NewBroadcaster()
	config.Recorder = eventcast.NewRecorder(kapi.EventSource{Component: kapi.DefaultSchedulerName})
	eventcast.StartRecordingToSink(c.KubeClient.Events(""))

	s := scheduler.New(config)
	s.Run()
}

// RunResourceQuotaManager starts the resource quota manager
func (c *MasterConfig) RunResourceQuotaManager() {
	client := internalclientset.FromUnversionedClient(c.KubeClient)
	resourceQuotaRegistry := quotainstall.NewRegistry(client)
	groupKindsToReplenish := []unversioned.GroupKind{
		kapi.Kind("Pod"),
		kapi.Kind("Service"),
		kapi.Kind("ReplicationController"),
		kapi.Kind("PersistentVolumeClaim"),
		kapi.Kind("Secret"),
		kapi.Kind("ConfigMap"),
	}
	resourceQuotaControllerOptions := &kresourcequota.ResourceQuotaControllerOptions{
		KubeClient:                client,
		ResyncPeriod:              controller.StaticResyncPeriodFunc(c.ControllerManager.ResourceQuotaSyncPeriod.Duration),
		Registry:                  resourceQuotaRegistry,
		GroupKindsToReplenish:     groupKindsToReplenish,
		ControllerFactory:         kresourcequota.NewReplenishmentControllerFactory(client),
		ReplenishmentResyncPeriod: kctrlmgr.ResyncPeriod(c.ControllerManager),
	}
	go kresourcequota.NewResourceQuotaController(resourceQuotaControllerOptions).Run(c.ControllerManager.ConcurrentResourceQuotaSyncs, utilwait.NeverStop)
}

func (c *MasterConfig) RunGCController(client *client.Client) {
	if c.ControllerManager.TerminatedPodGCThreshold > 0 {
		gcController := gccontroller.New(internalclientset.FromUnversionedClient(client), kctrlmgr.ResyncPeriod(c.ControllerManager), c.ControllerManager.TerminatedPodGCThreshold)
		go gcController.Run(utilwait.NeverStop)
	}
}

// RunNodeController starts the node controller
func (c *MasterConfig) RunNodeController() {
	s := c.ControllerManager

	// this cidr has been validated already
	_, clusterCIDR, _ := net.ParseCIDR(s.ClusterCIDR)

	controller := nodecontroller.NewNodeController(
		c.CloudProvider,
		internalclientset.FromUnversionedClient(c.KubeClient),
		s.PodEvictionTimeout.Duration,

		util.NewTokenBucketRateLimiter(s.DeletingPodsQps, s.DeletingPodsBurst),
		util.NewTokenBucketRateLimiter(s.DeletingPodsQps, s.DeletingPodsBurst), // upstream uses the same ones too

		s.NodeMonitorGracePeriod.Duration,
		s.NodeStartupGracePeriod.Duration,
		s.NodeMonitorPeriod.Duration,

		clusterCIDR,
		s.AllocateNodeCIDRs,
	)

	controller.Run(s.NodeSyncPeriod.Duration)
}

func (c *MasterConfig) createSchedulerConfig() (*scheduler.Config, error) {
	var policy schedulerapi.Policy
	var configData []byte

	// TODO make the rate limiter configurable
	configFactory := factory.NewConfigFactory(c.KubeClient, kapi.DefaultSchedulerName)
	if _, err := os.Stat(c.Options.SchedulerConfigFile); err == nil {
		configData, err = ioutil.ReadFile(c.Options.SchedulerConfigFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read scheduler config: %v", err)
		}
		err = runtime.DecodeInto(latestschedulerapi.Codec, configData, &policy)
		if err != nil {
			return nil, fmt.Errorf("invalid scheduler configuration: %v", err)
		}

		return configFactory.CreateFromConfig(policy)
	}

	// if the config file isn't provided, use the default provider
	return configFactory.CreateFromProvider(factory.DefaultProvider)
}
