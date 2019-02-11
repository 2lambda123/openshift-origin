package openshiftadmission

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	mutatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"
	validatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/validating"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/kubernetes/plugin/pkg/admission/gc"
	"k8s.io/kubernetes/plugin/pkg/admission/resourcequota"

	buildsecretinjector "github.com/openshift/origin/pkg/build/apiserver/admission/secretinjector"
	buildstrategyrestrictions "github.com/openshift/origin/pkg/build/apiserver/admission/strategyrestrictions"
	imagepolicyapi "github.com/openshift/origin/pkg/image/apiserver/admission/apis/imagepolicy"
	"github.com/openshift/origin/pkg/image/apiserver/admission/imagepolicy"
	imageadmission "github.com/openshift/origin/pkg/image/apiserver/admission/limitrange"
	projectrequestlimit "github.com/openshift/origin/pkg/project/apiserver/admission/requestlimit"
	quotaclusterresourcequota "github.com/openshift/origin/pkg/quota/apiserver/admission/clusterresourcequota"
	schedulerpodnodeconstraints "github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints"
)

// TODO register this per apiserver or at least per process
var OriginAdmissionPlugins = admission.NewPlugins()

func init() {
	RegisterAllAdmissionPlugins(OriginAdmissionPlugins)
}

// RegisterAllAdmissionPlugins registers all admission plugins
func RegisterAllAdmissionPlugins(plugins *admission.Plugins) {
	// register gc protection plugin
	gc.Register(plugins)
	resourcequota.Register(plugins)

	genericapiserver.RegisterAllAdmissionPlugins(plugins)
	RegisterOpenshiftAdmissionPlugins(plugins)
}

func RegisterOpenshiftAdmissionPlugins(plugins *admission.Plugins) {
	projectrequestlimit.Register(plugins)
	buildsecretinjector.Register(plugins)
	buildstrategyrestrictions.Register(plugins)
	imageadmission.Register(plugins)
	schedulerpodnodeconstraints.Register(plugins)
	imagepolicy.Register(plugins)
	quotaclusterresourcequota.Register(plugins)
}

var (
	// OpenShiftAdmissionPlugins gives the in-order default admission chain for openshift resources.
	OpenShiftAdmissionPlugins = []string{
		lifecycle.PluginName,
		"OwnerReferencesPermissionEnforcement",
		"project.openshift.io/ProjectRequestLimit",
		"build.openshift.io/BuildConfigSecretInjector",
		"build.openshift.io/BuildByStrategy",
		imageadmission.PluginName,
		"scheduling.openshift.io/PodNodeConstraints",
		imagepolicyapi.PluginName,
		"quota.openshift.io/ClusterResourceQuota",
		mutatingwebhook.PluginName,
		validatingwebhook.PluginName,
		"ResourceQuota",
	}

	DefaultOnPlugins = sets.NewString(
		lifecycle.PluginName,
		"build.openshift.io/BuildConfigSecretInjector",
		"build.openshift.io/BuildByStrategy",
		imageadmission.PluginName,
		"OwnerReferencesPermissionEnforcement",
		imagepolicyapi.PluginName,
		mutatingwebhook.PluginName,
		validatingwebhook.PluginName,
		"ResourceQuota",
		"quota.openshift.io/ClusterResourceQuota",
		"project.openshift.io/ProjectRequestLimit",
		"PodNodeConstraints",
	)
)
