/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fake

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
	api "k8s.io/kubernetes/pkg/api"
)

// FakeSecurityContextConstraints implements SecurityContextConstraintsInterface
type FakeSecurityContextConstraints struct {
	Fake *FakeCore
}

var securitycontextconstraintsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "securitycontextconstraints"}

func (c *FakeSecurityContextConstraints) Create(securityContextConstraints *api.SecurityContextConstraints) (result *api.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(securitycontextconstraintsResource, securityContextConstraints), &api.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.SecurityContextConstraints), err
}

func (c *FakeSecurityContextConstraints) Update(securityContextConstraints *api.SecurityContextConstraints) (result *api.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(securitycontextconstraintsResource, securityContextConstraints), &api.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.SecurityContextConstraints), err
}

func (c *FakeSecurityContextConstraints) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(securitycontextconstraintsResource, name), &api.SecurityContextConstraints{})
	return err
}

func (c *FakeSecurityContextConstraints) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(securitycontextconstraintsResource, listOptions)

	_, err := c.Fake.Invokes(action, &api.SecurityContextConstraintsList{})
	return err
}

func (c *FakeSecurityContextConstraints) Get(name string, options v1.GetOptions) (result *api.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(securitycontextconstraintsResource, name), &api.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.SecurityContextConstraints), err
}

func (c *FakeSecurityContextConstraints) List(opts v1.ListOptions) (result *api.SecurityContextConstraintsList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(securitycontextconstraintsResource, opts), &api.SecurityContextConstraintsList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.SecurityContextConstraintsList{}
	for _, item := range obj.(*api.SecurityContextConstraintsList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested securityContextConstraints.
func (c *FakeSecurityContextConstraints) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(securitycontextconstraintsResource, opts))
}

// Patch applies the patch and returns the patched securityContextConstraints.
func (c *FakeSecurityContextConstraints) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(securitycontextconstraintsResource, name, data, subresources...), &api.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.SecurityContextConstraints), err
}
