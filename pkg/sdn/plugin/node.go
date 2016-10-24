package plugin

import (
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/golang/glog"

	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/util/netutils"
	"github.com/openshift/origin/pkg/util/ovs"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	kubeletTypes "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/labels"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	kubeutilnet "k8s.io/kubernetes/pkg/util/net"
)

type OsdnNode struct {
	multitenant        bool
	kClient            *kclient.Client
	osClient           *osclient.Client
	ovs                *ovs.Interface
	networkInfo        *NetworkInfo
	localIP            string
	hostName           string
	podNetworkReady    chan struct{}
	vnids              *nodeVNIDMap
	iptablesSyncPeriod time.Duration
	mtu                uint32
	egressPolicies     map[uint32][]*osapi.EgressNetworkPolicy
}

// Called by higher layers to create the plugin SDN node instance
func NewNodePlugin(pluginName string, osClient *osclient.Client, kClient *kclient.Client, hostname string, selfIP string, iptablesSyncPeriod time.Duration, mtu uint32) (*OsdnNode, error) {
	if !osapi.IsOpenShiftNetworkPlugin(pluginName) {
		return nil, nil
	}

	log.Infof("Initializing SDN node of type %q with configured hostname %q (IP %q), iptables sync period %q", pluginName, hostname, selfIP, iptablesSyncPeriod.String())
	if hostname == "" {
		output, err := kexec.New().Command("uname", "-n").CombinedOutput()
		if err != nil {
			return nil, err
		}
		hostname = strings.TrimSpace(string(output))
		log.Infof("Resolved hostname to %q", hostname)
	}
	if selfIP == "" {
		var err error
		selfIP, err = netutils.GetNodeIP(hostname)
		if err != nil {
			log.V(5).Infof("Failed to determine node address from hostname %s; using default interface (%v)", hostname, err)
			var defaultIP net.IP
			defaultIP, err = kubeutilnet.ChooseHostInterface()
			if err != nil {
				return nil, err
			}
			selfIP = defaultIP.String()
			log.Infof("Resolved IP address to %q", selfIP)
		}
	}

	ovsif, err := ovs.New(kexec.New(), BR)
	if err != nil {
		return nil, err
	}

	plugin := &OsdnNode{
		multitenant:        osapi.IsOpenShiftMultitenantNetworkPlugin(pluginName),
		kClient:            kClient,
		osClient:           osClient,
		ovs:                ovsif,
		localIP:            selfIP,
		hostName:           hostname,
		vnids:              newNodeVNIDMap(),
		podNetworkReady:    make(chan struct{}),
		iptablesSyncPeriod: iptablesSyncPeriod,
		mtu:                mtu,
		egressPolicies:     make(map[uint32][]*osapi.EgressNetworkPolicy),
	}
	return plugin, nil
}

func (node *OsdnNode) Start() error {
	var err error
	node.networkInfo, err = getNetworkInfo(node.osClient)
	if err != nil {
		return fmt.Errorf("Failed to get network information: %v", err)
	}

	nodeIPTables := newNodeIPTables(node.networkInfo.ClusterNetwork.String(), node.iptablesSyncPeriod)
	if err = nodeIPTables.Setup(); err != nil {
		return fmt.Errorf("Failed to set up iptables: %v", err)
	}

	networkChanged, err := node.SetupSDN()
	if err != nil {
		return err
	}

	err = node.SubnetStartNode()
	if err != nil {
		return err
	}

	if node.multitenant {
		if err = node.VnidStartNode(); err != nil {
			return err
		}
		if err = node.SetupEgressNetworkPolicy(); err != nil {
			return err
		}
	}

	if networkChanged {
		var pods []kapi.Pod
		pods, err = node.GetLocalPods(kapi.NamespaceAll)
		if err != nil {
			return err
		}
		for _, p := range pods {
			containerID := getPodContainerID(&p)
			err = node.UpdatePod(p.Namespace, p.Name, kubeletTypes.DockerID(containerID))
			if err != nil {
				log.Warningf("Could not update pod %q (%s): %s", p.Name, containerID, err)
			}
		}
	}

	node.markPodNetworkReady()

	return nil
}

func (node *OsdnNode) GetLocalPods(namespace string) ([]kapi.Pod, error) {
	fieldSelector := fields.Set{"spec.nodeName": node.hostName}.AsSelector()
	opts := kapi.ListOptions{
		LabelSelector: labels.Everything(),
		FieldSelector: fieldSelector,
	}
	podList, err := node.kClient.Pods(namespace).List(opts)
	if err != nil {
		return nil, err
	}

	// Filter running pods
	pods := make([]kapi.Pod, 0, len(podList.Items))
	for _, pod := range podList.Items {
		if pod.Status.Phase == kapi.PodRunning {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

func (node *OsdnNode) markPodNetworkReady() {
	close(node.podNetworkReady)
}

func (node *OsdnNode) WaitForPodNetworkReady() error {
	logInterval := 10 * time.Second
	numIntervals := 12 // timeout: 2 mins

	for i := 0; i < numIntervals; i++ {
		select {
		// Wait for StartNode() to finish SDN setup
		case <-node.podNetworkReady:
			return nil
		case <-time.After(logInterval):
			log.Infof("Waiting for SDN pod network to be ready...")
		}
	}
	return fmt.Errorf("SDN pod network is not ready(timeout: 2 mins)")
}
