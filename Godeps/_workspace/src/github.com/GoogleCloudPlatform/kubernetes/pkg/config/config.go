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

package config

import (
	"fmt"

	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

// ClientFunc returns the RESTClient defined for given resource
type ClientFunc func(mapping *meta.RESTMapping) (*client.RESTClient, error)

// CreateObjects creates bulk of resources provided by items list. Each item must
// be valid API type. It requires ObjectTyper to parse the Version and Kind and
// RESTMapper to get the resource URI and REST client that knows how to create
// given type
func CreateObjects(typer runtime.ObjectTyper, mapper meta.RESTMapper, clientFor ClientFunc, objects []runtime.Object) util.ErrorList {
	allErrors := util.ErrorList{}
	for i, obj := range objects {
		version, kind, err := typer.ObjectVersionAndKind(obj)
		if err != nil {
			allErrors = append(allErrors, fmt.Errorf("Config.item[%d] kind: %v", i, err))
			continue
		}

		mapping, err := mapper.RESTMapping(version, kind)
		if err != nil {
			allErrors = append(allErrors, fmt.Errorf("Config.item[%d] mapping: %v", i, err))
			continue
		}

		client, err := clientFor(mapping)
		if err != nil {
			allErrors = append(allErrors, fmt.Errorf("Config.item[%d] client: %v", i, err))
			continue
		}

		if err := CreateObject(client, mapping, obj); err != nil {
			allErrors = append(allErrors, fmt.Errorf("Config.item[%d]: %v", i, err))
		}
	}

	return allErrors
}

// CreateObject creates the obj using the provided clients and the resource URI
// mapping. It reports ValidationError when the object is missing the Metadata
// or the Name and it will report any error occured during create REST call
func CreateObject(client *client.RESTClient, mapping *meta.RESTMapping, obj runtime.Object) *errs.ValidationError {
	name, err := mapping.MetadataAccessor.Name(obj)
	if err != nil || name == "" {
		return errs.NewFieldRequired("name", err)
	}

	namespace, err := mapping.Namespace(obj)
	if err != nil {
		return errs.NewFieldRequired("namespace", err)
	}

	// TODO: This should be using RESTHelper
	err = client.Post().Path(mapping.Resource).Namespace(namespace).Body(obj).Do().Error()
	if err != nil {
		return errs.NewFieldInvalid(name, obj, err.Error())
	}

	return nil
}
