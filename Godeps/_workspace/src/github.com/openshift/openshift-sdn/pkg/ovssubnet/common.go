package ovssubnet

import (
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/golang/glog"

	"github.com/openshift/openshift-sdn/pkg/netutils"
	"github.com/openshift/openshift-sdn/pkg/ovssubnet/api"
	"github.com/openshift/openshift-sdn/pkg/ovssubnet/controller/kube"
	"github.com/openshift/openshift-sdn/pkg/ovssubnet/controller/multitenant"

	utildbus "k8s.io/kubernetes/pkg/util/dbus"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	kexec "k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/iptables"
)

const (
	// Maximum VXLAN Network Identifier as per RFC#7348
	MaxVNID = ((1 << 24) - 1)
	// VNID for the admin namespaces
	AdminVNID = uint(0)
)

type OvsController struct {
	subnetRegistry  api.SubnetRegistry
	localIP         string
	localSubnet     *api.Subnet
	hostName        string
	subnetAllocator *netutils.SubnetAllocator
	sig             chan struct{}
	ready           chan struct{}
	flowController  FlowController
	VNIDMap         map[string]uint
	netIDManager    *netutils.NetIDAllocator
	AdminNamespaces []string
	services        map[string]api.Service
}

type FlowController interface {
	Setup(localSubnetCIDR, clusterNetworkCIDR, serviceNetworkCIDR string, mtu uint) error

	AddOFRules(nodeIP, nodeSubnetCIDR, localIP string) error
	DelOFRules(nodeIP, localIP string) error

	AddServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error
	DelServiceOFRules(netID uint, IP string, protocol api.ServiceProtocol, port uint) error

	UpdatePod(namespace, podName, containerID string, netID uint) error
}

func NewKubeController(sub api.SubnetRegistry, hostname string, selfIP string, ready chan struct{}) (*OvsController, error) {
	kubeController, err := NewController(sub, hostname, selfIP, ready)
	if err == nil {
		kubeController.flowController = kube.NewFlowController()
	}
	return kubeController, err
}

func NewMultitenantController(sub api.SubnetRegistry, hostname string, selfIP string, ready chan struct{}) (*OvsController, error) {
	mtController, err := NewController(sub, hostname, selfIP, ready)
	if err == nil {
		mtController.flowController = multitenant.NewFlowController()
	}
	return mtController, err
}

func NewController(sub api.SubnetRegistry, hostname string, selfIP string, ready chan struct{}) (*OvsController, error) {
	if selfIP == "" {
		var err error
		selfIP, err = GetNodeIP(hostname)
		if err != nil {
			return nil, err
		}
	}
	log.Infof("Self IP: %s.", selfIP)
	return &OvsController{
		subnetRegistry:  sub,
		localIP:         selfIP,
		hostName:        hostname,
		localSubnet:     nil,
		subnetAllocator: nil,
		VNIDMap:         make(map[string]uint),
		sig:             make(chan struct{}),
		ready:           ready,
		AdminNamespaces: make([]string, 0),
		services:        make(map[string]api.Service),
	}, nil
}

func (oc *OvsController) isMultitenant() bool {
	_, is_mt := oc.flowController.(*multitenant.FlowController)
	return is_mt
}

func (oc *OvsController) validateClusterNetwork(networkCIDR string, subnetsInUse []string, hostIPNets []*net.IPNet) error {
	clusterIP, clusterIPNet, err := net.ParseCIDR(networkCIDR)
	if err != nil {
		return fmt.Errorf("Failed to parse network address: %s", networkCIDR)
	}

	errList := []error{}
	for _, ipNet := range hostIPNets {
		if ipNet.Contains(clusterIP) {
			errList = append(errList, fmt.Errorf("Error: Cluster IP: %s conflicts with host network: %s", clusterIP.String(), ipNet.String()))
		}
		if clusterIPNet.Contains(ipNet.IP) {
			errList = append(errList, fmt.Errorf("Error: Host network with IP: %s conflicts with cluster network: %s", ipNet.IP.String(), networkCIDR))
		}
	}

	for _, netStr := range subnetsInUse {
		subnetIP, _, err := net.ParseCIDR(netStr)
		if err != nil {
			errList = append(errList, fmt.Errorf("Failed to parse network address: %s", netStr))
			continue
		}
		if !clusterIPNet.Contains(subnetIP) {
			errList = append(errList, fmt.Errorf("Error: Existing node subnet: %s is not part of cluster network: %s", netStr, networkCIDR))
		}
	}
	return kerrors.NewAggregate(errList)
}

