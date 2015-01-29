package policy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Registry is an interface for things that know how to store Policies.
type Registry interface {
	// ListPolicies obtains list of policys that match a selector.
	ListPolicies(ctx kapi.Context, labels, fields klabels.Selector) (*authorizationapi.PolicyList, error)
	// GetPolicy retrieves a specific policy.
	GetPolicy(ctx kapi.Context, id string) (*authorizationapi.Policy, error)
	// CreatePolicy creates a new policy.
	CreatePolicy(ctx kapi.Context, policy *authorizationapi.Policy) error
	// UpdatePolicy updates a policy.
	UpdatePolicy(ctx kapi.Context, policy *authorizationapi.Policy) error
	// DeletePolicy deletes a policy.
	DeletePolicy(ctx kapi.Context, id string) error
}
