package node

import (
	"fmt"
	"net"
	"sync"
	"syscall"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/common"
	networkinformers "github.com/openshift/origin/pkg/network/generated/informers/internalversion"
	"github.com/openshift/origin/pkg/util/netutils"

	"github.com/vishvananda/netlink"
)

type nodeEgress struct {
	nodeIP       string
	sdnIP        string
	requestedIPs sets.String
	offline      bool
}

type namespaceEgress struct {
	vnid         uint32
	requestedIPs []string
}

type egressIPInfo struct {
	ip string

	nodes      []*nodeEgress
	namespaces []*namespaceEgress

	assignedNodeIP       string
	assignedIPTablesMark string
}

type egressIPWatcher struct {
	sync.Mutex

	oc            *ovsController
	localIP       string
	masqueradeBit uint32

	networkInformers networkinformers.SharedInformerFactory
	iptables         *NodeIPTables
	vxlanMonitor     *egressVXLANMonitor

	nodesByNodeIP    map[string]*nodeEgress
	namespacesByVNID map[uint32]*namespaceEgress
	egressIPs        map[string]*egressIPInfo

	changedEgressIPs  map[*egressIPInfo]bool
	changedNamespaces map[*namespaceEgress]bool

	localEgressLink netlink.Link
	localEgressNet  *net.IPNet

	testModeChan chan string
}

func newEgressIPWatcher(oc *ovsController, localIP string, masqueradeBit *int32) *egressIPWatcher {
	eip := &egressIPWatcher{
		oc:      oc,
		localIP: localIP,

		nodesByNodeIP:    make(map[string]*nodeEgress),
		namespacesByVNID: make(map[uint32]*namespaceEgress),
		egressIPs:        make(map[string]*egressIPInfo),

		changedEgressIPs:  make(map[*egressIPInfo]bool),
		changedNamespaces: make(map[*namespaceEgress]bool),
	}
	if masqueradeBit != nil {
		eip.masqueradeBit = 1 << uint32(*masqueradeBit)
	}
	return eip
}

func (eip *egressIPWatcher) Start(networkInformers networkinformers.SharedInformerFactory, iptables *NodeIPTables) error {
	var err error
	if eip.localEgressLink, eip.localEgressNet, err = GetLinkDetails(eip.localIP); err != nil {
		// Not expected, should already be caught by node.New()
		return nil
	}

	eip.networkInformers = networkInformers
	eip.iptables = iptables

	updates := make(chan *egressVXLANNode)
	eip.vxlanMonitor = newEgressVXLANMonitor(eip.oc.ovs, updates)
	go eip.watchVXLAN(updates)

	eip.watchHostSubnets()
	eip.watchNetNamespaces()
	return nil
}

// Convert vnid to a hex value that is not 0, does not have masqueradeBit set, and isn't
// the same value as would be returned for any other valid vnid.
func getMarkForVNID(vnid, masqueradeBit uint32) string {
	if vnid == 0 {
		vnid = 0xff000000
	}
	if (vnid & masqueradeBit) != 0 {
		vnid = (vnid | 0x01000000) ^ masqueradeBit
	}
	return fmt.Sprintf("0x%08x", vnid)
}

func (eip *egressIPWatcher) ensureEgressIPInfo(egressIP string) *egressIPInfo {
	eg := eip.egressIPs[egressIP]
	if eg == nil {
		eg = &egressIPInfo{ip: egressIP}
		eip.egressIPs[egressIP] = eg
	}
	return eg
}

func (eip *egressIPWatcher) egressIPChanged(eg *egressIPInfo) {
	eip.changedEgressIPs[eg] = true
	for _, ns := range eg.namespaces {
		eip.changedNamespaces[ns] = true
	}
}

func (eip *egressIPWatcher) addNode(egressIP string, node *nodeEgress) {
	eg := eip.ensureEgressIPInfo(egressIP)
	eg.nodes = append(eg.nodes, node)
	eip.egressIPChanged(eg)
}

func (eip *egressIPWatcher) deleteNode(egressIP string, node *nodeEgress) {
	eg := eip.egressIPs[egressIP]
	if eg == nil {
		return
	}

	for i := range eg.nodes {
		if eg.nodes[i] == node {
			eip.egressIPChanged(eg)
			eg.nodes = append(eg.nodes[:i], eg.nodes[i+1:]...)
			return
		}
	}
}

func (eip *egressIPWatcher) addNamespace(egressIP string, ns *namespaceEgress) {
	eg := eip.ensureEgressIPInfo(egressIP)
	eg.namespaces = append(eg.namespaces, ns)
	eip.egressIPChanged(eg)
}

