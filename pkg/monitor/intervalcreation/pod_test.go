package intervalcreation

import (
	_ "embed"
	"encoding/json"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"github.com/google/go-cmp/cmp"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

//go:embed pod_test_01_simple.json
var simplePodLifecyleJSON []byte

func TestIntervalCreation(t *testing.T) {
	inputIntervals, err := monitorserialization.EventsFromJSON(simplePodLifecyleJSON)
	if err != nil {
		t.Fatal(err)
	}
	startTime, err := time.Parse(time.RFC3339, "2022-03-07T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	endTime, err := time.Parse(time.RFC3339, "2022-03-07T23:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	result := CreatePodIntervalsFromInstants(inputIntervals, monitorapi.ResourcesMap{}, startTime, endTime)

	resultBytes, err := monitorserialization.EventsToJSON(result)
	if err != nil {
		t.Fatal(err)
	}

	expectedJSON := `{
	"items": [
		{
			"level": "Info",
			"locator": "ns/e2e-kubectl-3271 pod/without-label uid/e185b70c-ea3e-4600-850a-b2370a729a73",
			"message": "constructed/true reason/Created ",
			"from": "2022-03-07T18:41:46Z",
			"to": "2022-03-07T18:41:46Z"
		},
		{
			"level": "Info",
			"locator": "ns/e2e-kubectl-3271 pod/without-label uid/e185b70c-ea3e-4600-850a-b2370a729a73",
			"message": "constructed/true reason/Scheduled node/ip-10-0-141-9.us-west-2.compute.internal",
			"from": "2022-03-07T18:41:46Z",
			"to": "2022-03-07T18:41:54Z"
		},
		{
			"level": "Info",
			"locator": "ns/e2e-kubectl-3271 pod/without-label uid/e185b70c-ea3e-4600-850a-b2370a729a73 container/without-label",
			"message": "constructed/true reason/ContainerWait missed real \"ContainerWait\"",
			"from": "2022-03-07T18:41:46Z",
			"to": "2022-03-07T18:41:52Z"
		},
		{
			"level": "Info",
			"locator": "ns/e2e-kubectl-3271 pod/without-label uid/e185b70c-ea3e-4600-850a-b2370a729a73 container/without-label",
			"message": "constructed/true reason/NotReady missed real \"NotReady\"",
			"from": "2022-03-07T18:41:52Z",
			"to": "2022-03-07T18:41:52Z"
		},
		{
			"level": "Info",
			"locator": "ns/e2e-kubectl-3271 pod/without-label uid/e185b70c-ea3e-4600-850a-b2370a729a73 container/without-label",
			"message": "constructed/true reason/ContainerStart cause/ duration/6.00s",
			"from": "2022-03-07T18:41:52Z",
			"to": "2022-03-07T18:41:54Z"
		},
		{
			"level": "Info",
			"locator": "ns/e2e-kubectl-3271 pod/without-label uid/e185b70c-ea3e-4600-850a-b2370a729a73 container/without-label",
			"message": "constructed/true reason/Ready ",
			"from": "2022-03-07T18:41:52Z",
			"to": "2022-03-07T18:41:54Z"
		}
	]
}`

	expectedJSON = strings.ReplaceAll(expectedJSON, "\t", "    ")

	resultJSON := string(resultBytes)
	if expectedJSON != resultJSON {
		t.Fatal(cmp.Diff(expectedJSON, resultJSON))
	}
}

//go:embed pod_test_02_trailing_ready.json
var trailingReadyPodLifecyleJSON []byte

func TestIntervalCreation_TrailingReady(t *testing.T) {
	inputIntervals, err := monitorserialization.EventsFromJSON(trailingReadyPodLifecyleJSON)
	if err != nil {
		t.Fatal(err)
	}
	startTime, err := time.Parse(time.RFC3339, "2022-03-07T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	endTime, err := time.Parse(time.RFC3339, "2022-03-10T23:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	result := CreatePodIntervalsFromInstants(inputIntervals, monitorapi.ResourcesMap{}, startTime, endTime)

	resultBytes, err := monitorserialization.EventsToJSON(result)
	if err != nil {
		t.Fatal(err)
	}

	expectedJSON := `{
	"items": [
		{
			"level": "Info",
			"locator": "ns/openshift-marketplace pod/community-operators-sp6lm uid/efb1885a-1fe1-4f5b-ad41-044e55f806a9",
			"message": "constructed/true reason/Created ",
			"from": "2022-03-07T22:47:04Z",
			"to": "2022-03-07T22:47:04Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-marketplace pod/community-operators-sp6lm uid/efb1885a-1fe1-4f5b-ad41-044e55f806a9",
			"message": "constructed/true reason/Scheduled node/ip-10-0-154-151.ec2.internal",
			"from": "2022-03-07T22:47:04Z",
			"to": "2022-03-07T22:47:15Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-marketplace pod/community-operators-sp6lm uid/efb1885a-1fe1-4f5b-ad41-044e55f806a9 container/registry-server",
			"message": "constructed/true reason/ContainerWait missed real \"ContainerWait\"",
			"from": "2022-03-07T22:47:04Z",
			"to": "2022-03-07T22:47:07Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-marketplace pod/community-operators-sp6lm uid/efb1885a-1fe1-4f5b-ad41-044e55f806a9 container/registry-server",
			"message": "constructed/true reason/NotReady missed real \"NotReady\"",
			"from": "2022-03-07T22:47:07Z",
			"to": "2022-03-07T22:47:14Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-marketplace pod/community-operators-sp6lm uid/efb1885a-1fe1-4f5b-ad41-044e55f806a9 container/registry-server",
			"message": "constructed/true reason/ContainerStart cause/ duration/3.00s",
			"from": "2022-03-07T22:47:07Z",
			"to": "2022-03-07T22:47:15Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-marketplace pod/community-operators-sp6lm uid/efb1885a-1fe1-4f5b-ad41-044e55f806a9 container/registry-server",
			"message": "constructed/true reason/Ready ",
			"from": "2022-03-07T22:47:14Z",
			"to": "2022-03-07T22:47:15Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-marketplace pod/community-operators-sp6lm uid/efb1885a-1fe1-4f5b-ad41-044e55f806a9 container/registry-server",
			"message": "constructed/true reason/NotReady ",
			"from": "2022-03-07T22:47:15Z",
			"to": "2022-03-07T22:47:15Z"
		}
	]
}`

	expectedJSON = strings.ReplaceAll(expectedJSON, "\t", "    ")

	resultJSON := string(resultBytes)
	if expectedJSON != resultJSON {
		t.Fatal(resultJSON)
	}
}

//go:embed pod_test_03_trailing_ready_2.json
var trailingReady2PodLifecyleJSON []byte

func TestIntervalCreation_TrailingReady2(t *testing.T) {
	inputIntervals, err := monitorserialization.EventsFromJSON(trailingReady2PodLifecyleJSON)
	if err != nil {
		t.Fatal(err)
	}
	startTime, err := time.Parse(time.RFC3339, "2022-03-07T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	endTime, err := time.Parse(time.RFC3339, "2022-03-10T23:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	result := CreatePodIntervalsFromInstants(inputIntervals, monitorapi.ResourcesMap{}, startTime, endTime)

	resultBytes, err := monitorserialization.EventsToJSON(result)
	if err != nil {
		t.Fatal(err)
	}

	expectedJSON := `{
	"items": [
		{
			"level": "Info",
			"locator": "ns/openshift-machine-config-operator pod/machine-config-operator-7d5bf78cff-bbbwb uid/27e57fd1-c8f9-4528-8a04-0054dad5d38f",
			"message": "constructed/true reason/Created ",
			"from": "2022-03-08T23:17:18Z",
			"to": "2022-03-08T23:17:18Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-machine-config-operator pod/machine-config-operator-7d5bf78cff-bbbwb uid/27e57fd1-c8f9-4528-8a04-0054dad5d38f",
			"message": "constructed/true reason/Scheduled node/ip-10-0-231-18.us-east-2.compute.internal",
			"from": "2022-03-08T23:17:18Z",
			"to": "2022-03-10T23:00:00Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-machine-config-operator pod/machine-config-operator-7d5bf78cff-bbbwb uid/27e57fd1-c8f9-4528-8a04-0054dad5d38f container/machine-config-operator",
			"message": "constructed/true reason/NotReady missed real \"NotReady\"",
			"from": "2022-03-08T23:17:18Z",
			"to": "2022-03-08T23:17:18Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-machine-config-operator pod/machine-config-operator-7d5bf78cff-bbbwb uid/27e57fd1-c8f9-4528-8a04-0054dad5d38f container/machine-config-operator",
			"message": "constructed/true reason/Ready ",
			"from": "2022-03-08T23:17:18Z",
			"to": "2022-03-10T23:00:00Z"
		}
	]
}`

	expectedJSON = strings.ReplaceAll(expectedJSON, "\t", "    ")

	resultJSON := string(resultBytes)
	if expectedJSON != resultJSON {
		t.Fatal(resultJSON)
	}
}

//go:embed pod_test_04_run_once_done.json
var runOnceDone []byte

func TestIntervalCreation_RunOnceDone(t *testing.T) {
	inputIntervals, err := monitorserialization.EventsFromJSON(runOnceDone)
	if err != nil {
		t.Fatal(err)
	}
	startTime, err := time.Parse(time.RFC3339, "2022-03-07T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	endTime, err := time.Parse(time.RFC3339, "2022-03-15T23:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	podTerminateTime, err := time.Parse(time.RFC3339, "2022-03-14T15:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	result := CreatePodIntervalsFromInstants(inputIntervals, monitorapi.ResourcesMap{
		"pods": monitorapi.InstanceMap{
			"openshift-kube-scheduler/installer-3-ip-10-0-136-132.us-west-2.compute.internal": &corev1.Pod{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									FinishedAt: metav1.Time{
										Time: podTerminateTime,
									},
								},
							},
						},
					},
				},
			},
		},
	}, startTime, endTime)

	resultBytes, err := monitorserialization.EventsToJSON(result)
	if err != nil {
		t.Fatal(err)
	}

	expectedJSON := `{
	"items": [
		{
			"level": "Info",
			"locator": "ns/openshift-kube-scheduler pod/installer-3-ip-10-0-136-132.us-west-2.compute.internal uid/b7d89367-600a-49a3-95e1-a3ef2c91ecb9",
			"message": "constructed/true reason/Created ",
			"from": "2022-03-10T22:46:20Z",
			"to": "2022-03-10T22:46:20Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-kube-scheduler pod/installer-3-ip-10-0-136-132.us-west-2.compute.internal uid/b7d89367-600a-49a3-95e1-a3ef2c91ecb9",
			"message": "constructed/true reason/Scheduled node/ip-10-0-136-132.us-west-2.compute.internal",
			"from": "2022-03-10T22:46:20Z",
			"to": "2022-03-14T15:00:00Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-kube-scheduler pod/installer-3-ip-10-0-136-132.us-west-2.compute.internal uid/b7d89367-600a-49a3-95e1-a3ef2c91ecb9 container/installer",
			"message": "constructed/true reason/NotReady ",
			"from": "2022-03-10T22:46:20Z",
			"to": "2022-03-14T15:00:00Z"
		}
	]
}`

	expectedJSON = strings.ReplaceAll(expectedJSON, "\t", "    ")

	resultJSON := string(resultBytes)
	if expectedJSON != resultJSON {
		t.Fatal(resultJSON)
	}
}

