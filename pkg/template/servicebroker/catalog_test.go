package servicebroker

import (
	"reflect"
	"testing"

	schema "github.com/lestrrat/go-jsschema"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/openservicebroker/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

func TestServiceFromTemplate(t *testing.T) {
	template := &templateapi.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name: "name",
			UID:  "ee33151d-a34d-442d-a0ca-6353b73a58fd",
			Annotations: map[string]string{
				"description": "description",
				"tags":        "tag1,tag2",
				"openshift.io/display-name":                   "displayName",
				"iconClass":                                   "iconClass",
				"template.openshift.io/long-description":      "longDescription",
				"template.openshift.io/provider-display-name": "providerDisplayName",
				"template.openshift.io/documentation-url":     "documentationURL",
				"template.openshift.io/support-url":           "supportURL",
			},
		},
		Parameters: []templateapi.Parameter{
			{
				Name:     "param1",
				Required: true,
			},
			{
				Name: "param2",
			},
			{
				Name:     "param3",
				Generate: "expression",
			},
			{
				Name:     "param4",
				Generate: "expression",
				Required: true,
			},
		},
	}

	expectedService := &api.Service{
		Name:        "name",
		ID:          "ee33151d-a34d-442d-a0ca-6353b73a58fd",
		Description: "description",
		Tags:        []string{"tag1", "tag2"},
		Bindable:    true,
		Metadata: map[string]interface{}{
			"providerDisplayName":            "providerDisplayName",
			"documentationUrl":               "documentationURL",
			"supportUrl":                     "supportURL",
			"displayName":                    "displayName",
			"console.openshift.io/iconClass": "iconClass",
			"longDescription":                "longDescription",
		},
		Plans: []api.Plan{
			{
				ID:          "ee33151d-a34d-442d-a0ca-6353b73a58fd",
				Name:        "default",
				Description: "Default plan",
				Free:        true,
				Bindable:    true,
				Schemas: api.Schema{
					ServiceInstance: api.ServiceInstances{
						Create: map[string]*schema.Schema{
							"parameters": {
								Type:      schema.PrimitiveTypes{schema.ObjectType},
								SchemaRef: "http://json-schema.org/draft-04/schema",
								Required: []string{
									"param1",
								},
								Properties: map[string]*schema.Schema{
									"param1": {
										Default: "",
										Type:    schema.PrimitiveTypes{schema.StringType},
									},
									"param2": {
										Default: "",
										Type:    schema.PrimitiveTypes{schema.StringType},
									},
									"param3": {
										Default: "",
										Type:    schema.PrimitiveTypes{schema.StringType},
									},
									"param4": {
										Default: "",
										Type:    schema.PrimitiveTypes{schema.StringType},
									},
								},
							},
						},
					},
					ServiceBinding: api.ServiceBindings{
						Create: map[string]*schema.Schema{
							"parameters": {
								Type:       schema.PrimitiveTypes{schema.ObjectType},
								SchemaRef:  "http://json-schema.org/draft-04/schema",
								Required:   []string{},
								Properties: map[string]*schema.Schema{},
							},
						},
					},
				},
			},
		},
	}

	service := serviceFromTemplate(template)

	if !reflect.DeepEqual(service, expectedService) {
		t.Error("service did not match expectedService")
	}
}
