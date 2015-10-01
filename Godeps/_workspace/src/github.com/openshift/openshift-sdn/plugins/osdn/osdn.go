package osdn

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/watch"

	osdn "github.com/openshift/openshift-sdn/pkg/ovssubnet"
	osdnapi "github.com/openshift/openshift-sdn/pkg/ovssubnet/api"

	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	"github.com/openshift/origin/pkg/sdn/api"
)

type OsdnRegistryInterface struct {
	oClient osclient.Interface
	kClient kclient.Interface
}

func NewOsdnRegistryInterface(osClient *osclient.Client, kClient *kclient.Client) OsdnRegistryInterface {
	return OsdnRegistryInterface{osClient, kClient}
}

func (oi *OsdnRegistryInterface) InitSubnets() error {
	return nil
}

func (oi *OsdnRegistryInterface) GetSubnets() ([]osdnapi.Subnet, string, error) {
	hostSubnetList, err := oi.oClient.HostSubnets().List()
	if err != nil {
		return nil, "", err
	}
	// convert HostSubnet to osdnapi.Subnet
	subList := make([]osdnapi.Subnet, 0, len(hostSubnetList.Items))
	for _, subnet := range hostSubnetList.Items {
		subList = append(subList, osdnapi.Subnet{NodeIP: subnet.HostIP, SubnetCIDR: subnet.Subnet})
	}
	return subList, hostSubnetList.ListMeta.ResourceVersion, nil
}

func (oi *OsdnRegistryInterface) GetSubnet(nodeName string) (*osdnapi.Subnet, error) {
	hs, err := oi.oClient.HostSubnets().Get(nodeName)
	if err != nil {
		return nil, err
	}
	return &osdnapi.Subnet{NodeIP: hs.HostIP, SubnetCIDR: hs.Subnet}, nil
}

func (oi *OsdnRegistryInterface) DeleteSubnet(nodeName string) error {
	return oi.oClient.HostSubnets().Delete(nodeName)
}

func (oi *OsdnRegistryInterface) CreateSubnet(nodeName string, sub *osdnapi.Subnet) error {
	hs := &api.HostSubnet{
		TypeMeta:   kapi.TypeMeta{Kind: "HostSubnet"},
		ObjectMeta: kapi.ObjectMeta{Name: nodeName},
		Host:       nodeName,
		HostIP:     sub.NodeIP,
		Subnet:     sub.SubnetCIDR,
	}
	_, err := oi.oClient.HostSubnets().Create(hs)
	return err
}

func (oi *OsdnRegistryInterface) WatchSubnets(receiver chan<- *osdnapi.SubnetEvent, ready chan<- bool, start <-chan string, stop <-chan bool) error {
	eventQueue, startVersion := oi.createAndRunEventQueue("HostSubnet", nil, ready, start)

	checkCondition := true
	for {
		eventType, obj, err := getEvent(eventQueue, startVersion, &checkCondition)
		if err != nil {
			return err
		}
		hs := obj.(*api.HostSubnet)

		switch eventType {
		case watch.Added, watch.Modified:
			receiver <- &osdnapi.SubnetEvent{Type: osdnapi.Added, NodeName: hs.Host, Subnet: osdnapi.Subnet{NodeIP: hs.HostIP, SubnetCIDR: hs.Subnet}}
		case watch.Deleted:
			receiver <- &osdnapi.SubnetEvent{Type: osdnapi.Deleted, NodeName: hs.Host, Subnet: osdnapi.Subnet{NodeIP: hs.HostIP, SubnetCIDR: hs.Subnet}}
		}
	}
}

func (oi *OsdnRegistryInterface) InitNodes() error {
	// return no error, as this gets initialized by apiserver
	return nil
}

func (oi *OsdnRegistryInterface) GetNodes() ([]osdnapi.Node, string, error) {
	knodes, err := oi.kClient.Nodes().List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, "", err
	}

	nodes := make([]osdnapi.Node, 0, len(knodes.Items))
	for _, node := range knodes.Items {
		var nodeIP string
		if len(node.Status.Addresses) > 0 {
			nodeIP = node.Status.Addresses[0].Address
		} else {
			var err error
			nodeIP, err = osdn.GetNodeIP(node.ObjectMeta.Name)
			if err != nil {
				return nil, "", err
			}
		}
		nodes = append(nodes, osdnapi.Node{Name: node.ObjectMeta.Name, IP: nodeIP})
	}
	return nodes, knodes.ListMeta.ResourceVersion, nil
}

func (oi *OsdnRegistryInterface) CreateNode(nodeName string, data string) error {
	return fmt.Errorf("Feature not supported in native mode. SDN cannot create/register nodes.")
}

