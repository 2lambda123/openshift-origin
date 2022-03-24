package synthetictests

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"github.com/openshift/origin/pkg/monitor/intervalcreation"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"k8s.io/client-go/rest"
)

type testCategorizer struct {
	by        string
	substring string
}

func testPodSandboxCreation(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
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
		{by: " by ovn default network ready", substring: "have you checked that your default network is ready? still waiting for readinessindicatorfile"},
		{by: " by other", substring: " "}, // always matches
	}

	failures := []string{}
	flakes := []string{}
	operatorsProgressing := intervalcreation.IntervalsFromEvents_OperatorProgressing(events, time.Time{}, time.Time{})
	networkOperatorProgressing := operatorsProgressing.Filter(func(ev monitorapi.EventInterval) bool {
		return ev.Locator == "clusteroperator/network" || ev.Locator == "clusteroperator/machine-config"
	})
	eventsForPods := getEventsByPod(events)
	for _, event := range events {
		if !strings.Contains(event.Message, "reason/FailedCreatePodSandBox Failed to create pod sandbox") {
			continue
		}
		if strings.Contains(event.Message, "Multus") &&
			strings.Contains(event.Message, "error getting pod") &&
			(strings.Contains(event.Message, "connection refused") || strings.Contains(event.Message, "i/o timeout")) {
			flakes = append(flakes, fmt.Sprintf("%v - multus is unable to get pods due to LB disruption https://bugzilla.redhat.com/show_bug.cgi?id=1927264 - %v", event.Locator, event.Message))
			continue
		}
		if strings.Contains(event.Message, "Multus") && strings.Contains(event.Message, "error getting pod: Unauthorized") {
			flakes = append(flakes, fmt.Sprintf("%v - multus is unable to get pods due to authorization https://bugzilla.redhat.com/show_bug.cgi?id=1972490 - %v", event.Locator, event.Message))
			continue
		}
		if strings.Contains(event.Message, "Multus") &&
			strings.Contains(event.Message, "have you checked that your default network is ready? still waiting for readinessindicatorfile") {
			flakes = append(flakes, fmt.Sprintf("%v - multus is unable to get pods as ovnkube-node pod has not yet written readinessindicatorfile (possibly not running due to image pull delays) https://bugzilla.redhat.com/show_bug.cgi?id=20671320 - %v", event.Locator, event.Message))
			continue
		}
		deletionTime := getPodDeletionTime(eventsForPods[event.Locator], event.Locator)
		if deletionTime == nil {
			// mark sandboxes errors as flakes if networking is being updated
			match := -1
			for i := range networkOperatorProgressing {
				matchesFrom := event.From.After(networkOperatorProgressing[i].From)
				matchesTo := event.To.Before(networkOperatorProgressing[i].To)
				if matchesFrom && matchesTo {
					match = i
					break
				}
			}
			if match != -1 {
				flakes = append(flakes, fmt.Sprintf("%v - never deleted - network rollout - %v", event.Locator, event.Message))
			} else {
				failures = append(failures, fmt.Sprintf("%v - never deleted - %v", event.Locator, event.Message))
			}
		} else {
			timeBetweenDeleteAndFailure := event.From.Sub(*deletionTime)
			switch {
			case timeBetweenDeleteAndFailure < 1*time.Second:
				// nothing here, one second is close enough to be ok, the kubelet and CNI just didn't know
			case timeBetweenDeleteAndFailure < 5*time.Second:
				// withing five seconds, it ought to be long enough to know, but it's close enough to flake and not fail
				flakes = append(flakes, fmt.Sprintf("%v - %0.2f seconds after deletion - %v", event.Locator, timeBetweenDeleteAndFailure.Seconds(), event.Message))
			case deletionTime.Before(event.From):
				// something went wrong.  More than five seconds after the pod ws deleted, the CNI is trying to set up pod sandboxes and can't
				failures = append(failures, fmt.Sprintf("%v - %0.2f seconds after deletion - %v", event.Locator, timeBetweenDeleteAndFailure.Seconds(), event.Message))
			default:
				// something went wrong.  deletion happend after we had a failure to create the pod sandbox
				failures = append(failures, fmt.Sprintf("%v - deletion came AFTER sandbox failure - %v", event.Locator, event.Message))
			}
		}
	}
	failuresBySubtest, flakesBySubtest := categorizeBySubset(bySubStrings, failures, flakes)
	successes := []*junitapi.JUnitTestCase{}
	for _, by := range bySubStrings {
		if _, ok := failuresBySubtest[by.by]; ok {
			continue
		}
		if _, ok := flakesBySubtest[by.by]; ok {
			continue
		}

		successes = append(successes, &junitapi.JUnitTestCase{Name: testName + by.by})
	}

	if len(failures) == 0 && len(flakes) == 0 {
		return successes
	}

	ret := []*junitapi.JUnitTestCase{}
	// now iterate the individual failures to create failure entries
	for by, subFailures := range failuresBySubtest {
		failure := &junitapi.JUnitTestCase{
			Name:      testName + by,
			SystemOut: strings.Join(subFailures, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("%d failures to create the sandbox\n\n%v", len(subFailures), strings.Join(subFailures, "\n")),
			},
		}
		ret = append(ret, failure)
	}
	for by, subFlakes := range flakesBySubtest {
		flake := &junitapi.JUnitTestCase{
			Name:      testName + by,
			SystemOut: strings.Join(subFlakes, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("%d failures to create the sandbox\n\n%v", len(subFlakes), strings.Join(subFlakes, "\n")),
			},
		}
		ret = append(ret, flake)
		// write a passing test to trigger detection of this issue as a flake. Doing this first to try to see how frequent the issue actually is
		success := &junitapi.JUnitTestCase{
			Name: testName + by,
		}
		ret = append(ret, success)
	}

	// add our successes
	ret = append(ret, successes...)

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

