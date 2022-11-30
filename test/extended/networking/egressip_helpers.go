package networking

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	cloudnetworkv1 "github.com/openshift/api/cloudnetwork/v1"
	configv1 "github.com/openshift/api/config/v1"
	networkv1 "github.com/openshift/api/network/v1"
	routev1 "github.com/openshift/api/route/v1"
	cloudnetwork "github.com/openshift/client-go/cloudnetwork/clientset/versioned"
	networkclient "github.com/openshift/client-go/network/clientset/versioned/typed/network/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"

	imageutils "k8s.io/kubernetes/test/utils/image"
)

// Add EgressIP types (copy/paste) instead of vendoring them.
// See https://coreos.slack.com/archives/C01CQA76KMX/p1652187067459359?thread_ts=1652129799.456939&cid=C01CQA76KMX

// EgressIP is a CRD allowing the user to define a fixed
// source IP for all egress traffic originating from any pods which
// match the EgressIP resource according to its spec definition.
type EgressIP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of EgressIP.
	Spec EgressIPSpec `json:"spec"`
	// Observed status of EgressIP. Read-only.
	// +optional
	Status EgressIPStatus `json:"status,omitempty"`
}

// EgressIPStatus reports the EgressIP status.
type EgressIPStatus struct {
	// The list of assigned egress IPs and their corresponding node assignment.
	Items []EgressIPStatusItem `json:"items"`
}

// EgressIPStatusItem reports the per node status, for those egress IPs who have been assigned.
type EgressIPStatusItem struct {
	// Assigned node name
	Node string `json:"node"`
	// Assigned egress IP
	EgressIP string `json:"egressIP"`
}

// EgressIPSpec is a desired state description of EgressIP.
type EgressIPSpec struct {
	// EgressIPs is the list of egress IP addresses requested. Can be IPv4 and/or IPv6.
	// This field is mandatory.
	EgressIPs []string `json:"egressIPs"`
	// NamespaceSelector applies the egress IP only to the namespace(s) whose label
	// matches this definition. This field is mandatory.
	NamespaceSelector metav1.LabelSelector `json:"namespaceSelector"`
	// PodSelector applies the egress IP only to the pods whose label
	// matches this definition. This field is optional, and in case it is not set:
	// results in the egress IP being applied to all pods in the namespace(s)
	// matched by the NamespaceSelector. In case it is set: is intersected with
	// the NamespaceSelector, thus applying the egress IP to the pods
	// (in the namespace(s) already matched by the NamespaceSelector) which
	// match this pod selector.
	// +optional
	PodSelector metav1.LabelSelector `json:"podSelector,omitempty"`
}

// EgressIPList is the list of EgressIPList.
type EgressIPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// List of EgressIP.
	Items []EgressIP `json:"items"`
}

var (
	// GroupVersionResource of EgressIP
	egressIPGvr = schema.GroupVersionResource{
		Group:    "k8s.ovn.org",
		Version:  "v1",
		Resource: "egressips",
	}

	egressIPYamlTemplatePodAndNamespaceSelector = `apiVersion: k8s.ovn.org/v1
kind: EgressIP
metadata:
    name: %s
spec:
    egressIPs: %s
    podSelector:
        matchLabels:
            %s
    namespaceSelector:
        matchLabels:
            %s`

	egressIPYamlTemplateNamespaceSelector = `apiVersion: k8s.ovn.org/v1
kind: EgressIP
metadata:
    name: %s
spec:
    egressIPs: %s
    namespaceSelector:
        matchLabels:
            %s`
)

const (
	nodeEgressIPConfigAnnotationKey = "cloud.network.openshift.io/egress-ipconfig"
)

// TODO: make port allocator a singleton, shared among all test processes for egressip
// TODO: add an egressip allocator similar to the port allocator

type byName []corev1.Node

func (n byName) Len() int           { return len(n) }
func (n byName) Swap(i, j int)      { n[i], n[j] = n[j], n[i] }
func (n byName) Less(i, j int) bool { return n[i].Name < n[j].Name }

// findDefaultInterfaceForOpenShiftSDN returns the default interface for a node with the OpenShiftSDN plugin.
func findDefaultInterfaceForOpenShiftSDN(oc *exutil.CLI, nodeName string) (string, error) {
	var podName string
	var out string
	var err error

	type route struct {
		Dev string
	}
	var defaultRoutes []route

	out, err = runOcWithRetry(oc.AsAdmin(), "get",
		"pods",
		"-o", "name",
		"-n", "openshift-sdn",
		"--field-selector", fmt.Sprintf("spec.nodeName=%s", nodeName),
		"-l", "app=sdn")
	if err != nil {
		return "", err
	}
	outReader := bufio.NewScanner(strings.NewReader(out))
	re := regexp.MustCompile("^pod/(.*)")
	for outReader.Scan() {
		match := re.FindSubmatch([]byte(outReader.Text()))
		if len(match) != 2 {
			continue
		}
		podName = string(match[1])
		break
	}
	if podName == "" {
		return "", fmt.Errorf("Could not find a valid sdn pod on node '%s'", nodeName)
	}
	out, err = adminExecInPod(oc, "openshift-sdn", podName, "sdn", "ip -j route show default")
	if err != nil {
		return "", err
	}
	err = json.Unmarshal([]byte(out), &defaultRoutes)
	if err != nil {
		return "", err
	}
	if len(defaultRoutes) < 1 {
		return "", fmt.Errorf("Invalid default route configuration for node %s: %s", nodeName, out)
	}
	// Return the first default route in the list, ip route show default should correctly order routes
	// by metric.
	return defaultRoutes[0].Dev, nil
}

// adminExecInPod runs a command as admin in the provides pod inside the provided namespace.
func adminExecInPod(oc *exutil.CLI, namespace, pod, container, script string) (string, error) {
	var out string
	waitErr := wait.PollImmediate(1*time.Second, 3*time.Minute, func() (bool, error) {
		var err error
		out, err = runOcWithRetry(oc.AsAdmin(), "exec", pod, "-n", namespace, "-c", container, "--", "/bin/bash", "-c", script)
		return true, err
	})
	return out, waitErr
}

