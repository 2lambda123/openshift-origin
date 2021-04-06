package ginkgo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/openshift/origin/test/extended/testdata"

	"github.com/onsi/ginkgo/config"
	"github.com/openshift/origin/pkg/monitor"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Options is used to run a suite of tests by invoking each test
// as a call to a child worker (the run-tests command).
type Options struct {
	Parallelism int
	Count       int
	FailFast    bool
	Timeout     time.Duration
	JUnitDir    string
	TestFile    string
	OutFile     string

	// Regex allows a selection of a subset of tests
	Regex string
	// MatchFn if set is also used to filter the suite contents
	MatchFn func(name string) bool

	// SyntheticEventTests allows the caller to translate events or outside
	// context into a failure.
	SyntheticEventTests JUnitsForEvents

	IncludeSuccessOutput bool

	CommandEnv []string

	DryRun        bool
	PrintCommands bool
	Out, ErrOut   io.Writer

	StartTime time.Time
}

func (opt *Options) AsEnv() []string {
	var args []string
	args = append(args, fmt.Sprintf("TEST_SUITE_START_TIME=%d", opt.StartTime.Unix()))
	args = append(args, opt.CommandEnv...)
	return args
}

func (opt *Options) SelectSuite(suites []*TestSuite, args []string) (*TestSuite, error) {
	var suite *TestSuite

	if len(opt.TestFile) > 0 {
		var in []byte
		var err error
		if opt.TestFile == "-" {
			in, err = ioutil.ReadAll(os.Stdin)
			if err != nil {
				return nil, err
			}
		} else {
			in, err = ioutil.ReadFile(opt.TestFile)
		}
		if err != nil {
			return nil, err
		}
		suite, err = newSuiteFromFile("files", in)
		if err != nil {
			return nil, fmt.Errorf("could not read test suite from input: %v", err)
		}
	}

	if suite == nil && len(args) == 0 {
		fmt.Fprintf(opt.ErrOut, SuitesString(suites, "Select a test suite to run against the server:\n\n"))
		return nil, fmt.Errorf("specify a test suite to run, for example: %s run %s", filepath.Base(os.Args[0]), suites[0].Name)
	}
	if suite == nil && len(args) > 0 {
		for _, s := range suites {
			if s.Name == args[0] {
				suite = s
				break
			}
		}
	}
	if suite == nil {
		fmt.Fprintf(opt.ErrOut, SuitesString(suites, "Select a test suite to run against the server:\n\n"))
		return nil, fmt.Errorf("suite %q does not exist", args[0])
	}
	return suite, nil
}

