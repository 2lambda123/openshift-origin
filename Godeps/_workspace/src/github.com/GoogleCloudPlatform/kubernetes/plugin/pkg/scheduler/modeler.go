/*
Copyright 2015 Google Inc. All rights reserved.

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

package scheduler

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	algorithm "github.com/GoogleCloudPlatform/kubernetes/pkg/scheduler"

	"github.com/golang/glog"
)

var (
	_ = SystemModeler(&FakeModeler{})
	_ = SystemModeler(&SimpleModeler{})
)

// ExtendedPodLister: SimpleModeler needs to be able to check for a pod's
// existance in addition to listing the pods.
type ExtendedPodLister interface {
	algorithm.PodLister
	Exists(pod *api.Pod) (bool, error)
}

// actionLocker implements lockedAction (so the fake and SimpleModeler can both
// use it)
type actionLocker struct {
	sync.Mutex
}

// LockedAction serializes calls of whatever is passed as 'do'.
func (a *actionLocker) LockedAction(do func()) {
	a.Lock()
	defer a.Unlock()
	do()
}

// FakeModeler implements the SystemModeler interface.
type FakeModeler struct {
	AssumePodFunc func(pod *api.Pod)
	ForgetPodFunc func(pod *api.Pod)
	actionLocker
}

// AssumePod calls the function variable if it is not nil.
func (f *FakeModeler) AssumePod(pod *api.Pod) {
	if f.AssumePodFunc != nil {
		f.AssumePodFunc(pod)
	}
}

// ForgetPod calls the function variable if it is not nil.
func (f *FakeModeler) ForgetPod(pod *api.Pod) {
	if f.ForgetPodFunc != nil {
		f.ForgetPodFunc(pod)
	}
}

// SimpleModeler implements the SystemModeler interface with a timed pod cache.
type SimpleModeler struct {
	queuedPods    ExtendedPodLister
	scheduledPods ExtendedPodLister

	// assumedPods holds the pods that we think we've scheduled, but that
	// haven't yet shown up in the scheduledPods variable.
	// TODO: periodically clear this.
	assumedPods *cache.StoreToPodLister

	actionLocker
}

// NewSimpleModeler returns a new SimpleModeler.
//   queuedPods: a PodLister that will return pods that have not been scheduled yet.
//   scheduledPods: a PodLister that will return pods that we know for sure have been scheduled.
func NewSimpleModeler(queuedPods, scheduledPods ExtendedPodLister) *SimpleModeler {
	return &SimpleModeler{
		queuedPods:    queuedPods,
		scheduledPods: scheduledPods,
		assumedPods: &cache.StoreToPodLister{
			cache.NewTTLStore(cache.MetaNamespaceKeyFunc, 30*time.Second),
		},
	}
}

func (s *SimpleModeler) AssumePod(pod *api.Pod) {
	s.assumedPods.Add(pod)
}

func (s *SimpleModeler) ForgetPod(pod *api.Pod) {
	s.assumedPods.Delete(pod)
}

// Extract names for readable logging.
func podNames(pods []api.Pod) []string {
	out := make([]string, len(pods))
	for i := range pods {
		out[i] = fmt.Sprintf("'%v/%v (%v)'", pods[i].Namespace, pods[i].Name, pods[i].UID)
	}
	return out
}

func (s *SimpleModeler) listPods(selector labels.Selector) (pods []api.Pod, err error) {
	assumed, err := s.assumedPods.List(selector)
	if err != nil {
		return nil, err
	}
	// Since the assumed list will be short, just check every one.
	// Goal here is to stop making assumptions about a pod once it shows
	// up in one of these other lists.
	for _, pod := range assumed {
		qExist, err := s.queuedPods.Exists(&pod)
		if err != nil {
			return nil, err
		}
		if qExist {
			s.assumedPods.Store.Delete(&pod)
			continue
		}
		sExist, err := s.scheduledPods.Exists(&pod)
		if err != nil {
			return nil, err
		}
		if sExist {
			s.assumedPods.Store.Delete(&pod)
			continue
		}
	}

	scheduled, err := s.scheduledPods.List(selector)
	if err != nil {
		return nil, err
	}
	// Listing purges the ttl cache and re-gets, in case we deleted any entries.
	assumed, err = s.assumedPods.List(selector)
	if err != nil {
		return nil, err
	}
	if len(assumed) == 0 {
		return scheduled, nil
	}
	glog.V(2).Infof(
		"listing pods: [%v] assumed to exist in addition to %v known pods.",
		strings.Join(podNames(assumed), ","),
		len(scheduled),
	)
	return append(scheduled, assumed...), nil
}

// PodLister returns a PodLister that will list pods that we think we have scheduled in
// addition to pods that we know have been scheduled.
func (s *SimpleModeler) PodLister() algorithm.PodLister {
	return simpleModelerPods{s}
}

// simpleModelerPods is an adaptor so that SimpleModeler can be a PodLister.
type simpleModelerPods struct {
	simpleModeler *SimpleModeler
}

// List returns pods known and assumed to exist.
func (s simpleModelerPods) List(selector labels.Selector) (pods []api.Pod, err error) {
	return s.simpleModeler.listPods(selector)
}
