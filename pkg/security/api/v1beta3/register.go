package v1beta3

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

const GroupName = ""

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = unversioned.GroupVersion{Group: GroupName, Version: "v1beta3"}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) unversioned.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns back a Group qualified GroupResource
func Resource(resource string) unversioned.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func AddToScheme(scheme *runtime.Scheme) {
	// Add the API to Scheme.
	addKnownTypes(scheme)
}

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&PodSpecSubjectReview{},
		&PodSpecSelfSubjectReview{},
		&PodSpecReview{},
	)
}

func (obj *PodSpecSubjectReview) GetObjectKind() unversioned.ObjectKind     { return &obj.TypeMeta }
func (obj *PodSpecSelfSubjectReview) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }
func (obj *PodSpecReview) GetObjectKind() unversioned.ObjectKind            { return &obj.TypeMeta }
