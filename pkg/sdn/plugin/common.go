package plugin

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/golang/glog"

	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/sdn/api"

	kapi "k8s.io/kubernetes/pkg/api"
	kcache "k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/fields"
)

func getPodContainerID(pod *kapi.Pod) string {
	if len(pod.Status.ContainerStatuses) > 0 {
		// Extract only container ID, pod.Status.ContainerStatuses[0].ContainerID is of the format: docker://<containerID>
		if parts := strings.Split(pod.Status.ContainerStatuses[0].ContainerID, "://"); len(parts) > 1 {
			return parts[1]
		}
	}
	return ""
}

func hostSubnetToString(subnet *osapi.HostSubnet) string {
	return fmt.Sprintf("%s (host: %q, ip: %q, subnet: %q)", subnet.Name, subnet.Host, subnet.HostIP, subnet.Subnet)
}

func clusterNetworkToString(n *osapi.ClusterNetwork) string {
	return fmt.Sprintf("%s (network: %q, hostSubnetBits: %d, serviceNetwork: %q, pluginName: %q)", n.Name, n.Network, n.HostSubnetLength, n.ServiceNetwork, n.PluginName)
}

type NetworkInfo struct {
	ClusterNetwork *net.IPNet
	ServiceNetwork *net.IPNet
}

func parseNetworkInfo(clusterNetwork string, serviceNetwork string) (*NetworkInfo, error) {
	_, cn, err := net.ParseCIDR(clusterNetwork)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse ClusterNetwork CIDR %s: %v", clusterNetwork, err)
	}
	_, sn, err := net.ParseCIDR(serviceNetwork)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse ServiceNetwork CIDR %s: %v", serviceNetwork, err)
	}

	return &NetworkInfo{
		ClusterNetwork: cn,
		ServiceNetwork: sn,
	}, nil
}

func (ni *NetworkInfo) validateNodeIP(nodeIP string) error {
	if nodeIP == "" || nodeIP == "127.0.0.1" {
		return fmt.Errorf("Invalid node IP %q", nodeIP)
	}

	// Ensure each node's NodeIP is not contained by the cluster network,
	// which could cause a routing loop. (rhbz#1295486)
	ipaddr := net.ParseIP(nodeIP)
	if ipaddr == nil {
		return fmt.Errorf("Failed to parse node IP %s", nodeIP)
	}

	if ni.ClusterNetwork.Contains(ipaddr) {
		return fmt.Errorf("Node IP %s conflicts with cluster network %s", nodeIP, ni.ClusterNetwork.String())
	}
	if ni.ServiceNetwork.Contains(ipaddr) {
		return fmt.Errorf("Node IP %s conflicts with service network %s", nodeIP, ni.ServiceNetwork.String())
	}

	return nil
}

func getNetworkInfo(osClient *osclient.Client) (*NetworkInfo, error) {
	cn, err := osClient.ClusterNetwork().Get(osapi.ClusterNetworkDefault)
	if err != nil {
		return nil, err
	}

	return parseNetworkInfo(cn.Network, cn.ServiceNetwork)
}

type ResourceName string

const (
	Nodes                 ResourceName = "Nodes"
	Namespaces            ResourceName = "Namespaces"
	NetNamespaces         ResourceName = "NetNamespaces"
	Services              ResourceName = "Services"
	HostSubnets           ResourceName = "HostSubnets"
	Pods                  ResourceName = "Pods"
	EgressNetworkPolicies ResourceName = "EgressNetworkPolicies"
)

// Run event queue for the given resource. The 'process' function is called
// repeatedly with each available cache.Delta that describes state changes
// to an object. If the process function returns an error queued changes
// for that object are dropped but processing continues with the next available
// object's cache.Deltas.  The error is logged with call stack information.
func runEventQueueForResource(client kcache.Getter, resourceName ResourceName, expectedType interface{}, selector fields.Selector, process ProcessEventFunc) {
	rn := strings.ToLower(string(resourceName))
	lw := kcache.NewListWatchFromClient(client, rn, kapi.NamespaceAll, selector)
	eventQueue := NewEventQueue(kcache.MetaNamespaceKeyFunc)
	// Repopulate event queue every 30 mins
	// Existing items in the event queue will have watch.Modified event type
	kcache.NewReflector(lw, expectedType, eventQueue, 30*time.Minute).Run()

	// Run the queue
	for {
		eventQueue.Pop(process)
	}
}

// Run event queue for the given resource
func RunEventQueue(client kcache.Getter, resourceName ResourceName, process ProcessEventFunc) {
	var expectedType interface{}

	switch resourceName {
	case HostSubnets:
		expectedType = &osapi.HostSubnet{}
	case NetNamespaces:
		expectedType = &osapi.NetNamespace{}
	case Nodes:
		expectedType = &kapi.Node{}
	case Namespaces:
		expectedType = &kapi.Namespace{}
	case Services:
		expectedType = &kapi.Service{}
	case Pods:
		expectedType = &kapi.Pod{}
	case EgressNetworkPolicies:
		expectedType = &osapi.EgressNetworkPolicy{}
	default:
		glog.Fatalf("Unknown resource %s during initialization of event queue", resourceName)
	}

	runEventQueueForResource(client, resourceName, expectedType, fields.Everything(), process)
}

func RunLocalPodsEventQueue(client kcache.Getter, nodeName string, process ProcessEventFunc) {
	if nodeName == "" {
		glog.Fatalf("LocalPods resource requires a node name")
	}

	runEventQueueForResource(client, Pods, &kapi.Pod{}, fields.Set{"spec.host": nodeName}.AsSelector(), process)
}
