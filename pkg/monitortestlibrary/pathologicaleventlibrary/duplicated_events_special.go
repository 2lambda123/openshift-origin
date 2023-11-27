package pathologicaleventlibrary

import (
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

type singleEventThresholdCheck struct {
	testName       string
	matcher        *SimplePathologicalEventMatcher
	failThreshold  int
	flakeThreshold int
}

// Test goes through the events, looks for a match using the s.recognizer function,
// if a match is found, marks it as failure or flake depending on if the pattern occurs
// above the fail/flake thresholds (this allows us to track the occurence as a specific
// Test. If the fail threshold is set to -1, the Test will only flake.
func (s *singleEventThresholdCheck) Test(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	success := &junitapi.JUnitTestCase{Name: s.testName}
	var failureOutput, flakeOutput []string
	for _, e := range events {
		if s.matcher.Allows(e, "") {
			msg := fmt.Sprintf("%s - %s", e.Locator, e.StructuredMessage.HumanMessage)
			times := GetTimesAnEventHappened(e.StructuredMessage)
			switch {
			case s.failThreshold > 0 && times > s.failThreshold:
				failureOutput = append(failureOutput, fmt.Sprintf("event [%s] happened %d times", msg, times))
			case times > s.flakeThreshold:
				flakeOutput = append(flakeOutput, fmt.Sprintf("event [%s] happened %d times", msg, times))
			}
		}
	}
	if len(failureOutput) > 0 {
		totalOutput := failureOutput
		failure := &junitapi.JUnitTestCase{
			Name:      s.testName,
			SystemOut: strings.Join(totalOutput, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: strings.Join(totalOutput, "\n"),
			},
		}

		return []*junitapi.JUnitTestCase{failure}
	}
	if len(flakeOutput) > 0 {
		failure := &junitapi.JUnitTestCase{
			Name:      s.testName,
			SystemOut: strings.Join(flakeOutput, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: strings.Join(flakeOutput, "\n"),
			},
		}
		return []*junitapi.JUnitTestCase{failure, success}
	}

	return []*junitapi.JUnitTestCase{success}
}

func NewSingleEventThresholdCheck(testName string, matcher *SimplePathologicalEventMatcher, failThreshold, flakeThreshold int) *singleEventThresholdCheck {
	return &singleEventThresholdCheck{
		testName:       testName,
		matcher:        matcher,
		failThreshold:  failThreshold,
		flakeThreshold: flakeThreshold,
	}
}

func MakeProbeTest(testName string, events monitorapi.Intervals, operatorName string,
	matcher *SimplePathologicalEventMatcher, eventFlakeThreshold int) []*junitapi.JUnitTestCase {
	return eventMatchThresholdTest(testName, operatorName, events, matcher, eventFlakeThreshold)
}

func EventExprMatchThresholdTest(testName string, events monitorapi.Intervals, matcher *SimplePathologicalEventMatcher, eventFlakeThreshold int) []*junitapi.JUnitTestCase {
	return eventMatchThresholdTest(testName, "", events, matcher, eventFlakeThreshold)
}

func eventMatchThresholdTest(testName, operatorName string, events monitorapi.Intervals, matcher *SimplePathologicalEventMatcher, eventFlakeThreshold int) []*junitapi.JUnitTestCase {
	var maxFailureOutput string
	maxTimes := 0
	for _, event := range events {
		// Layer in an additional namespace check, our matcher may work against multiple namespaces/operators, but we
		// want to limit to a specific one specific tests against a namespace/operator:
		if operatorName != "" && event.StructuredLocator.Keys[monitorapi.LocatorNamespaceKey] != operatorName {
			continue
		}

		if matcher.Allows(event, "") {
			// Place the failure time in the message to avoid having to extract the time from the events json file
			// (in artifacts) when viewing the Test failure output.
			failureOutput := fmt.Sprintf("%s %s\n", event.From.UTC().Format("15:04:05"), event.String())

			times := GetTimesAnEventHappened(event.StructuredMessage)

			// find the largest grouping of these events
			if times > maxTimes {
				maxTimes = times
				maxFailureOutput = failureOutput
			}
		}
	}

	test := &junitapi.JUnitTestCase{Name: testName}

	if maxTimes < eventFlakeThreshold {
		return []*junitapi.JUnitTestCase{test}
	}

	// Flake for now.
	test.FailureOutput = &junitapi.FailureOutput{
		Output: maxFailureOutput,
	}
	success := &junitapi.JUnitTestCase{Name: testName}
	return []*junitapi.JUnitTestCase{test, success}
}
