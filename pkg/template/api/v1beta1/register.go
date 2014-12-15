package v1beta1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta1",
		&Template{},
	)
	api.Scheme.AddKnownTypeWithName("v1beta1", "TemplateConfig", &Template{})
}

func (*Template) IsAnAPIObject() {}
