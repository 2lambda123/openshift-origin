package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OpenShiftAPIServerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []OpenShiftAPIServerConfig `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OpenShiftAPIServerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   OpenShiftAPIServerConfigSpec   `json:"spec"`
	Status OpenShiftAPIServerConfigStatus `json:"status"`
}

type OpenShiftAPIServerConfigSpec struct {
	// TODO I think this should eventually embed the entire master config
	// it will end up overlaying in the following order:
	// 1. hardcoded default
	// 2. existing config
	// 3. this config
	APIServerConfig APIServerConfig `json:"apiServerConfig"`

	// ImagePullSpec is the image to use
	ImagePullSpec string `json:"imagePullSpec"`

	// Version is major.minor.micro-patch?.  Usually patch is ignored.
	Version string `json:"version"`
}

type APIServerConfig struct {
	LogLevel int64 `json:"logLevel"`
	// TODO Port will actually be part of the embedded config object
	Port int `json:"port"`

	// TODO this will go away once we have configmaps and secrets
	HostPath string `json:"hostPath"`
}

type OpenShiftAPIServerConfigStatus struct {
	InProgressVersion     string `json:"inProgressVersion"`
	LastSuccessfulVersion string `json:"lastSuccessfulVersion"`

	// LastUnsuccessfulRunErrors tracks errors from the last run.  If we put the system back into a working state
	// these will be from the last failure.
	LastUnsuccessfulRunErrors []string `json:"lastUnsuccessfulRunErrors"`
}
