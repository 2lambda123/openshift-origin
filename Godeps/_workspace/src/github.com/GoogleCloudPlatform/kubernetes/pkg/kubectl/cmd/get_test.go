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

package cmd_test

import (
	"bytes"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
)

// Verifies that schemas that are not in the master tree of Kubernetes can be retrieved via Get.
func TestGetUnknownSchemaObject(t *testing.T) {
	f, tf, codec := NewTestFactory()
	tf.Printer = &testPrinter{}
	tf.Client = &client.FakeRESTClient{
		Codec: codec,
		Resp:  &http.Response{StatusCode: 200, Body: objBody(codec, &internalType{Name: "foo"})},
	}
	buf := bytes.NewBuffer([]byte{})

	cmd := f.NewCmdGet(buf)
	cmd.Flags().String("namespace", "test", "")
	cmd.Run(cmd, []string{"type", "foo"})

	expected := &internalType{Name: "foo"}
	actual := tf.Printer.(*testPrinter).Obj
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("unexpected object: %#v", actual)
	}
	if buf.String() != fmt.Sprintf("%#v", expected) {
		t.Errorf("unexpected output: %s", buf.String())
	}
}