//go:embed pod_test_05_installer_pod.json
var installerPod []byte

func TestIntervalCreation_InstallPod(t *testing.T) {
	inputIntervals, err := monitorserialization.EventsFromJSON(installerPod)
	if err != nil {
		t.Fatal(err)
	}
	startTime, err := time.Parse(time.RFC3339, "2022-03-07T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	endTime, err := time.Parse(time.RFC3339, "2022-03-25T23:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	podTerminateTime, err := time.Parse(time.RFC3339, "2022-03-21T21:37:56Z")
	if err != nil {
		t.Fatal(err)
	}
	result := CreatePodIntervalsFromInstants(inputIntervals, monitorapi.ResourcesMap{
		"pods": monitorapi.InstanceMap{
			"openshift-etcd/installer-9-ci-op-97t906zm-db044-bwrrn-master-0": &corev1.Pod{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									FinishedAt: metav1.Time{
										Time: podTerminateTime,
									},
								},
							},
						},
					},
				},
			},
		},
	}, startTime, endTime)

	resultBytes, err := monitorserialization.EventsToJSON(result)
	if err != nil {
		t.Fatal(err)
	}

	expectedJSON := `{
	"items": [
		{
			"level": "Info",
			"locator": "ns/openshift-etcd pod/installer-9-ci-op-97t906zm-db044-bwrrn-master-0 uid/6fb10c53-7ed9-4f51-88db-f8a689050f21",
			"message": "constructed/true reason/Created ",
			"from": "2022-03-21T21:37:20Z",
			"to": "2022-03-21T21:37:20Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-etcd pod/installer-9-ci-op-97t906zm-db044-bwrrn-master-0 uid/6fb10c53-7ed9-4f51-88db-f8a689050f21",
			"message": "constructed/true reason/Scheduled node/ci-op-97t906zm-db044-bwrrn-master-0",
			"from": "2022-03-21T21:37:20Z",
			"to": "2022-03-21T21:37:56Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-etcd pod/installer-9-ci-op-97t906zm-db044-bwrrn-master-0 uid/6fb10c53-7ed9-4f51-88db-f8a689050f21 container/installer",
			"message": "constructed/true reason/ContainerWait missed real \"ContainerWait\"",
			"from": "2022-03-21T21:37:20Z",
			"to": "2022-03-21T21:37:23Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-etcd pod/installer-9-ci-op-97t906zm-db044-bwrrn-master-0 uid/6fb10c53-7ed9-4f51-88db-f8a689050f21 container/installer",
			"message": "constructed/true reason/NotReady ",
			"from": "2022-03-21T21:37:23Z",
			"to": "2022-03-21T21:37:23Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-etcd pod/installer-9-ci-op-97t906zm-db044-bwrrn-master-0 uid/6fb10c53-7ed9-4f51-88db-f8a689050f21 container/installer",
			"message": "constructed/true reason/ContainerStart cause/ duration/3.00s",
			"from": "2022-03-21T21:37:23Z",
			"to": "2022-03-21T21:37:56Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-etcd pod/installer-9-ci-op-97t906zm-db044-bwrrn-master-0 uid/6fb10c53-7ed9-4f51-88db-f8a689050f21 container/installer",
			"message": "constructed/true reason/Ready ",
			"from": "2022-03-21T21:37:23Z",
			"to": "2022-03-21T21:37:56Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-etcd pod/installer-9-ci-op-97t906zm-db044-bwrrn-master-0 uid/6fb10c53-7ed9-4f51-88db-f8a689050f21 container/installer",
			"message": "constructed/true reason/NotReady ",
			"from": "2022-03-21T21:37:56Z",
			"to": "2022-03-21T21:37:56Z"
		}
	]
}`

	expectedJSON = strings.ReplaceAll(expectedJSON, "\t", "    ")

	resultJSON := string(resultBytes)
	if expectedJSON != resultJSON {
		t.Fatal(resultJSON)
	}
}

