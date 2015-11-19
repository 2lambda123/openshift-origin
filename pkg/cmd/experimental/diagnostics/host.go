package diagnostics

import (
	"fmt"

	"k8s.io/kubernetes/pkg/util/sets"

	hostdiags "github.com/openshift/origin/pkg/diagnostics/host"
	systemddiags "github.com/openshift/origin/pkg/diagnostics/systemd"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

var (
	// availableHostDiagnostics contains the names of host diagnostics that can be executed
	// during a single run of diagnostics. Add more diagnostics to the list as they are defined.
	availableHostDiagnostics = sets.NewString(systemddiags.AnalyzeLogsName, systemddiags.UnitStatusName, hostdiags.MasterConfigCheckName, hostdiags.NodeConfigCheckName)
)

// buildHostDiagnostics builds host Diagnostic objects based on the host environment.
// Returns the Diagnostics built, "ok" bool for whether to proceed or abort, and an error if any was encountered during the building of diagnostics.) {
func (o DiagnosticsOptions) buildHostDiagnostics() ([]types.Diagnostic, bool, error) {
	requestedDiagnostics := intersection(sets.NewString(o.RequestedDiagnostics...), availableHostDiagnostics).List()
	if len(requestedDiagnostics) == 0 { // no diagnostics to run here
		return nil, true, nil // don't waste time on discovery
	}
	isHost := o.IsHost
	if len(o.MasterConfigLocation) > 0 || len(o.NodeConfigLocation) > 0 {
		isHost = true
	}

	// If we're not looking at a host, don't try the diagnostics
	if !isHost {
		return nil, true, nil
	}

	diagnostics := []types.Diagnostic{}
	systemdUnits := systemddiags.GetSystemdUnits(o.Logger)
	for _, diagnosticName := range requestedDiagnostics {
		switch diagnosticName {
		case systemddiags.AnalyzeLogsName:
			diagnostics = append(diagnostics, systemddiags.AnalyzeLogs{SystemdUnits: systemdUnits})

		case systemddiags.UnitStatusName:
			diagnostics = append(diagnostics, systemddiags.UnitStatus{SystemdUnits: systemdUnits})

		case hostdiags.MasterConfigCheckName:
			if len(o.MasterConfigLocation) > 0 {
				diagnostics = append(diagnostics, hostdiags.MasterConfigCheck{MasterConfigFile: o.MasterConfigLocation})
			}

		case hostdiags.NodeConfigCheckName:
			if len(o.NodeConfigLocation) > 0 {
				diagnostics = append(diagnostics, hostdiags.NodeConfigCheck{NodeConfigFile: o.NodeConfigLocation})
			}

		default:
			return diagnostics, false, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
	}

	return diagnostics, true, nil
}
