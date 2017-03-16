package user

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kstorage "k8s.io/apiserver/pkg/storage"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/api/validation"
)

// userStrategy implements behavior for Users
type userStrategy struct {
	runtime.ObjectTyper
}

// Strategy is the default logic that applies when creating and updating User
// objects via the REST API.
var Strategy = userStrategy{kapi.Scheme}

func (userStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {}

// NamespaceScoped is false for users
func (userStrategy) NamespaceScoped() bool {
	return false
}

func (userStrategy) GenerateName(base string) string {
	return base
}

func (userStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
}

// Validate validates a new user
func (userStrategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateUser(obj.(*api.User))
}

// AllowCreateOnUpdate is false for users
func (userStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (userStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// Canonicalize normalizes the object after validation.
func (userStrategy) Canonicalize(obj runtime.Object) {
}

// ValidateUpdate is the default update validation for an end user.
func (userStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateUserUpdate(obj.(*api.User), old.(*api.User))
}

// GetAttrs returns labels and fields of a given object for filtering purposes
func GetAttrs(o runtime.Object) (labels.Set, fields.Set, error) {
	obj, ok := o.(*api.User)
	if !ok {
		return nil, nil, fmt.Errorf("not a User")
	}
	return labels.Set(obj.Labels), SelectableFields(obj), nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) kstorage.SelectionPredicate {
	return kstorage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *api.User) fields.Set {
	return api.UserToSelectableFields(obj)
}
