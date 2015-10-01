/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package latest

import (
	"fmt"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	_ "k8s.io/kubernetes/pkg/apis/experimental"
	"k8s.io/kubernetes/pkg/apis/experimental/v1"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
)

var (
	Version  string
	Versions []string

	accessor   = meta.NewAccessor()
	Codec      runtime.Codec
	SelfLinker = runtime.SelfLinker(accessor)
	RESTMapper meta.RESTMapper
)

const importPrefix = "k8s.io/kubernetes/pkg/apis/experimental"

func init() {
	// hardcode these values to be correct for now.  The next rebase completely changes how these are registered
	Version = "v1"
	Versions = []string{"v1"}

	Codec = runtime.CodecFor(api.Scheme, Version)

	// the list of kinds that are scoped at the root of the api hierarchy
	// if a kind is not enumerated here, it is assumed to have a namespace scope
	rootScoped := sets.NewString()

	ignoredKinds := sets.NewString()

	RESTMapper = api.NewDefaultRESTMapper("experimental", Versions, InterfacesFor, importPrefix, ignoredKinds, rootScoped)
	api.RegisterRESTMapper(RESTMapper)
}

// InterfacesFor returns the default Codec and ResourceVersioner for a given version
// string, or an error if the version is not known.
func InterfacesFor(version string) (*meta.VersionInterfaces, error) {
	switch version {
	case "v1":
		return &meta.VersionInterfaces{
			Codec:            v1.Codec,
			ObjectConvertor:  api.Scheme,
			MetadataAccessor: accessor,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported storage version: %s (valid: %s)", version, strings.Join(Versions, ", "))
	}
}