func getIngressDomain(oc *exutil.CLI) (string, error) {
	ic, err := oc.AdminOperatorClient().OperatorV1().IngressControllers("openshift-ingress-operator").Get(context.Background(), "default", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return ic.Status.Domain, nil
}

// createAgnhostDeploymentAndIngressRoute creates the route, service and deployment that will be used as
// a source for EgressIP tests. Returns the route name that can be queried to run queries against the source pods.
func createAgnhostDeploymentAndIngressRoute(oc *exutil.CLI, namespace, alias, ingressDomain string, replicas int, scheduleOnHosts []string) (string, string, error) {
	f := oc.KubeFramework()
	clientset := f.ClientSet

	targetPort := 8000
	if alias == "" {
		alias = "0"
	}
	namespaceAlias := fmt.Sprintf("%s-%s", namespace, alias)
	routeName := fmt.Sprintf("%s-route", namespaceAlias)
	routeHost := fmt.Sprintf("%s.%s", namespaceAlias, ingressDomain)
	serviceName := fmt.Sprintf("%s-service", namespaceAlias)
	deploymentName := fmt.Sprintf("%s-deployment", namespaceAlias)
	weight := int32(100)
	podLabels := map[string]string{
		"app": deploymentName,
	}
	// TODO: As soon as the framework switches to k8s.gcr.io/e2e-test-images/agnhost:2.36,
	// it would be nice to add:
	//		"--udp-port",
	//		"-1",
	// to disable UDP (which we don't use) for the agnhost binary.
	podCommand := []string{
		"/agnhost",
		"netexec",
		"--http-port",
		fmt.Sprintf("%d", targetPort),
	}
	replicaCount := int32(replicas)

	// create route
	routeDefinition := routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: namespace,
		},
		Spec: routev1.RouteSpec{
			Host: routeHost,
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromInt(targetPort),
			},
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   serviceName,
				Weight: &weight,
			},
		},
	}
	// we need to run this as admin because we manage several namespaces
	_, err := oc.AdminRouteClient().RouteV1().Routes(namespace).Create(context.TODO(), &routeDefinition, metav1.CreateOptions{})
	if err != nil {
		return "", "", err
	}

	// create service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": deploymentName,
			},
			Ports: []corev1.ServicePort{
				{
					Protocol: corev1.ProtocolTCP,
					Port:     int32(targetPort),
				},
			},
		},
	}
	_, err = clientset.CoreV1().Services(namespace).Create(
		context.Background(),
		service,
		metav1.CreateOptions{})
	if err != nil {
		return "", "", err
	}

	// create deployment
	nodeAffinity := v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/hostname",
								Operator: v1.NodeSelectorOpIn,
								Values:   scheduleOnHosts,
							},
						},
					},
				},
			},
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: podLabels},
			Replicas: &replicaCount,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": deploymentName,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    "agnhost",
							Image:   imageutils.GetE2EImage(imageutils.Agnhost),
							Command: podCommand,
						},
					},
					Affinity: &nodeAffinity,
				},
			},
		},
	}
	_, err = clientset.AppsV1().Deployments(namespace).Create(
		context.Background(),
		deployment,
		metav1.CreateOptions{})
	if err != nil {
		return "", "", err
	}

	// block until the deployment's pods are ready
	wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
		framework.Logf("Verifying if deployment %s is ready ...", deploymentName)
		d, err := clientset.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return d.Status.AvailableReplicas == *d.Spec.Replicas, nil
	})

	return deploymentName, routeHost, nil
}

// updateDeploymentAffinity updates the deployment's Affinity to match the scheduleOnHosts parameter and
// scales down and back up the replica count of the deployment.
func updateDeploymentAffinity(oc *exutil.CLI, namespace, deploymentName string, scheduleOnHosts []string) error {
	f := oc.KubeFramework()
	clientset := f.ClientSet

	// update deployment affinity
	nodeAffinity := v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/hostname",
								Operator: v1.NodeSelectorOpIn,
								Values:   scheduleOnHosts,
							},
						},
					},
				},
			},
		},
	}

	var currentReplicaNumber int32
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		framework.Logf("Updating deployment affinity and lowering replica count to 0")
		// get deployment
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(
			context.Background(),
			deploymentName,
			metav1.GetOptions{})
		if err != nil {
			return err
		}

		// update the affinity and lower the replica number to 0
		deployment.Spec.Template.Spec.Affinity = &nodeAffinity
		currentReplicaNumber = *deployment.Spec.Replicas
		deployment.Spec.Replicas = &currentReplicaNumber

		_, err = clientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
	if retryErr != nil {
		return fmt.Errorf("Update failed: %v", retryErr)
	}

	retryErr = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		framework.Logf("Increasing deployment replica count")
		// get deployment
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(
			context.Background(),
			deploymentName,
			metav1.GetOptions{})
		if err != nil {
			return err
		}

		// update the replica count back to what it used to be
		deployment.Spec.Replicas = &currentReplicaNumber

		_, err = clientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
	if retryErr != nil {
		return fmt.Errorf("Update failed: %v", retryErr)
	}

	// block until the deployment's pods are ready
	wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
		framework.Logf("Verifying if deployment %s is ready ...", deploymentName)
		d, err := clientset.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return d.Status.AvailableReplicas == *d.Spec.Replicas, nil
	})

	return nil
}

// NodeEgressIPConfiguration reports the node egress IP config.
// from github.com/openshift/cloud-network-config-controller/pkg/cloudprovider/cloudprovider.go
type NodeEgressIPConfiguration struct {
	Interface string   `json:"interface"`
	IFAddr    ifAddr   `json:"ifaddr"`
	Capacity  capacity `json:"capacity"`
}

type ifAddr struct {
	IPv4 string `json:"ipv4,omitempty"`
	IPv6 string `json:"ipv6,omitempty"`
}

type capacity struct {
	IPv4 int `json:"ipv4,omitempty"`
	IPv6 int `json:"ipv6,omitempty"`
	IP   int `json:"ip,omitempty"`
}