func (eip *egressIPWatcher) deleteNamespace(egressIP string, ns *namespaceEgress) {
	eg := eip.egressIPs[egressIP]
	if eg == nil {
		return
	}

	for i := range eg.namespaces {
		if eg.namespaces[i] == ns {
			eip.egressIPChanged(eg)
			eg.namespaces = append(eg.namespaces[:i], eg.namespaces[i+1:]...)
			return
		}
	}
}

func (eip *egressIPWatcher) watchHostSubnets() {
	funcs := common.InformerFuncs(&networkapi.HostSubnet{}, eip.handleAddOrUpdateHostSubnet, eip.handleDeleteHostSubnet)
	eip.networkInformers.Network().InternalVersion().HostSubnets().Informer().AddEventHandler(funcs)
}

func (eip *egressIPWatcher) handleAddOrUpdateHostSubnet(obj, _ interface{}, eventType watch.EventType) {
	hs := obj.(*networkapi.HostSubnet)
	glog.V(5).Infof("Watch %s event for HostSubnet %q", eventType, hs.Name)

	_, cidr, err := net.ParseCIDR(hs.Subnet)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("could not parse HostSubnet %q CIDR: %v", hs.Name, err))
	}
	sdnIP := netutils.GenerateDefaultGateway(cidr).String()

	eip.updateNodeEgress(hs.HostIP, sdnIP, hs.EgressIPs)
}

func (eip *egressIPWatcher) handleDeleteHostSubnet(obj interface{}) {
	hs := obj.(*networkapi.HostSubnet)
	glog.V(5).Infof("Watch %s event for HostSubnet %q", watch.Deleted, hs.Name)

	eip.updateNodeEgress(hs.HostIP, "", nil)
}

func (eip *egressIPWatcher) updateNodeEgress(nodeIP, sdnIP string, nodeEgressIPs []string) {
	eip.Lock()
	defer eip.Unlock()

	node := eip.nodesByNodeIP[nodeIP]
	if node == nil {
		if len(nodeEgressIPs) == 0 {
			return
		}
		node = &nodeEgress{
			nodeIP:       nodeIP,
			sdnIP:        sdnIP,
			requestedIPs: sets.NewString(),
		}
		eip.nodesByNodeIP[nodeIP] = node
		if eip.vxlanMonitor != nil && node.nodeIP != eip.localIP {
			eip.vxlanMonitor.AddNode(node.nodeIP, node.sdnIP)
		}
	} else if len(nodeEgressIPs) == 0 {
		delete(eip.nodesByNodeIP, nodeIP)
		if eip.vxlanMonitor != nil {
			eip.vxlanMonitor.RemoveNode(node.nodeIP)
		}
	}
	oldRequestedIPs := node.requestedIPs
	node.requestedIPs = sets.NewString(nodeEgressIPs...)

	// Process new and removed EgressIPs
	for _, ip := range node.requestedIPs.Difference(oldRequestedIPs).UnsortedList() {
		eip.addNode(ip, node)
	}
	for _, ip := range oldRequestedIPs.Difference(node.requestedIPs).UnsortedList() {
		eip.deleteNode(ip, node)
	}

	eip.syncEgressIPs()
}

func (eip *egressIPWatcher) watchNetNamespaces() {
	funcs := common.InformerFuncs(&networkapi.NetNamespace{}, eip.handleAddOrUpdateNetNamespace, eip.handleDeleteNetNamespace)
	eip.networkInformers.Network().InternalVersion().NetNamespaces().Informer().AddEventHandler(funcs)
}

func (eip *egressIPWatcher) handleAddOrUpdateNetNamespace(obj, _ interface{}, eventType watch.EventType) {
	netns := obj.(*networkapi.NetNamespace)
	glog.V(5).Infof("Watch %s event for NetNamespace %q", eventType, netns.Name)

	eip.updateNamespaceEgress(netns.NetID, netns.EgressIPs)
}

func (eip *egressIPWatcher) handleDeleteNetNamespace(obj interface{}) {
	netns := obj.(*networkapi.NetNamespace)
	glog.V(5).Infof("Watch %s event for NetNamespace %q", watch.Deleted, netns.Name)

	eip.deleteNamespaceEgress(netns.NetID)
}

