/*
Copyright 2015 The Kubernetes Authors.

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

package exec

import (
	"testing"

	"k8s.io/apiserver/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
	kubelet "k8s.io/kubernetes/pkg/kubelet/types"
)

func TestDenyStaticPodExec(t *testing.T) {
	staticPod := validPod("static")
	staticPod.Annotations = map[string]string{
		kubelet.ConfigMirrorAnnotationKey: "present",
	}

	normalPod := validPod("hostPID")
	normalPod.Spec.SecurityContext = &api.PodSecurityContext{}
	normalPod.Spec.SecurityContext.HostPID = true

	testCases := map[string]struct {
		pod          *api.Pod
		shouldAccept bool
	}{
		"static": {
			shouldAccept: false,
			pod:          staticPod,
		},
		"normal": {
			shouldAccept: true,
			pod:          normalPod,
		},
	}

	// use the same code as NewDenyStaticPodExec, using the direct object though to allow testAdmission to
	// inject the client
	handler := &denyStaticPodExec{
		Handler: admission.NewHandler(admission.Connect),
	}
	for _, tc := range testCases {
		testAdmission(t, tc.pod, handler, handler, tc.shouldAccept)
	}

}