// findNodeEgressIPs will return a map available EgressIPs per node (map[<nodeName>]<egressIP>).
// The returned EgressIPs are chosen from the nodes' cloud.network.openshift.io/egress-ipconfig annotation and they
// depend on the current cloud type, on the currently used CloudPrivateIPConfigs, and on an internal reservation
// manager. A maximum of <egressIPsPerNode> number of IPs will be returned per node.
// Note: On most clouds, there is no dedicated CIDR per node and instead all EgressIPs come from a common pool.
// Thus, this is only an artificial assignment of EgressIP to node on these cloud platforms and the EgressIP feature
// will pick the actual node.
func findNodeEgressIPs(oc *exutil.CLI, clientset kubernetes.Interface, cloudNetworkClientset cloudnetwork.Interface,
	cloudType configv1.PlatformType, egressIPsPerNode int, nodeNames ...string) (map[string][]string, error) {
	// Get the node API objects corresponding to the node names.
	var nodeList []*v1.Node
	for _, nodeName := range nodeNames {
		node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		nodeList = append(nodeList, node)
	}

	// Build the list of reserved IPs. To do so, look at the currently used cloudprivateipconfigs
	// and egressips as well as nodes.
	var reservedIPs []string
	reservedIPs, err := buildReservedEgressIPList(oc, clientset, cloudNetworkClientset)
	if err != nil {
		return nil, err
	}

	// For each node, get the node's Egress IP range (annotation cloud.network.openshift.io/egress-ipconfig).
	// Then, get the first free suitable IP address(es) for this node and add the mapping <node name>:[<ip address>, <ip address>,...]
	// to the map.
	nodeEgressIPs := make(map[string][]string)
	for _, node := range nodeList {
		nodeEgressIPConfigs, err := getNodeEgressIPConfiguration(node)
		if err != nil {
			return nil, err
		}
		if l := len(nodeEgressIPConfigs); l != 1 {
			return nil, fmt.Errorf("Unexpected length of slice for node egress IP configuration: %d", l)
		}
		// TODO - not ready for dualstack (?)
		ipnetStr := nodeEgressIPConfigs[0].IFAddr.IPv4
		if ipnetStr == "" {
			ipnetStr = nodeEgressIPConfigs[0].IFAddr.IPv6
		}
		freeIPs, err := getFirstFreeIPs(ipnetStr, reservedIPs, cloudType, egressIPsPerNode)
		if err != nil {
			return nil, err
		}
		nodeEgressIPs[node.Name] = freeIPs
		// Most cloud environments such as GCP report a single, common CIDR for
		// EgresiIPs. Therefore, just add the IPs for this node to the reservedPool.
		for _, freeIP := range freeIPs {
			reservedIPs = append(reservedIPs, freeIP)
		}
	}

	return nodeEgressIPs, nil
}

// buildReservedEgressIPList builds the list of reserved IPs. To do so, look at the currently used cloudprivateipconfigs
// and egressips as well as the node IP addresses.
// Warning: Some cloud environments have a common CIDR for EgressIPs. In those environments, it is not possible to attribute
// a specific EgressIP to a specific node so this is just a "best effort" allocation and should be kept in mind when writing
// tests.
// TODO: add an internal reservation system based on a singleton to avoid race conditions during
// concurrent tests.
func buildReservedEgressIPList(oc *exutil.CLI, clientset kubernetes.Interface, cloudNetworkClientset cloudnetwork.Interface) ([]string, error) {
	var reservedIPs []string

	// cloudprivateipconfigs
	cloudPrivateIPConfigs, err := cloudNetworkClientset.CloudV1().CloudPrivateIPConfigs().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, cloudPrivateIPConfig := range cloudPrivateIPConfigs.Items {
		reservedIPs = append(reservedIPs, cloudPrivateIPConfig.Name)
	}

	// egressip for OVNKubernetes - if we receive a failure here, it may simply be because
	// we are on OpenShiftSDN, so ignore the error.
	egressipList, err := listEgressIPs(oc)
	if err == nil {
		for _, egressip := range egressipList.Items {
			for _, ip := range egressip.Spec.EgressIPs {
				reservedIPs = append(reservedIPs, ip)
			}
		}
	}
	// egressip for OpenShiftSDN - if we receive a failure here, it may simply be because
	// we are on OVNKubernetes, so ignore the error.
	networkClient := networkclient.NewForConfigOrDie(oc.AdminConfig())
	hostSubnets, err := networkClient.HostSubnets().List(context.Background(), metav1.ListOptions{})
	if err == nil {
		for _, hostSubnet := range hostSubnets.Items {
			for _, eip := range hostSubnet.EgressIPs {
				reservedIPs = append(reservedIPs, string(eip))
			}
		}
	}

	// node internal IP addresses
	nodes, err := clientset.CoreV1().Nodes().List(
		context.TODO(),
		metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				reservedIPs = append(reservedIPs, addr.Address)
			}
		}
	}

	return reservedIPs, nil
}