func (eip *egressIPWatcher) updateNamespaceEgress(vnid uint32, egressIPs []string) {
	eip.Lock()
	defer eip.Unlock()

	ns := eip.namespacesByVNID[vnid]
	if ns == nil {
		if len(egressIPs) == 0 {
			return
		}
		ns = &namespaceEgress{vnid: vnid}
		eip.namespacesByVNID[vnid] = ns
	} else if len(egressIPs) == 0 {
		delete(eip.namespacesByVNID, vnid)
	}

	oldRequestedIPs := sets.NewString(ns.requestedIPs...)
	newRequestedIPs := sets.NewString(egressIPs...)
	ns.requestedIPs = egressIPs

	// Process new and removed EgressIPs
	for _, ip := range newRequestedIPs.Difference(oldRequestedIPs).UnsortedList() {
		eip.addNamespace(ip, ns)
	}
	for _, ip := range oldRequestedIPs.Difference(newRequestedIPs).UnsortedList() {
		eip.deleteNamespace(ip, ns)
	}

	// Even IPs that weren't added/removed need to be considered "changed", to
	// ensure we correctly process reorderings, duplicates added/removed, etc.
	for _, ip := range newRequestedIPs.Intersection(oldRequestedIPs).UnsortedList() {
		eip.egressIPChanged(eip.egressIPs[ip])
	}

	eip.syncEgressIPs()
}

func (eip *egressIPWatcher) deleteNamespaceEgress(vnid uint32) {
	eip.updateNamespaceEgress(vnid, nil)
}

func (eip *egressIPWatcher) egressIPActive(eg *egressIPInfo) (bool, error) {
	if len(eg.nodes) == 0 || len(eg.namespaces) == 0 {
		return false, nil
	}
	if len(eg.nodes) > 1 {
		return false, fmt.Errorf("Multiple nodes (%s, %s) claiming EgressIP %s", eg.nodes[0].nodeIP, eg.nodes[1].nodeIP, eg.ip)
	}
	if len(eg.namespaces) > 1 {
		return false, fmt.Errorf("Multiple namespaces (%d, %d) claiming EgressIP %s", eg.namespaces[0].vnid, eg.namespaces[1].vnid, eg.ip)
	}
	for _, ip := range eg.namespaces[0].requestedIPs {
		eg2 := eip.egressIPs[ip]
		if eg2 != eg && len(eg2.nodes) == 1 && eg2.nodes[0] == eg.nodes[0] {
			return false, fmt.Errorf("Multiple EgressIPs (%s, %s) for VNID %d on node %s", eg.ip, eg2.ip, eg.namespaces[0].vnid, eg.nodes[0].nodeIP)
		}
	}
	return true, nil
}

func (eip *egressIPWatcher) syncEgressIPs() {
	for eg := range eip.changedEgressIPs {
		active, err := eip.egressIPActive(eg)
		if err != nil {
			utilruntime.HandleError(err)
		}
		eip.syncEgressNodeState(eg, active)
	}
	eip.changedEgressIPs = make(map[*egressIPInfo]bool)

	for ns := range eip.changedNamespaces {
		err := eip.syncEgressNamespaceState(ns)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Error updating Namespace egress rules for VNID %d: %v", ns.vnid, err))
		}
	}
	eip.changedNamespaces = make(map[*namespaceEgress]bool)
}

func (eip *egressIPWatcher) syncEgressNodeState(eg *egressIPInfo, active bool) {
	if active && eg.assignedNodeIP != eg.nodes[0].nodeIP {
		glog.V(4).Infof("Assigning egress IP %s to node %s", eg.ip, eg.nodes[0].nodeIP)
		eg.assignedNodeIP = eg.nodes[0].nodeIP
		eg.assignedIPTablesMark = getMarkForVNID(eg.namespaces[0].vnid, eip.masqueradeBit)
		if eg.assignedNodeIP == eip.localIP {
			if err := eip.assignEgressIP(eg.ip, eg.assignedIPTablesMark); err != nil {
				utilruntime.HandleError(fmt.Errorf("Error assigning Egress IP %q: %v", eg.ip, err))
				eg.assignedNodeIP = ""
			}
		}
	} else if !active && eg.assignedNodeIP != "" {
		glog.V(4).Infof("Removing egress IP %s from node %s", eg.ip, eg.assignedNodeIP)
		if eg.assignedNodeIP == eip.localIP {
			if err := eip.releaseEgressIP(eg.ip, eg.assignedIPTablesMark); err != nil {
				utilruntime.HandleError(fmt.Errorf("Error releasing Egress IP %q: %v", eg.ip, err))
			}
		}
		eg.assignedNodeIP = ""
		eg.assignedIPTablesMark = ""
	}
}

