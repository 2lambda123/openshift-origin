package v1

// This file contains methods that can be used by the go-restful package to generate Swagger
// documentation for the object types found in 'types.go' This file is automatically generated
// by hack/update-generated-swagger-descriptions.sh and should be run after a full build of OpenShift.
// ==== DO NOT EDIT THIS FILE MANUALLY ====

var map_BuildOverridesConfig = map[string]string{
	"":             "BuildOverridesConfig controls override settings for builds",
	"forcePull":    "ForcePull indicates whether the build strategy should always be set to ForcePull=true",
	"nodeSelector": "nodeSelector is a selector which must be true for the build pod to fit on a node",
}

func (BuildOverridesConfig) SwaggerDoc() map[string]string {
	return map_BuildOverridesConfig
}
