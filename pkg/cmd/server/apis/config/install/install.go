package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/apis/apiserver"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	"k8s.io/apiserver/pkg/apis/audit"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapiv1 "github.com/openshift/origin/pkg/cmd/server/apis/config/v1"
	imagepolicyinstall "github.com/openshift/origin/pkg/image/apiserver/admission/apis/imagepolicy/install"
	requestlimitinstall "github.com/openshift/origin/pkg/project/apiserver/admission/apis/requestlimit/install"
	clusterresourceoverrideinstall "github.com/openshift/origin/pkg/quota/apiserver/admission/apis/clusterresourceoverride/install"
	runoncedurationinstall "github.com/openshift/origin/pkg/quota/apiserver/admission/apis/runonceduration/install"
	ingressadmissioninstall "github.com/openshift/origin/pkg/route/apiserver/admission/apis/ingressadmission/install"
	podnodeconstraintsinstall "github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints/install"
	externaliprangerinstall "github.com/openshift/origin/pkg/service/admission/apis/externalipranger/install"
	restrictedendpointsinstall "github.com/openshift/origin/pkg/service/admission/apis/restrictedendpoints/install"
)

func init() {
	InstallLegacyInternal(configapi.Scheme)
}

func InstallLegacyInternal(scheme *runtime.Scheme) {
	configapi.InstallLegacy(scheme)
	configapiv1.InstallLegacy(scheme)

	// we additionally need to enable audit versions, since we embed the audit
	// policy file inside master-config.yaml
	audit.AddToScheme(scheme)
	auditv1alpha1.AddToScheme(scheme)
	auditv1beta1.AddToScheme(scheme)
	apiserver.AddToScheme(scheme)
	apiserverv1alpha1.AddToScheme(scheme)

	// add the other admission config types we have
	requestlimitinstall.InstallInternal(scheme)

	// add the other admission config types we have to the core group if they are legacy types
	imagepolicyinstall.InstallInternal(scheme)
	ingressadmissioninstall.InstallInternal(scheme)
	clusterresourceoverrideinstall.InstallInternal(scheme)
	runoncedurationinstall.InstallInternal(scheme)
	podnodeconstraintsinstall.InstallInternal(scheme)
	restrictedendpointsinstall.InstallInternal(scheme)
	externaliprangerinstall.InstallInternal(scheme)
}
