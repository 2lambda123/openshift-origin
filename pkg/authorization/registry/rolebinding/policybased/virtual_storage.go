package policybased

import (
	"errors"
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationinterfaces "github.com/openshift/origin/pkg/authorization/interfaces"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	rolebindingregistry "github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type VirtualStorage struct {
	BindingRegistry policybindingregistry.Registry

	RuleResolver   rulevalidation.AuthorizationRuleResolver
	CreateStrategy rest.RESTCreateStrategy
	UpdateStrategy rest.RESTUpdateStrategy
}

// NewVirtualStorage creates a new REST for policies.
func NewVirtualStorage(bindingRegistry policybindingregistry.Registry, ruleResolver rulevalidation.AuthorizationRuleResolver) rolebindingregistry.Storage {
	return &VirtualStorage{
		BindingRegistry: bindingRegistry,

		RuleResolver:   ruleResolver,
		CreateStrategy: rolebindingregistry.LocalStrategy,
		UpdateStrategy: rolebindingregistry.LocalStrategy,
	}
}

func (m *VirtualStorage) New() runtime.Object {
	return &authorizationapi.RoleBinding{}
}
func (m *VirtualStorage) NewList() runtime.Object {
	return &authorizationapi.RoleBindingList{}
}

func (m *VirtualStorage) List(ctx kapi.Context, options *kapi.ListOptions) (runtime.Object, error) {
	policyBindingList, err := m.BindingRegistry.ListPolicyBindings(ctx, options)
	if err != nil {
		return nil, err
	}

	labelSelector, fieldSelector := oapi.ListOptionsToSelectors(options)

	roleBindingList := &authorizationapi.RoleBindingList{}
	for _, policyBinding := range policyBindingList.Items {
		for _, roleBinding := range policyBinding.RoleBindings {
			if labelSelector.Matches(labels.Set(roleBinding.Labels)) &&
				fieldSelector.Matches(authorizationapi.RoleBindingToSelectableFields(roleBinding)) {
				roleBindingList.Items = append(roleBindingList.Items, *roleBinding)
			}
		}
	}

	return roleBindingList, nil
}

func (m *VirtualStorage) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	policyBinding, err := m.getPolicyBindingOwningRoleBinding(ctx, name)
	if err != nil && kapierrors.IsNotFound(err) {
		return nil, kapierrors.NewNotFound(authorizationapi.Resource("rolebinding"), name)
	}
	if err != nil {
		return nil, err
	}

	binding, exists := policyBinding.RoleBindings[name]
	if !exists {
		return nil, kapierrors.NewNotFound(authorizationapi.Resource("rolebinding"), name)
	}
	return binding, nil
}

func (m *VirtualStorage) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	owningPolicyBinding, err := m.getPolicyBindingOwningRoleBinding(ctx, name)
	if err != nil && kapierrors.IsNotFound(err) {
		return nil, kapierrors.NewNotFound(authorizationapi.Resource("rolebinding"), name)
	}
	if err != nil {
		return nil, err
	}

	if _, exists := owningPolicyBinding.RoleBindings[name]; !exists {
		return nil, kapierrors.NewNotFound(authorizationapi.Resource("rolebinding"), name)
	}

	delete(owningPolicyBinding.RoleBindings, name)
	owningPolicyBinding.LastModified = unversioned.Now()

	if err := m.BindingRegistry.UpdatePolicyBinding(ctx, owningPolicyBinding); err != nil {
		return nil, err
	}

	return &unversioned.Status{Status: unversioned.StatusSuccess}, nil
}

func (m *VirtualStorage) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	return m.createRoleBinding(ctx, obj, false)
}

func (m *VirtualStorage) CreateRoleBindingWithEscalation(ctx kapi.Context, obj *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, error) {
	return m.createRoleBinding(ctx, obj, true)
}

func (m *VirtualStorage) createRoleBinding(ctx kapi.Context, obj runtime.Object, allowEscalation bool) (*authorizationapi.RoleBinding, error) {
	if err := rest.BeforeCreate(m.CreateStrategy, ctx, obj); err != nil {
		return nil, err
	}

	roleBinding := obj.(*authorizationapi.RoleBinding)

	if err := m.validateReferentialIntegrity(ctx, roleBinding); err != nil {
		return nil, err
	}
	if !allowEscalation {
		if err := m.confirmNoEscalation(ctx, roleBinding); err != nil {
			return nil, err
		}
	}

	policyBinding, err := m.getPolicyBindingForPolicy(ctx, roleBinding.RoleRef.Namespace, allowEscalation)
	if err != nil {
		return nil, err
	}

	_, exists := policyBinding.RoleBindings[roleBinding.Name]
	if exists {
		return nil, kapierrors.NewAlreadyExists(authorizationapi.Resource("rolebinding"), roleBinding.Name)
	}

	roleBinding.ResourceVersion = policyBinding.ResourceVersion
	policyBinding.RoleBindings[roleBinding.Name] = roleBinding
	policyBinding.LastModified = unversioned.Now()

	if err := m.BindingRegistry.UpdatePolicyBinding(ctx, policyBinding); err != nil {
		return nil, err
	}

	return roleBinding, nil
}

func (m *VirtualStorage) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return m.updateRoleBinding(ctx, obj, false)
}
func (m *VirtualStorage) UpdateRoleBindingWithEscalation(ctx kapi.Context, obj *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, bool, error) {
	return m.updateRoleBinding(ctx, obj, true)
}