func (oi *OsdnRegistryInterface) getNodeAddressMap() (map[types.UID]string, error) {
	nodeAddressMap := map[types.UID]string{}

	nodes, err := oi.kClient.Nodes().List(labels.Everything(), fields.Everything())
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

func (oi *OsdnRegistryInterface) WatchNodes(receiver chan<- *osdnapi.NodeEvent, ready chan<- bool, start <-chan string, stop <-chan bool) error {
	eventQueue, startVersion := oi.createAndRunEventQueue("Node", nil, ready, start)

	nodeAddressMap, err := oi.getNodeAddressMap()
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
			nodeIP, err = osdn.GetNodeIP(node.ObjectMeta.Name)
			if err != nil {
				return err
			}
		}

		switch eventType {
		case watch.Added:
			receiver <- &osdnapi.NodeEvent{Type: osdnapi.Added, Node: osdnapi.Node{Name: node.ObjectMeta.Name, IP: nodeIP}}
			nodeAddressMap[node.ObjectMeta.UID] = nodeIP
		case watch.Modified:
			oldNodeIP, ok := nodeAddressMap[node.ObjectMeta.UID]
			if ok && oldNodeIP != nodeIP {
				// Node Added event will handle update subnet if there is ip mismatch
				receiver <- &osdnapi.NodeEvent{Type: osdnapi.Added, Node: osdnapi.Node{Name: node.ObjectMeta.Name, IP: nodeIP}}
				nodeAddressMap[node.ObjectMeta.UID] = nodeIP
			}
		case watch.Deleted:
			receiver <- &osdnapi.NodeEvent{Type: osdnapi.Deleted, Node: osdnapi.Node{Name: node.ObjectMeta.Name}}
			delete(nodeAddressMap, node.ObjectMeta.UID)
		}
	}
}

func (oi *OsdnRegistryInterface) WriteNetworkConfig(network string, subnetLength uint, serviceNetwork string) error {
	cn, err := oi.oClient.ClusterNetwork().Get("default")
	if err == nil {
		if cn.Network == network && cn.HostSubnetLength == int(subnetLength) && cn.ServiceNetwork == serviceNetwork {
			return nil
		} else if cn.Network == network && cn.HostSubnetLength == int(subnetLength) && cn.ServiceNetwork == "" {
			// Upgrade from 3.0.0
			cn.ServiceNetwork = serviceNetwork
			_, err = oi.oClient.ClusterNetwork().Update(cn)
			return err
		} else {
			return fmt.Errorf("A network already exists and does not match the new network's parameters - Existing: (%s, %d, %s); New: (%s, %d, %s) ", cn.Network, cn.HostSubnetLength, cn.ServiceNetwork, network, subnetLength, serviceNetwork)
		}
	}
	cn = &api.ClusterNetwork{
		TypeMeta:         kapi.TypeMeta{Kind: "ClusterNetwork"},
		ObjectMeta:       kapi.ObjectMeta{Name: "default"},
		Network:          network,
		HostSubnetLength: int(subnetLength),
		ServiceNetwork:   serviceNetwork,
	}
	_, err = oi.oClient.ClusterNetwork().Create(cn)
	return err
}

func (oi *OsdnRegistryInterface) GetClusterNetworkCIDR() (string, error) {
	cn, err := oi.oClient.ClusterNetwork().Get("default")
	return cn.Network, err
}

func (oi *OsdnRegistryInterface) GetServicesNetworkCIDR() (string, error) {
	cn, err := oi.oClient.ClusterNetwork().Get("default")
	return cn.ServiceNetwork, err
}

func (oi *OsdnRegistryInterface) CheckEtcdIsAlive(seconds uint64) bool {
	// always assumed to be true as we run through the apiserver
	return true
}

func (oi *OsdnRegistryInterface) GetNamespaces() ([]string, string, error) {
	namespaceList, err := oi.kClient.Namespaces().List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, "", err
	}
	namespaces := make([]string, 0, len(namespaceList.Items))
	for _, ns := range namespaceList.Items {
		namespaces = append(namespaces, ns.Name)
	}
	return namespaces, namespaceList.ListMeta.ResourceVersion, nil
}

func (oi *OsdnRegistryInterface) WatchNamespaces(receiver chan<- *osdnapi.NamespaceEvent, ready chan<- bool, start <-chan string, stop <-chan bool) error {
	eventQueue, startVersion := oi.createAndRunEventQueue("Namespace", nil, ready, start)

	checkCondition := true
	for {
		eventType, obj, err := getEvent(eventQueue, startVersion, &checkCondition)
		if err != nil {
			return err
		}
		ns := obj.(*kapi.Namespace)

		switch eventType {
		case watch.Added:
			receiver <- &osdnapi.NamespaceEvent{Type: osdnapi.Added, Name: ns.ObjectMeta.Name}
		case watch.Deleted:
			receiver <- &osdnapi.NamespaceEvent{Type: osdnapi.Deleted, Name: ns.ObjectMeta.Name}
		case watch.Modified:
			// Ignore, we don't need to update SDN in case of namespace updates
		}
	}
}

