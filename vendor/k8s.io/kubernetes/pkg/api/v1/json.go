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

package v1

import (
	"encoding/json"
	"reflect"
)

func (_ EndpointSubsetList) MarshalJSONSchema() reflect.Type {
	return reflect.TypeOf([]EndpointSubset{})
}
func (l EndpointSubsetList) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]EndpointSubset(l))
}

func (_ LimitRangeItemList) MarshalJSONSchema() reflect.Type {
	return reflect.TypeOf([]LimitRangeItem{})
}
func (l LimitRangeItemList) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]LimitRangeItem(l))
}

func (_ VolumeProjectionList) MarshalJSONSchema() reflect.Type {
	return reflect.TypeOf([]VolumeProjection{})
}
func (l VolumeProjectionList) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]VolumeProjection(l))
}
