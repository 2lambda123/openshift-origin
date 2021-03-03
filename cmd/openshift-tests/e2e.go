package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/test/ginkgo"

	_ "github.com/openshift/origin/test/extended"
	exutil "github.com/openshift/origin/test/extended/util"
	_ "github.com/openshift/origin/test/extended/util/annotate/generated"
	"github.com/openshift/origin/test/extended/util/disruption"
)

func isDisabled(name string) bool {
	return strings.Contains(name, "[Disabled")
}

type testSuite struct {
	ginkgo.TestSuite

	PreSuite  func(opt *runOptions) error
	PostSuite func(opt *runOptions)

	PreTest func() error
}

type testSuites []testSuite

func (s testSuites) TestSuites() []*ginkgo.TestSuite {
	copied := make([]*ginkgo.TestSuite, 0, len(s))
	for i := range s {
		copied = append(copied, &s[i].TestSuite)
	}
	return copied
}

// staticSuites are all known test suites this binary should run
var staticSuites = testSuites{
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/conformance",
			Description: templates.LongDesc(`
		Tests that ensure an OpenShift cluster and components are working properly.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return strings.Contains(name, "[Suite:openshift/conformance/")
			},
			Parallelism:         30,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(stableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/conformance/parallel",
			Description: templates.LongDesc(`
		Only the portion of the openshift/conformance test suite that run in parallel.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return strings.Contains(name, "[Suite:openshift/conformance/parallel")
			},
			Parallelism:          30,
			MaximumAllowedFlakes: 15,
			SyntheticEventTests:  ginkgo.JUnitForEventsFunc(stableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/conformance/serial",
			Description: templates.LongDesc(`
		Only the portion of the openshift/conformance test suite that run serially.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return strings.Contains(name, "[Suite:openshift/conformance/serial") || isStandardEarlyOrLateTest(name)
			},
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(stableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/disruptive",
			Description: templates.LongDesc(`
		The disruptive test suite.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				// excluded due to stopped instance handling until https://bugzilla.redhat.com/show_bug.cgi?id=1905709 is fixed
				if strings.Contains(name, "Cluster should survive master and worker failure and recover with machine health checks") {
					return false
				}
				return strings.Contains(name, "[Feature:EtcdRecovery]") || strings.Contains(name, "[Feature:NodeRecovery]") || isStandardEarlyTest(name)

			},
			TestTimeout:         60 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(systemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "kubernetes/conformance",
			Description: templates.LongDesc(`
		The default Kubernetes conformance suite.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return strings.Contains(name, "[Suite:k8s]") && strings.Contains(name, "[Conformance]")
			},
			Parallelism:         30,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(stableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/build",
			Description: templates.LongDesc(`
		Tests that exercise the OpenShift build functionality.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return strings.Contains(name, "[Feature:Builds]") || isStandardEarlyOrLateTest(name)
			},
			Parallelism: 7,
			// TODO: Builds are really flaky right now, remove when we land perf updates and fix io on workers
			MaximumAllowedFlakes: 3,
			// Jenkins tests can take a really long time
			TestTimeout:         60 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(stableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/templates",
			Description: templates.LongDesc(`
		Tests that exercise the OpenShift template functionality.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return strings.Contains(name, "[Feature:Templates]") || isStandardEarlyOrLateTest(name)
			},
			Parallelism:         1,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(stableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/image-registry",
			Description: templates.LongDesc(`
		Tests that exercise the OpenShift image-registry functionality.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) || strings.Contains(name, "[Local]") {
					return false
				}
				return strings.Contains(name, "[sig-imageregistry]") || isStandardEarlyOrLateTest(name)
			},
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(stableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/image-ecosystem",
			Description: templates.LongDesc(`
		Tests that exercise language and tooling images shipped as part of OpenShift.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) || strings.Contains(name, "[Local]") {
					return false
				}
				return strings.Contains(name, "[Feature:ImageEcosystem]") || isStandardEarlyOrLateTest(name)
			},
			Parallelism:         7,
			TestTimeout:         20 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(stableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/jenkins-e2e",
			Description: templates.LongDesc(`
		Tests that exercise the OpenShift / Jenkins integrations provided by the OpenShift Jenkins image/plugins and the Pipeline Build Strategy.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return strings.Contains(name, "[Feature:Jenkins]") || isStandardEarlyOrLateTest(name)
			},
			Parallelism:         4,
			TestTimeout:         20 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(stableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/jenkins-e2e-rhel-only",
			Description: templates.LongDesc(`
		Tests that exercise the OpenShift / Jenkins integrations provided by the OpenShift Jenkins image/plugins and the Pipeline Build Strategy.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return strings.Contains(name, "[Feature:JenkinsRHELImagesOnly]") || isStandardEarlyOrLateTest(name)
			},
			Parallelism:         4,
			TestTimeout:         20 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(stableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/scalability",
			Description: templates.LongDesc(`
		Tests that verify the scalability characteristics of the cluster. Currently this is focused on core performance behaviors and preventing regressions.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return strings.Contains(name, "[Suite:openshift/scalability]")
			},
			Parallelism: 1,
			TestTimeout: 20 * time.Minute,
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/conformance-excluded",
			Description: templates.LongDesc(`
		Run only tests that are excluded from conformance. Makes identifying omitted tests easier.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return !strings.Contains(name, "[Suite:openshift/conformance/")
			},
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(stableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/test-cmd",
			Description: templates.LongDesc(`
		Run only tests for test-cmd.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return strings.Contains(name, "[Feature:LegacyCommandTests]") || isStandardEarlyOrLateTest(name)
			},
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(stableSystemEventInvariants),
		},
		PreSuite: suiteWithNoProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/csi",
			Description: templates.LongDesc(`
		Run tests for an CSI driver. Set the TEST_CSI_DRIVER_FILES environment variable to the name of file with
		CSI driver test manifest. The manifest specifies Kubernetes + CSI features to test with the driver.
		See https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/external/README.md for required format of the file.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return strings.Contains(name, "External Storage [Driver:") && !strings.Contains(name, "[Disruptive]")
			},
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(stableSystemEventInvariants),
		},
		PreSuite: suiteWithKubeTestInitializationPreSuite,
		PostSuite: func(opt *runOptions) {
			printStorageCapabilities(opt.Out)
		},
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/network/stress",
			Description: templates.LongDesc(`
		This test suite repeatedly verifies the networking function of the cluster in parallel to find flakes.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return (strings.Contains(name, "[Suite:openshift/conformance/") && strings.Contains(name, "[sig-network]")) || isStandardEarlyOrLateTest(name)
			},
			Parallelism:         60,
			Count:               12,
			TestTimeout:         20 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(stableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/network/third-party",
			Description: templates.LongDesc(`
		The conformance testing suite for certified third-party CNI plugins.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return inCNISuite(name)
			},
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "all",
			Description: templates.LongDesc(`
		Run all tests.
		`),
			Matches: func(name string) bool {
				return true
			},
		},
		PreSuite: suiteWithInitializedProviderPreSuite,
	},
}

// isStandardEarlyTest returns true if a test is considered part of the normal
// pre or post condition tests.
func isStandardEarlyTest(name string) bool {
	if !strings.Contains(name, "[Early]") {
		return false
	}
	return strings.Contains(name, "[Suite:openshift/conformance/parallel")
}

// isStandardEarlyOrLateTest returns true if a test is considered part of the normal
// pre or post condition tests.
func isStandardEarlyOrLateTest(name string) bool {
	if !strings.Contains(name, "[Early]") && !strings.Contains(name, "[Late]") {
		return false
	}
	return strings.Contains(name, "[Suite:openshift/conformance/parallel")
}

// suiteWithInitializedProviderPreSuite loads the provider info, but does not
// exclude any tests specific to that provider.
func suiteWithInitializedProviderPreSuite(opt *runOptions) error {
	config, err := decodeProvider(opt.Provider, opt.DryRun, true)
	if err != nil {
		return err
	}
	opt.config = config

	opt.Provider = config.ToJSONString()
	return nil
}

// suiteWithProviderPreSuite ensures that the suite filters out tests from providers
// that aren't relevant (see getProviderMatchFn) by loading the provider info from the
// cluster or flags.
func suiteWithProviderPreSuite(opt *runOptions) error {
	if err := suiteWithInitializedProviderPreSuite(opt); err != nil {
		return err
	}
	opt.MatchFn = getProviderMatchFn(opt.config)
	return nil
}

// suiteWithNoProviderPreSuite blocks out provider settings from being passed to
// child tests. Used with suites that should not have cloud specific behavior.
func suiteWithNoProviderPreSuite(opt *runOptions) error {
	opt.Provider = `none`
	return suiteWithProviderPreSuite(opt)
}

// suiteWithKubeTestInitialization invokes the Kube suite in order to populate
// data from the environment for the CSI suite. Other suites should use
// suiteWithProviderPreSuite.
func suiteWithKubeTestInitializationPreSuite(opt *runOptions) error {
	if err := suiteWithProviderPreSuite(opt); err != nil {
		return err
	}
	return initializeTestFramework(exutil.TestContext, opt.config, opt.DryRun)
}

const (
	// Max. duration of API server unreachability, in fraction of total test duration.
	tolerateDisruptionPercent = 0.01
)

// stableSystemEventInvariants are invariants that should hold true when a cluster is in
// steady state (not being changed externally). Use these with suites that assume the
// cluster is under no adversarial change (config changes, induced disruption to nodes,
// etcd, or apis).
func stableSystemEventInvariants(events monitor.EventIntervals, duration time.Duration) (tests []*ginkgo.JUnitTestCase, passed bool) {
	tests, _ = systemEventInvariants(events, duration)
	tests = append(tests, testKubeAPIServerGracefulTermination(events)...)
	tests = append(tests, testServerAvailability(monitor.LocatorKubeAPIServerNewConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorOpenshiftAPIServerNewConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorOAuthAPIServerNewConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorKubeAPIServerReusedConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorOpenshiftAPIServerReusedConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorOAuthAPIServerReusedConnection, events, duration)...)
	return tests, true
}

// systemUpgradeEventInvariants are invariants tested against events that should hold true in a cluster
// that is being upgraded without induced disruption
func systemUpgradeEventInvariants(events monitor.EventIntervals, duration time.Duration) (tests []*ginkgo.JUnitTestCase, passed bool) {
	tests, _ = systemEventInvariants(events, duration)
	tests = append(tests, testKubeAPIServerGracefulTermination(events)...)
	results, nodeUpgradeOk := testNodeUpgradeTransitions(events)
	tests = append(tests, results...)
	return tests, nodeUpgradeOk
}

// systemEventInvariants are invariants tested against events that should hold true in any cluster,
// even one undergoing disruption. These are usually focused on things that must be true on a single
// machine, even if the machine crashes.
func systemEventInvariants(events monitor.EventIntervals, duration time.Duration) (tests []*ginkgo.JUnitTestCase, passed bool) {
	tests = append(tests, testPodTransitions(events)...)
	tests = append(tests, testSystemDTimeout(events)...)
	tests = append(tests, testPodSandboxCreation(events)...)
	return tests, true
}

func testServerAvailability(locator string, events []*monitor.EventInterval, duration time.Duration) []*ginkgo.JUnitTestCase {
	errDuration, errMessages := disruption.GetDisruption(events, locator)

	testName := fmt.Sprintf("[sig-api-machinery] %s should be available", locator)
	successTest := &ginkgo.JUnitTestCase{
		Name:     testName,
		Duration: duration.Seconds(),
	}
	if percent := float64(errDuration) / float64(duration); percent > tolerateDisruptionPercent {
		test := &ginkgo.JUnitTestCase{
			Name:     testName,
			Duration: duration.Seconds(),
			FailureOutput: &ginkgo.FailureOutput{
				Output: fmt.Sprintf("%s was failing for %s seconds (%0.0f%% of the test duration)", locator, errDuration.Truncate(time.Second), 100*percent),
			},
			SystemOut: strings.Join(errMessages, "\n"),
		}
		// Return *two* tests results to pretend this is a flake not to fail whole testsuite.
		return []*ginkgo.JUnitTestCase{test, successTest}
	} else {
		successTest.SystemOut = fmt.Sprintf("%s was failing for %s seconds (%0.0f%% of the test duration)", locator, errDuration.Truncate(time.Second), 100*percent)
		return []*ginkgo.JUnitTestCase{successTest}
	}
}

func testKubeAPIServerGracefulTermination(events []*monitor.EventInterval) []*ginkgo.JUnitTestCase {
	const testName = "[sig-node] kubelet terminates kube-apiserver gracefully"
	const testNamePre = "[sig-node] kubelet terminates kube-apiserver gracefully before suite execution"

	ignoreEventsBefore := exutil.LimitTestsToStartTime()
	// suiteStartTime can be inferred from the monitor interval, we can't use exutil.SuiteStartTime because that
	// is run within sub processes
	var suiteStartTime time.Time
	if len(events) > 0 {
		suiteStartTime = events[0].From.Add((-time.Second))
	}

	var preFailures, failures []string
	for _, event := range events {
		// from https://github.com/openshift/kubernetes/blob/1f35e4f63be8fbb19e22c9ff1df31048f6b42ddf/cmd/watch-termination/main.go#L96
		if strings.Contains(event.Message, "did not terminate gracefully") {
			if event.From.Before(ignoreEventsBefore) {
				// we explicitly ignore graceful terminations when we are told to check up to a certain point
				continue
			}
			if event.From.Before(suiteStartTime) {
				preFailures = append(preFailures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
			} else {
				failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
			}
		}
	}

	// failures during a run always fail the test suite
	var tests []*ginkgo.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &ginkgo.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &ginkgo.FailureOutput{
				Output: fmt.Sprintf("%d kube-apiserver reports a non-graceful termination. Probably kubelet or CRI-O is not giving the time to cleanly shut down. This can lead to connection refused and network I/O timeout errors in other components.\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})
	} else {
		tests = append(tests, &ginkgo.JUnitTestCase{Name: testName})
	}

	// pre-failures are flakes for now
	if len(preFailures) > 0 {
		tests = append(tests,
			&ginkgo.JUnitTestCase{Name: testNamePre},
			&ginkgo.JUnitTestCase{
				Name:      testNamePre,
				SystemOut: strings.Join(preFailures, "\n"),
				FailureOutput: &ginkgo.FailureOutput{
					Output: fmt.Sprintf("%d kube-apiserver reports a non-graceful termination outside of the test suite (during install or upgrade), which is reported as a flake. Probably kubelet or CRI-O is not giving the time to cleanly shut down. This can lead to connection refused and network I/O timeout errors in other components.\n\n%v", len(preFailures), strings.Join(preFailures, "\n")),
				},
			},
		)
	} else {
		tests = append(tests, &ginkgo.JUnitTestCase{Name: testNamePre})
	}
	return tests
}

func testPodTransitions(events []*monitor.EventInterval) []*ginkgo.JUnitTestCase {
	const testName = "[sig-node] pods should never transition back to pending"
	success := &ginkgo.JUnitTestCase{Name: testName}

	failures := []string{}
	for _, event := range events {
		if strings.Contains(event.Message, "pod should not transition") || strings.Contains(event.Message, "pod moved back to Pending") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
		}
	}
	if len(failures) == 0 {
		return []*ginkgo.JUnitTestCase{success}
	}

	failure := &ginkgo.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &ginkgo.FailureOutput{
			Output: fmt.Sprintf("%d pods illegally transitioned to Pending\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}

	// TODO an upgrade job that starts before 4.6 may need to make this test flake instead of fail.  This will depend on which `openshift-tests`
	//  is used to run that upgrade test.  I recommend waiting to a flake until we know and even then find a way to constrain it.
	return []*ginkgo.JUnitTestCase{failure}
}

func formatTimes(times []time.Time) []string {
	var s []string
	for _, t := range times {
		s = append(s, t.UTC().Format(time.RFC3339))
	}
	return s
}

func testNodeUpgradeTransitions(events []*monitor.EventInterval) ([]*ginkgo.JUnitTestCase, bool) {
	const testName = "[sig-node] nodes should not go unready after being upgraded and go unready only once"

	nodesWentReady, nodesWentUnready := make(map[string][]time.Time), make(map[string][]time.Time)
	currentNodeReady := make(map[string]bool)

	var buf bytes.Buffer
	var testCases []*ginkgo.JUnitTestCase
	for len(events) > 0 {
		var text string
		var failures []string
		var foundEnd bool
		for i, event := range events {
			// treat multiple sequential upgrades as distinct test failures
			if event.Locator == "clusterversion/cluster" && strings.Contains(event.Message, "reason/UpgradeStarted") {
				text = event.Message
				events = events[i+1:]
				foundEnd = true
				fmt.Fprintf(&buf, "DEBUG: found upgrade start event: %v\n", event.String())
				break
			}
			if !strings.HasPrefix(event.Locator, "node/") {
				continue
			}
			if !strings.HasPrefix(event.Message, "condition/Ready ") || !strings.HasSuffix(event.Message, " changed") {
				continue
			}
			node := strings.TrimPrefix(event.Locator, "node/")
			if strings.Contains(event.Message, " status/True ") {
				if currentNodeReady[node] {
					failures = append(failures, fmt.Sprintf("Node %s was reported ready twice in a row, this should be impossible", node))
					continue
				}
				currentNodeReady[node] = true
				nodesWentReady[node] = append(nodesWentReady[node], event.From)
				fmt.Fprintf(&buf, "DEBUG: found node ready transition: %v\n", event.String())
			} else {
				if !currentNodeReady[node] && len(nodesWentUnready[node]) > 0 {
					fmt.Fprintf(&buf, "DEBUG: node already considered not ready, ignoring: %v\n", event.String())
					continue
				}
				currentNodeReady[node] = false
				nodesWentUnready[node] = append(nodesWentUnready[node], event.From)
				fmt.Fprintf(&buf, "DEBUG: found node not ready transition: %v\n", event.String())
			}
		}
		if !foundEnd {
			events = nil
		}

		abnormalNodes := sets.NewString()
		for node, unready := range nodesWentUnready {
			ready := nodesWentReady[node]
			if len(unready) > 1 {
				failures = append(failures, fmt.Sprintf("Node %s went unready multiple times: %s", node, strings.Join(formatTimes(unready), ", ")))
				abnormalNodes.Insert(node)
			}
			if len(ready) > 1 {
				failures = append(failures, fmt.Sprintf("Node %s went ready multiple times: %s", node, strings.Join(formatTimes(ready), ", ")))
				abnormalNodes.Insert(node)
			}

			switch {
			case len(ready) == 0, len(unready) == 1 && len(ready) == 1 && !unready[0].Before(ready[0]):
				failures = append(failures, fmt.Sprintf("Node %s went unready at %s, never became ready again", node, strings.Join(formatTimes(unready[:1]), ", ")))
				abnormalNodes.Insert(node)
			}
		}
		for node, ready := range nodesWentReady {
			unready := nodesWentUnready[node]
			if len(unready) > 0 {
				continue
			}
			switch {
			case len(ready) > 1:
				failures = append(failures, fmt.Sprintf("Node %s went ready multiple times without going unready: %s", node, strings.Join(formatTimes(ready), ", ")))
				abnormalNodes.Insert(node)
			case len(ready) == 1:
				failures = append(failures, fmt.Sprintf("Node %s went ready at %s but had no record of going unready, may not have upgraded", node, strings.Join(formatTimes(ready), ", ")))
				abnormalNodes.Insert(node)
			}
		}

		if len(failures) == 0 {
			continue
		}

		testCases = append(testCases, &ginkgo.JUnitTestCase{
			Name:      testName,
			SystemOut: fmt.Sprintf("%s\n\n%s", strings.Join(failures, "\n"), buf.String()),
			FailureOutput: &ginkgo.FailureOutput{
				Output: fmt.Sprintf("%d nodes violated upgrade expectations:\n\n%s\n\n%s", abnormalNodes.Len(), strings.Join(failures, "\n"), text),
			},
		})
	}
	if len(testCases) == 0 {
		testCases = append(testCases, &ginkgo.JUnitTestCase{Name: testName})
		return testCases, true
	}
	return testCases, false
}

func testSystemDTimeout(events []*monitor.EventInterval) []*ginkgo.JUnitTestCase {
	const testName = "[sig-node] pods should not fail on systemd timeouts"
	success := &ginkgo.JUnitTestCase{Name: testName}

	failures := []string{}
	for _, event := range events {
		if strings.Contains(event.Message, "systemd timed out for pod") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
		}
	}
	if len(failures) == 0 {
		return []*ginkgo.JUnitTestCase{success}
	}

	failure := &ginkgo.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &ginkgo.FailureOutput{
			Output: fmt.Sprintf("%d systemd timed out for pod occurrences\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}

	// write a passing test to trigger detection of this issue as a flake. Doing this first to try to see how frequent the issue actually is
	return []*ginkgo.JUnitTestCase{failure, success}
}

type testCategorizer struct {
	by        string
	substring string
}

func testPodSandboxCreation(events []*monitor.EventInterval) []*ginkgo.JUnitTestCase {
	const testName = "[sig-network] pods should successfully create sandboxes"
	// we can further refine this signal by subdividing different failure modes if it is pertinent.  Right now I'm seeing
	// 1. error reading container (probably exited) json message: EOF
	// 2. dial tcp 10.0.76.225:6443: i/o timeout
	// 3. error getting pod: pods "terminate-cmd-rpofb45fa14c-96bb-40f7-bd9e-346721740cac" not found
	// 4. write child: broken pipe
	bySubStrings := []testCategorizer{
		{by: " by reading container", substring: "error reading container (probably exited) json message: EOF"},
		{by: " by not timing out", substring: "i/o timeout"},
		{by: " by writing network status", substring: "error setting the networks status"},
		{by: " by getting pod", substring: " error getting pod: pods"},
		{by: " by writing child", substring: "write child: broken pipe"},
		{by: " by other", substring: " "}, // always matches
	}

	failures := []string{}
	flakes := []string{}
	eventsForPods := getEventsByPod(events)
	for _, event := range events {
		if !strings.Contains(event.Message, "reason/FailedCreatePodSandBox Failed to create pod sandbox") {
			continue
		}
		deletionTime := getPodDeletionTime(eventsForPods[event.Locator], event.Locator)
		if deletionTime == nil {
			// this indicates a failure to create the sandbox that should not happen
			failures = append(failures, fmt.Sprintf("%v - never deleted - %v", event.Locator, event.Message))
		} else {
			timeBetweenDeleteAndFailure := event.From.Sub(*deletionTime)
			switch {
			case timeBetweenDeleteAndFailure < 1*time.Second:
				// nothing here, one second is close enough to be ok, the kubelet and CNI just didn't know
			case timeBetweenDeleteAndFailure < 5*time.Second:
				// withing five seconds, it ought to be long enough to know, but it's close enough to flake and not fail
				flakes = append(failures, fmt.Sprintf("%v - %0.2f seconds after deletion - %v", event.Locator, timeBetweenDeleteAndFailure.Seconds(), event.Message))
			case deletionTime.Before(event.From):
				// something went wrong.  More than five seconds after the pod ws deleted, the CNI is trying to set up pod sandboxes and can't
				failures = append(failures, fmt.Sprintf("%v - %0.2f seconds after deletion - %v", event.Locator, timeBetweenDeleteAndFailure.Seconds(), event.Message))
			default:
				// something went wrong.  deletion happend after we had a failure to create the pod sandbox
				failures = append(failures, fmt.Sprintf("%v - deletion came AFTER sandbox failure - %v", event.Locator, event.Message))
			}
		}
	}
	if len(failures) == 0 && len(flakes) == 0 {
		successes := []*ginkgo.JUnitTestCase{}
		for _, by := range bySubStrings {
			successes = append(successes, &ginkgo.JUnitTestCase{Name: testName + by.by})
		}
		return successes
	}

	ret := []*ginkgo.JUnitTestCase{}
	failuresBySubtest, flakesBySubtest := categorizeBySubset(bySubStrings, failures, flakes)

	// now iterate the individual failures to create failure entries
	for by, subFailures := range failuresBySubtest {
		failure := &ginkgo.JUnitTestCase{
			Name:      testName + by,
			SystemOut: strings.Join(subFailures, "\n"),
			FailureOutput: &ginkgo.FailureOutput{
				Output: fmt.Sprintf("%d failures to create the sandbox\n\n%v", len(subFailures), strings.Join(subFailures, "\n")),
			},
		}
		ret = append(ret, failure)
	}
	for by, subFlakes := range flakesBySubtest {
		flake := &ginkgo.JUnitTestCase{
			Name:      testName + by,
			SystemOut: strings.Join(subFlakes, "\n"),
			FailureOutput: &ginkgo.FailureOutput{
				Output: fmt.Sprintf("%d failures to create the sandbox\n\n%v", len(subFlakes), strings.Join(subFlakes, "\n")),
			},
		}
		ret = append(ret, flake)
		// write a passing test to trigger detection of this issue as a flake. Doing this first to try to see how frequent the issue actually is
		success := &ginkgo.JUnitTestCase{
			Name: testName + by,
		}
		ret = append(ret, success)
	}

	return append(ret)
}

// categorizeBySubset returns a map keyed by category for failures and flakes.  If a category is present in both failures and flakes, all are listed under failures.
func categorizeBySubset(categorizers []testCategorizer, failures, flakes []string) (map[string][]string, map[string][]string) {
	failuresBySubtest := map[string][]string{}
	flakesBySubtest := map[string][]string{}
	for _, failure := range failures {
		for _, by := range categorizers {
			if strings.Contains(failure, by.substring) {
				failuresBySubtest[by.by] = append(failuresBySubtest[by.by], failure)
				break // break after first match so we only add each failure one bucket
			}
		}
	}

	for _, flake := range flakes {
		for _, by := range categorizers {
			if strings.Contains(flake, by.substring) {
				if _, isFailure := failuresBySubtest[by.by]; isFailure {
					failuresBySubtest[by.by] = append(failuresBySubtest[by.by], flake)
				} else {
					flakesBySubtest[by.by] = append(flakesBySubtest[by.by], flake)
				}
				break // break after first match so we only add each failure one bucket
			}
		}
	}
	return failuresBySubtest, flakesBySubtest
}

func getPodCreationTime(events []*monitor.EventInterval, podLocator string) *time.Time {
	for _, event := range events {
		if event.Locator == podLocator && event.Message == "reason/Created" {
			return &event.From
		}
	}
	return nil
}

func getPodDeletionTime(events []*monitor.EventInterval, podLocator string) *time.Time {
	for _, event := range events {
		if event.Locator == podLocator && event.Message == "reason/Deleted" {
			return &event.From
		}
	}
	return nil
}

// getEventsByPod returns map keyed by pod locator with all events associated with it.
func getEventsByPod(events []*monitor.EventInterval) map[string][]*monitor.EventInterval {
	eventsByPods := map[string][]*monitor.EventInterval{}
	for _, event := range events {
		if !strings.Contains(event.Locator, "pod/") {
			continue
		}
		eventsByPods[event.Locator] = append(eventsByPods[event.Locator], event)
	}
	return eventsByPods
}