//go:embed pod_test_06_end_before_begin.json
var podBeforeBegin []byte

func TestIntervalCreation_PodBeforeBegin(t *testing.T) {
	inputIntervals, err := monitorserialization.EventsFromJSON(podBeforeBegin)
	if err != nil {
		t.Fatal(err)
	}
	startTime, err := time.Parse(time.RFC3339, "2022-03-21T16:43:39Z")
	if err != nil {
		t.Fatal(err)
	}
	endTime, err := time.Parse(time.RFC3339, "2022-03-25T23:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	podTerminateTime, err := time.Parse(time.RFC3339, "2022-03-21T16:43:14Z")
	if err != nil {
		t.Fatal(err)
	}
	result := CreatePodIntervalsFromInstants(inputIntervals, monitorapi.ResourcesMap{
		"pods": monitorapi.InstanceMap{
			"openshift-kube-apiserver/revision-pruner-7-ip-10-0-214-214.us-west-1.compute.internal": &corev1.Pod{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									FinishedAt: metav1.Time{
										Time: podTerminateTime,
									},
								},
							},
						},
					},
				},
			},
		},
	}, startTime, endTime)

	resultBytes, err := monitorserialization.EventsToJSON(result)
	if err != nil {
		t.Fatal(err)
	}

	expectedJSON := `{
	"items": [
		{
			"level": "Info",
			"locator": "ns/openshift-kube-apiserver pod/revision-pruner-7-ip-10-0-214-214.us-west-1.compute.internal uid/6a81964d-169c-47e0-a986-551429370ae9",
			"message": "constructed/true reason/Created ",
			"from": "2022-03-21T16:43:14Z",
			"to": "2022-03-21T16:43:14Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-kube-apiserver pod/revision-pruner-7-ip-10-0-214-214.us-west-1.compute.internal uid/6a81964d-169c-47e0-a986-551429370ae9",
			"message": "constructed/true reason/Scheduled node/ip-10-0-214-214.us-west-1.compute.internal",
			"from": "2022-03-21T16:43:14Z",
			"to": "2022-03-21T16:43:14Z"
		},
		{
			"level": "Info",
			"locator": "ns/openshift-kube-apiserver pod/revision-pruner-7-ip-10-0-214-214.us-west-1.compute.internal uid/6a81964d-169c-47e0-a986-551429370ae9 container/pruner",
			"message": "constructed/true reason/NotReady ",
			"from": "2022-03-21T16:43:14Z",
			"to": "2022-03-21T16:43:14Z"
		}
	]
}`

	expectedJSON = strings.ReplaceAll(expectedJSON, "\t", "    ")

	resultJSON := string(resultBytes)
	if expectedJSON != resultJSON {
		t.Fatal(resultJSON)
	}
}

