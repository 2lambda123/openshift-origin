package resourceread

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	appsScheme = runtime.NewScheme()
	appsCodecs = serializer.NewCodecFactory(appsScheme)
)

func init() {
	if err := appsv1.AddToScheme(appsScheme); err != nil {
		panic(err)
	}
}

func ReadDeploymentOrDie(objBytes []byte) *appsv1.Deployment {
	requiredObj, err := runtime.Decode(appsCodecs.UniversalDecoder(appsv1.SchemeGroupVersion), []byte(objBytes))
	if err != nil {
		panic(err)
	}
	return requiredObj.(*appsv1.Deployment)
}

func ReadDaemonSetOrDie(objBytes []byte) *appsv1.DaemonSet {
	requiredObj, err := runtime.Decode(appsCodecs.UniversalDecoder(appsv1.SchemeGroupVersion), []byte(objBytes))
	if err != nil {
		panic(err)
	}
	return requiredObj.(*appsv1.DaemonSet)
}
