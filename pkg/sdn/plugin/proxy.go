package plugin

import (
	"fmt"
	"net"
	"sync"

	"github.com/golang/glog"

	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/sdn/api"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	pconfig "k8s.io/kubernetes/pkg/proxy/config"
)

type firewallItem struct {
	ruleType osapi.EgressNetworkPolicyRuleType
	net      *net.IPNet
}

type proxyFirewallItem struct {
	namespaceFirewalls map[ktypes.UID][]firewallItem
	activePolicy       *ktypes.UID
}

type OsdnProxy struct {
	kClient              kclientset.Interface
	osClient             *osclient.Client
	networkInfo          *NetworkInfo
	egressDNS            *EgressDNS
	baseEndpointsHandler pconfig.EndpointsConfigHandler

	lock         sync.Mutex
	firewall     map[string]*proxyFirewallItem
	allEndpoints []*kapi.Endpoints

	idLock sync.Mutex
	ids    map[string]uint32
}

// Called by higher layers to create the proxy plugin instance; only used by nodes
func NewProxyPlugin(pluginName string, osClient *osclient.Client, kClient kclientset.Interface) (*OsdnProxy, error) {
	if !osapi.IsOpenShiftMultitenantNetworkPlugin(pluginName) {
		return nil, nil
	}

	return &OsdnProxy{
		kClient:   kClient,
		osClient:  osClient,
		ids:       make(map[string]uint32),
		egressDNS: NewEgressDNS(),
		firewall:  make(map[string]*proxyFirewallItem),
	}, nil
}

func (proxy *OsdnProxy) Start(baseHandler pconfig.EndpointsConfigHandler) error {
	glog.Infof("Starting multitenant SDN proxy endpoint filter")

	var err error
	proxy.networkInfo, err = getNetworkInfo(proxy.osClient)
	if err != nil {
		return fmt.Errorf("could not get network info: %s", err)
	}
	proxy.baseEndpointsHandler = baseHandler

	policies, err := proxy.osClient.EgressNetworkPolicies(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("could not get EgressNetworkPolicies: %s", err)
	}

	proxy.lock.Lock()
	defer proxy.lock.Unlock()

	for _, policy := range policies.Items {
		proxy.egressDNS.Add(policy)
		proxy.updateEgressNetworkPolicy(policy)
	}

	go utilwait.Forever(proxy.syncEgressDNSProxyFirewall, 0)
	go utilwait.Forever(proxy.watchEgressNetworkPolicies, 0)
	go utilwait.Forever(proxy.watchNetNamespaces, 0)
	return nil
}

func (proxy *OsdnProxy) watchEgressNetworkPolicies() {
	RunEventQueue(proxy.osClient, EgressNetworkPolicies, func(delta cache.Delta) error {
		policy := delta.Object.(*osapi.EgressNetworkPolicy)

		proxy.egressDNS.Delete(*policy)
		if delta.Type == cache.Deleted {
			policy.Spec.Egress = nil
		} else {
			proxy.egressDNS.Add(*policy)
		}

		func() {
			proxy.lock.Lock()
			defer proxy.lock.Unlock()
			proxy.updateEgressNetworkPolicy(*policy)
			if proxy.allEndpoints != nil {
				proxy.updateEndpoints()
			}
		}()
		return nil
	})
}

// TODO: Abstract common code shared between proxy and node
func (proxy *OsdnProxy) watchNetNamespaces() {
	RunEventQueue(proxy.osClient, NetNamespaces, func(delta cache.Delta) error {
		netns := delta.Object.(*osapi.NetNamespace)
		name := netns.ObjectMeta.Name

		glog.V(5).Infof("Watch %s event for NetNamespace %q", delta.Type, name)
		proxy.idLock.Lock()
		defer proxy.idLock.Unlock()
		switch delta.Type {
		case cache.Sync, cache.Added, cache.Updated:
			proxy.ids[name] = netns.NetID
		case cache.Deleted:
			delete(proxy.ids, name)
		}
		return nil
	})
}

func (proxy *OsdnProxy) isNamespaceGlobal(ns string) bool {
	proxy.idLock.Lock()
	defer proxy.idLock.Unlock()

	if proxy.ids[ns] == osapi.GlobalVNID {
		return true
	}
	return false
}