// getEventsByPod returns map keyed by pod locator with all events associated with it.
func getEventsByPod(events monitorapi.Intervals) map[string]monitorapi.Intervals {
	eventsByPods := map[string]monitorapi.Intervals{}
	for _, event := range events {
		if !strings.Contains(event.Locator, "pod/") {
			continue
		}
		eventsByPods[event.Locator] = append(eventsByPods[event.Locator], event)
	}
	return eventsByPods
}

func getPodDeletionTime(events monitorapi.Intervals, podLocator string) *time.Time {
	for _, event := range events {
		if event.Locator == podLocator && event.Message == "reason/Deleted" {
			return &event.From
		}
	}
	return nil
}

// bug is tracked here: https://bugzilla.redhat.com/show_bug.cgi?id=2057181
func testOvnNodeReadinessProbe(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	const testName = "[bz-networking] ovnkube-node readiness probe should not fail repeatedly"
	regExp := regexp.MustCompile(ovnReadinessRegExpStr)
	var tests []*junitapi.JUnitTestCase
	var failureOutput string
	msgMap := map[string]bool{}
	for _, event := range events {
		msg := fmt.Sprintf("%s - %s", event.Locator, event.Message)
		if regExp.MatchString(msg) {
			if _, ok := msgMap[msg]; !ok {
				msgMap[msg] = true
				eventDisplayMessage, times := getTimesAnEventHappened(msg)
				if times > duplicateEventThreshold {
					// if the readiness probe failure for this pod happened AFTER the initial installation was complete,
					// then this probe failure is unexpected and should fail.
					isDuringInstall, err := isEventDuringInstallation(event, kubeClientConfig, regExp)
					if err != nil {
						failureOutput += fmt.Sprintf("error [%v] happened when processing event [%s]\n", err, eventDisplayMessage)
					} else if !isDuringInstall {
						failureOutput += fmt.Sprintf("event [%s] happened too frequently for %d times\n", eventDisplayMessage, times)
					}
				}
			}
		}
	}
	test := &junitapi.JUnitTestCase{Name: testName}
	if len(failureOutput) > 0 {
		test.FailureOutput = &junitapi.FailureOutput{
			Output: failureOutput,
		}
	}
	tests = append(tests, test)
	return tests
}
