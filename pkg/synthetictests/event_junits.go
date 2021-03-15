package synthetictests

import (
	"time"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/test/ginkgo"
)

// stableSystemEventInvariants are invariants that should hold true when a cluster is in
// steady state (not being changed externally). Use these with suites that assume the
// cluster is under no adversarial change (config changes, induced disruption to nodes,
// etcd, or apis).
func StableSystemEventInvariants(events monitor.EventIntervals, duration time.Duration) (tests []*ginkgo.JUnitTestCase, passed bool) {
	tests, _ = SystemEventInvariants(events, duration)
	results, kubeAPIOk := testKubeAPIServerGracefulTermination(events)
	tests = append(tests, results...)
	results, kubeletOk := testKubeletToAPIServerGracefulTermination(events)
	tests = append(tests, results...)
	tests = append(tests, testServerAvailability(monitor.LocatorKubeAPIServerNewConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorOpenshiftAPIServerNewConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorOAuthAPIServerNewConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorKubeAPIServerReusedConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorOpenshiftAPIServerReusedConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorOAuthAPIServerReusedConnection, events, duration)...)
	tests = append(tests, testOperatorStateTransitions(events)...)

	return tests, kubeAPIOk && kubeletOk
}

// systemUpgradeEventInvariants are invariants tested against events that should hold true in a cluster
// that is being upgraded without induced disruption
func SystemUpgradeEventInvariants(events monitor.EventIntervals, duration time.Duration) (tests []*ginkgo.JUnitTestCase, passed bool) {
	tests, _ = SystemEventInvariants(events, duration)
	results, kubeAPIOk := testKubeAPIServerGracefulTermination(events)
	tests = append(tests, results...)
	results, kubeletOk := testKubeletToAPIServerGracefulTermination(events)
	results, nodeUpgradeOk := testNodeUpgradeTransitions(events)
	tests = append(tests, results...)
	return tests, kubeAPIOk && kubeletOk && nodeUpgradeOk
}

// systemEventInvariants are invariants tested against events that should hold true in any cluster,
// even one undergoing disruption. These are usually focused on things that must be true on a single
// machine, even if the machine crashes.
func SystemEventInvariants(events monitor.EventIntervals, duration time.Duration) (tests []*ginkgo.JUnitTestCase, passed bool) {
	tests = append(tests, testPodTransitions(events)...)
	tests = append(tests, testSystemDTimeout(events)...)
	tests = append(tests, testPodSandboxCreation(events)...)
	return tests, true
}
