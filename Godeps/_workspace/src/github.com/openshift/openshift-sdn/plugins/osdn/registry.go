package osdn

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	log "github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	pconfig "k8s.io/kubernetes/pkg/proxy/config"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/openshift-sdn/pkg/netutils"

	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	osapi "github.com/openshift/origin/pkg/sdn/api"
)

type Registry struct {
	oClient          osclient.Interface
	kClient          kclient.Interface
	namespaceOfPodIP map[string]string
	serviceNetwork   *net.IPNet
	clusterNetwork   *net.IPNet
	hostSubnetLength int

	// These are only set if SetBaseEndpointsHandler() has been called
	baseEndpointsHandler pconfig.EndpointsConfigHandler
}

type EventType string

const (
	Added    EventType = "ADDED"
	Deleted  EventType = "DELETED"
	Modified EventType = "MODIFIED"
)

type HostSubnetEvent struct {
	Type       EventType
	HostSubnet *osapi.HostSubnet
}

type NodeEvent struct {
	Type EventType
	Node *kapi.Node
}

type NetNamespaceEvent struct {
	Type         EventType
	NetNamespace *osapi.NetNamespace
}

type NamespaceEvent struct {
	Type      EventType
	Namespace *kapi.Namespace
}

type ServiceEvent struct {
	Type    EventType
	Service *kapi.Service
}

func NewRegistry(osClient *osclient.Client, kClient *kclient.Client) *Registry {
	return &Registry{
		oClient:          osClient,
		kClient:          kClient,
		namespaceOfPodIP: make(map[string]string),
	}
}