func (opt *Options) Run(suite *TestSuite) error {
	if len(opt.Regex) > 0 {
		if err := filterWithRegex(suite, opt.Regex); err != nil {
			return err
		}
	}
	if opt.MatchFn != nil {
		original := suite.Matches
		suite.Matches = func(name string) bool {
			return original(name) && opt.MatchFn(name)
		}
	}

	syntheticEventTests := JUnitsForAllEvents{
		opt.SyntheticEventTests,
		suite.SyntheticEventTests,
	}

	tests, err := testsForSuite(config.GinkgoConfig)
	if err != nil {
		return err
	}

	// This ensures that tests in the identified paths do not run in parallel, because
	// the test suite reuses shared resources without considering whether another test
	// could be running at the same time. While these are technically [Serial], ginkgo
	// parallel mode provides this guarantee. Doing this for all suites would be too
	// slow.
	setTestExclusion(tests, func(suitePath string, t *testCase) bool {
		for _, name := range []string{
			"/k8s.io/kubernetes/test/e2e/apps/disruption.go",
		} {
			if strings.HasSuffix(suitePath, name) {
				return true
			}
		}
		return false
	})

	tests = suite.Filter(tests)
	if len(tests) == 0 {
		return fmt.Errorf("suite %q does not contain any tests", suite.Name)
	}

	count := opt.Count
	if count == 0 {
		count = suite.Count
	}

	start := time.Now()
	if opt.StartTime.IsZero() {
		opt.StartTime = start
	}

	if opt.PrintCommands {
		status := newTestStatus(opt.Out, true, len(tests), time.Minute, &monitor.Monitor{}, monitor.NewNoOpMonitor(), opt.AsEnv())
		newParallelTestQueue().Execute(context.Background(), tests, 1, status.OutputCommand)
		return nil
	}
	if opt.DryRun {
		for _, test := range sortedTests(tests) {
			fmt.Fprintf(opt.Out, "%q\n", test.name)
		}
		return nil
	}

	if len(opt.JUnitDir) > 0 {
		if _, err := os.Stat(opt.JUnitDir); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("could not access --junit-dir: %v", err)
			}
			if err := os.MkdirAll(opt.JUnitDir, 0755); err != nil {
				return fmt.Errorf("could not create --junit-dir: %v", err)
			}
		}
	}

	parallelism := opt.Parallelism
	if parallelism == 0 {
		parallelism = suite.Parallelism
	}
	if parallelism == 0 {
		parallelism = 10
	}
	timeout := opt.Timeout
	if timeout == 0 {
		timeout = suite.TestTimeout
	}
	if timeout == 0 {
		timeout = 15 * time.Minute
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	abortCh := make(chan os.Signal)
	go func() {
		<-abortCh
		fmt.Fprintf(opt.ErrOut, "Interrupted, terminating tests\n")
		cancelFn()
		sig := <-abortCh
		fmt.Fprintf(opt.ErrOut, "Interrupted twice, exiting (%s)\n", sig)
		switch sig {
		case syscall.SIGINT:
			os.Exit(130)
		default:
			os.Exit(0)
		}
	}()
	signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

	m, err := monitor.Start(ctx)
	if err != nil {
		return err
	}
	// if we run a single test, always include success output
	includeSuccess := opt.IncludeSuccessOutput
	if len(tests) == 1 && count == 1 {
		includeSuccess = true
	}

	early, normal := splitTests(tests, func(t *testCase) bool {
		return strings.Contains(t.name, "[Early]")
	})

	late, normal := splitTests(normal, func(t *testCase) bool {
		return strings.Contains(t.name, "[Late]")
	})

	expectedTestCount := len(early) + len(late)
	if count != -1 {
		original := normal
		for i := 1; i < count; i++ {
			normal = append(normal, copyTests(original)...)
		}
	}
	expectedTestCount += len(normal)

	status := newTestStatus(opt.Out, includeSuccess, expectedTestCount, timeout, m, m, opt.AsEnv())
	testCtx := ctx
	if opt.FailFast {
		var cancelFn context.CancelFunc
		testCtx, cancelFn = context.WithCancel(testCtx)
		status.AfterTest(func(t *testCase) {
			if t.failed {
				cancelFn()
			}
		})
	}

	tests = nil

	// run our Early tests
	q := newParallelTestQueue()
	q.Execute(testCtx, early, parallelism, status.Run)
	tests = append(tests, early...)

	// repeat the normal suite until context cancel when in the forever loop
	for i := 0; (i < 1 || count == -1) && testCtx.Err() == nil; i++ {
		copied := copyTests(normal)
		q.Execute(testCtx, copied, parallelism, status.Run)
		tests = append(tests, copied...)
	}

	// run Late test suits after everything else
	q.Execute(testCtx, late, parallelism, status.Run)
	tests = append(tests, late...)

	// calculate the effective test set we ran, excluding any incompletes
	tests, _ = splitTests(tests, func(t *testCase) bool { return t.success || t.failed || t.skipped })

	duration := time.Now().Sub(start).Round(time.Second / 10)
	if duration > time.Minute {
		duration = duration.Round(time.Second)
	}

	pass, fail, skip, failing := summarizeTests(tests)

	// monitor the cluster while the tests are running and report any detected anomalies
	var syntheticTestResults []*JUnitTestCase
	var syntheticFailure bool
	timeSuffix := fmt.Sprintf("_%s", start.UTC().Format("20060102-150405"))
	events := m.EventIntervals(time.Time{}, time.Time{})
	if err = monitorserialization.EventsToFile(path.Join(os.Getenv("ARTIFACT_DIR"), fmt.Sprintf("e2e-events%s.json", timeSuffix)), events); err != nil {
		fmt.Fprintf(opt.Out, "Failed to write event file: %v\n", err)
	}
	if err = monitorserialization.EventsIntervalsToFile(path.Join(os.Getenv("ARTIFACT_DIR"), fmt.Sprintf("e2e-intervals%s.json", timeSuffix)), events); err != nil {
		fmt.Fprintf(opt.Out, "Failed to write event file: %v\n", err)
	}
	if eventIntervalsJSON, err := monitorserialization.EventsIntervalsToJSON(events); err == nil {
		e2eChartTemplate := testdata.MustAsset("e2echart/e2e-chart-template.html")
		e2eChartHTML := bytes.ReplaceAll(e2eChartTemplate, []byte("EVENT_INTERVAL_JSON_GOES_HERE"), eventIntervalsJSON)
		e2eChartHTMLPath := path.Join(os.Getenv("ARTIFACT_DIR"), fmt.Sprintf("e2e-intervals%s.html", timeSuffix))
		if err := ioutil.WriteFile(e2eChartHTMLPath, e2eChartHTML, 0644); err != nil {
			fmt.Fprintf(opt.Out, "Failed to write event html: %v\n", err)
		}
	} else {
		fmt.Fprintf(opt.Out, "Failed to write event html: %v\n", err)
	}

	if len(events) > 0 {
		eventsForTests := createEventsForTests(tests)

		var buf *bytes.Buffer
		syntheticTestResults, buf, _ = createSyntheticTestsFromMonitor(m, eventsForTests, duration)
		testCases := syntheticEventTests.JUnitsForEvents(events, duration)
		syntheticTestResults = append(syntheticTestResults, testCases...)

		if len(syntheticTestResults) > 0 {
			// mark any failures by name
			failing, flaky := sets.NewString(), sets.NewString()
			for _, test := range syntheticTestResults {
				if test.FailureOutput != nil {
					failing.Insert(test.Name)
				}
			}
			// if a test has both a pass and a failure, flag it
			// as a flake
			for _, test := range syntheticTestResults {
				if test.FailureOutput == nil {
					if failing.Has(test.Name) {
						flaky.Insert(test.Name)
					}
				}
			}
			failing = failing.Difference(flaky)
			if failing.Len() > 0 {
				fmt.Fprintf(buf, "Failing invariants:\n\n%s\n\n", strings.Join(failing.List(), "\n"))
				syntheticFailure = true
			}
			if flaky.Len() > 0 {
				fmt.Fprintf(buf, "Flaky invariants:\n\n%s\n\n", strings.Join(flaky.List(), "\n"))
			}
		}

		opt.Out.Write(buf.Bytes())
	}

	// attempt to retry failures to do flake detection
	if fail > 0 && fail <= suite.MaximumAllowedFlakes {
		var retries []*testCase
		for _, test := range failing {
			retry := test.Retry()
			retries = append(retries, retry)
			tests = append(tests, retry)
			if len(retries) > suite.MaximumAllowedFlakes {
				break
			}
		}

		q := newParallelTestQueue()
		status := newTestStatus(ioutil.Discard, opt.IncludeSuccessOutput, len(retries), timeout, m, m, opt.AsEnv())
		q.Execute(testCtx, retries, parallelism, status.Run)
		var flaky []string
		var repeatFailures []*testCase
		for _, test := range retries {
			if test.success {
				flaky = append(flaky, test.name)
			} else {
				repeatFailures = append(repeatFailures, test)
			}
		}
		if len(flaky) > 0 {
			failing = repeatFailures
			sort.Strings(flaky)
			fmt.Fprintf(opt.Out, "Flaky tests:\n\n%s\n\n", strings.Join(flaky, "\n"))
		}
	}

	// report the outcome of the test
	if len(failing) > 0 {
		names := sets.NewString(testNames(failing)...).List()
		fmt.Fprintf(opt.Out, "Failing tests:\n\n%s\n\n", strings.Join(names, "\n"))
	}

	if len(opt.JUnitDir) > 0 {
		if err := writeJUnitReport("junit_e2e", "openshift-tests", tests, opt.JUnitDir, duration, opt.ErrOut, syntheticTestResults...); err != nil {
			fmt.Fprintf(opt.Out, "error: Unable to write e2e JUnit results: %v", err)
		}
	}

	if fail > 0 {
		if len(failing) > 0 || suite.MaximumAllowedFlakes == 0 {
			return fmt.Errorf("%d fail, %d pass, %d skip (%s)", fail, pass, skip, duration)
		}
		fmt.Fprintf(opt.Out, "%d flakes detected, suite allows passing with only flakes\n\n", fail)
	}

	if syntheticFailure {
		return fmt.Errorf("failed because an invariant was violated, %d pass, %d skip (%s)\n", pass, skip, duration)
	}

	fmt.Fprintf(opt.Out, "%d pass, %d skip (%s)\n", pass, skip, duration)
	return ctx.Err()
}
