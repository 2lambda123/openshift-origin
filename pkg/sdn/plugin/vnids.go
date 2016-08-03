package plugin

import (
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/container"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	osapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/util/netutils"
)

const (
	// Maximum VXLAN Network Identifier as per RFC#7348
	MaxVNID = uint32((1 << 24) - 1)
	// VNID for the admin namespaces
	AdminVNID = uint32(0)
)

type vnidMap struct {
	ids        map[string]uint32
	namespaces map[uint32]sets.String
	lock       sync.Mutex
}

func newVnidMap() *vnidMap {
	return &vnidMap{
		ids:        make(map[string]uint32),
		namespaces: make(map[uint32]sets.String),
	}
}

func (vmap *vnidMap) addNamespaceToSet(name string, vnid uint32) {
	set, found := vmap.namespaces[vnid]
	if !found {
		set = sets.NewString()
		vmap.namespaces[vnid] = set
	}
	set.Insert(name)
}

func (vmap *vnidMap) removeNamespaceFromSet(name string, vnid uint32) {
	if set, found := vmap.namespaces[vnid]; found {
		set.Delete(name)
		if set.Len() == 0 {
			delete(vmap.namespaces, vnid)
		}
	}
}

func (vmap *vnidMap) GetVNID(name string) (uint32, error) {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	if id, ok := vmap.ids[name]; ok {
		return id, nil
	}
	// In case of error, return some value which is not a valid VNID
	return uint32(MaxVNID + 1), fmt.Errorf("Failed to find netid for namespace: %s in vnid map", name)
}

// Nodes asynchronously watch for both NetNamespaces and services
// NetNamespaces populates vnid map and services/pod-setup depend on vnid map
// If for some reason, vnid map propagation from master to node is slow
// and if service/pod-setup tries to lookup vnid map then it may fail.
// So, use this method to alleviate this problem. This method will
// retry vnid lookup before giving up.
func (vmap *vnidMap) WaitAndGetVNID(name string) (uint32, error) {
	// Try few times up to 2 seconds
	retries := 20
	retryInterval := 100 * time.Millisecond
	for i := 0; i < retries; i++ {
		if id, err := vmap.GetVNID(name); err == nil {
			return id, nil
		}
		time.Sleep(retryInterval)
	}

	// In case of error, return some value which is not a valid VNID
	return MaxVNID + 1, fmt.Errorf("Failed to find netid for namespace: %s in vnid map", name)
}

func (vmap *vnidMap) GetNamespaces(id uint32) []string {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	if set, ok := vmap.namespaces[id]; ok {
		return set.List()
	} else {
		return nil
	}
}

func (vmap *vnidMap) SetVNID(name string, id uint32) {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	if oldId, found := vmap.ids[name]; found {
		vmap.removeNamespaceFromSet(name, oldId)
	}
	vmap.ids[name] = id
	vmap.addNamespaceToSet(name, id)

	log.Infof("Associate netid %d to namespace %q", id, name)
}

func (vmap *vnidMap) UnsetVNID(name string) (id uint32, err error) {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	id, found := vmap.ids[name]
	if !found {
		// In case of error, return some value which is not a valid VNID
		return MaxVNID + 1, fmt.Errorf("Failed to find netid for namespace: %s in vnid map", name)
	}
	vmap.removeNamespaceFromSet(name, id)
	delete(vmap.ids, name)
	log.Infof("Dissociate netid %d from namespace %q", id, name)
	return id, nil
}

func (vmap *vnidMap) CheckVNID(id uint32) bool {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	_, found := vmap.namespaces[id]
	return found
}

func (vmap *vnidMap) GetAllocatedVNIDs() []uint32 {
	vmap.lock.Lock()
	defer vmap.lock.Unlock()

	ids := []uint32{}
	idSet := sets.Int{}
	for _, id := range vmap.ids {
		if id != AdminVNID {
			if !idSet.Has(int(id)) {
				ids = append(ids, id)
				idSet.Insert(int(id))
			}
		}
	}
	return ids
}

func (vmap *vnidMap) PopulateVNIDs(registry *Registry) error {
	nets, err := registry.GetNetNamespaces()
	if err != nil {
		return err
	}

	for _, net := range nets {
		vmap.SetVNID(net.Name, net.NetID)
	}
	return nil
}

