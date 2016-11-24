package nodes

import (
	"github.com/gonum/graph"

	kapi "k8s.io/kubernetes/pkg/api"
	kapps "k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/extensions"

	osgraph "github.com/openshift/origin/pkg/api/graph"
)

func EnsurePodNode(g osgraph.MutableUniqueGraph, pod *kapi.Pod) *PodNode {
	podNodeName := PodNodeName(pod)
	podNode := osgraph.EnsureUnique(g,
		podNodeName,
		func(node osgraph.Node) graph.Node {
			return &PodNode{node, pod}
		},
	).(*PodNode)

	podSpecNode := EnsurePodSpecNode(g, &pod.Spec, pod.Namespace, podNodeName)
	g.AddEdge(podNode, podSpecNode, osgraph.ContainsEdgeKind)

	return podNode
}

func EnsurePodSpecNode(g osgraph.MutableUniqueGraph, podSpec *kapi.PodSpec, namespace string, ownerName osgraph.UniqueName) *PodSpecNode {
	return osgraph.EnsureUnique(g,
		PodSpecNodeName(podSpec, ownerName),
		func(node osgraph.Node) graph.Node {
			return &PodSpecNode{node, podSpec, namespace, ownerName}
		},
	).(*PodSpecNode)
}

// EnsureServiceNode adds the provided service to the graph if it does not already exist.
func EnsureServiceNode(g osgraph.MutableUniqueGraph, svc *kapi.Service) *ServiceNode {
	return osgraph.EnsureUnique(g,
		ServiceNodeName(svc),
		func(node osgraph.Node) graph.Node {
			return &ServiceNode{node, svc, true}
		},
	).(*ServiceNode)
}

// FindOrCreateSyntheticServiceNode returns the existing service node or creates a synthetic node in its place
func FindOrCreateSyntheticServiceNode(g osgraph.MutableUniqueGraph, svc *kapi.Service) *ServiceNode {
	return osgraph.EnsureUnique(g,
		ServiceNodeName(svc),
		func(node osgraph.Node) graph.Node {
			return &ServiceNode{node, svc, false}
		},
	).(*ServiceNode)
}

func EnsureServiceAccountNode(g osgraph.MutableUniqueGraph, o *kapi.ServiceAccount) *ServiceAccountNode {
	return osgraph.EnsureUnique(g,
		ServiceAccountNodeName(o),
		func(node osgraph.Node) graph.Node {
			return &ServiceAccountNode{node, o, true}
		},
	).(*ServiceAccountNode)
}

func FindOrCreateSyntheticServiceAccountNode(g osgraph.MutableUniqueGraph, o *kapi.ServiceAccount) *ServiceAccountNode {
	return osgraph.EnsureUnique(g,
		ServiceAccountNodeName(o),
		func(node osgraph.Node) graph.Node {
			return &ServiceAccountNode{node, o, false}
		},
	).(*ServiceAccountNode)
}

func EnsureSecretNode(g osgraph.MutableUniqueGraph, o *kapi.Secret) *SecretNode {
	return osgraph.EnsureUnique(g,
		SecretNodeName(o),
		func(node osgraph.Node) graph.Node {
			return &SecretNode{
				Node:    node,
				Secret:  o,
				IsFound: true,
			}
		},
	).(*SecretNode)
}

func FindOrCreateSyntheticSecretNode(g osgraph.MutableUniqueGraph, o *kapi.Secret) *SecretNode {
	return osgraph.EnsureUnique(g,
		SecretNodeName(o),
		func(node osgraph.Node) graph.Node {
			return &SecretNode{
				Node:    node,
				Secret:  o,
				IsFound: false,
			}
		},
	).(*SecretNode)
}

// EnsureReplicationControllerNode adds a graph node for the ReplicationController if it does not already exist.
func EnsureReplicationControllerNode(g osgraph.MutableUniqueGraph, rc *kapi.ReplicationController) *ReplicationControllerNode {
	rcNodeName := ReplicationControllerNodeName(rc)
	rcNode := osgraph.EnsureUnique(g,
		rcNodeName,
		func(node osgraph.Node) graph.Node {
			return &ReplicationControllerNode{node, rc, true}
		},
	).(*ReplicationControllerNode)

	rcSpecNode := EnsureReplicationControllerSpecNode(g, &rc.Spec, rc.Namespace, rcNodeName)
	g.AddEdge(rcNode, rcSpecNode, osgraph.ContainsEdgeKind)

	return rcNode
}