// getFirstFreeIPs returns the first available IP addresses from the IP network (CIDR notation). reservedIPs are
// eliminated from the choice and the cloudType is taken into account.
func getFirstFreeIPs(ipnetStr string, reservedIPs []string, cloudType configv1.PlatformType, egressIPsPerNode int) ([]string, error) {
	// Parse the CIDR notation and enumerate all IPs inside the subnet.
	_, ipnet, err := net.ParseCIDR(ipnetStr)
	if err != nil {
		return []string{}, err
	}
	ipList, err := SubnetIPs(*ipnet)
	if err != nil {
		return []string{}, err
	}

	// For AWS, skip the first 5 addresses:
	// https://stackoverflow.com/questions/64212709/how-do-i-assign-an-ec2-instance-to-a-fixed-ip-address-within-a-subnet
	// For Azure, skip the first 3 addresses:
	// https://docs.microsoft.com/en-us/azure/virtual-network/virtual-networks-faq
	// For GCP, the .1 address is reserved / already used.
	switch cloudType {
	case configv1.AWSPlatformType:
		if len(ipList) < 6 {
			return []string{}, fmt.Errorf("Cloud type is AWS, but there are less than 6 IPs available in the IP network %s", ipnetStr)
		}
		ipList = ipList[5 : len(ipList)-1]
	case configv1.AzurePlatformType:
		if len(ipList) < 5 {
			return []string{}, fmt.Errorf("Cloud type is Azure, but there are less than 4 IPs available in the IP network %s", ipnetStr)
		}
		ipList = ipList[4 : len(ipList)-1]
	case configv1.GCPPlatformType:
		if len(ipList) < 3 {
			return []string{}, fmt.Errorf("Cloud type is GCP, but there are less than 3 IPs available in the IP network %s", ipnetStr)
		}
		ipList = ipList[2 : len(ipList)-1]
	case configv1.OpenStackPlatformType:
		// For OpenStack as a heuristic use the last 32 IP addresses inside the subnet. We require the subnet to hold
		// at least 64 addresses so that we always end up at least at the lower half. That should be sufficiently safe
		// to avoid conflicts with infra IPs. In our CI tests, the OSP env usually spawns a 10.0.0.0/16 so we
		// should be totally safe here. The currently required allocations should be here, but let's play it safe nevertheless:
		// https://github.com/openshift/installer/blob/1884f8bda4ffbde7bc808900400aa62a7806fa21/pkg/types/openstack/defaults/platform.go#L30
		// https://github.com/openshift/installer/blob/1884f8bda4ffbde7bc808900400aa62a7806fa21/pkg/types/openstack/defaults/platform.go#L40
		// len(ipList)-1 will ignore the last element in the ipList which is the network's broadcast.
		if len(ipList) < 64 {
			return []string{}, fmt.Errorf("Cloud type is OpenStack, but there are less than 64 IPs available in the IP network %s", ipnetStr)
		}
		ipList = ipList[len(ipList)-32 : len(ipList)-1]
	default:
		// Skip the network address and the broadcast address
		ipList = ipList[1 : len(ipList)-1]
	}

	// Eliminate reserved IPs and return the first hits
	var freeIPList []string
outer:
	for _, ip := range ipList {
		for _, rip := range reservedIPs {
			if ip.String() == rip {
				continue outer
			}
		}

		freeIPList = append(freeIPList, ip.String())
		if len(freeIPList) >= egressIPsPerNode {
			return freeIPList, nil
		}
	}

	return freeIPList, fmt.Errorf(
		"Could not find requested number of suitable IPs ipnet: %s, reserved IPs %v. Only got %v",
		ipnetStr, reservedIPs, freeIPList)
}

// getNodeEgressIPConfiguration returns the parsed NodeEgressIPConfiguration for the node.
func getNodeEgressIPConfiguration(node *corev1.Node) ([]*NodeEgressIPConfiguration, error) {
	annotation, ok := node.Annotations[nodeEgressIPConfigAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("Cannot find annotation %s on node %s", nodeEgressIPConfigAnnotationKey, node)
	}

	var nodeEgressIPConfigs []*NodeEgressIPConfiguration
	err := json.Unmarshal([]byte(annotation), &nodeEgressIPConfigs)
	if err != nil {
		return nil, err
	}

	return nodeEgressIPConfigs, nil
}

// createProberPod creates a prober pod in the proberPodNamespace.
func createProberPod(oc *exutil.CLI, proberPodNamespace, proberPodName string) *v1.Pod {
	f := oc.KubeFramework()
	clientset := f.ClientSet

	return frameworkpod.CreateExecPodOrFail(clientset, proberPodNamespace, proberPodName, func(pod *corev1.Pod) {})
}

// destroyProberPod destroys the given proberPod.
func destroyProberPod(oc *exutil.CLI, proberPod *v1.Pod) error {
	if oc == nil {
		return fmt.Errorf("nil pointer to exutil.CLI oc was provided in destroyProberPod")
	}
	if proberPod == nil {
		return fmt.Errorf("nil pointer to proberPod was provided in destroyProberPod")
	}
	f := oc.KubeFramework()
	clientset := f.ClientSet

	// delete the exec pod again - in foreground, so that it blocks
	deletePolicy := metav1.DeletePropagationForeground
	return clientset.CoreV1().Pods(proberPod.Namespace).Delete(
		context.TODO(),
		proberPod.Name,
		metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		},
	)
}

// sendEgressIPProbesAndCheckOutput sends requests from the prober pod to the target and queries /clientip. Each
// query is sent in its own go function to speed up the total process.
// As soon as all requests were sent, parse the output and for each clientIP address that the target reported, count
// how often it occurred.
// Sum up the hits of all EgressIPs and make sure that the expectedHits number of requests were seen.
//
// Parameters:
// oc:             The CLI client.
// proberPod:      A pointer to the pod that sends the probes (this is not an EgressIP pod, instead, it dials
//                 into the route/service of the EgressIP pods).
// routeName:      Name of the routes to dial into the EgressIP pods' agnhost netexec.
// targetHost:     A host that resides outside of the cluster, target for EgressIP traffic.
// targetPort:     The target port of outbound EgressIP traffic.
// probeCount:     The number of probes to send.
// expectedHits:   The number of expected log hits (usually, either 0 or expectedHits == probeCount).
// egressIPSet:    Those are the EgressIPs that we are looking for. Expect sum(EgressIP clientIPs) == expectedHits.
func sendEgressIPProbesAndCheckOutput(oc *exutil.CLI, proberPod *corev1.Pod, routeName, targetHost string, targetPort,
	probeCount, expectedHits int, egressIPSet map[string]string) (bool, error) {

	if oc == nil {
		return false, fmt.Errorf("nil pointer to exutil.CLI oc was provided in sendProbesToHostPort")
	}
	if proberPod == nil {
		return false, fmt.Errorf("nil pointer to proberPod was provided in sendProbesToHostPort")
	}

	// Connect to the url, instruct the netexec server running on the other side to dial HTTP targetHost:targetPort
	// and to run the clientip request.
	requestStr := "clientip"
	targetProtocol := "http"
	request := fmt.Sprintf("http://%s/dial?protocol=%s&host=%s&port=%d&request=%s", routeName, targetProtocol,
		targetHost, targetPort, requestStr)
	clientIPs := make(map[string]int)
	var wg sync.WaitGroup
	outputChan := make(chan string, probeCount)
	for i := 0; i < probeCount; i++ {
		// Make sure that we don´t reuse the i variable when passing it to the go func.
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Randomize the start time a little bit per go routine.
			// Max of 250 ms * current iteration counter
			n := rand.Intn(250) * i
			framework.Logf("Sleeping for %d ms for iteration %d", n, i)
			time.Sleep(time.Duration(n) * time.Millisecond)

			output, err := runOcWithRetry(oc.AsAdmin(), "exec", proberPod.Name, "--", "curl", "--max-time", "15", "-s", request)
			if err != nil {
				framework.Logf("Query failed. Request: %s, Output: %s, Error: %v", request, output, err)
				return
			}
			outputChan <- output
		}()
	}
	wg.Wait()
	close(outputChan)

	for output := range outputChan {
		framework.Logf("Output is: %s", output)
		resp := struct {
			Responses []string
		}{}
		err := json.Unmarshal([]byte(output), &resp)
		if err != nil {
			framework.Logf("Could not unmarshal output %s", output)
			continue
		}
		if len(resp.Responses) != 1 {
			framework.Logf("Could not parse output %s", output)
			continue
		}
		ipPort := strings.Split(resp.Responses[0], ":")
		if len(ipPort) != 2 {
			framework.Logf("Could not parse output %s", output)
			continue
		}
		ip := ipPort[0]
		clientIPs[ip]++
	}

	numHits := 0
	for eip := range egressIPSet {
		numHits += clientIPs[eip]
	}
	framework.Logf("Reported clientIPs are: %v", clientIPs)
	if numHits != expectedHits {
		framework.Logf("Unexpected number of hits for EgressIPs %v, expected: %d, got: %d",
			egressIPSet, expectedHits, numHits)
		return false, nil
	}

	return true, nil
}

