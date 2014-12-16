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

package kubectl

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/testapi"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

func objBody(obj runtime.Object) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(testapi.Codec(), obj))))
}

func TestRESTHelperDelete(t *testing.T) {
	tests := []struct {
		Err     bool
		Req     func(*http.Request) bool
		Resp    *http.Response
		HttpErr error
	}{
		{
			HttpErr: errors.New("failure"),
			Err:     true,
		},
		{
			Resp: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       objBody(&api.Status{Status: api.StatusFailure}),
			},
			Err: true,
		},
		{
			Resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       objBody(&api.Status{Status: api.StatusSuccess}),
			},
			Req: func(req *http.Request) bool {
				if req.Method != "DELETE" {
					t.Errorf("unexpected method: %#v", req)
					return false
				}
				if !strings.HasSuffix(req.URL.Path, "/foo") {
					t.Errorf("url doesn't contain name: %#v", req)
					return false
				}
				if req.URL.Query().Get("namespace") != "bar" {
					t.Errorf("url doesn't contain namespace: %#v", req)
					return false
				}
				return true
			},
		},
	}
	for _, test := range tests {
		client := &client.FakeRESTClient{
			Codec: testapi.Codec(),
			Resp:  test.Resp,
			Err:   test.HttpErr,
		}
		modifier := &RESTHelper{
			RESTClient: client,
		}
		err := modifier.Delete("bar", "foo", labels.Everything())
		if (err != nil) != test.Err {
			t.Errorf("unexpected error: %t %v", test.Err, err)
		}
		if err != nil {
			continue
		}
		if test.Req != nil && !test.Req(client.Req) {
			t.Errorf("unexpected request: %#v", client.Req)
		}
	}
}

func TestRESTHelperCreate(t *testing.T) {
	expectPost := func(req *http.Request) bool {
		if req.Method != "POST" {
			t.Errorf("unexpected method: %#v", req)
			return false
		}
		if req.URL.Query().Get("namespace") != "bar" {
			t.Errorf("url doesn't contain namespace: %#v", req)
			return false
		}
		return true
	}

	tests := []struct {
		Resp     *http.Response
		RespFunc client.HttpClientFunc
		HttpErr  error
		Modify   bool
		Object   runtime.Object

		ExpectObject runtime.Object
		Err          bool
		Req          func(*http.Request) bool
	}{
		{
			HttpErr: errors.New("failure"),
			Err:     true,
		},
		{
			Resp: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       objBody(&api.Status{Status: api.StatusFailure}),
			},
			Err: true,
		},
		{
			Resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       objBody(&api.Status{Status: api.StatusSuccess}),
			},
			Object:       &api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo"}},
			ExpectObject: &api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo"}},
			Req:          expectPost,
		},
		{
			Modify:       false,
			Object:       &api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo", ResourceVersion: "10"}},
			ExpectObject: &api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo", ResourceVersion: "10"}},
			Resp:         &http.Response{StatusCode: http.StatusOK, Body: objBody(&api.Status{Status: api.StatusSuccess})},
			Req:          expectPost,
		},
		{
			Modify:       true,
			Object:       &api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo", ResourceVersion: "10"}},
			ExpectObject: &api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo"}},
			Resp:         &http.Response{StatusCode: http.StatusOK, Body: objBody(&api.Status{Status: api.StatusSuccess})},
			Req:          expectPost,
		},
	}
	for i, test := range tests {
		client := &client.FakeRESTClient{
			Codec: testapi.Codec(),
			Resp:  test.Resp,
			Err:   test.HttpErr,
		}
		if test.RespFunc != nil {
			client.Client = test.RespFunc
		}
		modifier := &RESTHelper{
			RESTClient: client,
			Codec:      testapi.Codec(),
			Versioner:  testapi.MetadataAccessor(),
		}
		data := []byte{}
		if test.Object != nil {
			data = []byte(runtime.EncodeOrDie(testapi.Codec(), test.Object))
		}
		err := modifier.Create("bar", test.Modify, data)
		if (err != nil) != test.Err {
			t.Errorf("%d: unexpected error: %t %v", i, test.Err, err)
		}
		if err != nil {
			continue
		}
		if test.Req != nil && !test.Req(client.Req) {
			t.Errorf("%d: unexpected request: %#v", i, client.Req)
		}
		body, err := ioutil.ReadAll(client.Req.Body)
		if err != nil {
			t.Fatalf("%d: unexpected error: %#v", i, err)
		}
		t.Logf("got body: %s", string(body))
		expect := []byte{}
		if test.ExpectObject != nil {
			expect = []byte(runtime.EncodeOrDie(testapi.Codec(), test.ExpectObject))
		}
		if !reflect.DeepEqual(expect, body) {
			t.Errorf("%d: unexpected body: %s", i, string(body))
		}

	}
}

