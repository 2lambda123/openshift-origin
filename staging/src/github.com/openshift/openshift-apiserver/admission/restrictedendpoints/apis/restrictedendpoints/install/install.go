package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/openshift-apiserver/admission/restrictedendpoints/apis/restrictedendpoints"
	"github.com/openshift/openshift-apiserver/admission/restrictedendpoints/apis/restrictedendpoints/v1"
)

func InstallInternal(scheme *runtime.Scheme) {
	utilruntime.Must(restrictedendpoints.Install(scheme))
	utilruntime.Must(v1.Install(scheme))
}
