package kubernetes

import (
	"fmt"
	"time"

	"github.com/emicklei/go-restful"
	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/resource"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	minionControllerPkg "github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider/controller"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/controller"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/resourcequota"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/service"
	kubeutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/scheduler"
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/scheduler/algorithmprovider"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/scheduler/factory"
)

const (
	KubeAPIPrefix        = "/api"
	KubeAPIPrefixV1Beta1 = "/api/v1beta1"
	KubeAPIPrefixV1Beta2 = "/api/v1beta2"
	KubeAPIPrefixV1Beta3 = "/api/v1beta3"
)

// TODO: Longer term we should read this from some config store, rather than a flag.
func (c *MasterConfig) EnsurePortalFlags() {
	if c.PortalNet == nil {
		glog.Fatal("No --portal-net specified")
	}
}

// InstallAPI starts a Kubernetes master and registers the supported REST APIs
// into the provided mux, then returns an array of strings indicating what
// endpoints were started (these are format strings that will expect to be sent
// a single string value).
func (c *MasterConfig) InstallAPI(container *restful.Container) []string {
	kubeletClient, err := kclient.NewKubeletClient(
		&kclient.KubeletConfig{
			Port: 10250,
		},
	)
	if err != nil {
		glog.Fatalf("Unable to configure Kubelet client: %v", err)
	}

	masterConfig := &master.Config{
		PublicAddress: c.MasterIP,
		ReadWritePort: c.MasterPort,
		ReadOnlyPort:  c.MasterPort,

		Client:     c.KubeClient,
		EtcdHelper: c.EtcdHelper,

		EventTTL: 2 * time.Hour,

		EnableV1Beta3: true,

		PortalNet: c.PortalNet,

		RequestContextMapper: c.RequestContextMapper,

		RestfulContainer: container,
		KubeletClient:    kubeletClient,
		APIPrefix:        KubeAPIPrefix,

		Authorizer:       c.Authorizer,
		AdmissionControl: c.AdmissionControl,
	}
	_ = master.New(masterConfig)

	return []string{
		fmt.Sprintf("Started Kubernetes API at %%s%s", KubeAPIPrefixV1Beta1),
		fmt.Sprintf("Started Kubernetes API at %%s%s", KubeAPIPrefixV1Beta2),
		fmt.Sprintf("Started Kubernetes API at %%s%s (experimental)", KubeAPIPrefixV1Beta3),
	}
}

// RunReplicationController starts the Kubernetes replication controller sync loop
func (c *MasterConfig) RunReplicationController() {
	controllerManager := controller.NewReplicationManager(c.KubeClient)
	controllerManager.Run(10 * time.Second)
	glog.Infof("Started Kubernetes Replication Manager")
}

// RunEndpointController starts the Kubernetes replication controller sync loop
func (c *MasterConfig) RunEndpointController() {
	endpoints := service.NewEndpointController(c.KubeClient)
	go kubeutil.Forever(func() { endpoints.SyncServiceEndpoints() }, time.Second*10)

	glog.Infof("Started Kubernetes Endpoint Controller")
}

// RunScheduler starts the Kubernetes scheduler
func (c *MasterConfig) RunScheduler() {
	configFactory := factory.NewConfigFactory(c.KubeClient)
	config, err := configFactory.CreateFromProvider(factory.DefaultProvider)
	if err != nil {
		glog.Fatalf("Unable to start scheduler: %v", err)
	}
	s := scheduler.New(config)
	s.Run()
	glog.Infof("Started Kubernetes Scheduler")
}

func (c *MasterConfig) RunResourceQuotaManager() {
	resourceQuotaManager := resourcequota.NewResourceQuotaManager(c.KubeClient)
	resourceQuotaManager.Run(10 * time.Second)
}

func (c *MasterConfig) RunMinionController() {
	nodeResources := &kapi.NodeResources{
		Capacity: kapi.ResourceList{
			kapi.ResourceCPU:    *resource.NewMilliQuantity(int64(1*1000), resource.DecimalSI),
			kapi.ResourceMemory: *resource.NewQuantity(int64(3*1024*1024*1024), resource.BinarySI),
		},
	}

	// TODO: enable this for TLS and make configurable
	kubeletClient, err := kclient.NewKubeletClient(&kclient.KubeletConfig{
		Port:        10250,
		EnableHttps: false,
	})
	if err != nil {
		glog.Fatalf("Failure to create kubelet client: %v", err)
	}
	minionController := minionControllerPkg.NewNodeController(nil, "", c.NodeHosts, nodeResources, c.KubeClient, kubeletClient, 10, 5*time.Minute)
	minionController.Run(10*time.Second, true)

	glog.Infof("Started Kubernetes Minion Controller")
}