func FindOrCreateSyntheticReplicationControllerNode(g osgraph.MutableUniqueGraph, rc *kapi.ReplicationController) *ReplicationControllerNode {
	return osgraph.EnsureUnique(g,
		ReplicationControllerNodeName(rc),
		func(node osgraph.Node) graph.Node {
			return &ReplicationControllerNode{node, rc, false}
		},
	).(*ReplicationControllerNode)
}

func EnsureReplicationControllerSpecNode(g osgraph.MutableUniqueGraph, rcSpec *kapi.ReplicationControllerSpec, namespace string, ownerName osgraph.UniqueName) *ReplicationControllerSpecNode {
	rcSpecName := ReplicationControllerSpecNodeName(rcSpec, ownerName)
	rcSpecNode := osgraph.EnsureUnique(g,
		rcSpecName,
		func(node osgraph.Node) graph.Node {
			return &ReplicationControllerSpecNode{node, rcSpec, namespace, ownerName}
		},
	).(*ReplicationControllerSpecNode)

	if rcSpec.Template != nil {
		ptSpecNode := EnsurePodTemplateSpecNode(g, rcSpec.Template, namespace, rcSpecName)
		g.AddEdge(rcSpecNode, ptSpecNode, osgraph.ContainsEdgeKind)
	}

	return rcSpecNode
}

func EnsurePodTemplateSpecNode(g osgraph.MutableUniqueGraph, ptSpec *kapi.PodTemplateSpec, namespace string, ownerName osgraph.UniqueName) *PodTemplateSpecNode {
	ptSpecName := PodTemplateSpecNodeName(ptSpec, ownerName)
	ptSpecNode := osgraph.EnsureUnique(g,
		ptSpecName,
		func(node osgraph.Node) graph.Node {
			return &PodTemplateSpecNode{node, ptSpec, namespace, ownerName}
		},
	).(*PodTemplateSpecNode)

	podSpecNode := EnsurePodSpecNode(g, &ptSpec.Spec, namespace, ptSpecName)
	g.AddEdge(ptSpecNode, podSpecNode, osgraph.ContainsEdgeKind)

	return ptSpecNode
}

func EnsurePersistentVolumeClaimNode(g osgraph.MutableUniqueGraph, pvc *kapi.PersistentVolumeClaim) *PersistentVolumeClaimNode {
	return osgraph.EnsureUnique(g,
		PersistentVolumeClaimNodeName(pvc),
		func(node osgraph.Node) graph.Node {
			return &PersistentVolumeClaimNode{Node: node, PersistentVolumeClaim: pvc, IsFound: true}
		},
	).(*PersistentVolumeClaimNode)
}

func FindOrCreateSyntheticPVCNode(g osgraph.MutableUniqueGraph, pvc *kapi.PersistentVolumeClaim) *PersistentVolumeClaimNode {
	return osgraph.EnsureUnique(g,
		PersistentVolumeClaimNodeName(pvc),
		func(node osgraph.Node) graph.Node {
			return &PersistentVolumeClaimNode{Node: node, PersistentVolumeClaim: pvc, IsFound: false}
		},
	).(*PersistentVolumeClaimNode)
}

func EnsureHorizontalPodAutoscalerNode(g osgraph.MutableUniqueGraph, hpa *autoscaling.HorizontalPodAutoscaler) *HorizontalPodAutoscalerNode {
	return osgraph.EnsureUnique(g,
		HorizontalPodAutoscalerNodeName(hpa),
		func(node osgraph.Node) graph.Node {
			return &HorizontalPodAutoscalerNode{Node: node, HorizontalPodAutoscaler: hpa}
		},
	).(*HorizontalPodAutoscalerNode)
}

