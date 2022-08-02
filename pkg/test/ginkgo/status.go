package ginkgo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/openshift/origin/pkg/monitor"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type testStatus struct {
	out             io.Writer
	timeout         time.Duration
	monitorRecorder monitor.Recorder
	env             []string

	afterTestFn func(t *testCase)

	includeSuccessfulOutput bool

	lock     sync.Mutex
	failures int
	index    int
	total    int
}

func newTestStatus(out io.Writer, includeSuccessfulOutput bool, total int, timeout time.Duration, monitorRecorder monitor.Recorder, testEnv []string) *testStatus {
	return &testStatus{
		out:             out,
		total:           total,
		timeout:         timeout,
		monitorRecorder: monitorRecorder,
		env:             testEnv,

		includeSuccessfulOutput: includeSuccessfulOutput,
	}
}

// AfterTest registers a function to be invoked after each test completes.
func (s *testStatus) AfterTest(fn func(t *testCase)) {
	s.afterTestFn = fn
}

// fprintf formats the provided string with the status of the test with arguments failures, index, and total
func (s *testStatus) fprintf(format string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.index < s.total {
		s.index++
	} else {
		s.index++
		s.total++
	}
	fmt.Fprintf(s.out, format, s.failures, s.index, s.total)
}

// finalizeTest outputs the result of the test to s.out, increments s.failures if necessary,
// and invokes afterTestFn if registered.
func (s *testStatus) finalizeTest(test *testCase) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// executed under the lock deliberately to prevent races in client code
	if s.afterTestFn != nil {
		defer s.afterTestFn(test)
	}

	eventMessage := "finishedStatus/Unknown reason/Unknown"
	eventLevel := monitorapi.Warning
	// output the status of the test
	switch {
	case test.flake:
		s.out.Write(test.out)
		fmt.Fprintln(s.out)
		fmt.Fprintf(s.out, "flaked: (%s) %s %q\n\n", test.duration, test.end.UTC().Format("2006-01-02T15:04:05"), test.name)
		eventMessage = "finishedStatus/Flaked"
		eventLevel = monitorapi.Error
	case test.success:
		if s.includeSuccessfulOutput {
			s.out.Write(test.out)
			fmt.Fprintln(s.out)
		}
		fmt.Fprintf(s.out, "passed: (%s) %s %q\n\n", test.duration, test.end.UTC().Format("2006-01-02T15:04:05"), test.name)
		eventMessage = "finishedStatus/Passed"
		eventLevel = monitorapi.Info
	case test.skipped:
		if s.includeSuccessfulOutput {
			s.out.Write(test.out)
			fmt.Fprintln(s.out)
		} else {
			message := lastLinesUntil(string(test.out), 100, "skip [")
			if len(message) > 0 {
				fmt.Fprintln(s.out, message)
				fmt.Fprintln(s.out)
			}
		}
		fmt.Fprintf(s.out, "skipped: (%s) %s %q\n\n", test.duration, test.end.UTC().Format("2006-01-02T15:04:05"), test.name)
		eventMessage = "finishedStatus/Skipped"
		eventLevel = monitorapi.Info
	case test.failed:
		s.failures++
		s.out.Write(test.out)
		fmt.Fprintln(s.out)
		fmt.Fprintf(s.out, "failed: (%s) %s %q\n\n", test.duration, test.end.UTC().Format("2006-01-02T15:04:05"), test.name)
		eventMessage = "finishedStatus/Failed"
		eventLevel = monitorapi.Error
		if test.timedOut {
			eventMessage = "finishedStatus/Failed  reason/Timeout"
		}
	}

	s.monitorRecorder.Record(monitorapi.Condition{
		Level:   eventLevel,
		Locator: monitorapi.E2ETestLocator(test.name),
		Message: eventMessage,
	})
}

// OutputCommand prints to stdout what would have been executed.
func (s *testStatus) OutputCommand(ctx context.Context, test *testCase) {
	buf := &bytes.Buffer{}
	for _, env := range s.env {
		parts := strings.SplitN(env, "=", 2)
		fmt.Fprintf(buf, "%s=%q ", parts[0], parts[1])
	}
	fmt.Fprintf(buf, "%s %s %q", os.Args[0], "run-test", test.name)
	fmt.Fprintln(s.out, buf.String())
}

func (s *testStatus) Run(ctx context.Context, test *testCase) {
	s.monitorRecorder.Record(monitorapi.Condition{
		Level:   monitorapi.Info,
		Locator: monitorapi.E2ETestLocator(test.name),
		Message: "started",
	})
	defer s.finalizeTest(test)

	test.start = time.Now()
	c := exec.Command(os.Args[0], "run-test", test.name)
	c.Env = append(os.Environ(), s.env...)
	s.fprintf(fmt.Sprintf("started: (%s) %q\n\n", "%d/%d/%d", test.name))

	timeout := s.timeout
	if test.testTimeout != 0 {
		timeout = test.testTimeout
	}

	out, err := runWithTimeout(ctx, c, timeout)
	test.end = time.Now()

	duration := test.end.Sub(test.start).Round(time.Second / 10)
	if duration > time.Minute {
		duration = duration.Round(time.Second)
	}
	test.duration = duration
	test.out = out
	if err == nil {
		test.success = true
		return
	}

	if ctx.Err() != nil {
		test.skipped = true
		test.flake = false
		test.failed = false
		test.success = false
		return
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		switch exitErr.ProcessState.Sys().(syscall.WaitStatus).ExitStatus() {
		case 1:
			// failed
			test.failed = true
		case 2:
			// timeout (ABRT is an exit code 2)
			test.failed = true
			test.timedOut = true
		case 3:
			// skipped
			test.skipped = true
		case 4:
			// flaky, do not retry
			test.success = true
			test.flake = true
		default:
			test.failed = true
		}
		return
	}
	test.failed = true
}

func summarizeTests(tests []*testCase) (int, int, int, []*testCase) {
	var pass, fail, skip int
	var failingTests []*testCase
	for _, t := range tests {
		switch {
		case t.success:
			pass++
		case t.failed:
			fail++
			failingTests = append(failingTests, t)
		case t.skipped:
			skip++
		}
	}
	return pass, fail, skip, failingTests
}

func sortedTests(tests []*testCase) []*testCase {
	copied := make([]*testCase, len(tests))
	copy(copied, tests)
	sort.Slice(copied, func(i, j int) bool { return copied[i].name < copied[j].name })
	return copied
}
