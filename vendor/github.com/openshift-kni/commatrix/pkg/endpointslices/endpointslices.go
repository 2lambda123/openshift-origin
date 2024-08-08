package endpointslices

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift-kni/commatrix/pkg/client"
	"github.com/openshift-kni/commatrix/pkg/consts"
	"github.com/openshift-kni/commatrix/pkg/types"
)

type EndpointSlicesInfo struct {
	EndpointSlice discoveryv1.EndpointSlice
	Service       corev1.Service
	Pods          []corev1.Pod
}

type EndpointSlicesExporter struct {
	*client.ClientSet
	nodeToRole map[string]string
	sliceInfo  []EndpointSlicesInfo
}

func New(cs *client.ClientSet) (*EndpointSlicesExporter, error) {
	nodeList := &corev1.NodeList{}
	err := cs.List(context.TODO(), nodeList)
	if err != nil {
		return nil, err
	}

	nodeToRole := map[string]string{}
	for _, node := range nodeList.Items {
		nodeToRole[node.Name], err = types.GetNodeRole(&node)
		if err != nil {
			return nil, err
		}
	}

	return &EndpointSlicesExporter{cs, nodeToRole, []EndpointSlicesInfo{}}, nil
}

// load endpoint slices for services from type loadbalancer and node port only.
func (ep *EndpointSlicesExporter) LoadEndpointSlicesInfo() error {
	// get all the services
	servicesList := &corev1.ServiceList{}
	err := ep.List(context.TODO(), servicesList, &rtclient.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}
	epsliceInfos := []EndpointSlicesInfo{}
	for _, service := range servicesList.Items {
		// get the endpoint slice for this object
		epl := &discoveryv1.EndpointSliceList{}
		label, err := labels.Parse(fmt.Sprintf("kubernetes.io/service-name=%s", service.Name))
		if err != nil {
			return fmt.Errorf("failed to create selector for endpoint slice, %v", err)
		}
		err = ep.List(context.TODO(), epl, &rtclient.ListOptions{Namespace: service.Namespace, LabelSelector: label})
		if err != nil {
			return fmt.Errorf("failed to list endpoint slice, %v", err)
		}

		if len(epl.Items) == 0 {
			log.Debug("no endpoint slice found for service name", service.Name)
			continue
		}

		label = labels.SelectorFromSet(service.Spec.Selector)
		pods := &corev1.PodList{}
		err = ep.List(context.TODO(), pods, &rtclient.ListOptions{Namespace: service.Namespace, LabelSelector: label})
		if err != nil {
			return fmt.Errorf("failed to list pods, %v", err)
		}

		if len(pods.Items) == 0 {
			log.Debug("no pods found for service name", service.Name)
			continue
		}

		if !filterServiceTypes(service) && !filterHostNetwork(pods.Items[0]) {
			continue
		}

		epsliceInfo := createEPSliceInfo(service, epl.Items[0], pods.Items)
		log.Debug("epsliceInfo created", epsliceInfo)
		epsliceInfos = append(epsliceInfos, epsliceInfo)
	}

	log.Debug("length of the created epsliceInfos slice: ", len(epsliceInfos))
	ep.sliceInfo = epsliceInfos
	return nil
}

func (ep *EndpointSlicesExporter) ToComDetails() ([]types.ComDetails, error) {
	comDetails := make([]types.ComDetails, 0)

	for _, epSliceInfo := range ep.sliceInfo {
		cds, err := epSliceInfo.toComDetails(ep.nodeToRole)
		if err != nil {
			return nil, err
		}

		comDetails = append(comDetails, cds...)
	}

	cleanedComDetails := removeDups(comDetails)
	return cleanedComDetails, nil
}

func createEPSliceInfo(service corev1.Service, ep discoveryv1.EndpointSlice, pods []corev1.Pod) EndpointSlicesInfo {
	return EndpointSlicesInfo{
		EndpointSlice: ep,
		Service:       service,
		Pods:          pods,
	}
}

