package cluster

// The purpose of this diagnostic is to detect nodes that are out of commission
// (which may affect the ability to schedule pods) for user awareness.

import (
	"errors"
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

const (
	clientErrorGettingNodes = `Client error while retrieving node records. Client retrieved records
during discovery, so this is likely to be a transient error. Try running
diagnostics again. If this message persists, there may be a permissions
problem with getting node records. The error was:

(%T) %[1]v`

	nodeNotReady = `Node {{.node}} is defined but is not marked as ready.
Ready status is {{.status}} because "{{.reason}}"
If the node is not intentionally disabled, check that the master can
reach the node hostname for a health check and the node is checking in
to the master with the same hostname.

While in this state, pods should not be scheduled to deploy on the node,
and any existing scheduled pods will be considered failed and removed.
`

	nodeNotSched = `Node {{.node}} is ready but is marked Unschedulable.
This is usually set manually for administrative reasons.
An administrator can mark the node schedulable with:
    oadm manage-node {{.node}} --schedulable=true

While in this state, pods should not be scheduled to deploy on the node.
Existing pods will continue to run until completed or evacuated (see
other options for 'oadm manage-node').
`
)

// NodeDefinitions is a Diagnostic for analyzing the nodes in a cluster.
type NodeDefinitions struct {
	KubeClient *kclient.Client
	OsClient   *osclient.Client
}

const NodeDefinitionsName = "NodeDefinitions"

func (d *NodeDefinitions) Name() string {
	return NodeDefinitionsName
}

func (d *NodeDefinitions) Description() string {
	return "Check node records on master"
}

func (d *NodeDefinitions) CanRun() (bool, error) {
	if d.KubeClient == nil || d.OsClient == nil {
		return false, errors.New("must have kube and os client")
	}
	can, err := adminCan(d.OsClient, kapi.NamespaceDefault, &authorizationapi.SubjectAccessReview{
		Verb:     "list",
		Resource: "nodes",
	})
	if err != nil {
		msg := log.Message{ID: "clGetNodesFailed", EvaluatedText: fmt.Sprintf(clientErrorGettingNodes, err)}
		return false, types.DiagnosticError{msg.ID, &msg, err}
	} else if !can {
		msg := log.Message{ID: "clGetNodesFailed", EvaluatedText: "Client does not have cluster-admin access and cannot see node records"}
		return false, types.DiagnosticError{msg.ID, &msg, err}
	}
	return true, nil
}

func (d *NodeDefinitions) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult("NodeDefinition")

	nodes, err := d.KubeClient.Nodes().List(labels.LabelSelector{}, fields.Everything())
	if err != nil {
		r.Errorf("clGetNodesFailed", err, clientErrorGettingNodes, err)
		return r
	}

	anyNodesAvail := false
	for _, node := range nodes.Items {
		var ready *kapi.NodeCondition
		for i, condition := range node.Status.Conditions {
			switch condition.Type {
			// Each condition appears only once. Currently there's only one... used to be more
			case kapi.NodeReady:
				ready = &node.Status.Conditions[i]
			}
		}

		if ready == nil || ready.Status != kapi.ConditionTrue {
			templateData := log.Hash{"node": node.Name}
			if ready == nil {
				templateData["status"] = "None"
				templateData["reason"] = "There is no readiness record."
			} else {
				templateData["status"] = ready.Status
				templateData["reason"] = ready.Reason
			}
			r.Warnt("clNodeNotReady", nil, nodeNotReady, templateData)
		} else if node.Spec.Unschedulable {
			r.Warnt("clNodeNotSched", nil, nodeNotSched, log.Hash{"node": node.Name})
		} else {
			anyNodesAvail = true
		}
	}
	if !anyNodesAvail {
		r.Error("clNoAvailNodes", nil, "There were no nodes available to use. No new pods can be scheduled.")
	}

	return r
}