// PortAllocator is a simple class to allocate ports serially.
type PortAllocator struct {
	minPort, maxPort int
	reservedPorts    map[int]struct{}
	nextPort         int
	numAllocated     int
	mu               sync.Mutex
}

// NewPortAllocator initialized a new object of type PortAllocator and returns
// a pointer to that object.
func NewPortAllocator(minPort, maxPort int) *PortAllocator {
	pa := PortAllocator{
		minPort:       minPort,
		maxPort:       maxPort,
		reservedPorts: make(map[int]struct{}),
		nextPort:      minPort,
		numAllocated:  0,
	}
	return &pa
}

// AllocateNextPort will allocate a new port, serially. If the end of the range is
// reached, start over again at the start of the range and look for gaps.
func (p *PortAllocator) AllocateNextPort() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	rangeSize := p.maxPort - p.minPort
	if p.numAllocated > rangeSize {
		return -1, fmt.Errorf("No more free ports to allocate")
	}

	for i := 0; i < rangeSize; i++ {
		if p.nextPort > p.maxPort || p.nextPort < p.minPort {
			p.nextPort = p.minPort
		}
		if p.allocatePort(p.nextPort) == nil {
			return p.nextPort, nil
		}
		p.nextPort++
	}

	return -1, fmt.Errorf("cannot allocate new port after %d tries", rangeSize)
}

// ReleasePort will release the allocatoin for port <port>.
func (p *PortAllocator) ReleasePort(port int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.reservedPorts[port]; !ok {
		return fmt.Errorf("Chosen port %d is not allocated", port)
	}

	delete(p.reservedPorts, port)
	p.numAllocated--

	return nil
}

// isPortFree is a helper method. Not to be used by its own.
func (p *PortAllocator) isPortFree(port int) bool {
	_, ok := p.reservedPorts[port]
	return !ok
}

// allocatemPort is a helper method. Not to be used by its own.
func (p *PortAllocator) allocatePort(port int) error {
	if port < p.minPort || port > p.maxPort {
		return fmt.Errorf("Chosen port %d is not part of valid range %d - %d",
			port, p.minPort, p.maxPort)
	}

	if _, ok := p.reservedPorts[port]; ok {
		return fmt.Errorf("Chosen port %d is already reserved", port)
	}

	p.reservedPorts[port] = struct{}{}
	p.numAllocated++

	return nil
}

// deleteDaemonSet deletes the Daemonset <namespace>/<dsName>.
func deleteDaemonSet(clientset kubernetes.Interface, namespace, dsName string) error {
	deleteOptions := metav1.DeleteOptions{}
	if err := clientset.AppsV1().DaemonSets(namespace).Delete(context.TODO(), dsName, deleteOptions); err != nil {
		return fmt.Errorf("Failed to delete DaemonSet %s/%s: %v", namespace, dsName, err)
	}
	return nil
}

// createHostNetworkedDaemonSetAndProbe creates a host networked pod in namespace <namespace> on
// node <nodeName>. It will allocate a port to listen on and it will return
// the DaemonSet or an error.
func createHostNetworkedDaemonSet(clientset kubernetes.Interface, namespace, nodeName, daemonsetName string, containerPort int) (*appsv1.DaemonSet, error) {
	// TODO: As soon as the framework switches to k8s.gcr.io/e2e-test-images/agnhost:2.36,
	// it would be nice to add:
	//		"--udp-port",
	//		"-1",
	// to disable UDP (which we don't use) for the agnhost binary.
	// Also disable the UDP port reservation.
	podCommand := []string{
		"/agnhost",
		"netexec",
		"--udp-port",
		fmt.Sprintf("%d", containerPort),
		"--http-port",
		fmt.Sprintf("%d", containerPort),
	}

	podLabels := map[string]string{
		"app": daemonsetName,
	}
	nodeSelector := map[string]string{"kubernetes.io/hostname": nodeName}
	containerPorts := []v1.ContainerPort{
		{
			Name:          fmt.Sprintf("port-%d-tcp", containerPort),
			HostPort:      int32(containerPort),
			ContainerPort: int32(containerPort),
			Protocol:      v1.ProtocolTCP,
		},
		{
			Name:          fmt.Sprintf("port-%d-udp", containerPort),
			HostPort:      int32(containerPort),
			ContainerPort: int32(containerPort),
			Protocol:      v1.ProtocolUDP,
		},
	}
	readinessProbe := &v1.Probe{
		ProbeHandler: v1.ProbeHandler{
			HTTPGet: &v1.HTTPGetAction{
				Port: intstr.FromInt(int(containerPort)),
				Path: "/clientip",
			},
		},
	}

	dsDefinition := &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      daemonsetName,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: podLabels},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: corev1.PodSpec{
					NodeSelector: nodeSelector,
					HostNetwork:  true,
					Containers: []v1.Container{
						{
							Name:           daemonsetName,
							Image:          imageutils.GetE2EImage(imageutils.Agnhost),
							Command:        podCommand,
							Ports:          containerPorts,
							ReadinessProbe: readinessProbe,
						},
					},
				},
			},
		},
	}
	ds, err := clientset.AppsV1().DaemonSets(namespace).Create(context.TODO(), dsDefinition, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return ds, nil
}

