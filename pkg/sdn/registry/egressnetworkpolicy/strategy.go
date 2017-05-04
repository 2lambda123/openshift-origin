package egressnetworkpolicy

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/api/validation"
)

// enpStrategy implements behavior for EgressNetworkPolicies
type enpStrategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating EgressNetworkPolicy
// objects via the REST API.
var Strategy = enpStrategy{kapi.Scheme}

func (enpStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {}

// NamespaceScoped is true for egress network policy
func (enpStrategy) NamespaceScoped() bool {
	return true
}

func (enpStrategy) GenerateName(base string) string {
	return base
}

func (enpStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
}

// Canonicalize normalizes the object after validation.
func (enpStrategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new egress network policy
func (enpStrategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateEgressNetworkPolicy(obj.(*api.EgressNetworkPolicy))
}

// AllowCreateOnUpdate is false for egress network policies
func (enpStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (enpStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for a EgressNetworkPolicy
func (enpStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateEgressNetworkPolicyUpdate(obj.(*api.EgressNetworkPolicy), old.(*api.EgressNetworkPolicy))
}

// GetAttrs returns labels and fields of a given object for filtering purposes
func GetAttrs(o runtime.Object) (labels.Set, fields.Set, error) {
	obj, ok := o.(*api.EgressNetworkPolicy)
	if !ok {
		return nil, nil, fmt.Errorf("not an EgressNetworkPolicy")
	}
	return labels.Set(obj.Labels), SelectableFields(obj), nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *api.EgressNetworkPolicy) fields.Set {
	return api.EgressNetworkPolicyToSelectableFields(obj)
}