func (master *OsdnMaster) VnidStartMaster() error {
	err := master.vnids.PopulateVNIDs(master.registry)
	if err != nil {
		return err
	}

	// VNID: 0 reserved for default namespace and can reach any network in the cluster
	// VNID: 1 to 9 are internally reserved for any special cases in the future
	master.netIDManager, err = netutils.NewNetIDAllocator(10, MaxVNID, master.vnids.GetAllocatedVNIDs())
	if err != nil {
		return err
	}

	// 'default' namespace is currently always an admin namespace
	master.adminNamespaces = append(master.adminNamespaces, "default")

	go utilwait.Forever(master.watchNamespaces, 0)
	return nil
}

func (master *OsdnMaster) isAdminNamespace(nsName string) bool {
	for _, name := range master.adminNamespaces {
		if name == nsName {
			return true
		}
	}
	return false
}

func (master *OsdnMaster) assignVNID(namespaceName string) error {
	// Nothing to do if the netid is in the vnid map
	if _, err := master.vnids.GetVNID(namespaceName); err == nil {
		return nil
	}

	// If NetNamespace is present, update vnid map
	netns, err := master.registry.GetNetNamespace(namespaceName)
	if err == nil {
		master.vnids.SetVNID(namespaceName, netns.NetID)
		return nil
	}

	// NetNamespace not found, so allocate new NetID
	var netid uint32
	if master.isAdminNamespace(namespaceName) {
		netid = AdminVNID
	} else {
		netid, err = master.netIDManager.GetNetID()
		if err != nil {
			return err
		}
	}

	// Create NetNamespace Object and update vnid map
	err = master.registry.WriteNetNamespace(namespaceName, netid)
	if err != nil {
		e := master.netIDManager.ReleaseNetID(netid)
		if e != nil {
			log.Errorf("Error while releasing netid: %v", e)
		}
		return err
	}
	master.vnids.SetVNID(namespaceName, netid)
	return nil
}

func (master *OsdnMaster) revokeVNID(namespaceName string) error {
	// Remove NetID from vnid map
	netid_found := true
	netid, err := master.vnids.UnsetVNID(namespaceName)
	if err != nil {
		log.Error(err)
		netid_found = false
	}

	// Delete NetNamespace object
	err = master.registry.DeleteNetNamespace(namespaceName)
	if err != nil {
		return err
	}

	// Skip NetID release if
	// - Value matches AdminVNID as it is not part of NetID allocation or
	// - NetID is not found in the vnid map
	if (netid == AdminVNID) || !netid_found {
		return nil
	}

	// Check if this netid is used by any other namespaces
	// If not, then release the netid
	if !master.vnids.CheckVNID(netid) {
		err = master.netIDManager.ReleaseNetID(netid)
		if err != nil {
			return fmt.Errorf("Error while releasing netid %d for namespace %q, %v", netid, namespaceName, err)
		}
		log.Infof("Released netid %d for namespace %q", netid, namespaceName)
	} else {
		log.V(5).Infof("netid %d for namespace %q is still in use", netid, namespaceName)
	}
	return nil
}

func (master *OsdnMaster) watchNamespaces() {
	eventQueue := master.registry.RunEventQueue(Namespaces)

	for {
		eventType, obj, err := eventQueue.Pop()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("EventQueue failed for namespaces: %v", err))
			return
		}
		ns := obj.(*kapi.Namespace)
		name := ns.ObjectMeta.Name

		log.V(5).Infof("Watch %s event for Namespace %q", strings.Title(string(eventType)), name)
		switch eventType {
		case watch.Added, watch.Modified:
			err = master.assignVNID(name)
			if err != nil {
				log.Errorf("Error assigning netid: %v", err)
				continue
			}
		case watch.Deleted:
			err = master.revokeVNID(name)
			if err != nil {
				log.Errorf("Error revoking netid: %v", err)
				continue
			}
		}
	}
}

func (node *OsdnNode) VnidStartNode() error {
	// Populate vnid map synchronously so that existing services can fetch vnid
	err := node.vnids.PopulateVNIDs(node.registry)
	if err != nil {
		return err
	}

	go utilwait.Forever(node.watchNetNamespaces, 0)
	go utilwait.Forever(node.watchServices, 0)
	return nil
}