// createHostNetworkedDaemonSetAndProbe creates a host networked pod in namespace <namespace> on
// node <nodeName>. It will allocate a port to listen on and it will return
// the DaemonSet or an error. It will probe the container and return an error such as the custom
// "Port conflict when creating pod" error message when the pod failed due to port binding issues.
func createHostNetworkedDaemonSetAndProbe(clientset kubernetes.Interface, namespace, nodeName, daemonsetName string, port, pollInterval, retries int) (*appsv1.DaemonSet, error) {
	targetDaemonset, err := createHostNetworkedDaemonSet(
		clientset,
		namespace,
		nodeName,
		daemonsetName,
		port,
	)
	if err != nil {
		return targetDaemonset, err
	}

	var ds *appsv1.DaemonSet
	for i := 0; i < retries; i++ {
		// Get the DS
		ds, err = clientset.AppsV1().DaemonSets(namespace).Get(context.TODO(), daemonsetName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		// Check if NumberReady == DesiredNumberScheduled.
		// In that case, simply return as all went well.
		if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
			return ds, nil
		}

		// Iterate over the pods (should only be one) and check if we couldn't spawn
		// because of duplicate assigned ports.
		// In case of a duplicate port conflict, we return an error message starting with
		// 'Port conflict when creating pod' so that other parts of the code can react to this.
		pods, err := clientset.CoreV1().Pods(namespace).List(
			context.TODO(),
			metav1.ListOptions{LabelSelector: labels.Set(ds.Spec.Selector.MatchLabels).String()})
		if err != nil {
			return nil, err
		}
		for _, pod := range pods.Items {
			hasPortConflict, err := podHasPortConflict(clientset, pod)
			if err != nil {
				return ds, err
			}
			if hasPortConflict {
				return ds, fmt.Errorf("Port conflict when creating pod %s/%s", namespace, pod.Name)
			}
		}

		// If no port conflict error was found, simply sleep for pollInterval and then
		// check again.
		time.Sleep(time.Duration(pollInterval) * time.Second)
	}

	// The DaemonSet is not ready, but this is not because of a port conflict.
	// This shouldn't happen and other parts of the code will likely report this error
	// as a CI failure.
	return ds, fmt.Errorf("Daemonset still not ready after %d tries", retries)
}

// podHasPortConflict scans the pod for a port conflict message and also scans the
// pod's logs for error messages that might indicate such a conflict.
func podHasPortConflict(clientset kubernetes.Interface, pod v1.Pod) (bool, error) {
	msg := "have free ports for the requested pod ports"
	if pod.Status.Phase == v1.PodPending {
		conditions := pod.Status.Conditions
		for _, condition := range conditions {
			if strings.Contains(condition.Message, msg) {
				return true, nil
			}

		}
	} else if pod.Status.Phase == v1.PodRunning {
		logOptions := corev1.PodLogOptions{}
		req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &logOptions)
		logs, err := req.Stream(context.TODO())
		if err != nil {
			return false, fmt.Errorf("Error in opening log stream")
		}
		defer logs.Close()

		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, logs)
		if err != nil {
			return false, fmt.Errorf("Error in copying info from pod logs to buffer")
		}
		logStr := buf.String()
		if strings.Contains(logStr, "address already in use") {
			return true, nil
		}
	}
	return false, nil
}

// getDaemonSetPodIPs returns the IPs of all pods in the DaemonSet.
func getDaemonSetPodIPs(clientset kubernetes.Interface, namespace, daemonsetName string) ([]string, error) {
	var ds *appsv1.DaemonSet
	var podIPs []string
	// Get the DS
	ds, err := clientset.AppsV1().DaemonSets(namespace).Get(context.TODO(), daemonsetName, metav1.GetOptions{})
	if err != nil {
		return []string{}, err
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(
		context.TODO(),
		metav1.ListOptions{LabelSelector: labels.Set(ds.Spec.Selector.MatchLabels).String()})
	if err != nil {
		return []string{}, err
	}
	for _, pod := range pods.Items {
		podIPs = append(podIPs, pod.Status.PodIP)
	}

	return podIPs, nil
}

// probeForClientIPs spawns a prober pod inside the prober namespace. It then runs curl against http://%s/dial?host=%s&port=%d&request=/clientip
// for the specified number of iterations and returns a set of the clientIP addresses that were returned.
// At the end of the test, the prober pod is deleted again.
func probeForClientIPs(oc *exutil.CLI, proberPodNamespace, proberPodName, url, targetIP string, targetPort, iterations int) (map[string]struct{}, error) {
	if oc == nil {
		return nil, fmt.Errorf("nil pointer to exutil.CLI oc was provided in SendProbesToHostPort")
	}

	f := oc.KubeFramework()
	clientset := f.ClientSet

	clientIPSet := make(map[string]struct{})

	proberPod := frameworkpod.CreateExecPodOrFail(clientset, proberPodNamespace, probePodName, func(pod *corev1.Pod) {
		// pod.ObjectMeta.Annotations = annotation
	})
	request := fmt.Sprintf("http://%s/dial?host=%s&port=%d&request=/clientip", url, targetIP, targetPort)
	maxTimeouts := 3
	for i := 0; i < iterations; i++ {
		output, err := runOcWithRetry(oc.AsAdmin(), "exec", "--", "curl", "-s", request)
		if err != nil {
			// if we hit an i/o timeout, retry
			if timeoutError, _ := regexp.Match("^Unable to connect to the server: dial tcp.*i/o timeout$", []byte(output)); timeoutError && maxTimeouts > 0 {
				framework.Logf("Query failed. Request: %s, Output: %s, Error: %v", request, output, err)
				iterations++
				maxTimeouts--
				continue
			}
			return nil, fmt.Errorf("Query failed. Request: %s, Output: %s, Error: %v", request, output, err)
		}
		dialResponse := &struct {
			Responses []string
		}{}
		err = json.Unmarshal([]byte(output), dialResponse)
		if err != nil {
			continue
		}
		if len(dialResponse.Responses) != 1 {
			continue
		}
		clientIPPort := strings.Split(dialResponse.Responses[0], ":")
		if len(clientIPPort) != 2 {
			continue
		}
		clientIP := clientIPPort[0]
		clientIPSet[clientIP] = struct{}{}
	}

	// delete the exec pod again - in foreground, so that it blocks
	deletePolicy := metav1.DeletePropagationForeground
	if err := clientset.CoreV1().Pods(proberPod.Namespace).Delete(context.TODO(), proberPod.Name, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		return nil, err
	}

	return clientIPSet, nil
}

// cloudPrivateIPConfigExists returns if a given ip was found as a cloudprivateipconfigs object
// and if it was assigned to a node as a separate value.
// Returns the following: exists bool, isAssigned bool, err error.
func cloudPrivateIPConfigExists(oc *exutil.CLI, cloudNetworkClientset cloudnetwork.Interface, ip string) (bool, bool, error) {
	cpic, err := cloudNetworkClientset.CloudV1().CloudPrivateIPConfigs().Get(context.Background(), ip, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("Error looking up cloudprivateipconfigs %s, err: %v", ip, err)
	}
	for _, c := range cpic.Status.Conditions {
		if c.Type == "Assigned" && c.Status == metav1.ConditionTrue {
			return true, true, nil
		}
	}

	return true, false, nil
}

// createCloudPrivateIpConfigoc creates a CloudPrivateIPConfig.
func createCloudPrivateIPConfig(cloudNetworkClientset cloudnetwork.Interface, ip, nodeName string) error {
	cpic := cloudnetworkv1.CloudPrivateIPConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: ip,
		},
		Spec: cloudnetworkv1.CloudPrivateIPConfigSpec{
			Node: nodeName,
		},
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := cloudNetworkClientset.CloudV1().CloudPrivateIPConfigs().Create(context.TODO(), &cpic, metav1.CreateOptions{})
		return err
	})
}

