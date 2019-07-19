package main

import (
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	"github.com/openshift/origin/pkg/test/ginkgo"

	_ "github.com/openshift/origin/test/extended"
)

// staticSuites are all known test suites this binary should run
var staticSuites = []*ginkgo.TestSuite{
	{
		Name: "openshift/conformance",
		Description: templates.LongDesc(`
		Tests that ensure an OpenShift cluster and components are working properly.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Suite:openshift/conformance/")
		},
		Parallelism: 30,
	},
	{
		Name: "openshift/conformance/parallel",
		Description: templates.LongDesc(`
		Only the portion of the openshift/conformance test suite that run in parallel.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Suite:openshift/conformance/parallel")
		},
		Parallelism:          30,
		MaximumAllowedFlakes: 15,
	},
	{
		Name: "openshift/conformance/serial",
		Description: templates.LongDesc(`
		Only the portion of the openshift/conformance test suite that run serially.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Suite:openshift/conformance/serial")
		},
	},
	{
		Name: "openshift/conformance/multitenant",
		Description: templates.LongDesc(`
		Only the portion of the openshift/conformance test suite that applies to the openshift-sdn multitenant plugin.
		`),
		Matches: func(name string) bool {
			return !strings.Contains(name, "[Feature:NetworkPolicy]") && strings.Contains(name, "[Suite:openshift/conformance/")
		},
		Parallelism: 30,
	},
	{
		Name: "openshift/disruptive",
		Description: templates.LongDesc(`
		The disruptive test suite.
		`),
		Matches: func(name string) bool {
			return !strings.Contains(name, "[Disabled") && strings.Contains(name, "[Disruptive]")
		},
	},
	{
		Name: "kubernetes/conformance",
		Description: templates.LongDesc(`
		The default Kubernetes conformance suite.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Suite:k8s]") && strings.Contains(name, "[Conformance]")
		},
		Parallelism: 30,
	},
	{
		Name: "openshift/build",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift build functionality.
		`),
		Matches: func(name string) bool {
			return !strings.Contains(name, "[Disabled") && strings.Contains(name, "[Feature:Builds]")
		},
		Parallelism: 7,
		// TODO: Builds are really flaky right now, remove when we land perf updates and fix io on workers
		MaximumAllowedFlakes: 3,
		// Jenkins tests can take a really long time
		TestTimeout: 60 * time.Minute,
	},
	{
		Name: "openshift/image-registry",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift image-registry functionality.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[registry]") && !strings.Contains(name, "[Local]")
		},
	},
	{
		Name: "openshift/image-ecosystem",
		Description: templates.LongDesc(`
		Tests that exercise language and tooling images shipped as part of OpenShift.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[image_ecosystem]") && !strings.Contains(name, "[Local]")
		},
		Parallelism: 7,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/jenkins-e2e",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift / Jenkins integrations provided by the OpenShift Jenkins image/plugins and the Pipeline Build Strategy.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Feature:Jenkins]")
		},
		Parallelism: 3,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/scalability",
		Description: templates.LongDesc(`
		Tests that verify the scalability characteristics of the cluster. Currently this is focused on core performance behaviors and preventing regressions.
		`),
		Matches: func(name string) bool {
			return strings.Contains(name, "[Suite:openshift/scalability]")
		},
		Parallelism: 1,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/conformance-excluded",
		Description: templates.LongDesc(`
		Run only tests that are excluded from conformance. Makes identifying omitted tests easier.
		`),
		Matches: func(name string) bool { return !strings.Contains(name, "[Suite:openshift/conformance/") },
	},
	{
		Name: "all",
		Description: templates.LongDesc(`
		Run all tests.
		`),
		Matches: func(name string) bool { return true },
	},
}
