package api

import "github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

func init() {
	runtime.AddKnownTypes("",
		User{},
		Identity{},
		UserIdentityMapping{},
	)
}