func TestRESTHelperGet(t *testing.T) {
	tests := []struct {
		Err     bool
		Req     func(*http.Request) bool
		Resp    *http.Response
		HttpErr error
	}{
		{
			HttpErr: errors.New("failure"),
			Err:     true,
		},
		{
			Resp: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       objBody(&api.Status{Status: api.StatusFailure}),
			},
			Err: true,
		},
		{
			Resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       objBody(&api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo"}}),
			},
			Req: func(req *http.Request) bool {
				if req.Method != "GET" {
					t.Errorf("unexpected method: %#v", req)
					return false
				}
				if !strings.HasSuffix(req.URL.Path, "/foo") {
					t.Errorf("url doesn't contain name: %#v", req)
					return false
				}
				if req.URL.Query().Get("namespace") != "bar" {
					t.Errorf("url doesn't contain namespace: %#v", req)
					return false
				}
				return true
			},
		},
	}
	for _, test := range tests {
		client := &client.FakeRESTClient{
			Codec: testapi.Codec(),
			Resp:  test.Resp,
			Err:   test.HttpErr,
		}
		modifier := &RESTHelper{
			RESTClient: client,
		}
		obj, err := modifier.Get("bar", "foo", labels.Everything())
		if (err != nil) != test.Err {
			t.Errorf("unexpected error: %t %v", test.Err, err)
		}
		if err != nil {
			continue
		}
		if obj.(*api.Pod).Name != "foo" {
			t.Errorf("unexpected object: %#v", obj)
		}
		if test.Req != nil && !test.Req(client.Req) {
			t.Errorf("unexpected request: %#v", client.Req)
		}
	}
}

func TestRESTHelperUpdate(t *testing.T) {
	expectPut := func(req *http.Request) bool {
		if req.Method != "PUT" {
			t.Errorf("unexpected method: %#v", req)
			return false
		}
		if !strings.HasSuffix(req.URL.Path, "/foo") {
			t.Errorf("url doesn't contain name: %#v", req)
			return false
		}
		if req.URL.Query().Get("namespace") != "bar" {
			t.Errorf("url doesn't contain namespace: %#v", req)
			return false
		}
		return true
	}

	tests := []struct {
		Resp      *http.Response
		RespFunc  client.HttpClientFunc
		HttpErr   error
		Overwrite bool
		Object    runtime.Object

		ExpectObject runtime.Object
		Err          bool
		Req          func(*http.Request) bool
	}{
		{
			HttpErr: errors.New("failure"),
			Err:     true,
		},
		{
			Object: &api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo"}},
			Resp: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       objBody(&api.Status{Status: api.StatusFailure}),
			},
			Err: true,
		},
		{
			Object:       &api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo"}},
			ExpectObject: &api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo"}},
			Resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       objBody(&api.Status{Status: api.StatusSuccess}),
			},
			Req: expectPut,
		},
		{
			Object:       &api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo"}},
			ExpectObject: &api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo", ResourceVersion: "10"}},

			Overwrite: true,
			RespFunc: func(req *http.Request) (*http.Response, error) {
				if req.Method == "PUT" {
					return &http.Response{StatusCode: http.StatusOK, Body: objBody(&api.Status{Status: api.StatusSuccess})}, nil
				}
				return &http.Response{StatusCode: http.StatusOK, Body: objBody(&api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo", ResourceVersion: "10"}})}, nil
			},
			Req: expectPut,
		},
		{
			Object:       &api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo", ResourceVersion: "10"}},
			ExpectObject: &api.Pod{ObjectMeta: api.ObjectMeta{Name: "foo", ResourceVersion: "10"}},
			Resp:         &http.Response{StatusCode: http.StatusOK, Body: objBody(&api.Status{Status: api.StatusSuccess})},
			Req:          expectPut,
		},
	}
	for i, test := range tests {
		client := &client.FakeRESTClient{
			Codec: testapi.Codec(),
			Resp:  test.Resp,
			Err:   test.HttpErr,
		}
		if test.RespFunc != nil {
			client.Client = test.RespFunc
		}
		modifier := &RESTHelper{
			RESTClient: client,
			Codec:      testapi.Codec(),
			Versioner:  testapi.MetadataAccessor(),
		}
		data := []byte{}
		if test.Object != nil {
			data = []byte(runtime.EncodeOrDie(testapi.Codec(), test.Object))
		}
		err := modifier.Update("bar", "foo", test.Overwrite, data)
		if (err != nil) != test.Err {
			t.Errorf("%d: unexpected error: %t %v", i, test.Err, err)
		}
		if err != nil {
			continue
		}
		if test.Req != nil && !test.Req(client.Req) {
			t.Errorf("%d: unexpected request: %#v", i, client.Req)
		}
		body, err := ioutil.ReadAll(client.Req.Body)
		if err != nil {
			t.Fatalf("%d: unexpected error: %#v", i, err)
		}
		t.Logf("got body: %s", string(body))
		expect := []byte{}
		if test.ExpectObject != nil {
			expect = []byte(runtime.EncodeOrDie(testapi.Codec(), test.ExpectObject))
		}
		if !reflect.DeepEqual(expect, body) {
			t.Errorf("%d: unexpected body: %s", i, string(body))
		}
	}
}