func (node *OsdnNode) updatePodNetwork(namespace string, oldNetID, netID uint32) error {
	// FIXME: this is racy; traffic coming from the pods gets switched to the new
	// VNID before the service and firewall rules are updated to match. We need
	// to do the updates as a single transaction (ovs-ofctl --bundle).

	pods, err := node.GetLocalPods(namespace)
	if err != nil {
		return err
	}
	services, err := node.registry.GetServicesForNamespace(namespace)
	if err != nil {
		return err
	}

	errList := []error{}

	// Update OF rules for the existing/old pods in the namespace
	for _, pod := range pods {
		err = node.UpdatePod(pod.Namespace, pod.Name, kubetypes.DockerID(getPodContainerID(&pod)))
		if err != nil {
			errList = append(errList, err)
		}
	}

	// Update OF rules for the old services in the namespace
	for _, svc := range services {
		if err = node.DeleteServiceRules(&svc); err != nil {
			log.Error(err)
		}
		if err = node.AddServiceRules(&svc, netID); err != nil {
			errList = append(errList, err)
		}
	}

	// Update namespace references in egress firewall rules
	if err = node.UpdateEgressNetworkPolicyVNID(namespace, oldNetID, netID); err != nil {
		errList = append(errList, err)
	}

	return kerrors.NewAggregate(errList)
}

func (node *OsdnNode) watchNetNamespaces() {
	eventQueue := node.registry.RunEventQueue(NetNamespaces)

	for {
		eventType, obj, err := eventQueue.Pop()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("EventQueue failed for network namespaces: %v", err))
			return
		}
		netns := obj.(*osapi.NetNamespace)

		log.V(5).Infof("Watch %s event for NetNamespace %q", strings.Title(string(eventType)), netns.ObjectMeta.Name)
		switch eventType {
		case watch.Added, watch.Modified:
			// Skip this event if the old and new network ids are same
			var oldNetID uint32
			oldNetID, err = node.vnids.GetVNID(netns.NetName)
			if (err == nil) && (oldNetID == netns.NetID) {
				continue
			}
			node.vnids.SetVNID(netns.NetName, netns.NetID)

			err = node.updatePodNetwork(netns.NetName, oldNetID, netns.NetID)
			if err != nil {
				log.Errorf("Failed to update pod network for namespace '%s', error: %s", netns.NetName, err)
				node.vnids.SetVNID(netns.NetName, oldNetID)
				continue
			}
		case watch.Deleted:
			// updatePodNetwork needs vnid, so unset vnid after this call
			err = node.updatePodNetwork(netns.NetName, netns.NetID, AdminVNID)
			if err != nil {
				log.Errorf("Failed to update pod network for namespace '%s', error: %s", netns.NetName, err)
			}
			node.vnids.UnsetVNID(netns.NetName)
		}
	}
}

func isServiceChanged(oldsvc, newsvc *kapi.Service) bool {
	if len(oldsvc.Spec.Ports) == len(newsvc.Spec.Ports) {
		for i := range oldsvc.Spec.Ports {
			if oldsvc.Spec.Ports[i].Protocol != newsvc.Spec.Ports[i].Protocol ||
				oldsvc.Spec.Ports[i].Port != newsvc.Spec.Ports[i].Port {
				return true
			}
		}
		return false
	}
	return true
}

func (node *OsdnNode) watchServices() {
	services := make(map[string]*kapi.Service)
	eventQueue := node.registry.RunEventQueue(Services)

	for {
		eventType, obj, err := eventQueue.Pop()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("EventQueue failed for services: %v", err))
			return
		}
		serv := obj.(*kapi.Service)

		// Ignore headless services
		if !kapi.IsServiceIPSet(serv) {
			continue
		}

		log.V(5).Infof("Watch %s event for Service %q", strings.Title(string(eventType)), serv.ObjectMeta.Name)
		switch eventType {
		case watch.Added, watch.Modified:
			oldsvc, exists := services[string(serv.UID)]
			if exists {
				if !isServiceChanged(oldsvc, serv) {
					continue
				}
				if err = node.DeleteServiceRules(oldsvc); err != nil {
					log.Error(err)
				}
			}

			var netid uint32
			netid, err = node.vnids.WaitAndGetVNID(serv.Namespace)
			if err != nil {
				log.Errorf("Skipped adding service rules for serviceEvent: %v, Error: %v", eventType, err)
				continue
			}

			if err = node.AddServiceRules(serv, netid); err != nil {
				log.Error(err)
				continue
			}
			services[string(serv.UID)] = serv
		case watch.Deleted:
			delete(services, string(serv.UID))

			if err = node.DeleteServiceRules(serv); err != nil {
				log.Error(err)
			}
		}
	}
}
