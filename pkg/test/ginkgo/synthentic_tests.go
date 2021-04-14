package ginkgo

import (
	"bytes"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// JUnitsForEvents returns a set of JUnit results for the provided events encountered
// during a test suite run.
type JUnitsForEvents interface {
	// JUnitsForEvents returns a set of additional test passes or failures implied by the
	// events sent during the test suite run. If passed is false, the entire suite is failed.
	// To set a test as flaky, return a passing and failing JUnitTestCase with the same name.
	JUnitsForEvents(events monitorapi.EventIntervals, duration time.Duration) []*JUnitTestCase
}

// JUnitForEventsFunc converts a function into the JUnitForEvents interface.
type JUnitForEventsFunc func(events monitorapi.EventIntervals, duration time.Duration) []*JUnitTestCase

func (fn JUnitForEventsFunc) JUnitsForEvents(events monitorapi.EventIntervals, duration time.Duration) []*JUnitTestCase {
	return fn(events, duration)
}

// JUnitsForAllEvents aggregates multiple JUnitsForEvent interfaces and returns
// the result of all invocations. It ignores nil interfaces.
type JUnitsForAllEvents []JUnitsForEvents

func (a JUnitsForAllEvents) JUnitsForEvents(events monitorapi.EventIntervals, duration time.Duration) []*JUnitTestCase {
	var all []*JUnitTestCase
	for _, obj := range a {
		if obj == nil {
			continue
		}
		results := obj.JUnitsForEvents(events, duration)
		all = append(all, results...)
	}
	return all
}

func createSyntheticTestsFromMonitor(events monitorapi.EventIntervals, monitorDuration time.Duration) ([]*JUnitTestCase, *bytes.Buffer, *bytes.Buffer) {
	var syntheticTestResults []*JUnitTestCase

	buf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	fmt.Fprintf(buf, "\nTimeline:\n\n")
	errorCount := 0
	for _, event := range events {
		if event.Level == monitorapi.Error {
			errorCount++
			fmt.Fprintln(errBuf, event.String())
		}
		fmt.Fprintln(buf, event.String())
	}
	fmt.Fprintln(buf)

	if errorCount > 0 {
		syntheticTestResults = append(
			syntheticTestResults,
			&JUnitTestCase{
				Name:      "[sig-arch] Monitor cluster while tests execute",
				SystemOut: buf.String(),
				Duration:  monitorDuration.Seconds(),
				FailureOutput: &FailureOutput{
					Output: fmt.Sprintf("%d error level events were detected during this test run:\n\n%s", errorCount, errBuf.String()),
				},
			},
			// write a passing test to trigger detection of this issue as a flake, indicating we have no idea whether
			// these are actual failures or not
			&JUnitTestCase{
				Name:     "[sig-arch] Monitor cluster while tests execute",
				Duration: monitorDuration.Seconds(),
			},
		)
	}

	return syntheticTestResults, buf, errBuf
}