// deleteCloudPrivateIpConfigoc creates a CloudPrivateIPConfig.
func deleteCloudPrivateIPConfig(cloudNetworkClientset cloudnetwork.Interface, ip string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := cloudNetworkClientset.CloudV1().CloudPrivateIPConfigs().Delete(context.TODO(), ip, metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	})
}

// egressIPStatusHasIP returns if a given ip was found in a given EgressIP object's status field.
func egressIPStatusHasIP(oc *exutil.CLI, egressIPObjectName string, ip string) (bool, error) {
	eip, err := getEgressIP(oc, egressIPObjectName)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("Error looking up EgressIP %s, err: %v", egressIPObjectName, err)
	}
	for _, egressIPStatusItem := range eip.Status.Items {
		if egressIPStatusItem.EgressIP == ip {
			return true, nil
		}
	}

	return false, nil
}

// sdnNamespaceAddEgressIP adds EgressIP <egressip> to netnamespace <namespace>.
// oc patch netnamespace project1 --type=merge \  -p '{"egressIPs": ["192.168.1.100","192.168.1.101"]}'
func sdnNamespaceAddEgressIP(oc *exutil.CLI, namespace string, egressIP string) error {
	networkClient := networkclient.NewForConfigOrDie(oc.AdminConfig())
	netns, err := networkClient.NetNamespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}
	netns.EgressIPs = append(netns.EgressIPs, networkv1.NetNamespaceEgressIP(egressIP))
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = networkClient.NetNamespaces().Update(context.Background(), netns, metav1.UpdateOptions{})
		return err
	})
	if retryErr != nil {
		return fmt.Errorf("Update failed: %v", retryErr)
	}
	return nil
}

// sdnHostsubnetAddEgressIP adds EgressIP <egressIP> to hostsubnet <nodeName>.
// oc patch hostsubnet node1 --type=merge -p \'{"egressIPs": ["192.168.1.100", "192.168.1.101", "192.168.1.102"]}'
func sdnHostsubnetAddEgressIP(oc *exutil.CLI, nodeName string, egressIP string) error {
	networkClient := networkclient.NewForConfigOrDie(oc.AdminConfig())
	hostSubnet, err := networkClient.HostSubnets().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	hostSubnet.EgressIPs = append(hostSubnet.EgressIPs, networkv1.HostSubnetEgressIP(egressIP))
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = networkClient.HostSubnets().Update(context.Background(), hostSubnet, metav1.UpdateOptions{})
		return err
	})
	if retryErr != nil {
		return fmt.Errorf("Update failed: %v", retryErr)
	}
	return nil
}

// sdnNamespaceRemoveEgressIP removes EgressIP <egressip> to netnamespace <namespace>.
func sdnNamespaceRemoveEgressIP(oc *exutil.CLI, namespace string, egressIP string) error {
	networkClient := networkclient.NewForConfigOrDie(oc.AdminConfig())
	netns, err := networkClient.NetNamespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}
	var newEgressIPs []networkv1.NetNamespaceEgressIP
	for _, eip := range netns.EgressIPs {
		if eip != networkv1.NetNamespaceEgressIP(egressIP) {
			newEgressIPs = append(newEgressIPs, eip)
		}
	}
	netns.EgressIPs = newEgressIPs
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = networkClient.NetNamespaces().Update(context.Background(), netns, metav1.UpdateOptions{})
		return err
	})
	if retryErr != nil {
		return fmt.Errorf("Update failed: %v", retryErr)
	}
	return nil
}

// sdnHostsubnetRemoveEgressIP removes EgressIP <egressIP> to hostsubnet <nodeName>.
func sdnHostsubnetRemoveEgressIP(oc *exutil.CLI, nodeName string, egressIP string) error {
	networkClient := networkclient.NewForConfigOrDie(oc.AdminConfig())
	hostSubnet, err := networkClient.HostSubnets().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	var newEgressIPs []networkv1.HostSubnetEgressIP
	for _, eip := range hostSubnet.EgressIPs {
		if eip != networkv1.HostSubnetEgressIP(egressIP) {
			newEgressIPs = append(newEgressIPs, eip)
		}
	}
	hostSubnet.EgressIPs = newEgressIPs
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = networkClient.HostSubnets().Update(context.Background(), hostSubnet, metav1.UpdateOptions{})
		return err
	})
	if retryErr != nil {
		return fmt.Errorf("Update failed: %v", retryErr)
	}
	return nil
}

