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

package event

import (
	"path"

	api "github.com/GoogleCloudPlatform/kubernetes/pkg/api2"
	etcderr "github.com/GoogleCloudPlatform/kubernetes/pkg/api2/errors/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry2/generic"
	etcdgeneric "github.com/GoogleCloudPlatform/kubernetes/pkg/registry2/generic/etcd"
	runtime "github.com/GoogleCloudPlatform/kubernetes/pkg/runtime2"
	tools "github.com/GoogleCloudPlatform/kubernetes/pkg/tools2"
)

// registry implements custom changes to generic.Etcd.
type registry struct {
	*etcdgeneric.Etcd
	ttl uint64
}

// Create stores the object with a ttl, so that events don't stay in the system forever.
func (r registry) Create(ctx api.Context, id string, obj runtime.Object) error {
	err := r.Etcd.Helper.CreateObj(r.Etcd.KeyFunc(id), obj, r.ttl)
	return etcderr.InterpretCreateError(err, r.Etcd.EndpointName, id)
}

// NewEtcdRegistry returns a registry which will store Events in the given
// EtcdHelper. ttl is the time that Events will be retained by the system.
func NewEtcdRegistry(h tools.EtcdHelper, ttl uint64) generic.Registry {
	return registry{
		Etcd: &etcdgeneric.Etcd{
			NewFunc:      func() runtime.Object { return &api.Event{} },
			NewListFunc:  func() runtime.Object { return &api.EventList{} },
			EndpointName: "events",
			KeyRoot:      "/registry/events",
			KeyFunc: func(id string) string {
				return path.Join("/registry/events", id)
			},
			Helper: h,
		},
		ttl: ttl,
	}
}