type podIntervalTest struct {
	events      []byte
	results     string
	startTime   string
	endTime     string
	podData     [][]byte
	resourceMap monitorapi.ResourcesMap
}

func (p podIntervalTest) test(t *testing.T) {
	resourceMap := monitorapi.ResourcesMap{}
	if p.resourceMap != nil {
		resourceMap = p.resourceMap
	}
	for _, curr := range p.podData {
		pod := &corev1.Pod{}
		if err := json.Unmarshal(curr, pod); err != nil {
			t.Fatal(err)
		}
		podMap, ok := resourceMap["pods"]
		if !ok {
			podMap = monitorapi.InstanceMap{}
		}
		podMap[pod.Namespace+"/"+pod.Name] = pod
		resourceMap["pods"] = podMap
	}

	inputIntervals, err := monitorserialization.EventsFromJSON(p.events)
	if err != nil {
		t.Fatal(err)
	}
	startTime, err := time.Parse(time.RFC3339, p.startTime)
	if err != nil {
		t.Fatal(err)
	}
	endTime, err := time.Parse(time.RFC3339, p.endTime)
	if err != nil {
		t.Fatal(err)
	}
	result := CreatePodIntervalsFromInstants(inputIntervals, resourceMap, startTime, endTime)

	resultBytes, err := monitorserialization.EventsToJSON(result)
	if err != nil {
		t.Fatal(err)
	}

	resultJSON := string(resultBytes)
	if p.results != resultJSON {
		t.Log(p.results)
		t.Fatal(resultJSON)
	}
}