func (oi *OsdnRegistryInterface) WatchNetNamespaces(receiver chan<- *osdnapi.NetNamespaceEvent, ready chan<- bool, start <-chan string, stop <-chan bool) error {
	eventQueue, startVersion := oi.createAndRunEventQueue("NetNamespace", nil, ready, start)

	checkCondition := true
	for {
		eventType, obj, err := getEvent(eventQueue, startVersion, &checkCondition)
		if err != nil {
			return err
		}
		netns := obj.(*api.NetNamespace)

		switch eventType {
		case watch.Added:
			receiver <- &osdnapi.NetNamespaceEvent{Type: osdnapi.Added, Name: netns.NetName, NetID: netns.NetID}
		case watch.Deleted:
			receiver <- &osdnapi.NetNamespaceEvent{Type: osdnapi.Deleted, Name: netns.NetName}
		case watch.Modified:
			// Ignore, we don't need to update SDN in case of network namespace updates
		}
	}
}

func (oi *OsdnRegistryInterface) GetNetNamespaces() ([]osdnapi.NetNamespace, string, error) {
	netNamespaceList, err := oi.oClient.NetNamespaces().List()
	if err != nil {
		return nil, "", err
	}
	// convert api.NetNamespace to osdnapi.NetNamespace
	nsList := make([]osdnapi.NetNamespace, 0, len(netNamespaceList.Items))
	for _, netns := range netNamespaceList.Items {
		nsList = append(nsList, osdnapi.NetNamespace{Name: netns.Name, NetID: netns.NetID})
	}
	return nsList, netNamespaceList.ListMeta.ResourceVersion, nil
}

func (oi *OsdnRegistryInterface) GetNetNamespace(name string) (osdnapi.NetNamespace, error) {
	netns, err := oi.oClient.NetNamespaces().Get(name)
	if err != nil {
		return osdnapi.NetNamespace{}, err
	}
	return osdnapi.NetNamespace{Name: netns.Name, NetID: netns.NetID}, nil
}

func (oi *OsdnRegistryInterface) WriteNetNamespace(name string, id uint) error {
	netns := &api.NetNamespace{
		TypeMeta:   kapi.TypeMeta{Kind: "NetNamespace"},
		ObjectMeta: kapi.ObjectMeta{Name: name},
		NetName:    name,
		NetID:      id,
	}
	_, err := oi.oClient.NetNamespaces().Create(netns)
	return err
}

func (oi *OsdnRegistryInterface) DeleteNetNamespace(name string) error {
	return oi.oClient.NetNamespaces().Delete(name)
}

func (oi *OsdnRegistryInterface) InitServices() error {
	return nil
}

func (oi *OsdnRegistryInterface) GetServices() ([]osdnapi.Service, string, error) {
	kNsList, err := oi.kClient.Namespaces().List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, "", err
	}
	oServList := make([]osdnapi.Service, 0)
	for _, ns := range kNsList.Items {
		kServList, err := oi.kClient.Services(ns.Name).List(labels.Everything())
		if err != nil {
			return nil, "", err
		}

		// convert kube ServiceList into []osdnapi.Service
		for _, kService := range kServList.Items {
			if kService.Spec.ClusterIP == "None" {
				continue
			}
			for _, port := range kService.Spec.Ports {
				oServList = append(oServList, newSDNService(&kService, ns.Name, port))
			}
		}
	}
	return oServList, kNsList.ListMeta.ResourceVersion, nil
}

func (oi *OsdnRegistryInterface) WatchServices(receiver chan<- *osdnapi.ServiceEvent, ready chan<- bool, start <-chan string, stop <-chan bool) error {
	// watch for namespaces, and launch a go func for each namespace that is new
	// kill the watch for each namespace that is deleted
	nsevent := make(chan *osdnapi.NamespaceEvent)
	namespaceTable := make(map[string]chan bool)
	go oi.WatchNamespaces(nsevent, ready, start, stop)
	for {
		select {
		case ev := <-nsevent:
			switch ev.Type {
			case osdnapi.Added:
				stopChannel := make(chan bool)
				namespaceTable[ev.Name] = stopChannel
				go oi.watchServicesForNamespace(ev.Name, receiver, stopChannel)
			case osdnapi.Deleted:
				stopChannel, ok := namespaceTable[ev.Name]
				if ok {
					close(stopChannel)
					delete(namespaceTable, ev.Name)
				}
			}
		case <-stop:
			// call stop on all namespace watching
			for _, stopChannel := range namespaceTable {
				close(stopChannel)
			}
			return nil
		}
	}
}