func (proxy *OsdnProxy) updateEgressNetworkPolicy(policy osapi.EgressNetworkPolicy) {
	ns := policy.Namespace
	if proxy.isNamespaceGlobal(ns) {
		// Firewall not allowed for global namespaces
		glog.Errorf("EgressNetworkPolicy in global network namespace (%s) is not allowed (%s); ignoring firewall rules", ns, policy.Name)
		return
	}

	firewall := []firewallItem{}
	dnsFound := false
	for _, rule := range policy.Spec.Egress {
		if len(rule.To.CIDRSelector) > 0 {
			selector := rule.To.CIDRSelector
			if selector == "0.0.0.0/32" {
				// ovscontroller.go already logs a warning about this
				selector = "0.0.0.0/0"
			}
			_, cidr, err := net.ParseCIDR(selector)
			if err != nil {
				// should have been caught by validation
				glog.Errorf("illegal CIDR value %q in EgressNetworkPolicy rule for policy: %v", rule.To.CIDRSelector, policy.UID)
				continue
			}
			firewall = append(firewall, firewallItem{rule.Type, cidr})
		} else if len(rule.To.DNSName) > 0 {
			dnsFound = true
			cidrs := proxy.egressDNS.GetNetCIDRs(policy, rule.To.DNSName)
			for _, cidr := range cidrs {
				firewall = append(firewall, firewallItem{rule.Type, &cidr})
			}
		} else {
			// Should have been caught by validation
			glog.Errorf("invalid EgressNetworkPolicy rule: %v for policy: %v", rule, policy.UID)
		}
	}

	// Add/Update/Delete firewall rules for the namespace
	if len(firewall) > 0 {
		if _, ok := proxy.firewall[ns]; !ok {
			item := &proxyFirewallItem{}
			item.namespaceFirewalls = make(map[ktypes.UID][]firewallItem)
			item.activePolicy = nil
			proxy.firewall[ns] = item
		}
		proxy.firewall[ns].namespaceFirewalls[policy.UID] = firewall
	} else if _, ok := proxy.firewall[ns]; ok {
		delete(proxy.firewall[ns].namespaceFirewalls, policy.UID)
		if len(proxy.firewall[ns].namespaceFirewalls) == 0 {
			delete(proxy.firewall, ns)
		}
	}

	// Set active policy for the namespace
	if ref, ok := proxy.firewall[ns]; ok {
		if dnsFound {
			if err := CheckDNSResolver(); err != nil {
				ref.activePolicy = nil
				glog.Errorf("DNS resolver failed: %v, dropping all firewall rules for namespace: %q", err, ns)
				return
			}
		}

		if len(ref.namespaceFirewalls) == 1 {
			for uid := range ref.namespaceFirewalls {
				ref.activePolicy = &uid
				glog.Infof("Applied firewall egress network policy: %q to namespace: %q", uid, ns)
			}
		} else {
			ref.activePolicy = nil
			// We only allow one policy per namespace otherwise it's hard to determine which policy to apply first
			glog.Errorf("Found multiple egress policies, dropping all firewall rules for namespace: %q", ns)
		}
	}
}

func (proxy *OsdnProxy) firewallBlocksIP(namespace string, ip net.IP) bool {
	if ref, ok := proxy.firewall[namespace]; ok {
		if ref.activePolicy == nil {
			// Block all connections if active policy is not set
			return true
		}

		for _, item := range ref.namespaceFirewalls[*ref.activePolicy] {
			if item.net.Contains(ip) {
				return item.ruleType == osapi.EgressNetworkPolicyRuleDeny
			}
		}
	}
	return false
}

func (proxy *OsdnProxy) OnEndpointsUpdate(allEndpoints []*kapi.Endpoints) {
	proxy.lock.Lock()
	defer proxy.lock.Unlock()
	proxy.allEndpoints = allEndpoints
	proxy.updateEndpoints()
}

func (proxy *OsdnProxy) updateEndpoints() {
	if len(proxy.firewall) == 0 {
		proxy.baseEndpointsHandler.OnEndpointsUpdate(proxy.allEndpoints)
		return
	}

	filteredEndpoints := make([]*kapi.Endpoints, 0, len(proxy.allEndpoints))

EndpointLoop:
	for _, ep := range proxy.allEndpoints {
		ns := ep.ObjectMeta.Namespace
		for _, ss := range ep.Subsets {
			for _, addr := range ss.Addresses {
				IP := net.ParseIP(addr.IP)
				if !proxy.networkInfo.ClusterNetwork.Contains(IP) && !proxy.networkInfo.ServiceNetwork.Contains(IP) {
					if proxy.firewallBlocksIP(ns, IP) {
						glog.Warningf("Service '%s' in namespace '%s' has an Endpoint pointing to firewalled destination (%s)", ep.ObjectMeta.Name, ns, addr.IP)
						continue EndpointLoop
					}
				}
			}
		}
		filteredEndpoints = append(filteredEndpoints, ep)
	}

	proxy.baseEndpointsHandler.OnEndpointsUpdate(filteredEndpoints)
}

func (proxy *OsdnProxy) syncEgressDNSProxyFirewall() {
	policies, err := proxy.osClient.EgressNetworkPolicies(kapi.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		glog.Errorf("Could not get EgressNetworkPolicies: %v", err)
		return
	}

	go utilwait.Forever(proxy.egressDNS.Sync, 0)

	for {
		policyUpdates := <-proxy.egressDNS.updates
		glog.V(5).Infof("Egress dns sync: update proxy firewall for policy: %v", policyUpdates.UID)

		policy, ok := getPolicy(policyUpdates.UID, policies)
		if !ok {
			policies, err = proxy.osClient.EgressNetworkPolicies(kapi.NamespaceAll).List(metav1.ListOptions{})
			if err != nil {
				glog.Errorf("Failed to update proxy firewall for policy: %v, Could not get EgressNetworkPolicies: %v", policyUpdates.UID, err)
				continue
			}

			policy, ok = getPolicy(policyUpdates.UID, policies)
			if !ok {
				glog.Warningf("Unable to update proxy firewall for policy: %v, policy not found", policyUpdates.UID)
				continue
			}
		}

		proxy.lock.Lock()
		defer proxy.lock.Unlock()

		proxy.updateEgressNetworkPolicy(policy)
		if proxy.allEndpoints != nil {
			proxy.updateEndpoints()
		}
	}
}

func getPolicy(policyUID ktypes.UID, policies *osapi.EgressNetworkPolicyList) (osapi.EgressNetworkPolicy, bool) {
	for _, p := range policies.Items {
		if p.UID == policyUID {
			return p, true
		}
	}
	return osapi.EgressNetworkPolicy{}, false
}
