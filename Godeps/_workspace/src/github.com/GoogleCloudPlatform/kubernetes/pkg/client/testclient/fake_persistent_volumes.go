/*
Copyright 2014 Google Inc. All rights reserved.

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

package testclient

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
)

type FakePersistentVolumes struct {
	Fake      *Fake
	Namespace string
}

func (c *FakePersistentVolumes) List(label labels.Selector, field fields.Selector) (*api.PersistentVolumeList, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-persistentVolumes"}, &api.PersistentVolumeList{})
	return obj.(*api.PersistentVolumeList), err
}

func (c *FakePersistentVolumes) Get(name string) (*api.PersistentVolume, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-persistentVolumes", Value: name}, &api.PersistentVolume{})
	return obj.(*api.PersistentVolume), err
}

func (c *FakePersistentVolumes) Delete(name string) error {
	_, err := c.Fake.Invokes(FakeAction{Action: "delete-persistentVolumes", Value: name}, &api.PersistentVolume{})
	return err
}

func (c *FakePersistentVolumes) Create(persistentVolumes *api.PersistentVolume) (*api.PersistentVolume, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-persistentVolumes"}, &api.PersistentVolume{})
	return obj.(*api.PersistentVolume), err
}

func (c *FakePersistentVolumes) Update(persistentVolumes *api.PersistentVolume) (*api.PersistentVolume, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "update-persistentVolumes", Value: persistentVolumes.Name}, &api.PersistentVolume{})
	return obj.(*api.PersistentVolume), err
}

func (c *FakePersistentVolumes) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-persistentVolumes", Value: resourceVersion})
	return c.Fake.Watch, c.Fake.Err
}
