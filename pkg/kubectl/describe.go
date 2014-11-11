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
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/api2"
	client "github.com/openshift/origin/pkg/client2"
	"github.com/openshift/origin/pkg/labels"
)

type DescribeFunc func(namespace, name string) (string, error)

type PodDescriber struct {
	PodClient                   func(namespace string) (client.PodInterface, error)
	ReplicationControllerClient func(namespace string) (client.ReplicationControllerInterface, error)
}

func (d *PodDescriber) Describe(namespace, name string) (string, error) {
	rc, err := d.ReplicationControllerClient(namespace)
	if err != nil {
		return "", err
	}
	pc, err := d.PodClient(namespace)
	if err != nil {
		return "", err
	}

	pod, err := pc.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		fmt.Fprintf(out, "Name:\t%s\n", pod.Name)
		fmt.Fprintf(out, "Image(s):\t%s\n", makeImageList(pod.DesiredState.Manifest))
		fmt.Fprintf(out, "Host:\t%s\n", pod.CurrentState.Host+"/"+pod.CurrentState.HostIP)
		fmt.Fprintf(out, "Labels:\t%s\n", formatLabels(pod.Labels))
		fmt.Fprintf(out, "Status:\t%s\n", string(pod.CurrentState.Status))
		fmt.Fprintf(out, "Replication Controllers:\t%s\n", getReplicationControllersForLabels(rc, labels.Set(pod.Labels)))
		return nil
	})
}

type ReplicationControllerDescriber struct {
	ReplicationControllerClient func(namespace string) (client.ReplicationControllerInterface, error)
	PodClient                   func(namespace string) (client.PodInterface, error)
}

func (d *ReplicationControllerDescriber) Describe(namespace, name string) (string, error) {
	rc, err := d.ReplicationControllerClient(namespace)
	if err != nil {
		return "", err
	}
	pc, err := d.PodClient(namespace)
	if err != nil {
		return "", err
	}

	controller, err := rc.Get(name)
	if err != nil {
		return "", err
	}

	running, waiting, terminated, err := getPodStatusForReplicationController(pc, controller)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		fmt.Fprintf(out, "Name:\t%s\n", controller.Name)
		fmt.Fprintf(out, "Image(s):\t%s\n", makeImageList(controller.DesiredState.PodTemplate.DesiredState.Manifest))
		fmt.Fprintf(out, "Selector:\t%s\n", formatLabels(controller.DesiredState.ReplicaSelector))
		fmt.Fprintf(out, "Labels:\t%s\n", formatLabels(controller.Labels))
		fmt.Fprintf(out, "Replicas:\t%d current / %d desired\n", controller.CurrentState.Replicas, controller.DesiredState.Replicas)
		fmt.Fprintf(out, "Pods Status:\t%d Running / %d Waiting / %d Terminated\n", running, waiting, terminated)
		return nil
	})
}

type ServiceDescriber struct {
	ServiceClient func(namespace string) (client.ServiceInterface, error)
}

func (d *ServiceDescriber) Describe(namespace, name string) (string, error) {
	c, err := d.ServiceClient(namespace)
	if err != nil {
		return "", err
	}

	service, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		fmt.Fprintf(out, "Name:\t%s\n", service.Name)
		fmt.Fprintf(out, "Labels:\t%s\n", formatLabels(service.Labels))
		fmt.Fprintf(out, "Selector:\t%s\n", formatLabels(service.Selector))
		fmt.Fprintf(out, "Port:\t%d\n", service.Port)
		return nil
	})
}

type MinionDescriber struct {
	MinionClient func() (client.MinionInterface, error)
}

func (d *MinionDescriber) Describe(namespace, name string) (string, error) {
	mc, err := d.MinionClient()
	if err != nil {
		return "", err
	}
	minion, err := mc.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		fmt.Fprintf(out, "Name:\t%s\n", minion.Name)
		return nil
	})
}

// Get all replication controllers whose selectors would match a given set of
// labels.
// TODO Move this to pkg/client and ideally implement it server-side (instead
// of getting all RC's and searching through them manually).
func getReplicationControllersForLabels(c client.ReplicationControllerInterface, labelsToMatch labels.Labels) string {
	// Get all replication controllers.
	// TODO this needs a namespace scope as argument
	rcs, err := c.List(labels.Everything())
	if err != nil {
		glog.Fatalf("Error getting replication controllers: %v\n", err)
	}

	// Find the ones that match labelsToMatch.
	var matchingRCs []api.ReplicationController
	for _, controller := range rcs.Items {
		selector := labels.SelectorFromSet(controller.DesiredState.ReplicaSelector)
		if selector.Matches(labelsToMatch) {
			matchingRCs = append(matchingRCs, controller)
		}
	}

	// Format the matching RC's into strings.
	var rcStrings []string
	for _, controller := range matchingRCs {
		rcStrings = append(rcStrings, fmt.Sprintf("%s (%d/%d replicas created)", controller.Name, controller.CurrentState.Replicas, controller.DesiredState.Replicas))
	}

	list := strings.Join(rcStrings, ", ")
	if list == "" {
		return "<none>"
	}
	return list
}

func getPodStatusForReplicationController(c client.PodInterface, controller *api.ReplicationController) (running, waiting, terminated int, err error) {
	rcPods, err := c.List(labels.SelectorFromSet(controller.DesiredState.ReplicaSelector))
	if err != nil {
		return
	}
	for _, pod := range rcPods.Items {
		if pod.CurrentState.Status == api.PodRunning {
			running++
		} else if pod.CurrentState.Status == api.PodWaiting {
			waiting++
		} else if pod.CurrentState.Status == api.PodTerminated {
			terminated++
		}
	}
	return
}
