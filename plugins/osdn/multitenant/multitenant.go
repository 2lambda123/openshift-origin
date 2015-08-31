package multitenant

import (
	"github.com/golang/glog"
	"strings"

	kclient "k8s.io/kubernetes/pkg/client"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	"k8s.io/kubernetes/pkg/util/exec"

	"github.com/openshift/openshift-sdn/ovssubnet"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/plugins/osdn"
)

func NetworkPluginName() string {
	return "redhat/openshift-ovs-multitenant"
}

func Master(osClient *osclient.Client, kClient *kclient.Client, config *api.NetworkConfig) {
	osdnInterface := osdn.NewOsdnRegistryInterface(osClient, kClient)

	// get hostname from the gateway
	output, err := exec.New().Command("hostname", "-f").CombinedOutput()
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	host := strings.TrimSpace(string(output))

	kc, err := ovssubnet.NewMultitenantController(&osdnInterface, host, "", nil)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	for _, ns := range config.AdminNamespaces {
		kc.AdminNamespaces = append(kc.AdminNamespaces, ns)
	}
	err = kc.StartMaster(false, config.ClusterNetworkCIDR, config.HostSubnetLength, config.ServiceNetworkCIDR)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
}

func Node(osClient *osclient.Client, kClient *kclient.Client, hostname string, publicIP string, ready chan struct{}, plugin knetwork.NetworkPlugin) {
	mp, ok := plugin.(*MultitenantPlugin)
	if !ok {
		glog.Fatalf("Failed to type cast provided plugin to a multitenant type plugin")
	}
	osdnInterface := osdn.NewOsdnRegistryInterface(osClient, kClient)
	kc, err := ovssubnet.NewMultitenantController(&osdnInterface, hostname, publicIP, ready)
	if err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
	mp.OvsController = kc
	err = kc.StartNode(false, false)
	if err != nil {
		glog.Fatalf("SDN Node failed: %v", err)
	}
}
