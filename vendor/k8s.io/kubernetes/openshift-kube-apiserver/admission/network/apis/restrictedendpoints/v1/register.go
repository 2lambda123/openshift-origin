package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/network/apis/restrictedendpoints"
)

func (obj *RestrictedEndpointsAdmissionConfig) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }

var GroupVersion = schema.GroupVersion{Group: "network.openshift.io", Version: "v1"}

var (
	schemeBuilder = runtime.NewSchemeBuilder(
		addKnownTypes,
		restrictedendpoints.Install,
	)
	Install = schemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&RestrictedEndpointsAdmissionConfig{},
	)
	return nil
}
