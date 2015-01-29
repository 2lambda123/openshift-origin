package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type FakePolicyBindings struct {
	Fake *Fake
}

func (c *FakePolicyBindings) List(label, field labels.Selector) (*authorizationapi.PolicyBindingList, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-policyBindings"})
	return &authorizationapi.PolicyBindingList{}, nil
}

func (c *FakePolicyBindings) Get(name string) (*authorizationapi.PolicyBinding, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-policyBinding"})
	return &authorizationapi.PolicyBinding{}, nil
}

func (c *FakePolicyBindings) Create(policyBinding *authorizationapi.PolicyBinding) (*authorizationapi.PolicyBinding, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-policyBinding", Value: policyBinding})
	return &authorizationapi.PolicyBinding{}, nil
}

func (c *FakePolicyBindings) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-policyBinding", Value: name})
	return nil
}