func (oc *OvsController) validateServiceNetwork(networkCIDR string, hostIPNets []*net.IPNet) error {
	serviceIP, serviceIPNet, err := net.ParseCIDR(networkCIDR)
	if err != nil {
		return fmt.Errorf("Failed to parse network address: %s", networkCIDR)
	}

	errList := []error{}
	for _, ipNet := range hostIPNets {
		if ipNet.Contains(serviceIP) {
			errList = append(errList, fmt.Errorf("Error: Service IP: %s conflicts with host network: %s", ipNet.String(), networkCIDR))
		}
		if serviceIPNet.Contains(ipNet.IP) {
			errList = append(errList, fmt.Errorf("Error: Host network with IP: %s conflicts with service network: %s", ipNet.IP.String(), networkCIDR))
		}
	}

	services, _, err := oc.subnetRegistry.GetServices()
	if err != nil {
		return err
	}
	for _, svc := range services {
		if !serviceIPNet.Contains(net.ParseIP(svc.IP)) {
			errList = append(errList, fmt.Errorf("Error: Existing service with IP: %s is not part of service network: %s", svc.IP, networkCIDR))
		}
	}
	return kerrors.NewAggregate(errList)
}

func (oc *OvsController) validateNetworkConfig(clusterNetworkCIDR, serviceNetworkCIDR string, subnetsInUse []string) error {
	// TODO: Instead of hardcoding 'tun0' and 'lbr0', get it from common place.
	// This will ensure both the kube/multitenant scripts and master validations use the same name.
	hostIPNets, err := netutils.GetHostIPNetworks([]string{"tun0", "lbr0"})
	if err != nil {
		return err
	}

	errList := []error{}
	if err := oc.validateClusterNetwork(clusterNetworkCIDR, subnetsInUse, hostIPNets); err != nil {
		errList = append(errList, err)
	}
	if err := oc.validateServiceNetwork(serviceNetworkCIDR, hostIPNets); err != nil {
		errList = append(errList, err)
	}
	return kerrors.NewAggregate(errList)
}