func (registry *Registry) GetSubnets() ([]osapi.HostSubnet, string, error) {
	hostSubnetList, err := registry.oClient.HostSubnets().List(kapi.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	return hostSubnetList.Items, hostSubnetList.ListMeta.ResourceVersion, nil
}

func (registry *Registry) GetSubnet(nodeName string) (*osapi.HostSubnet, error) {
	return registry.oClient.HostSubnets().Get(nodeName)
}

func (registry *Registry) DeleteSubnet(nodeName string) error {
	return registry.oClient.HostSubnets().Delete(nodeName)
}

func (registry *Registry) CreateSubnet(nodeName, nodeIP, subnetCIDR string) error {
	hs := &osapi.HostSubnet{
		TypeMeta:   unversioned.TypeMeta{Kind: "HostSubnet"},
		ObjectMeta: kapi.ObjectMeta{Name: nodeName},
		Host:       nodeName,
		HostIP:     nodeIP,
		Subnet:     subnetCIDR,
	}
	_, err := registry.oClient.HostSubnets().Create(hs)
	return err
}

func (registry *Registry) WatchSubnets(receiver chan<- *HostSubnetEvent, ready chan<- bool, start <-chan string, stop <-chan bool) error {
	eventQueue, startVersion := registry.createAndRunEventQueue("HostSubnet", ready, start)

	checkCondition := true
	for {
		eventType, obj, err := getEvent(eventQueue, startVersion, &checkCondition)
		if err != nil {
			return err
		}
		hs := obj.(*osapi.HostSubnet)

		switch eventType {
		case watch.Added, watch.Modified:
			receiver <- &HostSubnetEvent{Type: Added, HostSubnet: hs}
		case watch.Deleted:
			receiver <- &HostSubnetEvent{Type: Deleted, HostSubnet: hs}
		}
	}
}

func (registry *Registry) GetPods() ([]kapi.Pod, string, error) {
	podList, err := registry.kClient.Pods(kapi.NamespaceAll).List(kapi.ListOptions{})
	if err != nil {
		return nil, "", err
	}

	for _, pod := range podList.Items {
		if pod.Status.PodIP != "" {
			registry.namespaceOfPodIP[pod.Status.PodIP] = pod.ObjectMeta.Namespace
		}
	}
	return podList.Items, podList.ListMeta.ResourceVersion, nil
}

func (registry *Registry) WatchPods(ready chan<- bool, start <-chan string, stop <-chan bool) error {
	eventQueue, startVersion := registry.createAndRunEventQueue("Pod", ready, start)

	checkCondition := true
	for {
		eventType, obj, err := getEvent(eventQueue, startVersion, &checkCondition)
		if err != nil {
			return err
		}
		pod := obj.(*kapi.Pod)

		switch eventType {
		case watch.Added, watch.Modified:
			registry.namespaceOfPodIP[pod.Status.PodIP] = pod.ObjectMeta.Namespace
		case watch.Deleted:
			delete(registry.namespaceOfPodIP, pod.Status.PodIP)
		}
	}
}

func (registry *Registry) GetRunningPods(nodeName, namespace string) ([]kapi.Pod, error) {
	fieldSelector := fields.Set{"spec.host": nodeName}.AsSelector()
	opts := kapi.ListOptions{
		LabelSelector: labels.Everything(),
		FieldSelector: fieldSelector,
	}
	podList, err := registry.kClient.Pods(namespace).List(opts)
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

func (registry *Registry) GetPod(nodeName, namespace, podName string) (*kapi.Pod, error) {
	fieldSelector := fields.Set{"spec.host": nodeName}.AsSelector()
	opts := kapi.ListOptions{
		LabelSelector: labels.Everything(),
		FieldSelector: fieldSelector,
	}
	podList, err := registry.kClient.Pods(namespace).List(opts)
	if err != nil {
		return nil, err
	}

	for _, pod := range podList.Items {
		if pod.ObjectMeta.Name == podName {
			return &pod, nil
		}
	}
	return nil, nil
}

func (registry *Registry) GetNodes() ([]kapi.Node, string, error) {
	nodes, err := registry.kClient.Nodes().List(kapi.ListOptions{})
	if err != nil {
		return nil, "", err
	}

	return nodes.Items, nodes.ListMeta.ResourceVersion, nil
}

func (registry *Registry) getNodeAddressMap() (map[types.UID]string, error) {
	nodeAddressMap := map[types.UID]string{}

	nodes, err := registry.kClient.Nodes().List(kapi.ListOptions{})
	if err != nil {
		return nodeAddressMap, err
	}
	for _, node := range nodes.Items {
		if len(node.Status.Addresses) > 0 {
			nodeAddressMap[node.ObjectMeta.UID] = node.Status.Addresses[0].Address
		}
	}
	return nodeAddressMap, nil
}

func (registry *Registry) WatchNodes(receiver chan<- *NodeEvent, ready chan<- bool, start <-chan string, stop <-chan bool) error {
	eventQueue, startVersion := registry.createAndRunEventQueue("Node", ready, start)

	nodeAddressMap, err := registry.getNodeAddressMap()
	if err != nil {
		return err
	}

	checkCondition := true
	for {
		eventType, obj, err := getEvent(eventQueue, startVersion, &checkCondition)
		if err != nil {
			return err
		}
		node := obj.(*kapi.Node)

		nodeIP := ""
		if len(node.Status.Addresses) > 0 {
			nodeIP = node.Status.Addresses[0].Address
		} else {
			nodeIP, err = netutils.GetNodeIP(node.ObjectMeta.Name)
			if err != nil {
				return err
			}
		}

		switch eventType {
		case watch.Added:
			receiver <- &NodeEvent{Type: Added, Node: node}
			nodeAddressMap[node.ObjectMeta.UID] = nodeIP
		case watch.Modified:
			oldNodeIP, ok := nodeAddressMap[node.ObjectMeta.UID]
			if ok && oldNodeIP != nodeIP {
				// Node Added event will handle update subnet if there is ip mismatch
				receiver <- &NodeEvent{Type: Added, Node: node}
				nodeAddressMap[node.ObjectMeta.UID] = nodeIP
			}
		case watch.Deleted:
			receiver <- &NodeEvent{Type: Deleted, Node: node}
			delete(nodeAddressMap, node.ObjectMeta.UID)
		}
	}
}

func (registry *Registry) UpdateClusterNetwork(clusterNetwork *net.IPNet, subnetLength int, serviceNetwork *net.IPNet) error {
	cn, err := registry.oClient.ClusterNetwork().Get("default")
	if err != nil {
		return err
	}
	cn.Network = clusterNetwork.String()
	cn.HostSubnetLength = subnetLength
	cn.ServiceNetwork = serviceNetwork.String()
	_, err = registry.oClient.ClusterNetwork().Update(cn)
	return err
}

func (registry *Registry) CreateClusterNetwork(clusterNetwork *net.IPNet, subnetLength int, serviceNetwork *net.IPNet) error {
	cn := &osapi.ClusterNetwork{
		TypeMeta:         unversioned.TypeMeta{Kind: "ClusterNetwork"},
		ObjectMeta:       kapi.ObjectMeta{Name: "default"},
		Network:          clusterNetwork.String(),
		HostSubnetLength: subnetLength,
		ServiceNetwork:   serviceNetwork.String(),
	}
	_, err := registry.oClient.ClusterNetwork().Create(cn)
	return err
}

func ValidateClusterNetwork(network string, hostSubnetLength int, serviceNetwork string) (*net.IPNet, int, *net.IPNet, error) {
	_, cn, err := net.ParseCIDR(network)
	if err != nil {
		return nil, -1, nil, fmt.Errorf("Failed to parse ClusterNetwork CIDR %s: %v", network, err)
	}

	_, sn, err := net.ParseCIDR(serviceNetwork)
	if err != nil {
		return nil, -1, nil, fmt.Errorf("Failed to parse ServiceNetwork CIDR %s: %v", serviceNetwork, err)
	}

	if hostSubnetLength <= 0 || hostSubnetLength > 32 {
		return nil, -1, nil, fmt.Errorf("Invalid HostSubnetLength %d (not between 1 and 32)", hostSubnetLength)
	}
	return cn, hostSubnetLength, sn, nil
}

func (registry *Registry) cacheClusterNetwork() error {
	// don't hit up the master if we have the values already
	if registry.clusterNetwork != nil && registry.serviceNetwork != nil {
		return nil
	}

	cn, err := registry.oClient.ClusterNetwork().Get("default")
	if err != nil {
		return err
	}

	registry.clusterNetwork, registry.hostSubnetLength, registry.serviceNetwork, err = ValidateClusterNetwork(cn.Network, cn.HostSubnetLength, cn.ServiceNetwork)

	return err
}

func (registry *Registry) GetNetworkInfo() (*net.IPNet, int, *net.IPNet, error) {
	if err := registry.cacheClusterNetwork(); err != nil {
		return nil, -1, nil, err
	}
	return registry.clusterNetwork, registry.hostSubnetLength, registry.serviceNetwork, nil
}

func (registry *Registry) GetClusterNetwork() (*net.IPNet, error) {
	if err := registry.cacheClusterNetwork(); err != nil {
		return nil, err
	}
	return registry.clusterNetwork, nil
}

func (registry *Registry) GetHostSubnetLength() (int, error) {
	if err := registry.cacheClusterNetwork(); err != nil {
		return -1, err
	}
	return registry.hostSubnetLength, nil
}

func (registry *Registry) GetServicesNetwork() (*net.IPNet, error) {
	if err := registry.cacheClusterNetwork(); err != nil {
		return nil, err
	}
	return registry.serviceNetwork, nil
}

func (registry *Registry) GetNamespaces() ([]kapi.Namespace, string, error) {
	namespaceList, err := registry.kClient.Namespaces().List(kapi.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	return namespaceList.Items, namespaceList.ListMeta.ResourceVersion, nil
}

func (registry *Registry) WatchNamespaces(receiver chan<- *NamespaceEvent, ready chan<- bool, start <-chan string, stop <-chan bool) error {
	eventQueue, startVersion := registry.createAndRunEventQueue("Namespace", ready, start)

	checkCondition := true
	for {
		eventType, obj, err := getEvent(eventQueue, startVersion, &checkCondition)
		if err != nil {
			return err
		}
		ns := obj.(*kapi.Namespace)

		switch eventType {
		case watch.Added:
			receiver <- &NamespaceEvent{Type: Added, Namespace: ns}
		case watch.Deleted:
			receiver <- &NamespaceEvent{Type: Deleted, Namespace: ns}
		case watch.Modified:
			// Ignore, we don't need to update SDN in case of namespace updates
		}
	}
}

func (registry *Registry) WatchNetNamespaces(receiver chan<- *NetNamespaceEvent, ready chan<- bool, start <-chan string, stop <-chan bool) error {
	eventQueue, startVersion := registry.createAndRunEventQueue("NetNamespace", ready, start)

	checkCondition := true
	for {
		eventType, obj, err := getEvent(eventQueue, startVersion, &checkCondition)
		if err != nil {
			return err
		}
		netns := obj.(*osapi.NetNamespace)

		switch eventType {
		case watch.Added, watch.Modified:
			receiver <- &NetNamespaceEvent{Type: Added, NetNamespace: netns}
		case watch.Deleted:
			receiver <- &NetNamespaceEvent{Type: Deleted, NetNamespace: netns}
		}
	}
}

func (registry *Registry) GetNetNamespaces() ([]osapi.NetNamespace, string, error) {
	netNamespaceList, err := registry.oClient.NetNamespaces().List(kapi.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	return netNamespaceList.Items, netNamespaceList.ListMeta.ResourceVersion, nil
}

func (registry *Registry) GetNetNamespace(name string) (*osapi.NetNamespace, error) {
	return registry.oClient.NetNamespaces().Get(name)
}

func (registry *Registry) WriteNetNamespace(name string, id uint) error {
	netns := &osapi.NetNamespace{
		TypeMeta:   unversioned.TypeMeta{Kind: "NetNamespace"},
		ObjectMeta: kapi.ObjectMeta{Name: name},
		NetName:    name,
		NetID:      id,
	}
	_, err := registry.oClient.NetNamespaces().Create(netns)
	return err
}

func (registry *Registry) DeleteNetNamespace(name string) error {
	return registry.oClient.NetNamespaces().Delete(name)
}

func (registry *Registry) GetServicesForNamespace(namespace string) ([]kapi.Service, error) {
	services, _, err := registry.getServices(namespace)
	return services, err
}

func (registry *Registry) GetServices() ([]kapi.Service, string, error) {
	return registry.getServices(kapi.NamespaceAll)
}

func (registry *Registry) getServices(namespace string) ([]kapi.Service, string, error) {
	kServList, err := registry.kClient.Services(namespace).List(kapi.ListOptions{})
	if err != nil {
		return nil, "", err
	}

	servList := make([]kapi.Service, 0, len(kServList.Items))
	for _, service := range kServList.Items {
		if !kapi.IsServiceIPSet(&service) {
			continue
		}
		servList = append(servList, service)
	}
	return servList, kServList.ListMeta.ResourceVersion, nil
}

func (registry *Registry) WatchServices(receiver chan<- *ServiceEvent, ready chan<- bool, start <-chan string, stop <-chan bool) error {
	eventQueue, startVersion := registry.createAndRunEventQueue("Service", ready, start)

	checkCondition := true
	for {
		eventType, obj, err := getEvent(eventQueue, startVersion, &checkCondition)
		if err != nil {
			return err
		}
		serv := obj.(*kapi.Service)

		// Ignore headless services
		if !kapi.IsServiceIPSet(serv) {
			continue
		}

		switch eventType {
		case watch.Added:
			receiver <- &ServiceEvent{Type: Added, Service: serv}
		case watch.Deleted:
			receiver <- &ServiceEvent{Type: Deleted, Service: serv}
		case watch.Modified:
			receiver <- &ServiceEvent{Type: Modified, Service: serv}
		}
	}
}

// Run event queue for the given resource
func (registry *Registry) runEventQueue(resourceName string) (*oscache.EventQueue, *cache.Reflector) {
	eventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	lw := &cache.ListWatch{}
	var expectedType interface{}
	switch strings.ToLower(resourceName) {
	case "hostsubnet":
		expectedType = &osapi.HostSubnet{}
		lw.ListFunc = func(options kapi.ListOptions) (runtime.Object, error) {
			return registry.oClient.HostSubnets().List(options)
		}
		lw.WatchFunc = func(options kapi.ListOptions) (watch.Interface, error) {
			return registry.oClient.HostSubnets().Watch(options)
		}
	case "node":
		expectedType = &kapi.Node{}
		lw.ListFunc = func(options kapi.ListOptions) (runtime.Object, error) {
			return registry.kClient.Nodes().List(options)
		}
		lw.WatchFunc = func(options kapi.ListOptions) (watch.Interface, error) {
			return registry.kClient.Nodes().Watch(options)
		}
	case "namespace":
		expectedType = &kapi.Namespace{}
		lw.ListFunc = func(options kapi.ListOptions) (runtime.Object, error) {
			return registry.kClient.Namespaces().List(options)
		}
		lw.WatchFunc = func(options kapi.ListOptions) (watch.Interface, error) {
			return registry.kClient.Namespaces().Watch(options)
		}
	case "netnamespace":
		expectedType = &osapi.NetNamespace{}
		lw.ListFunc = func(options kapi.ListOptions) (runtime.Object, error) {
			return registry.oClient.NetNamespaces().List(options)
		}
		lw.WatchFunc = func(options kapi.ListOptions) (watch.Interface, error) {
			return registry.oClient.NetNamespaces().Watch(options)
		}
	case "service":
		expectedType = &kapi.Service{}
		lw.ListFunc = func(options kapi.ListOptions) (runtime.Object, error) {
			return registry.kClient.Services(kapi.NamespaceAll).List(options)
		}
		lw.WatchFunc = func(options kapi.ListOptions) (watch.Interface, error) {
			return registry.kClient.Services(kapi.NamespaceAll).Watch(options)
		}
	case "pod":
		expectedType = &kapi.Pod{}
		lw.ListFunc = func(options kapi.ListOptions) (runtime.Object, error) {
			return registry.kClient.Pods(kapi.NamespaceAll).List(options)
		}
		lw.WatchFunc = func(options kapi.ListOptions) (watch.Interface, error) {
			return registry.kClient.Pods(kapi.NamespaceAll).Watch(options)
		}
	default:
		log.Fatalf("Unknown resource %s during initialization of event queue", resourceName)
	}
	reflector := cache.NewReflector(lw, expectedType, eventQueue, 4*time.Minute)
	reflector.Run()
	return eventQueue, reflector
}

// Ensures given event queue is ready for watching new changes
// and unblock other end of the ready channel
func sendWatchReadiness(reflector *cache.Reflector, ready chan<- bool) {
	// timeout: 1min
	retries := 120
	retryInterval := 500 * time.Millisecond
	// Try every retryInterval and bail-out if it exceeds max retries
	for i := 0; i < retries; i++ {
		// Reflector does list and watch of the resource
		// when listing of the resource is done, resourceVersion will be populated
		// and the event queue will be ready to watch any new changes
		version := reflector.LastSyncResourceVersion()
		if len(version) > 0 {
			ready <- true
			return
		}
		time.Sleep(retryInterval)
	}
	log.Fatalf("SDN event queue is not ready for watching new changes(timeout: 1min)")
}

// Get resource version from start channel
// Watch interface for the resource will process any item after this version
func getStartVersion(start <-chan string, resourceName string) uint64 {
	var version uint64
	var err error

	timeout := time.Minute
	select {
	case rv := <-start:
		version, err = strconv.ParseUint(rv, 10, 64)
		if err != nil {
			log.Fatalf("Invalid start version %s for %s, error: %v", rv, resourceName, err)
		}
	case <-time.After(timeout):
		log.Fatalf("Error fetching resource version for %s (timeout: %v)", resourceName, timeout)
	}
	return version
}

// createAndRunEventQueue will create and run event queue and also returns start version for watching any new changes
func (registry *Registry) createAndRunEventQueue(resourceName string, ready chan<- bool, start <-chan string) (*oscache.EventQueue, uint64) {
	eventQueue, reflector := registry.runEventQueue(resourceName)
	sendWatchReadiness(reflector, ready)
	startVersion := getStartVersion(start, resourceName)
	return eventQueue, startVersion
}

// getEvent returns next item in the event queue which satisfies item version greater than given start version
// checkCondition is an optimization that ignores version check when it is not needed
func getEvent(eventQueue *oscache.EventQueue, startVersion uint64, checkCondition *bool) (watch.EventType, interface{}, error) {
	if *checkCondition {
		// Ignore all events with version <= given start version
		for {
			eventType, obj, err := eventQueue.Pop()
			if err != nil {
				return watch.Error, nil, err
			}
			accessor, err := meta.Accessor(obj)
			if err != nil {
				return watch.Error, nil, err
			}
			currentVersion, err := strconv.ParseUint(accessor.GetResourceVersion(), 10, 64)
			if err != nil {
				return watch.Error, nil, err
			}
			if currentVersion <= startVersion {
				log.V(5).Infof("Ignoring %s with version %d, start version: %d", accessor.GetName(), currentVersion, startVersion)
				continue
			}
			*checkCondition = false
			return eventType, obj, nil
		}
	} else {
		return eventQueue.Pop()
	}
}

// FilteringEndpointsConfigHandler implementation
func (registry *Registry) SetBaseEndpointsHandler(base pconfig.EndpointsConfigHandler) {
	registry.baseEndpointsHandler = base
}

func (registry *Registry) OnEndpointsUpdate(allEndpoints []kapi.Endpoints) {
	clusterNetwork, _, serviceNetwork, err := registry.GetNetworkInfo()
	if err != nil {
		log.Warningf("Error fetching cluster network: %v", err)
		return
	}

	filteredEndpoints := make([]kapi.Endpoints, 0, len(allEndpoints))
EndpointLoop:
	for _, ep := range allEndpoints {
		ns := ep.ObjectMeta.Namespace
		for _, ss := range ep.Subsets {
			for _, addr := range ss.Addresses {
				IP := net.ParseIP(addr.IP)
				if serviceNetwork.Contains(IP) {
					log.Warningf("Service '%s' in namespace '%s' has an Endpoint inside the service network (%s)", ep.ObjectMeta.Name, ns, addr.IP)
					continue EndpointLoop
				}
				if clusterNetwork.Contains(IP) {
					podNamespace, ok := registry.namespaceOfPodIP[addr.IP]
					if !ok {
						log.Warningf("Service '%s' in namespace '%s' has an Endpoint pointing to non-existent pod (%s)", ep.ObjectMeta.Name, ns, addr.IP)
						continue EndpointLoop
					}
					if podNamespace != ns {
						log.Warningf("Service '%s' in namespace '%s' has an Endpoint pointing to pod %s in namespace '%s'", ep.ObjectMeta.Name, ns, addr.IP, podNamespace)
						continue EndpointLoop
					}
				}
			}
		}
		filteredEndpoints = append(filteredEndpoints, ep)
	}

	registry.baseEndpointsHandler.OnEndpointsUpdate(filteredEndpoints)
}