func (m *VirtualStorage) updateRoleBinding(ctx kapi.Context, obj runtime.Object, allowEscalation bool) (*authorizationapi.RoleBinding, bool, error) {
	roleBinding, ok := obj.(*authorizationapi.RoleBinding)
	if !ok {
		return nil, false, kapierrors.NewBadRequest(fmt.Sprintf("obj is not a role: %#v", obj))
	}

	old, err := m.Get(ctx, roleBinding.Name)
	if err != nil {
		return nil, false, err
	}

	if err := rest.BeforeUpdate(m.UpdateStrategy, ctx, obj, old); err != nil {
		return nil, false, err
	}

	if err := m.validateReferentialIntegrity(ctx, roleBinding); err != nil {
		return nil, false, err
	}
	if !allowEscalation {
		if err := m.confirmNoEscalation(ctx, roleBinding); err != nil {
			return nil, false, err
		}
	}

	policyBinding, err := m.getPolicyBindingForPolicy(ctx, roleBinding.RoleRef.Namespace, allowEscalation)
	if err != nil {
		return nil, false, err
	}

	previousRoleBinding, exists := policyBinding.RoleBindings[roleBinding.Name]
	if !exists {
		return nil, false, kapierrors.NewNotFound(authorizationapi.Resource("rolebinding"), roleBinding.Name)
	}
	if previousRoleBinding.RoleRef != roleBinding.RoleRef {
		return nil, false, errors.New("roleBinding.RoleRef may not be modified")
	}

	roleBinding.ResourceVersion = policyBinding.ResourceVersion
	policyBinding.RoleBindings[roleBinding.Name] = roleBinding
	policyBinding.LastModified = unversioned.Now()

	if err := m.BindingRegistry.UpdatePolicyBinding(ctx, policyBinding); err != nil {
		return nil, false, err
	}
	return roleBinding, false, nil
}

func (m *VirtualStorage) validateReferentialIntegrity(ctx kapi.Context, roleBinding *authorizationapi.RoleBinding) error {
	if _, err := m.RuleResolver.GetRole(authorizationinterfaces.NewLocalRoleBindingAdapter(roleBinding)); err != nil {
		return err
	}

	return nil
}

func (m *VirtualStorage) confirmNoEscalation(ctx kapi.Context, roleBinding *authorizationapi.RoleBinding) error {
	modifyingRole, err := m.RuleResolver.GetRole(authorizationinterfaces.NewLocalRoleBindingAdapter(roleBinding))
	if err != nil {
		return err
	}

	return rulevalidation.ConfirmNoEscalation(ctx, m.RuleResolver, modifyingRole)
}

// ensurePolicyBindingToMaster returns a PolicyBinding object that has a PolicyRef pointing to the Policy in the passed namespace.
func (m *VirtualStorage) ensurePolicyBindingToMaster(ctx kapi.Context, policyNamespace, policyBindingName string) (*authorizationapi.PolicyBinding, error) {
	policyBinding, err := m.BindingRegistry.GetPolicyBinding(ctx, policyBindingName)
	if err != nil {
		if !kapierrors.IsNotFound(err) {
			return nil, err
		}

		// if we have no policyBinding, go ahead and make one.  creating one here collapses code paths below.  We only take this hit once
		policyBinding = policybindingregistry.NewEmptyPolicyBinding(kapi.NamespaceValue(ctx), policyNamespace, policyBindingName)
		if err := m.BindingRegistry.CreatePolicyBinding(ctx, policyBinding); err != nil {
			return nil, err
		}

		policyBinding, err = m.BindingRegistry.GetPolicyBinding(ctx, policyBindingName)
		if err != nil {
			return nil, err
		}
	}

	if policyBinding.RoleBindings == nil {
		policyBinding.RoleBindings = make(map[string]*authorizationapi.RoleBinding)
	}

	return policyBinding, nil
}

// getPolicyBindingForPolicy returns a PolicyBinding that points to the specified policyNamespace.  It will autocreate ONLY if policyNamespace equals the master namespace
func (m *VirtualStorage) getPolicyBindingForPolicy(ctx kapi.Context, policyNamespace string, allowAutoProvision bool) (*authorizationapi.PolicyBinding, error) {
	// we can autocreate a PolicyBinding object if the RoleBinding is for the master namespace OR if we've been explicity told to create the policying binding.
	// the latter happens during priming
	if (policyNamespace == "") || allowAutoProvision {
		return m.ensurePolicyBindingToMaster(ctx, policyNamespace, authorizationapi.GetPolicyBindingName(policyNamespace))
	}

	policyBinding, err := m.BindingRegistry.GetPolicyBinding(ctx, authorizationapi.GetPolicyBindingName(policyNamespace))
	if err != nil {
		return nil, err
	}

	if policyBinding.RoleBindings == nil {
		policyBinding.RoleBindings = make(map[string]*authorizationapi.RoleBinding)
	}

	return policyBinding, nil
}

func (m *VirtualStorage) getPolicyBindingOwningRoleBinding(ctx kapi.Context, bindingName string) (*authorizationapi.PolicyBinding, error) {
	policyBindingList, err := m.BindingRegistry.ListPolicyBindings(ctx, &kapi.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, policyBinding := range policyBindingList.Items {
		_, exists := policyBinding.RoleBindings[bindingName]
		if exists {
			return &policyBinding, nil
		}
	}

	return nil, kapierrors.NewNotFound(authorizationapi.Resource("rolebinding"), bindingName)
}
