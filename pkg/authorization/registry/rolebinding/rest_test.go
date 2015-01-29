package rolebinding

import (
	"errors"
	"reflect"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/test"
	usertest "github.com/openshift/origin/pkg/user/registry/test"
)

func makeSimpleStorage() (*REST, *test.PolicyBindingRegistry) {
	bindingRegistry := &test.PolicyBindingRegistry{}
	policyRegistry := &test.PolicyRegistry{}
	policyRegistry.Policies = []authorizationapi.Policy{
		{
			ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName, Namespace: "master"},
			Roles: map[string]authorizationapi.Role{
				"admin": {ObjectMeta: kapi.ObjectMeta{Name: "admin"}},
			},
		}}
	userRegistry := &usertest.UserRegistry{}

	return &REST{bindingRegistry, policyRegistry, userRegistry, "master"}, bindingRegistry
}

func TestCreateValidationError(t *testing.T) {
	storage, _ := makeSimpleStorage()
	roleBinding := &authorizationapi.RoleBinding{}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, err := storage.Create(ctx, roleBinding)
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestCreateStorageError(t *testing.T) {
	storage, registry := makeSimpleStorage()
	registry.Err = errors.New("Sample Error")

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin", Namespace: "master"},
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	channel, err := storage.Create(ctx, roleBinding)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case r := <-channel:
		switch r := r.Object.(type) {
		case *kapi.Status:
			if r.Message == registry.Err.Error() {
				// expected case
			} else {
				t.Errorf("Got back unexpected error: %#v", r)
			}
		default:
			t.Errorf("Got back non-status result: %v", r)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestCreateValidAutoCreateMasterPolicyBindings(t *testing.T) {
	storage, _ := makeSimpleStorage()
	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin", Namespace: "master"},
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	channel, err := storage.Create(ctx, roleBinding)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case r := <-channel:
		switch r := r.Object.(type) {
		case *kapi.Status:
			t.Errorf("Got back unexpected status: %#v", r)
		case *authorizationapi.RoleBinding:
			// expected case
		default:
			t.Errorf("Got unexpected type: %#v", r)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestCreateValid(t *testing.T) {
	storage, registry := makeSimpleStorage()
	registry.PolicyBindings = append(make([]authorizationapi.PolicyBinding, 0),
		authorizationapi.PolicyBinding{
			ObjectMeta: kapi.ObjectMeta{Name: "master", Namespace: "unittest"},
		})

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin", Namespace: "master"},
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	channel, err := storage.Create(ctx, roleBinding)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case r := <-channel:
		switch r := r.Object.(type) {
		case *kapi.Status:
			t.Errorf("Got back unexpected status: %#v", r)
		case *authorizationapi.RoleBinding:
			// expected case
		default:
			t.Errorf("Got unexpected type: %#v", r)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestUpdate(t *testing.T) {
	storage, registry := makeSimpleStorage()
	registry.PolicyBindings = append(make([]authorizationapi.PolicyBinding, 0),
		authorizationapi.PolicyBinding{
			ObjectMeta: kapi.ObjectMeta{Name: "master", Namespace: "unittest"},
			RoleBindings: map[string]authorizationapi.RoleBinding{
				"my-roleBinding": {
					ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding"},
					RoleRef:    kapi.ObjectReference{Name: "admin", Namespace: "master"},
				},
			},
		})

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin", Namespace: "master"},
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	channel, err := storage.Update(ctx, roleBinding)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	select {
	case result := <-channel:
		switch obj := result.Object.(type) {
		case *kapi.Status:
			t.Errorf("Unexpected operation error: %v", obj)

		case *authorizationapi.RoleBinding:
			if !reflect.DeepEqual(roleBinding, obj) {
				t.Errorf("Updated roleBinding does not match input roleBinding."+
					" Expected: %v, Got: %v", roleBinding, obj)
			}
		default:
			t.Errorf("Unexpected result type: %v", result)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestUpdateError(t *testing.T) {
	storage, registry := makeSimpleStorage()
	registry.PolicyBindings = append(make([]authorizationapi.PolicyBinding, 0),
		authorizationapi.PolicyBinding{
			ObjectMeta: kapi.ObjectMeta{Name: "master", Namespace: "unittest"},
		})

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin", Namespace: "master"},
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, err := storage.Update(ctx, roleBinding)
	if err == nil {
		t.Errorf("Missing expected error")
		return
	}
	expectedErr := "roleBinding my-roleBinding does not exist"
	if err.Error() != expectedErr {
		t.Errorf("Expected %v, got %v", expectedErr, err)
	}
}

func TestDeleteError(t *testing.T) {
	registry := &test.PolicyBindingRegistry{}

	registry.Err = errors.New("Sample Error")
	policyRegistry := &test.PolicyRegistry{}
	userRegistry := &usertest.UserRegistry{}
	storage := &REST{registry, policyRegistry, userRegistry, "master"}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	channel, err := storage.Delete(ctx, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case r := <-channel:
		switch r := r.Object.(type) {
		case *kapi.Status:
			if r.Message == registry.Err.Error() {
				// expected case
			} else {
				t.Errorf("Got back unexpected error: %#v", r)
			}
		default:
			t.Errorf("Got back non-status result: %v", r)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestDeleteValid(t *testing.T) {
	registry := &test.PolicyBindingRegistry{}
	policyRegistry := &test.PolicyRegistry{}
	userRegistry := &usertest.UserRegistry{}
	storage := &REST{registry, policyRegistry, userRegistry, "master"}
	registry.PolicyBindings = append(make([]authorizationapi.PolicyBinding, 0),
		authorizationapi.PolicyBinding{
			ObjectMeta: kapi.ObjectMeta{Name: "master", Namespace: "unittest"},
			RoleBindings: map[string]authorizationapi.RoleBinding{
				"foo": {ObjectMeta: kapi.ObjectMeta{Name: "foo"}},
			},
		})

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	channel, err := storage.Delete(ctx, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case r := <-channel:
		switch r := r.Object.(type) {
		case *kapi.Status:
			if r.Status != "Success" {
				t.Fatalf("Got back non-success status: %#v", r)
			}
		default:
			t.Fatalf("Got back non-status result: %v", r)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}
