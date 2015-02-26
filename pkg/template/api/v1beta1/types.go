package v1beta1

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

// Template contains the inputs needed to produce a Config.
type Template struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Required: Items is an array of Kubernetes resources of Service,
	// Pod and/or ReplicationController kind.
	Items []runtime.RawExtension `json:"items"`

	// Optional: Parameters is an array of Parameters used during the
	// Template to Config transformation.
	Parameters []Parameter `json:"parameters,omitempty"`

	// Optional: Labels is a set of labels that are applied to every
	// object during the Template to Config transformation
	Labels map[string]string `json:"labels,omitempty"`
}

// TemplateList is a list of Template objects.
type TemplateList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []Template `json:"items"`
}

// Parameter defines a name/value variable that is to be processed during
// the Template to Config transformation.
type Parameter struct {
	// Required: Parameter name must be set and it can be referenced in Template
	// Items using ${PARAMETER_NAME}
	Name string `json:"name"`

	// Optional: Parameter can have description
	Description string `json:"description,omitempty"`

	// Optional: Value holds the Parameter data. If specified, the generator
	// will be ignored. The value replaces all occurrences of the Parameter
	// ${Name} expression during the Template to Config transformation.
	Value string `json:"value,omitempty"`

	// Optional: Generate specifies the generator to be used to generate
	// random string from an input value specified by From field. The result
	// string is stored into Value field. If empty, no generator is being
	// used, leaving the result Value untouched.
	Generate string `json:"generate,omitempty"`

	// Optional: From is an input value for the generator.
	From string `json:"from,omitempty"`
}

// RemoteTemplate is a resource that points to a template accessible via URL
type RemoteTemplate struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// URL for the remote template
	RemoteURL string
}