var (
	//go:embed pod_test_07_long_not_ready.json
	longNotReadyEvents []byte
	//go:embed pod_test_07_long_not_ready--expected.json
	longNotReadyResults string
	//go:embed pod_test_07_long_not_ready--pod.json
	longNotReadyPod []byte

	//go:embed pod_test_08_container_life.json
	podScheduledEvents []byte
	//go:embed pod_test_08_container_life--expected.json
	podScheduledResults string
	//go:embed pod_test_08_container_life--pod.json
	podScheduledPod []byte
)

func TestPodIntervalCreation(t *testing.T) {
	testCases := []struct {
		name string
		test podIntervalTest
	}{
		{
			name: "longNotReadyEvents",
			test: podIntervalTest{
				events:    longNotReadyEvents,
				results:   longNotReadyResults,
				startTime: "2022-03-22T19:00:00Z",
				endTime:   "2022-03-22T19:11:18Z",
				podData:   [][]byte{longNotReadyPod},
			},
		},
		{
			name: "podScheduled",
			test: podIntervalTest{
				events:    podScheduledEvents,
				results:   podScheduledResults,
				startTime: "2022-03-22T22:02:53Z",
				endTime:   "2022-03-22T22:29:35Z",
				podData:   [][]byte{podScheduledPod},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			test.test.test(t)
		})
	}
}