func (oc *OvsController) StartMaster(clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) error {
	subrange := make([]string, 0)
	subnets, _, err := oc.subnetRegistry.GetSubnets()
	if err != nil {
		log.Errorf("Error in initializing/fetching subnets: %v", err)
		return err
	}
	for _, sub := range subnets {
		subrange = append(subrange, sub.SubnetCIDR)
	}

	// Any mismatch in cluster/service network is handled by WriteNetworkConfig
	// For any new cluster/service network, ensure existing node subnets belong
	// to the given cluster network and service IPs belong to the given service network
	if _, err = oc.subnetRegistry.GetClusterNetworkCIDR(); err != nil {
		err = oc.validateNetworkConfig(clusterNetworkCIDR, serviceNetworkCIDR, subrange)
		if err != nil {
			return err
		}
	}

	err = oc.subnetRegistry.WriteNetworkConfig(clusterNetworkCIDR, clusterBitsPerSubnet, serviceNetworkCIDR)
	if err != nil {
		return err
	}

	oc.subnetAllocator, err = netutils.NewSubnetAllocator(clusterNetworkCIDR, clusterBitsPerSubnet, subrange)
	if err != nil {
		return err
	}

	result, err := oc.watchAndGetResource("Node")
	if err != nil {
		return err
	}
	nodes := result.([]api.Node)
	err = oc.serveExistingNodes(nodes)
	if err != nil {
		return err
	}

	if oc.isMultitenant() {
		nets, _, err := oc.subnetRegistry.GetNetNamespaces()
		if err != nil {
			return err
		}
		inUse := make([]uint, 0)
		for _, net := range nets {
			if net.NetID != AdminVNID {
				inUse = append(inUse, net.NetID)
			}
			oc.VNIDMap[net.Name] = net.NetID
		}
		// VNID: 0 reserved for default namespace and can reach any network in the cluster
		// VNID: 1 to 9 are internally reserved for any special cases in the future
		oc.netIDManager, err = netutils.NewNetIDAllocator(10, MaxVNID, inUse)
		if err != nil {
			return err
		}

		result, err := oc.watchAndGetResource("Namespace")
		if err != nil {
			return err
		}

		// Handle existing namespaces
		namespaces := result.([]string)
		for _, nsName := range namespaces {
			// Revoke invalid VNID for admin namespaces
			if oc.isAdminNamespace(nsName) {
				netid, ok := oc.VNIDMap[nsName]
				if ok && (netid != AdminVNID) {
					err := oc.revokeVNID(nsName)
					if err != nil {
						return err
					}
				}
			}
			_, found := oc.VNIDMap[nsName]
			// Assign VNID for the namespace if it doesn't exist
			if !found {
				err := oc.assignVNID(nsName)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (oc *OvsController) isAdminNamespace(nsName string) bool {
	for _, name := range oc.AdminNamespaces {
		if name == nsName {
			return true
		}
	}
	return false
}

func (oc *OvsController) assignVNID(namespaceName string) error {
	_, err := oc.subnetRegistry.GetNetNamespace(namespaceName)
	if err == nil {
		return nil
	}
	var netid uint
	if oc.isAdminNamespace(namespaceName) {
		netid = AdminVNID
	} else {
		var err error
		netid, err = oc.netIDManager.GetNetID()
		if err != nil {
			return err
		}
	}
	err = oc.subnetRegistry.WriteNetNamespace(namespaceName, netid)
	if err != nil {
		e := oc.netIDManager.ReleaseNetID(netid)
		if e != nil {
			log.Errorf("Error while releasing Net ID: %v", e)
		}
		return err
	}
	oc.VNIDMap[namespaceName] = netid
	return nil
}

func (oc *OvsController) revokeVNID(namespaceName string) error {
	err := oc.subnetRegistry.DeleteNetNamespace(namespaceName)
	if err != nil {
		return err
	}
	netid, found := oc.VNIDMap[namespaceName]
	if !found {
		return fmt.Errorf("Error while fetching Net ID for namespace: %s", namespaceName)
	}
	delete(oc.VNIDMap, namespaceName)

	// Skip AdminVNID as it is not part of Net ID allocation
	if netid == AdminVNID {
		return nil
	}

	// Check if this netid is used by any other namespaces
	// If not, then release the netid
	netid_inuse := false
	for _, id := range oc.VNIDMap {
		if id == netid {
			netid_inuse = true
			break
		}
	}
	if !netid_inuse {
		err = oc.netIDManager.ReleaseNetID(netid)
		if err != nil {
			return fmt.Errorf("Error while releasing Net ID: %v", err)
		}
	}
	return nil
}

func (oc *OvsController) watchNetworks(ready chan<- bool, start <-chan string) {
	nsevent := make(chan *api.NamespaceEvent)
	stop := make(chan bool)
	go oc.subnetRegistry.WatchNamespaces(nsevent, ready, start, stop)
	for {
		select {
		case ev := <-nsevent:
			switch ev.Type {
			case api.Added:
				err := oc.assignVNID(ev.Name)
				if err != nil {
					log.Errorf("Error assigning Net ID: %v", err)
					continue
				}
			case api.Deleted:
				err := oc.revokeVNID(ev.Name)
				if err != nil {
					log.Errorf("Error revoking Net ID: %v", err)
					continue
				}
			}
		case <-oc.sig:
			log.Error("Signal received. Stopping watching of nodes.")
			stop <- true
			return
		}
	}
}

func (oc *OvsController) serveExistingNodes(nodes []api.Node) error {
	for _, node := range nodes {
		_, err := oc.subnetRegistry.GetSubnet(node.Name)
		if err == nil {
			// subnet already exists, continue
			continue
		}
		err = oc.AddNode(node.Name, node.IP)
		if err != nil {
			return err
		}
	}
	return nil
}

func (oc *OvsController) AddNode(nodeName string, nodeIP string) error {
	sn, err := oc.subnetAllocator.GetNetwork()
	if err != nil {
		log.Errorf("Error creating network for node %s.", nodeName)
		return err
	}

	if nodeIP == "" || nodeIP == "127.0.0.1" {
		return fmt.Errorf("Invalid node IP")
	}

	subnet := &api.Subnet{
		NodeIP:     nodeIP,
		SubnetCIDR: sn.String(),
	}
	err = oc.subnetRegistry.CreateSubnet(nodeName, subnet)
	if err != nil {
		log.Errorf("Error writing subnet to etcd for node %s: %v", nodeName, sn)
		return err
	}
	return nil
}

func (oc *OvsController) DeleteNode(nodeName string) error {
	sub, err := oc.subnetRegistry.GetSubnet(nodeName)
	if err != nil {
		log.Errorf("Error fetching subnet for node %s for delete operation.", nodeName)
		return err
	}
	_, ipnet, err := net.ParseCIDR(sub.SubnetCIDR)
	if err != nil {
		log.Errorf("Error parsing subnet for node %s for deletion: %s", nodeName, sub.SubnetCIDR)
		return err
	}
	oc.subnetAllocator.ReleaseNetwork(ipnet)
	return oc.subnetRegistry.DeleteSubnet(nodeName)
}

func (oc *OvsController) StartNode(mtu uint) error {
	err := oc.initSelfSubnet()
	if err != nil {
		return err
	}

	// Assume we are working with IPv4
	clusterNetworkCIDR, err := oc.subnetRegistry.GetClusterNetworkCIDR()
	if err != nil {
		log.Errorf("Failed to obtain ClusterNetwork: %v", err)
		return err
	}
	servicesNetworkCIDR, err := oc.subnetRegistry.GetServicesNetworkCIDR()
	if err != nil {
		log.Errorf("Failed to obtain ServicesNetwork: %v", err)
		return err
	}
	err = oc.flowController.Setup(oc.localSubnet.SubnetCIDR, clusterNetworkCIDR, servicesNetworkCIDR, mtu)
	if err != nil {
		return err
	}

	ipt := iptables.New(kexec.New(), utildbus.New(), iptables.ProtocolIpv4)
	err = SetupIptables(ipt, clusterNetworkCIDR)
	if err != nil {
		return err
	}

	ipt.AddReloadFunc(func() {
		err := SetupIptables(ipt, clusterNetworkCIDR)
		if err != nil {
			log.Errorf("Error reloading iptables: %v\n", err)
		}
	})

	result, err := oc.watchAndGetResource("HostSubnet")
	if err != nil {
		return err
	}
	subnets := result.([]api.Subnet)
	for _, s := range subnets {
		oc.flowController.AddOFRules(s.NodeIP, s.SubnetCIDR, oc.localIP)
	}
	if oc.isMultitenant() {
		result, err := oc.watchAndGetResource("NetNamespace")
		if err != nil {
			return err
		}
		nslist := result.([]api.NetNamespace)
		for _, ns := range nslist {
			oc.VNIDMap[ns.Name] = ns.NetID
		}

		result, err = oc.watchAndGetResource("Service")
		if err != nil {
			return err
		}
		services := result.([]api.Service)
		for _, svc := range services {
			netid, found := oc.VNIDMap[svc.Namespace]
			if !found {
				return fmt.Errorf("Error fetching Net ID for namespace: %s", svc.Namespace)
			}
			oc.services[svc.UID] = svc
			for _, port := range svc.Ports {
				oc.flowController.AddServiceOFRules(netid, svc.IP, port.Protocol, port.Port)
			}
		}

		_, err = oc.watchAndGetResource("Pod")
		if err != nil {
			return err
		}
	}

	if oc.ready != nil {
		close(oc.ready)
	}
	return nil
}

func (oc *OvsController) updatePodNetwork(namespace string, netID, oldNetID uint) error {
	// Update OF rules for the existing/old pods in the namespace
	pods, err := oc.subnetRegistry.GetRunningPods(oc.hostName, namespace)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		err := oc.flowController.UpdatePod(pod.Namespace, pod.Name, pod.ContainerID, netID)
		if err != nil {
			return err
		}
	}

	// Update OF rules for the old services in the namespace
	services, err := oc.subnetRegistry.GetServicesForNamespace(namespace)
	if err != nil {
		return err
	}
	for _, svc := range services {
		for _, port := range svc.Ports {
			oc.flowController.DelServiceOFRules(oldNetID, svc.IP, port.Protocol, port.Port)
			oc.flowController.AddServiceOFRules(netID, svc.IP, port.Protocol, port.Port)
		}
	}
	return nil
}

func (oc *OvsController) watchVnids(ready chan<- bool, start <-chan string) {
	stop := make(chan bool)
	netNsEvent := make(chan *api.NetNamespaceEvent)
	go oc.subnetRegistry.WatchNetNamespaces(netNsEvent, ready, start, stop)
	for {
		select {
		case ev := <-netNsEvent:
			oldNetID, found := oc.VNIDMap[ev.Name]
			if !found {
				log.Errorf("Error fetching Net ID for namespace: %s, skipped netNsEvent: %v", ev.Name, ev)
			}
			switch ev.Type {
			case api.Added:
				// Skip this event if the old and new network ids are same
				if oldNetID == ev.NetID {
					continue
				}
				oc.VNIDMap[ev.Name] = ev.NetID
				err := oc.updatePodNetwork(ev.Name, ev.NetID, oldNetID)
				if err != nil {
					log.Errorf("Failed to update pod network for namespace '%s', error: %s", ev.Name, err)
				}
			case api.Deleted:
				err := oc.updatePodNetwork(ev.Name, AdminVNID, oldNetID)
				if err != nil {
					log.Errorf("Failed to update pod network for namespace '%s', error: %s", ev.Name, err)
				}
				delete(oc.VNIDMap, ev.Name)
			}
		case <-oc.sig:
			log.Error("Signal received. Stopping watching of NetNamespaces.")
			stop <- true
			return
		}
	}
}

func (oc *OvsController) initSelfSubnet() error {
	// timeout: 10 secs
	retries := 20
	retryInterval := 500 * time.Millisecond

	var err error
	var subnet *api.Subnet
	// Try every retryInterval and bail-out if it exceeds max retries
	for i := 0; i < retries; i++ {
		// Get subnet for current node
		subnet, err = oc.subnetRegistry.GetSubnet(oc.hostName)
		if err == nil {
			break
		}
		log.Warningf("Could not find an allocated subnet for node: %s, Waiting...", oc.hostName)
		time.Sleep(retryInterval)
	}
	if err != nil {
		return fmt.Errorf("Failed to get subnet for this host: %s, error: %v", oc.hostName, err)
	}
	oc.localSubnet = subnet
	return nil
}

func (oc *OvsController) watchNodes(ready chan<- bool, start <-chan string) {
	stop := make(chan bool)
	nodeEvent := make(chan *api.NodeEvent)
	go oc.subnetRegistry.WatchNodes(nodeEvent, ready, start, stop)
	for {
		select {
		case ev := <-nodeEvent:
			switch ev.Type {
			case api.Added:
				sub, err := oc.subnetRegistry.GetSubnet(ev.Node.Name)
				if err != nil {
					// subnet does not exist already
					oc.AddNode(ev.Node.Name, ev.Node.IP)
				} else {
					// Current node IP is obtained from event, ev.NodeIP to
					// avoid cached/stale IP lookup by net.LookupIP()
					if sub.NodeIP != ev.Node.IP {
						err = oc.subnetRegistry.DeleteSubnet(ev.Node.Name)
						if err != nil {
							log.Errorf("Error deleting subnet for node %s, old ip %s", ev.Node.Name, sub.NodeIP)
							continue
						}
						sub.NodeIP = ev.Node.IP
						err = oc.subnetRegistry.CreateSubnet(ev.Node.Name, sub)
						if err != nil {
							log.Errorf("Error creating subnet for node %s, ip %s", ev.Node.Name, sub.NodeIP)
							continue
						}
					}
				}
			case api.Deleted:
				oc.DeleteNode(ev.Node.Name)
			}
		case <-oc.sig:
			log.Error("Signal received. Stopping watching of nodes.")
			stop <- true
			return
		}
	}
}

func (oc *OvsController) watchServices(ready chan<- bool, start <-chan string) {
	stop := make(chan bool)
	svcevent := make(chan *api.ServiceEvent)
	go oc.subnetRegistry.WatchServices(svcevent, ready, start, stop)
	for {
		select {
		case ev := <-svcevent:
			netid, found := oc.VNIDMap[ev.Service.Namespace]
			if !found {
				log.Errorf("Error fetching Net ID for namespace: %s, skipped serviceEvent: %v", ev.Service.Namespace, ev)
			}
			switch ev.Type {
			case api.Added:
				oc.services[ev.Service.UID] = ev.Service
				for _, port := range ev.Service.Ports {
					oc.flowController.AddServiceOFRules(netid, ev.Service.IP, port.Protocol, port.Port)
				}
			case api.Deleted:
				delete(oc.services, ev.Service.UID)
				for _, port := range ev.Service.Ports {
					oc.flowController.DelServiceOFRules(netid, ev.Service.IP, port.Protocol, port.Port)
				}
			case api.Modified:
				oldsvc, exists := oc.services[ev.Service.UID]
				if exists && len(oldsvc.Ports) == len(ev.Service.Ports) {
					same := true
					for i := range oldsvc.Ports {
						if oldsvc.Ports[i].Protocol != ev.Service.Ports[i].Protocol || oldsvc.Ports[i].Port != ev.Service.Ports[i].Port {
							same = false
							break
						}
					}
					if same {
						continue
					}
				}
				if exists {
					for _, port := range oldsvc.Ports {
						oc.flowController.DelServiceOFRules(netid, oldsvc.IP, port.Protocol, port.Port)
					}
				}
				oc.services[ev.Service.UID] = ev.Service
				for _, port := range ev.Service.Ports {
					oc.flowController.AddServiceOFRules(netid, ev.Service.IP, port.Protocol, port.Port)
				}
			}
		case <-oc.sig:
			log.Error("Signal received. Stopping watching of services.")
			stop <- true
			return
		}
	}
}

func (oc *OvsController) watchPods(ready chan<- bool, start <-chan string) {
	stop := make(chan bool)
	go oc.subnetRegistry.WatchPods(ready, start, stop)

	<-oc.sig
	log.Error("Signal received. Stopping watching of pods.")
	stop <- true
}

func (oc *OvsController) watchCluster(ready chan<- bool, start <-chan string) {
	stop := make(chan bool)
	clusterEvent := make(chan *api.SubnetEvent)
	go oc.subnetRegistry.WatchSubnets(clusterEvent, ready, start, stop)
	for {
		select {
		case ev := <-clusterEvent:
			switch ev.Type {
			case api.Added:
				// add openflow rules
				oc.flowController.AddOFRules(ev.Subnet.NodeIP, ev.Subnet.SubnetCIDR, oc.localIP)
			case api.Deleted:
				// delete openflow rules meant for the node
				oc.flowController.DelOFRules(ev.Subnet.NodeIP, oc.localIP)
			}
		case <-oc.sig:
			stop <- true
			return
		}
	}
}

func (oc *OvsController) Stop() {
	close(oc.sig)
}

func GetNodeIP(nodeName string) (string, error) {
	ip := net.ParseIP(nodeName)
	if ip == nil {
		addrs, err := net.LookupIP(nodeName)
		if err != nil {
			log.Errorf("Failed to lookup IP address for node %s: %v", nodeName, err)
			return "", err
		}
		for _, addr := range addrs {
			if addr.String() != "127.0.0.1" {
				ip = addr
				break
			}
		}
	}
	if ip == nil || len(ip.String()) == 0 {
		return "", fmt.Errorf("Failed to obtain IP address from node name: %s", nodeName)
	}
	return ip.String(), nil
}

// Wait for ready signal from Watch interface for the given resource
// Closes the ready channel as we don't need it anymore after this point
func waitForWatchReadiness(ready chan bool, resourceName string) {
	timeout := time.Minute
	select {
	case <-ready:
		close(ready)
	case <-time.After(timeout):
		log.Fatalf("Watch for resource %s is not ready(timeout: %v)", resourceName, timeout)
	}
	return
}

// watchAndGetResource will fetch current items in etcd and watch for any new
// changes for the given resource.
// Supported resources: nodes, subnets, namespaces, services, netnamespaces, and pods.
//
// To avoid any potential race conditions during this process, these steps are followed:
// 1. Initiator(master/node): Watch for a resource as an async op, lets say WatchProcess
// 2. WatchProcess: When ready for watching, send ready signal to initiator
// 3. Initiator: Wait for watch resource to be ready
//    This is needed as step-1 is an asynchronous operation
// 4. WatchProcess: Collect new changes in the queue but wait for initiator
//    to indicate which version to start from
// 5. Initiator: Get existing items with their latest version for the resource
// 6. Initiator: Send version from step-5 to WatchProcess
// 7. WatchProcess: Ignore any items with version <= start version got from initiator on step-6
// 8. WatchProcess: Handle new changes
func (oc *OvsController) watchAndGetResource(resourceName string) (interface{}, error) {
	ready := make(chan bool)
	start := make(chan string)

	var getOutput interface{}
	var version string
	var err error

	switch strings.ToLower(resourceName) {
	case "hostsubnet":
		go oc.watchCluster(ready, start)
		waitForWatchReadiness(ready, resourceName)
		getOutput, version, err = oc.subnetRegistry.GetSubnets()
	case "node":
		go oc.watchNodes(ready, start)
		waitForWatchReadiness(ready, resourceName)
		getOutput, version, err = oc.subnetRegistry.GetNodes()
	case "namespace":
		go oc.watchNetworks(ready, start)
		waitForWatchReadiness(ready, resourceName)
		getOutput, version, err = oc.subnetRegistry.GetNamespaces()
	case "netnamespace":
		go oc.watchVnids(ready, start)
		waitForWatchReadiness(ready, resourceName)
		getOutput, version, err = oc.subnetRegistry.GetNetNamespaces()
	case "service":
		go oc.watchServices(ready, start)
		waitForWatchReadiness(ready, resourceName)
		getOutput, version, err = oc.subnetRegistry.GetServices()
	case "pod":
		go oc.watchPods(ready, start)
		waitForWatchReadiness(ready, resourceName)
		getOutput, version, err = oc.subnetRegistry.GetPods()
	default:
		log.Fatalf("Unknown resource %s for watch and get resource", resourceName)
	}
	if err != nil {
		return nil, err
	}

	start <- version

	return getOutput, nil
}

type FirewallRule struct {
	table string
	chain string
	args  []string
}

func SetupIptables(ipt iptables.Interface, clusterNetworkCIDR string) error {
	rules := []FirewallRule{
		{"nat", "POSTROUTING", []string{"-s", clusterNetworkCIDR, "!", "-d", clusterNetworkCIDR, "-j", "MASQUERADE"}},
		{"filter", "INPUT", []string{"-p", "udp", "-m", "multiport", "--dports", "4789", "-m", "comment", "--comment", "001 vxlan incoming", "-j", "ACCEPT"}},
		{"filter", "INPUT", []string{"-i", "tun0", "-m", "comment", "--comment", "traffic from docker for internet", "-j", "ACCEPT"}},
		{"filter", "FORWARD", []string{"-d", clusterNetworkCIDR, "-j", "ACCEPT"}},
		{"filter", "FORWARD", []string{"-s", clusterNetworkCIDR, "-j", "ACCEPT"}},
	}

	for _, rule := range rules {
		_, err := ipt.EnsureRule(iptables.Prepend, iptables.Table(rule.table), iptables.Chain(rule.chain), rule.args...)
		if err != nil {
			return err
		}
	}

	return nil
}
