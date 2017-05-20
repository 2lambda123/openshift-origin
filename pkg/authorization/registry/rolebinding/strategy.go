package rolebinding

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kstorage "k8s.io/apiserver/pkg/storage"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/api/validation"
)

// strategy implements behavior for nodes
type strategy struct {
	namespaced bool

	runtime.ObjectTyper
}

var ClusterStrategy = strategy{false, kapi.Scheme}
var LocalStrategy = strategy{true, kapi.Scheme}

// NamespaceScoped is false for rolebindings.
func (s strategy) NamespaceScoped() bool {
	return s.namespaced
}

// AllowCreateOnUpdate is false for rolebindings.
func (s strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) AllowUnconditionalUpdate() bool {
	return true
}

func (s strategy) GenerateName(base string) string {
	return kapi.SimpleNameGenerator.GenerateName(base)
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (s strategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	_ = obj.(*authorizationapi.RoleBinding)
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (s strategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	_ = obj.(*authorizationapi.RoleBinding)
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

// Validate validates a new role.
func (s strategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateRoleBinding(obj.(*authorizationapi.RoleBinding), s.namespaced)
}

// ValidateUpdate is the default update validation for an end user.
func (s strategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateRoleBindingUpdate(obj.(*authorizationapi.RoleBinding), old.(*authorizationapi.RoleBinding), s.namespaced)
}

func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	roleBinding, ok := obj.(*authorizationapi.RoleBinding)
	if !ok {
		return nil, nil, fmt.Errorf("not a rolebinding")
	}
	return labels.Set(roleBinding.ObjectMeta.Labels), authorizationapi.RoleBindingToSelectableFields(roleBinding), nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) kstorage.SelectionPredicate {
	return kstorage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}
