package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// FakeClusterRoleBindings implements ClusterRoleBindingInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterRoleBindings struct {
	Fake *Fake
}

var clusterRoleBindingsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "clusterrolebindings"}

func (c *FakeClusterRoleBindings) Get(name string, options metav1.GetOptions) (*authorizationapi.ClusterRoleBinding, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(clusterRoleBindingsResource, name), &authorizationapi.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) List(opts metainternal.ListOptions) (*authorizationapi.ClusterRoleBindingList, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(clusterRoleBindingsResource, optsv1), &authorizationapi.ClusterRoleBindingList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleBindingList), err
}

func (c *FakeClusterRoleBindings) Create(inObj *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(clusterRoleBindingsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) Update(inObj *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(clusterRoleBindingsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterRoleBinding), err
}

func (c *FakeClusterRoleBindings) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(clusterRoleBindingsResource, name), &authorizationapi.ClusterRoleBinding{})
	return err
}
