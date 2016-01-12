package diagnostics

import (
	"fmt"

	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	"k8s.io/kubernetes/pkg/util/sets"

	clientdiags "github.com/openshift/origin/pkg/diagnostics/client"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

var (
	// availableClientDiagnostics contains the names of client diagnostics that can be executed
	// during a single run of diagnostics. Add more diagnostics to the list as they are defined.
	availableClientDiagnostics = sets.NewString(clientdiags.ConfigContextsName)
)

// buildClientDiagnostics builds client Diagnostic objects based on the rawConfig passed in.
// Returns the Diagnostics built, "ok" bool for whether to proceed or abort, and an error if any was encountered during the building of diagnostics.) {
func (o DiagnosticsOptions) buildClientDiagnostics(rawConfig *clientcmdapi.Config) ([]types.Diagnostic, bool, error) {
	available := availableClientDiagnostics

	// osClient, kubeClient, clientErr := o.Factory.Clients() // use with a diagnostic that needs OpenShift/Kube client
	_, _, clientErr := o.Factory.Clients()
	if clientErr != nil {
		o.Logger.Notice("CED0001", "Could not configure a client, so client diagnostics are limited to testing configuration and connection")
		available = sets.NewString(clientdiags.ConfigContextsName)
	}

	diagnostics := []types.Diagnostic{}
	requestedDiagnostics := intersection(sets.NewString(o.RequestedDiagnostics...), available).List()
	for _, diagnosticName := range requestedDiagnostics {
		switch diagnosticName {
		case clientdiags.ConfigContextsName:
			seen := map[string]bool{}
			for contextName := range rawConfig.Contexts {
				diagnostic := clientdiags.ConfigContext{RawConfig: rawConfig, ContextName: contextName}
				if clusterUser, defined := diagnostic.ContextClusterUser(); !defined {
					// definitely want to diagnose the broken context
					diagnostics = append(diagnostics, diagnostic)
				} else if !seen[clusterUser] {
					seen[clusterUser] = true // avoid validating same user for multiple projects
					diagnostics = append(diagnostics, diagnostic)
				}
			}

		default:
			return nil, false, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
	}
	return diagnostics, true, clientErr
}