func (eip *egressIPWatcher) syncEgressNamespaceState(ns *namespaceEgress) error {
	if len(ns.requestedIPs) == 0 {
		return eip.oc.SetNamespaceEgressNormal(ns.vnid)
	}

	var active *egressIPInfo
	for _, ip := range ns.requestedIPs {
		eg := eip.egressIPs[ip]
		if eg == nil {
			continue
		}
		if len(eg.namespaces) > 1 {
			active = nil
			glog.V(4).Infof("VNID %d gets no egress due to multiply-assigned egress IP %s", ns.vnid, eg.ip)
			break
		}
		if active == nil {
			if eg.assignedNodeIP == "" {
				glog.V(4).Infof("VNID %d cannot use unassigned egress IP %s", ns.vnid, eg.ip)
			} else if len(ns.requestedIPs) > 1 && eg.nodes[0].offline {
				glog.V(4).Infof("VNID %d cannot use egress IP %s on offline node %s", ns.vnid, eg.ip, eg.assignedNodeIP)
			} else {
				active = eg
			}
		}
	}

	if active != nil {
		return eip.oc.SetNamespaceEgressViaEgressIP(ns.vnid, active.assignedNodeIP, active.assignedIPTablesMark)
	} else {
		return eip.oc.SetNamespaceEgressDropped(ns.vnid)
	}
}

func (eip *egressIPWatcher) assignEgressIP(egressIP, mark string) error {
	if egressIP == eip.localIP {
		return fmt.Errorf("desired egress IP %q is the node IP", egressIP)
	}

	if eip.testModeChan != nil {
		eip.testModeChan <- fmt.Sprintf("claim %s", egressIP)
		return nil
	}

	localEgressIPMaskLen, _ := eip.localEgressNet.Mask.Size()
	egressIPNet := fmt.Sprintf("%s/%d", egressIP, localEgressIPMaskLen)
	addr, err := netlink.ParseAddr(egressIPNet)
	if err != nil {
		return fmt.Errorf("could not parse egress IP %q: %v", egressIPNet, err)
	}
	if !eip.localEgressNet.Contains(addr.IP) {
		return fmt.Errorf("egress IP %q is not in local network %s of interface %s", egressIP, eip.localEgressNet.String(), eip.localEgressLink.Attrs().Name)
	}
	err = netlink.AddrAdd(eip.localEgressLink, addr)
	if err != nil {
		if err == syscall.EEXIST {
			glog.V(2).Infof("Egress IP %q already exists on %s", egressIPNet, eip.localEgressLink.Attrs().Name)
		} else {
			return fmt.Errorf("could not add egress IP %q to %s: %v", egressIPNet, eip.localEgressLink.Attrs().Name, err)
		}
	}

	if err := eip.iptables.AddEgressIPRules(egressIP, mark); err != nil {
		return fmt.Errorf("could not add egress IP iptables rule: %v", err)
	}

	return nil
}

func (eip *egressIPWatcher) releaseEgressIP(egressIP, mark string) error {
	if egressIP == eip.localIP {
		return nil
	}

	if eip.testModeChan != nil {
		eip.testModeChan <- fmt.Sprintf("release %s", egressIP)
		return nil
	}

	localEgressIPMaskLen, _ := eip.localEgressNet.Mask.Size()
	egressIPNet := fmt.Sprintf("%s/%d", egressIP, localEgressIPMaskLen)
	addr, err := netlink.ParseAddr(egressIPNet)
	if err != nil {
		return fmt.Errorf("could not parse egress IP %q: %v", egressIPNet, err)
	}
	err = netlink.AddrDel(eip.localEgressLink, addr)
	if err != nil {
		if err == syscall.EADDRNOTAVAIL {
			glog.V(2).Infof("Could not delete egress IP %q from %s: no such address", egressIPNet, eip.localEgressLink.Attrs().Name)
		} else {
			return fmt.Errorf("could not delete egress IP %q from %s: %v", egressIPNet, eip.localEgressLink.Attrs().Name, err)
		}
	}

	if err := eip.iptables.DeleteEgressIPRules(egressIP, mark); err != nil {
		return fmt.Errorf("could not delete egress IP iptables rule: %v", err)
	}

	return nil
}

func (eip *egressIPWatcher) watchVXLAN(updates chan *egressVXLANNode) {
	for node := range updates {
		eip.updateNode(node.nodeIP, node.offline)
	}
}

func (eip *egressIPWatcher) updateNode(nodeIP string, offline bool) {
	eip.Lock()
	defer eip.Unlock()

	node := eip.nodesByNodeIP[nodeIP]
	if node == nil {
		eip.vxlanMonitor.RemoveNode(nodeIP)
		return
	}

	node.offline = offline
	for _, ip := range node.requestedIPs.UnsortedList() {
		eg := eip.egressIPs[ip]
		if eg != nil {
			eip.egressIPChanged(eg)
		}
	}
	eip.syncEgressIPs()
}