// sdnHostsubnetSetEgressCIDR sets EgressIPCIDR <egressCIDR> for hostsubnet <nodeName>.
func sdnHostsubnetSetEgressCIDR(oc *exutil.CLI, nodeName string, egressCIDR string) error {
	networkClient := networkclient.NewForConfigOrDie(oc.AdminConfig())
	hostSubnet, err := networkClient.HostSubnets().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	hostSubnet.EgressCIDRs = []networkv1.HostSubnetEgressCIDR{networkv1.HostSubnetEgressCIDR(egressCIDR)}
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = networkClient.HostSubnets().Update(context.Background(), hostSubnet, metav1.UpdateOptions{})
		return err
	})
	if retryErr != nil {
		return fmt.Errorf("Update failed: %v", retryErr)
	}
	return nil
}

// sdnHostsubnetFlushEgressIPs removes all EgressIPs from hostsubnet <nodeName>.
func sdnHostsubnetFlushEgressIPs(oc *exutil.CLI, nodeName string) error {
	networkClient := networkclient.NewForConfigOrDie(oc.AdminConfig())
	hostSubnet, err := networkClient.HostSubnets().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	hostSubnet.EgressIPs = []networkv1.HostSubnetEgressIP{}
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = networkClient.HostSubnets().Update(context.Background(), hostSubnet, metav1.UpdateOptions{})
		return err
	})
	if retryErr != nil {
		return fmt.Errorf("Update failed: %v", retryErr)
	}
	return nil
}

// sdnHostsubnetFlushEgressCIDRs removes all EgressCIDRs from hostsubnet <nodeName>.
func sdnHostsubnetFlushEgressCIDRs(oc *exutil.CLI, nodeName string) error {
	networkClient := networkclient.NewForConfigOrDie(oc.AdminConfig())
	hostSubnet, err := networkClient.HostSubnets().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	hostSubnet.EgressCIDRs = []networkv1.HostSubnetEgressCIDR{}
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = networkClient.HostSubnets().Update(context.Background(), hostSubnet, metav1.UpdateOptions{})
		return err
	})
	if retryErr != nil {
		return fmt.Errorf("Update failed: %v", retryErr)
	}
	return nil
}

// runOcWithRetry runs the oc command with up to 5 retries if a timeout error occurred while running the command.
func runOcWithRetry(oc *exutil.CLI, cmd string, args ...string) (string, error) {
	var err error
	var output string
	maxRetries := 5

	for numRetries := 0; numRetries < maxRetries; numRetries++ {
		if numRetries > 0 {
			framework.Logf("Retrying oc command (retry count=%v/%v)", numRetries+1, maxRetries)
		}

		output, err = oc.Run(cmd).Args(args...).Output()
		// If an error was found, either return the error, or retry if a timeout error was found.
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "i/o timeout") {
				// Retry on "i/o timeout" errors
				framework.Logf("Warning: oc command encountered i/o timeout.\nerr=%v\n)", err)
				continue
			}
			return output, err
		}
		// Break out of loop if no error.
		break
	}
	return output, err
}

// listEgressIPs uses the dynamic admin client to return a pointer to
// a list of existing EgressIPs, or error.
func listEgressIPs(oc *exutil.CLI) (*EgressIPList, error) {
	dynamic := oc.AdminDynamicClient()
	unstructured, err := dynamic.Resource(egressIPGvr).Namespace("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	egressipList := &EgressIPList{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), egressipList)
	if err != nil {
		return nil, err
	}
	return egressipList, nil
}

// getEgressIP uses the dynamic admin client to return a pointer to
// an existing EgressIP, or error.
func getEgressIP(oc *exutil.CLI, name string) (*EgressIP, error) {
	dynamic := oc.AdminDynamicClient()
	unstructured, err := dynamic.Resource(egressIPGvr).Namespace("").Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	egressip := &EgressIP{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), egressip)
	if err != nil {
		return nil, err
	}
	return egressip, nil
}

// addIPAddressToHost adds an IP address to the host's interface.
// The IP address will be added by means of a debug pod.
// When a valid namespace name is provided, use that namespace to spwan the debug pod.
// Otherwise, temporarily create a new namespace, add the IP address and tear down the new project right away.
func addIPAddressToHost(oc *exutil.CLI, namespace, host, intf, ip string) error {
	if namespace == "" {
		namespace = oc.SetupProject()
		defer func() {
			_, err := oc.AsAdmin().Run("delete").Args("namespace", namespace).Output()
			if err != nil {
				framework.Logf("addIPAddressToHost: could not delete namespace %s, err: %q", namespace, err)
			}
		}()
	}
	cmd := fmt.Sprintf("ip address add %s/32 dev %s", ip, intf)
	_, err := oc.AsAdmin().SetNamespace(namespace).Run("debug").Args("node/"+host, "--", "/bin/bash", "-c", cmd).Output()
	return err
}

// removeIPAddressFromHost adds an IP address to the host's interface.
// The IP address will be deleted by means of a debug pod.
// When a valid namespace name is provided, use that namespace to spawn the debug pod. If namespace == "": We cannot
// use the default namespace: due to pod security constraints, the debug pod cannot manipulate IP addresses. Therefore,
// temporarily create a new namespace, remove the IP address and tear down the new project right away.
// If the IP address cannot be found then ignore the error and return nil.
func removeIPAddressFromHost(oc *exutil.CLI, namespace, host, intf, ip string) error {
	if namespace == "" {
		namespace = oc.SetupProject()
		defer func() {
			_, err := oc.AsAdmin().Run("delete").Args("namespace", namespace).Output()
			if err != nil {
				framework.Logf("removeIPAddressFromHost: could not delete namespace %s, err: %q", namespace, err)
			}
		}()
	}
	cmd := fmt.Sprintf("ip address delete %s/32 dev %s", ip, intf)
	_, err := oc.AsAdmin().SetNamespace(namespace).Run("debug").Args("node/"+host, "--", "/bin/bash", "-c", cmd).Output()
	if err != nil && strings.Contains(err.Error(), "Cannot assign requested address") {
		return nil
	}
	return err
}
