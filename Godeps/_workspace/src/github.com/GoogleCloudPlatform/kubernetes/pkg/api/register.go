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

package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

// Scheme is the default instance of runtime.Scheme to which types in the Kubernetes API are already registered.
var Scheme = runtime.NewScheme()

func init() {
	Scheme.AddKnownTypes("",
		&PodList{},
		&Pod{},
		&ReplicationControllerList{},
		&ReplicationController{},
		&ServiceList{},
		&Service{},
		&MinionList{},
		&Minion{},
		&Status{},
		&ServerOpList{},
		&ServerOp{},
		&Endpoints{},
		&EndpointsList{},
		&Binding{},
		&Event{},
		&EventList{},
		&ContainerManifest{},
		&ContainerManifestList{},
		&BoundPod{},
		&BoundPods{},
	)
}

func (*Pod) IsAnAPIObject()                       {}
func (*PodList) IsAnAPIObject()                   {}
func (*ReplicationController) IsAnAPIObject()     {}
func (*ReplicationControllerList) IsAnAPIObject() {}
func (*Service) IsAnAPIObject()                   {}
func (*ServiceList) IsAnAPIObject()               {}
func (*Endpoints) IsAnAPIObject()                 {}
func (*EndpointsList) IsAnAPIObject()             {}
func (*Minion) IsAnAPIObject()                    {}
func (*MinionList) IsAnAPIObject()                {}
func (*Binding) IsAnAPIObject()                   {}
func (*Status) IsAnAPIObject()                    {}
func (*ServerOp) IsAnAPIObject()                  {}
func (*ServerOpList) IsAnAPIObject()              {}
func (*Event) IsAnAPIObject()                     {}
func (*EventList) IsAnAPIObject()                 {}
func (*ContainerManifest) IsAnAPIObject()         {}
func (*ContainerManifestList) IsAnAPIObject()     {}
func (*BoundPod) IsAnAPIObject()                  {}
func (*BoundPods) IsAnAPIObject()                 {}