func (oi *OsdnRegistryInterface) watchServicesForNamespace(namespace string, receiver chan<- *osdnapi.ServiceEvent, stop chan bool) error {
	serviceEventQueue, _ := oi.runEventQueue("Service", namespace)
	go func() {
		select {
		case <-stop:
			serviceEventQueue.Cancel()
		}
	}()

	for {
		eventType, obj, err := serviceEventQueue.Pop()
		if err != nil {
			if _, ok := err.(oscache.EventQueueStopped); ok {
				return nil
			}
			return err
		}
		kServ := obj.(*kapi.Service)
		// Ignore headless services
		if kServ.Spec.ClusterIP == "None" {
			continue
		}

		switch eventType {
		case watch.Added:
			for _, port := range kServ.Spec.Ports {
				oServ := newSDNService(kServ, namespace, port)
				receiver <- &osdnapi.ServiceEvent{Type: osdnapi.Added, Service: oServ}
			}
		case watch.Deleted:
			for _, port := range kServ.Spec.Ports {
				oServ := newSDNService(kServ, namespace, port)
				receiver <- &osdnapi.ServiceEvent{Type: osdnapi.Deleted, Service: oServ}
			}
		case watch.Modified:
			// Ignore, we don't need to update SDN in case of service updates
		case watch.Error:
			// Check if the namespace is dead, if so quit
			_, err = oi.kClient.Namespaces().Get(namespace)
			if err != nil {
				break
			}
		}
	}
}

func newSDNService(kServ *kapi.Service, namespace string, port kapi.ServicePort) osdnapi.Service {
	return osdnapi.Service{
		Name:      kServ.ObjectMeta.Name,
		Namespace: namespace,
		IP:        kServ.Spec.ClusterIP,
		Protocol:  osdnapi.ServiceProtocol(port.Protocol),
		Port:      uint(port.Port),
	}
}

// Run event queue for the given resource
func (oi *OsdnRegistryInterface) runEventQueue(resourceName string, args interface{}) (*oscache.EventQueue, *cache.Reflector) {
	eventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	lw := &cache.ListWatch{}
	var expectedType interface{}
	switch strings.ToLower(resourceName) {
	case "hostsubnet":
		expectedType = &api.HostSubnet{}
		lw.ListFunc = func() (runtime.Object, error) {
			return oi.oClient.HostSubnets().List()
		}
		lw.WatchFunc = func(resourceVersion string) (watch.Interface, error) {
			return oi.oClient.HostSubnets().Watch(resourceVersion)
		}
	case "node":
		expectedType = &kapi.Node{}
		lw.ListFunc = func() (runtime.Object, error) {
			return oi.kClient.Nodes().List(labels.Everything(), fields.Everything())
		}
		lw.WatchFunc = func(resourceVersion string) (watch.Interface, error) {
			return oi.kClient.Nodes().Watch(labels.Everything(), fields.Everything(), resourceVersion)
		}
	case "namespace":
		expectedType = &kapi.Namespace{}
		lw.ListFunc = func() (runtime.Object, error) {
			return oi.kClient.Namespaces().List(labels.Everything(), fields.Everything())
		}
		lw.WatchFunc = func(resourceVersion string) (watch.Interface, error) {
			return oi.kClient.Namespaces().Watch(labels.Everything(), fields.Everything(), resourceVersion)
		}
	case "netnamespace":
		expectedType = &api.NetNamespace{}
		lw.ListFunc = func() (runtime.Object, error) {
			return oi.oClient.NetNamespaces().List()
		}
		lw.WatchFunc = func(resourceVersion string) (watch.Interface, error) {
			return oi.oClient.NetNamespaces().Watch(resourceVersion)
		}
	case "service":
		expectedType = &kapi.Service{}
		namespace := args.(string)
		lw.ListFunc = func() (runtime.Object, error) {
			return oi.kClient.Services(namespace).List(labels.Everything())
		}
		lw.WatchFunc = func(resourceVersion string) (watch.Interface, error) {
			return oi.kClient.Services(namespace).Watch(labels.Everything(), fields.Everything(), resourceVersion)
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
func (oi *OsdnRegistryInterface) createAndRunEventQueue(resourceName string, args interface{}, ready chan<- bool, start <-chan string) (*oscache.EventQueue, uint64) {
	eventQueue, reflector := oi.runEventQueue(resourceName, args)
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
			currentVersion, err := strconv.ParseUint(accessor.ResourceVersion(), 10, 64)
			if err != nil {
				return watch.Error, nil, err
			}
			if currentVersion <= startVersion {
				log.V(5).Infof("Ignoring %s with version %d, start version: %d", accessor.Name(), currentVersion, startVersion)
				continue
			}
			*checkCondition = false
			return eventType, obj, nil
		}
	} else {
		return eventQueue.Pop()
	}
}
