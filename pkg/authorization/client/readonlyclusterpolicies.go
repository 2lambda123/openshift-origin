package client

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// ClusterPoliciesReadOnlyInterface has methods to work with ClusterPolicies resources in a namespace
type ClusterPoliciesReadOnlyInterface interface {
	ReadOnlyClusterPolicies() ReadOnlyClusterPolicyInterface
}

// ReadOnlyClusterPolicyInterface exposes methods on ClusterPolicies resources
type ReadOnlyClusterPolicyInterface interface {
	List(label labels.Selector, field fields.Selector) (*authorizationapi.ClusterPolicyList, error)
	Get(name string) (*authorizationapi.ClusterPolicy, error)
}
