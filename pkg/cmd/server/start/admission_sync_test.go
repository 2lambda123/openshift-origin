package start

import (
	"testing"

	// this package has imports for all the admission controllers used in the kube api server
	// it causes all the admission plugins to be registered, giving us a full listing.
	_ "k8s.io/kubernetes/cmd/kube-apiserver/app"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
)

var admissionPluginsNotUsedByKube = sets.NewString(
	"AlwaysAdmit",            // from kube, no need for this by default
	"AlwaysDeny",             // from kube, no need for this by default
	"AlwaysPullImages",       // from kube, not enabled by default.  This is only applicable to some environments.  This will ensure that containers cannot use pre-pulled copies of images without authorization.
	"NamespaceAutoProvision", // from kube, it creates a namespace if a resource is created in a non-existent namespace.  We don't want this behavior
	"SecurityContextDeny",    // from kube, it denies pods that want to use SCC capabilities.  We have different rules to allow this in openshift.
	"DenyExecOnPrivileged",   // from kube (deprecated, see below), it denies exec to pods that have certain privileges.  This is superseded in origin by SCCExecRestrictions that checks against SCC rules.
	"DenyEscalatingExec",     // from kube, it denies exec to pods that have certain privileges.  This is superseded in origin by SCCExecRestrictions that checks against SCC rules.

	"BuildByStrategy",          // from origin, only needed for managing builds, not kubernetes resources
	"BuildDefaults",            // from origin, only needed for managing builds, not kubernetes resources
	"BuildOverrides",           // from origin, only needed for managing builds, not kubernetes resources
	"OriginNamespaceLifecycle", // from origin, only needed for rejecting openshift resources, so not needed by kube
	"ProjectRequestLimit",      // from origin, used for limiting project requests by user (online use case)
	"RunOnceDuration",          // from origin, used for overriding the ActiveDeadlineSeconds for run-once pods
	"OriginResourceQuota",      // from origin, used for quota abuse checks of openshift resources

	"NamespaceExists",  // superseded by NamespaceLifecycle
	"InitialResources", // do we want this? https://github.com/kubernetes/kubernetes/blob/master/docs/proposals/initial-resources.md

	"PersistentVolumeLabel", // do we want this? disable by default
)

func TestKubeAdmissionControllerUsage(t *testing.T) {
	registeredKubePlugins := sets.NewString(admission.GetPlugins()...)

	usedAdmissionPlugins := sets.NewString(kubernetes.AdmissionPlugins...)

	if missingPlugins := usedAdmissionPlugins.Difference(registeredKubePlugins); len(missingPlugins) != 0 {
		t.Errorf("%v not found", missingPlugins.List())
	}

	if notUsed := registeredKubePlugins.Difference(usedAdmissionPlugins); len(notUsed) != 0 {
		for pluginName := range notUsed {
			if !admissionPluginsNotUsedByKube.Has(pluginName) {
				t.Errorf("%v not used", pluginName)
			}
		}
	}
}
