package admission

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"

	overrideapi "github.com/openshift/origin/pkg/quota/apiserver/admission/apis/clusterresourceoverride"
	"github.com/openshift/origin/pkg/security/apiserver/admission/sccadmission"
	"github.com/openshift/origin/pkg/service/admission/externalipranger"
)

// legacyOpenshiftAdmissionPlugins holds names that already existed without a prefix.  We should come up with a migration
// plan (double register for a few releases?), but for now just make sure we don't get worse.
var legacyOpenshiftAdmissionPlugins = sets.NewString(
	"ProjectRequestLimit",
	"PodNodeConstraints",
	"BuildByStrategy",
	"RunOnceDuration",
	"OriginPodNodeEnvironment",
	overrideapi.PluginName,
	externalipranger.ExternalIPPluginName,
	sccadmission.PluginName,
	"SCCExecRestrictions",
	"ResourceQuota",
)

// TestAdmissionPluginNames makes sure that openshift admission plugins are prefixed with `openshift.io/`.
func TestAdmissionPluginNames(t *testing.T) {
	originAdmissionPlugins := admission.NewPlugins()
	RegisterOpenshiftAdmissionPlugins(originAdmissionPlugins)

	for _, plugin := range originAdmissionPlugins.Registered() {
		if !strings.HasPrefix(plugin, "openshift.io/") && !legacyOpenshiftAdmissionPlugins.Has(plugin) {
			t.Errorf("openshift admission plugins must be prefixed with openshift.io/ %v", plugin)
		}
	}
}
