package kubeadmission

import (
	"testing"

	"k8s.io/apiserver/pkg/admission"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/kubernetes/pkg/kubeapiserver/options"

	"github.com/openshift/origin/pkg/admission/admissionregistrationtesting"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation/customresourcevalidationregistration"
)

func TestAdmissionRegistration(t *testing.T) {
	orderedAdmissionChain := NewOrderedKubeAdmissionPlugins(options.AllOrderedPlugins)
	defaultOffPlugins := NewDefaultOffPluginsFunc(options.DefaultOffAdmissionPlugins())()
	registerAllAdmissionPlugins := func(plugins *admission.Plugins) {
		genericapiserver.RegisterAllAdmissionPlugins(plugins)
		options.RegisterAllAdmissionPlugins(plugins)
		RegisterOpenshiftKubeAdmissionPlugins(plugins)
		customresourcevalidationregistration.RegisterCustomResourceValidation(plugins)
	}
	plugins := admission.NewPlugins()
	registerAllAdmissionPlugins(plugins)

	err := admissionregistrationtesting.AdmissionRegistrationTest(plugins, orderedAdmissionChain, defaultOffPlugins)
	if err != nil {
		t.Fatal(err)
	}
}
