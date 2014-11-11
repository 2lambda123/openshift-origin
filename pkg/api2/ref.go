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
	"errors"
	"fmt"
	"regexp"

	"github.com/openshift/origin/pkg/api2/meta"
	"github.com/openshift/origin/pkg/runtime"
)

var ErrNilObject = errors.New("Can't reference a nil object")

var versionFromSelfLink = regexp.MustCompile("/api/([^/]*)/")

// GetReference returns an ObjectReference which refers to the given
// object, or an error if the object doesn't follow the conventions
// that would allow this.
func GetReference(obj runtime.Object) (*ObjectReference, error) {
	if obj == nil {
		return nil, ErrNilObject
	}
	meta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	_, kind, err := Scheme.ObjectVersionAndKind(obj)
	if err != nil {
		return nil, err
	}
	version := versionFromSelfLink.FindStringSubmatch(meta.SelfLink())
	if len(version) < 2 {
		return nil, fmt.Errorf("unexpected self link format: %v", meta.SelfLink())
	}
	return &ObjectReference{
		Kind:       kind,
		APIVersion: version[1],
		// TODO: correct Name and UID when TypeMeta makes a distinction
		Name:            meta.Name(),
		UID:             meta.UID(),
		ResourceVersion: meta.ResourceVersion(),
	}, nil
}