func EnsurePetSetNode(g osgraph.MutableUniqueGraph, petset *kapps.PetSet) *PetSetNode {
	nodeName := PetSetNodeName(petset)
	node := osgraph.EnsureUnique(g,
		nodeName,
		func(node osgraph.Node) graph.Node {
			return &PetSetNode{node, petset}
		},
	).(*PetSetNode)

	specNode := EnsurePetSetSpecNode(g, &petset.Spec, petset.Namespace, nodeName)
	g.AddEdge(node, specNode, osgraph.ContainsEdgeKind)

	return node
}

func EnsurePetSetSpecNode(g osgraph.MutableUniqueGraph, spec *kapps.PetSetSpec, namespace string, ownerName osgraph.UniqueName) *PetSetSpecNode {
	specName := PetSetSpecNodeName(spec, ownerName)
	specNode := osgraph.EnsureUnique(g,
		specName,
		func(node osgraph.Node) graph.Node {
			return &PetSetSpecNode{node, spec, namespace, ownerName}
		},
	).(*PetSetSpecNode)

	ptSpecNode := EnsurePodTemplateSpecNode(g, &spec.Template, namespace, specName)
	g.AddEdge(specNode, ptSpecNode, osgraph.ContainsEdgeKind)

	return specNode
}

func EnsureDeploymentSpecNode(g osgraph.MutableUniqueGraph, dSpec *extensions.DeploymentSpec, namespace string, ownerName osgraph.UniqueName) *DeploymentSpecNode {
	dSpecName := DeploymentSpecNodeName(dSpec, ownerName)
	dSpecNode := osgraph.EnsureUnique(g,
		dSpecName,
		func(node osgraph.Node) graph.Node {
			return &DeploymentSpecNode{node, dSpec, namespace, ownerName}
		},
	).(*DeploymentSpecNode)

	ptSpecNode := EnsurePodTemplateSpecNode(g, &dSpec.Template, namespace, dSpecName)
	g.AddEdge(dSpecNode, ptSpecNode, osgraph.ContainsEdgeKind)

	return dSpecNode
}

// EnsureDeploymentNode adds the provided deployment to the graph if it does not exist
func EnsureDeploymentNode(g osgraph.MutableUniqueGraph, d *extensions.Deployment) *DeploymentNode {
	dName := DeploymentNodeName(d)
	dNode := osgraph.EnsureUnique(
		g,
		dName,
		func(node osgraph.Node) graph.Node {
			return &DeploymentNode{Node: node, Deployment: d, IsFound: true}
		},
	).(*DeploymentNode)

	dSpecNode := EnsureDeploymentSpecNode(g, &d.Spec, d.Namespace, dName)
	g.AddEdge(dNode, dSpecNode, osgraph.ContainsEdgeKind)

	return dNode
}

func EnsureReplicaSetSpecNode(g osgraph.MutableUniqueGraph, rsSpec *extensions.ReplicaSetSpec, namespace string, ownerName osgraph.UniqueName) *ReplicaSetSpecNode {
	rsSpecName := ReplicaSetSpecNodeName(rsSpec, ownerName)
	rsSpecNode := osgraph.EnsureUnique(g,
		rsSpecName,
		func(node osgraph.Node) graph.Node {
			return &ReplicaSetSpecNode{node, rsSpec, namespace, ownerName}
		},
	).(*ReplicaSetSpecNode)

	ptSpecNode := EnsurePodTemplateSpecNode(g, &rsSpec.Template, namespace, rsSpecName)
	g.AddEdge(rsSpecNode, ptSpecNode, osgraph.ContainsEdgeKind)

	return rsSpecNode
}

// EnsureReplicaSetNode adds a graph node for the ReplicaSet if it does not already exist.
func EnsureReplicaSetNode(g osgraph.MutableUniqueGraph, rs *extensions.ReplicaSet) *ReplicaSetNode {
	rsNodeName := ReplicaSetNodeName(rs)
	rsNode := osgraph.EnsureUnique(g,
		rsNodeName,
		func(node osgraph.Node) graph.Node {
			return &ReplicaSetNode{node, rs, true}
		},
	).(*ReplicaSetNode)

	rsSpecNode := EnsureReplicaSetSpecNode(g, &rs.Spec, rs.Namespace, rsNodeName)
	g.AddEdge(rsNode, rsSpecNode, osgraph.ContainsEdgeKind)

	return rsNode
}
