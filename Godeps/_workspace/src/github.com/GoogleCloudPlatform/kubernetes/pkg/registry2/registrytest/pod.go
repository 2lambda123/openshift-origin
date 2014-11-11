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

package registrytest

import (
	"sync"

	api "github.com/GoogleCloudPlatform/kubernetes/pkg/api2"
	labels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels2"
	watch "github.com/GoogleCloudPlatform/kubernetes/pkg/watch2"
)

type PodRegistry struct {
	Err  error
	Pod  *api.Pod
	Pods *api.PodList
	sync.Mutex

	mux *watch.Mux
}

func NewPodRegistry(pods *api.PodList) *PodRegistry {
	return &PodRegistry{
		Pods: pods,
		mux:  watch.NewMux(0),
	}
}

func (r *PodRegistry) ListPodsPredicate(ctx api.Context, filter func(*api.Pod) bool) (*api.PodList, error) {
	r.Lock()
	defer r.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	var filtered []api.Pod
	for _, pod := range r.Pods.Items {
		if filter(&pod) {
			filtered = append(filtered, pod)
		}
	}
	pods := *r.Pods
	pods.Items = filtered
	return &pods, nil
}

func (r *PodRegistry) ListPods(ctx api.Context, selector labels.Selector) (*api.PodList, error) {
	return r.ListPodsPredicate(ctx, func(pod *api.Pod) bool {
		return selector.Matches(labels.Set(pod.Labels))
	})
}

func (r *PodRegistry) WatchPods(ctx api.Context, resourceVersion string, filter func(*api.Pod) bool) (watch.Interface, error) {
	// TODO: wire filter down into the mux; it needs access to current and previous state :(
	return r.mux.Watch(), nil
}

func (r *PodRegistry) GetPod(ctx api.Context, podId string) (*api.Pod, error) {
	r.Lock()
	defer r.Unlock()
	return r.Pod, r.Err
}

func (r *PodRegistry) CreatePod(ctx api.Context, pod *api.Pod) error {
	r.Lock()
	defer r.Unlock()
	r.Pod = pod
	r.mux.Action(watch.Added, pod)
	return r.Err
}

func (r *PodRegistry) UpdatePod(ctx api.Context, pod *api.Pod) error {
	r.Lock()
	defer r.Unlock()
	r.Pod = pod
	r.mux.Action(watch.Modified, pod)
	return r.Err
}

func (r *PodRegistry) DeletePod(ctx api.Context, podId string) error {
	r.Lock()
	defer r.Unlock()
	r.mux.Action(watch.Deleted, r.Pod)
	return r.Err
}
