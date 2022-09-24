package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/kubernetes/test/e2e/upgrades"

	"github.com/openshift/origin/pkg/synthetictests"
	"github.com/openshift/origin/pkg/test/ginkgo"
	"github.com/openshift/origin/test/e2e/upgrade"
	exutil "github.com/openshift/origin/test/extended/util"
)

// upgradeSuites are all known upgrade test suites this binary should run
var upgradeSuites = testSuites{
	{
		TestSuite: ginkgo.TestSuite{
			Name: "all",
			Description: templates.LongDesc(`
		Run all tests.
		`),
			Matches: func(name string) bool {
				if isStandardEarlyTest(name) {
					return true
				}
				return strings.Contains(name, "[Feature:ClusterUpgrade]") && !strings.Contains(name, "[Suite:k8s]")
			},
			TestTimeout:         240 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.SystemUpgradeEventInvariants),
		},
		PreSuite: upgradeTestPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "platform",
			Description: templates.LongDesc(`
		Run only the tests that verify the platform remains available.
		`),
			Matches: func(name string) bool {
				if isStandardEarlyTest(name) {
					return true
				}
				return strings.Contains(name, "[Feature:ClusterUpgrade]") && !strings.Contains(name, "[Suite:k8s]")
			},
			TestTimeout:         240 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.SystemUpgradeEventInvariants),
		},
		PreSuite: upgradeTestPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "none",
			Description: templates.LongDesc(`
	Don't run disruption tests.
		`),
			Matches: func(name string) bool {
				if isStandardEarlyTest(name) {
					return true
				}
				return strings.Contains(name, "[Feature:ClusterUpgrade]") && !strings.Contains(name, "[Suite:k8s]")
			},
			TestTimeout:         240 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.SystemUpgradeEventInvariants),
		},
		PreSuite: upgradeTestPreSuite,
	},
}

// upgradeTestPreSuite validates the test options and gathers data useful prior to launching the upgrade and it's
// related tests.
func upgradeTestPreSuite(opt *runOptions) error {
	if !opt.DryRun {
		testOpt := ginkgo.NewTestOptions(os.Stdout, os.Stderr)
		config, err := decodeProvider(os.Getenv("TEST_PROVIDER"), testOpt.DryRun, true, opt.JUnitDir, nil)
		if err != nil {
			return err
		}
		if err := initializeTestFramework(exutil.TestContext, config, testOpt.DryRun); err != nil {
			return err
		}
		klog.V(4).Infof("Loaded test configuration: %#v", exutil.TestContext)

		if err := upgrade.GatherPreUpgradeResourceCounts(); err != nil {
			return errors.Wrap(err, "error gathering preupgrade resource counts")
		}
	}

	// Upgrade test output is important for debugging because it shows linear progress
	// and when the CVO hangs.
	opt.IncludeSuccessOutput = true
	return parseUpgradeOptions(opt.TestOptions)
}

// upgradeTestPreTest uses variables set at suite execution time to prepare the upgrade
// test environment in process (setting constants in the upgrade packages).
func upgradeTestPreTest() error {
	value := os.Getenv("TEST_UPGRADE_OPTIONS")
	if len(value) == 0 {
		return nil
	}

	var opt UpgradeOptions
	if err := json.Unmarshal([]byte(value), &opt); err != nil {
		return err
	}
	parseUpgradeOptions(opt.TestOptions)
	upgrade.SetToImage(opt.ToImage)
	switch opt.Suite {
	case "none":
		return filterUpgrade(upgrade.NoTests(), func(string) bool { return true })
	case "platform":
		return filterUpgrade(upgrade.AllTests(), func(name string) bool {
			return false
		})
	default:
		return filterUpgrade(upgrade.AllTests(), func(string) bool { return true })
	}
}

func parseUpgradeOptions(options []string) error {
	for _, opt := range options {
		parts := strings.SplitN(opt, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("expected option of the form KEY=VALUE instead of %q", opt)
		}
		switch parts[0] {
		case "abort-at":
			if err := upgrade.SetUpgradeAbortAt(parts[1]); err != nil {
				return err
			}
		case "disrupt-reboot":
			if err := upgrade.SetUpgradeDisruptReboot(parts[1]); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unrecognized upgrade option: %s", parts[0])
		}
	}
	return nil
}

type UpgradeOptions struct {
	Suite       string
	ToImage     string
	TestOptions []string
}

func (o *UpgradeOptions) ToEnv() string {
	out, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	return string(out)
}

func filterUpgrade(tests []upgrades.Test, match func(string) bool) error {
	var scope []upgrades.Test
	for _, test := range tests {
		if match(test.Name()) {
			scope = append(scope, test)
		}
	}
	upgrade.SetTests(scope)
	return nil
}

func bindUpgradeOptions(opt *runOptions, flags *pflag.FlagSet) {
	flags.StringVar(&opt.ToImage, "to-image", opt.ToImage, "Specify the image to test an upgrade to.")
	flags.StringSliceVar(&opt.TestOptions, "options", opt.TestOptions, "A set of KEY=VALUE options to control the test. See the help text.")
}
