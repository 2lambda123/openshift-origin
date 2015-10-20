package api

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
		&Image{},
		&ImageList{},
		&ImageStream{},
		&ImageStreamList{},
		&ImageStreamMapping{},
		&ImageStreamTag{},
		&ImageStreamImage{},
		&ImageStreamImageList{},
		&ImageStreamDeletion{},
		&ImageStreamDeletionList{},
		&DockerImage{},
	)
}

func (*Image) IsAnAPIObject()                   {}
func (*ImageList) IsAnAPIObject()               {}
func (*DockerImage) IsAnAPIObject()             {}
func (*ImageStream) IsAnAPIObject()             {}
func (*ImageStreamList) IsAnAPIObject()         {}
func (*ImageStreamMapping) IsAnAPIObject()      {}
func (*ImageStreamTag) IsAnAPIObject()          {}
func (*ImageStreamImage) IsAnAPIObject()        {}
func (*ImageStreamImageList) IsAnAPIObject()    {}
func (*ImageStreamDeletion) IsAnAPIObject()     {}
func (*ImageStreamDeletionList) IsAnAPIObject() {}