// getEndpointSliceNodeRoles gets endpointslice Info struct and returns which node roles the services are on.
func (ei *EndpointSlicesInfo) getEndpointSliceNodeRoles(nodesRoles map[string]string) []string {
	// map to prevent duplications
	rolesMap := make(map[string]bool)
	for _, endpoint := range ei.EndpointSlice.Endpoints {
		role := nodesRoles[*endpoint.NodeName]
		rolesMap[role] = true
	}

	roles := []string{}
	for k := range rolesMap {
		roles = append(roles, k)
	}

	return roles
}

func (ei *EndpointSlicesInfo) toComDetails(nodesRoles map[string]string) ([]types.ComDetails, error) {
	if len(ei.EndpointSlice.OwnerReferences) == 0 {
		return nil, fmt.Errorf("empty OwnerReferences in EndpointSlice %s/%s. skipping", ei.EndpointSlice.Namespace, ei.EndpointSlice.Name)
	}

	res := make([]types.ComDetails, 0)

	// Get the Namespace and Pod's name from the service.
	namespace := ei.Service.Namespace
	name, err := extractControllerName(&ei.Pods[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get pod name for endpointslice %s: %w", ei.EndpointSlice.Name, err)
	}

	// Get the node roles of this endpointslice.
	roles := ei.getEndpointSliceNodeRoles(nodesRoles)

	epSlice := ei.EndpointSlice
	optional := isOptional(epSlice)

	for _, port := range epSlice.Ports {
		containerName, err := getContainerName(int(*port.Port), ei.Pods)
		if err != nil {
			log.Warningf("failed to get container name for EndpointSlice %s/%s: %s", namespace, name, err)
			continue
		}

		for _, role := range roles {
			res = append(res, types.ComDetails{
				Direction: consts.IngressLabel,
				Protocol:  string(*port.Protocol),
				Port:      int(*port.Port),
				Namespace: namespace,
				Pod:       name,
				Container: containerName,
				NodeRole:  role,
				Service:   ei.Service.Namespace,
				Optional:  optional,
			})
		}
	}
	return res, nil
}

func getContainerName(portNum int, pods []corev1.Pod) (string, error) {
	if len(pods) == 0 {
		return "", fmt.Errorf("got empty pods slice")
	}

	res := ""
	pod := pods[0]
	found := false

	for i := 0; i < len(pod.Spec.Containers); i++ {
		container := pod.Spec.Containers[i]

		if found {
			break
		}

		for _, port := range container.Ports {
			if port.ContainerPort == int32(portNum) {
				res = container.Name
				found = true
				break
			}
		}
	}

	if !found {
		return "", fmt.Errorf("couldn't find port %d in pods", portNum)
	}

	return res, nil
}

func extractControllerName(pod *corev1.Pod) (string, error) {
	if len(pod.OwnerReferences) == 0 {
		return pod.Name, nil
	}

	ownerRefName := pod.OwnerReferences[0].Name
	switch pod.OwnerReferences[0].Kind {
	case "Node":
		res, found := strings.CutSuffix(pod.Name, fmt.Sprintf("-%s", pod.Spec.NodeName))
		if !found {
			return "", fmt.Errorf("pod name %s is not ending with node name %s", pod.Name, pod.Spec.NodeName)
		}
		return res, nil
	case "ReplicaSet":
		return ownerRefName[:strings.LastIndex(ownerRefName, "-")], nil
	case "DaemonSet":
		return ownerRefName, nil
	case "StatefulSet":
		return ownerRefName, nil
	case "ReplicationController":
		return ownerRefName, nil
	}

	return "", fmt.Errorf("failed to extract pod name for %s", pod.Name)
}

func isOptional(epSlice discoveryv1.EndpointSlice) bool {
	optional := false
	if _, ok := epSlice.Labels[consts.OptionalLabel]; ok {
		optional = true
	}

	return optional
}

func removeDups(comDetails []types.ComDetails) []types.ComDetails {
	set := sets.New[types.ComDetails](comDetails...)
	res := set.UnsortedList()

	return res
}
