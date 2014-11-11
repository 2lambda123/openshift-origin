/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package record_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

type testEventRecorder struct {
	OnEvent func(e *api.Event) (*api.Event, error)
}

// CreateEvent records the event for testing.
func (t *testEventRecorder) Create(e *api.Event) (*api.Event, error) {
	if t.OnEvent != nil {
		return t.OnEvent(e)
	}
	return e, nil
}

func (t *testEventRecorder) clearOnEvent() {
	t.OnEvent = nil
}

func TestEventf(t *testing.T) {
	table := []struct {
		obj                       runtime.Object
		fieldPath, status, reason string
		messageFmt                string
		elements                  []interface{}
		expect                    *api.Event
		expectLog                 string
	}{
		{
			obj: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					SelfLink: "/api/v1beta1/pods/foo",
					Name:     "foo",
					UID:      "bar",
				},
			},
			fieldPath:  "desiredState.manifest.containers[2]",
			status:     "running",
			reason:     "started",
			messageFmt: "some verbose message: %v",
			elements:   []interface{}{1},
			expect: &api.Event{
				InvolvedObject: api.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					UID:        "bar",
					APIVersion: "v1beta1",
					FieldPath:  "desiredState.manifest.containers[2]",
				},
				Status:  "running",
				Reason:  "started",
				Message: "some verbose message: 1",
				Source:  "eventTest",
			},
			expectLog: `Event(api.ObjectReference{Kind:"Pod", Namespace:"", Name:"foo", UID:"bar", APIVersion:"v1beta1", ResourceVersion:"", FieldPath:"desiredState.manifest.containers[2]"}): status: 'running', reason: 'started' some verbose message: 1`,
		},
	}

	for _, item := range table {
		called := make(chan struct{})
		testEvents := testEventRecorder{
			OnEvent: func(event *api.Event) (*api.Event, error) {
				a := *event
				// Just check that the timestamp was set.
				if a.Timestamp.IsZero() {
					t.Errorf("timestamp wasn't set")
				}
				a.Timestamp = item.expect.Timestamp
				if e, a := item.expect, &a; !reflect.DeepEqual(e, a) {
					t.Errorf("diff: %s", util.ObjectDiff(e, a))
				}
				called <- struct{}{}
				return event, nil
			},
		}
		recorder := record.StartRecording(&testEvents, "eventTest")
		logger := record.StartLogging(t.Logf) // Prove that it is useful
		logger2 := record.StartLogging(func(formatter string, args ...interface{}) {
			if e, a := item.expectLog, fmt.Sprintf(formatter, args...); e != a {
				t.Errorf("Expected '%v', got '%v'", e, a)
			}
			called <- struct{}{}
		})

		record.Eventf(item.obj, item.fieldPath, item.status, item.reason, item.messageFmt, item.elements...)

		<-called
		<-called
		recorder.Stop()
		logger.Stop()
		logger2.Stop()
	}
}
