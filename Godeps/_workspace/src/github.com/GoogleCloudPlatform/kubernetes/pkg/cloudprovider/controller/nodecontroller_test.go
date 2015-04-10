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

package controller

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	apierrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	fake_cloud "github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider/fake"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/probe"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
)

const (
	testNodeMonitorGracePeriod = 40 * time.Second
	testNodeStartupGracePeriod = 60 * time.Second
	testNodeMonitorPeriod      = 5 * time.Second
)

// FakeNodeHandler is a fake implementation of NodesInterface and NodeInterface. It
// allows test cases to have fine-grained control over mock behaviors. We also need
// PodsInterface and PodInterface to test list & delet pods, which is implemented in
// the embeded client.Fake field.
type FakeNodeHandler struct {
	*testclient.Fake

	// Input: Hooks determine if request is valid or not
	CreateHook func(*FakeNodeHandler, *api.Node) bool
	Existing   []*api.Node

	// Output
	CreatedNodes []*api.Node
	DeletedNodes []*api.Node
	UpdatedNodes []*api.Node
	RequestCount int
}

func (c *FakeNodeHandler) Nodes() client.NodeInterface {
	return c
}

func (m *FakeNodeHandler) Create(node *api.Node) (*api.Node, error) {
	defer func() { m.RequestCount++ }()
	for _, n := range m.Existing {
		if n.Name == node.Name {
			return nil, apierrors.NewAlreadyExists("Minion", node.Name)
		}
	}
	if m.CreateHook == nil || m.CreateHook(m, node) {
		nodeCopy := *node
		m.CreatedNodes = append(m.CreatedNodes, &nodeCopy)
		return node, nil
	} else {
		return nil, errors.New("Create error.")
	}
}

func (m *FakeNodeHandler) Get(name string) (*api.Node, error) {
	return nil, nil
}

func (m *FakeNodeHandler) List(selector labels.Selector) (*api.NodeList, error) {
	defer func() { m.RequestCount++ }()
	var nodes []*api.Node
	for i := 0; i < len(m.UpdatedNodes); i++ {
		if !contains(m.UpdatedNodes[i], m.DeletedNodes) {
			nodes = append(nodes, m.UpdatedNodes[i])
		}
	}
	for i := 0; i < len(m.Existing); i++ {
		if !contains(m.Existing[i], m.DeletedNodes) && !contains(m.Existing[i], nodes) {
			nodes = append(nodes, m.Existing[i])
		}
	}
	for i := 0; i < len(m.CreatedNodes); i++ {
		if !contains(m.Existing[i], m.DeletedNodes) && !contains(m.CreatedNodes[i], nodes) {
			nodes = append(nodes, m.CreatedNodes[i])
		}
	}
	nodeList := &api.NodeList{}
	for _, node := range nodes {
		nodeList.Items = append(nodeList.Items, *node)
	}
	return nodeList, nil
}

func (m *FakeNodeHandler) Delete(id string) error {
	m.DeletedNodes = append(m.DeletedNodes, newNode(id))
	m.RequestCount++
	return nil
}

func (m *FakeNodeHandler) Update(node *api.Node) (*api.Node, error) {
	nodeCopy := *node
	m.UpdatedNodes = append(m.UpdatedNodes, &nodeCopy)
	m.RequestCount++
	return node, nil
}

func (m *FakeNodeHandler) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return nil, nil
}

// FakeKubeletClient is a fake implementation of KubeletClient.
type FakeKubeletClient struct {
	Status probe.Result
	Err    error
}

func (c *FakeKubeletClient) GetPodStatus(host, podNamespace, podID string) (api.PodStatusResult, error) {
	return api.PodStatusResult{}, errors.New("Not Implemented")
}

func (c *FakeKubeletClient) GetNodeInfo(host string) (api.NodeInfo, error) {
	return api.NodeInfo{}, errors.New("Not Implemented")
}

func (c *FakeKubeletClient) GetConnectionInfo(host string) (string, uint, http.RoundTripper, error) {
	return "", 0, nil, errors.New("Not Implemented")
}

func (c *FakeKubeletClient) HealthCheck(host string) (probe.Result, error) {
	return c.Status, c.Err
}

func TestRegisterNodes(t *testing.T) {
	table := []struct {
		fakeNodeHandler      *FakeNodeHandler
		machines             []string
		retryCount           int
		expectedRequestCount int
		expectedCreateCount  int
		expectedFail         bool
	}{
		{
			// Register two nodes normally.
			machines: []string{"node0", "node1"},
			fakeNodeHandler: &FakeNodeHandler{
				CreateHook: func(fake *FakeNodeHandler, node *api.Node) bool { return true },
			},
			retryCount:           1,
			expectedRequestCount: 2,
			expectedCreateCount:  2,
			expectedFail:         false,
		},
		{
			// Canonicalize node names.
			machines: []string{"NODE0", "node1"},
			fakeNodeHandler: &FakeNodeHandler{
				CreateHook: func(fake *FakeNodeHandler, node *api.Node) bool {
					if node.Name == "NODE0" {
						return false
					}
					return true
				},
			},
			retryCount:           1,
			expectedRequestCount: 2,
			expectedCreateCount:  2,
			expectedFail:         false,
		},
		{
			// No machine to register.
			machines: []string{},
			fakeNodeHandler: &FakeNodeHandler{
				CreateHook: func(fake *FakeNodeHandler, node *api.Node) bool { return true },
			},
			retryCount:           1,
			expectedRequestCount: 0,
			expectedCreateCount:  0,
			expectedFail:         false,
		},
		{
			// Fail the first two requests.
			machines: []string{"node0", "node1"},
			fakeNodeHandler: &FakeNodeHandler{
				CreateHook: func(fake *FakeNodeHandler, node *api.Node) bool {
					if fake.RequestCount == 0 || fake.RequestCount == 1 {
						return false
					}
					return true
				},
			},
			retryCount:           10,
			expectedRequestCount: 4,
			expectedCreateCount:  2,
			expectedFail:         false,
		},
		{
			// One node already exists
			machines: []string{"node0", "node1"},
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{
					{
						ObjectMeta: api.ObjectMeta{
							Name: "node1",
						},
					},
				},
			},
			retryCount:           10,
			expectedRequestCount: 2,
			expectedCreateCount:  1,
			expectedFail:         false,
		},
		{
			// The first node always fails.
			machines: []string{"node0", "node1"},
			fakeNodeHandler: &FakeNodeHandler{
				CreateHook: func(fake *FakeNodeHandler, node *api.Node) bool {
					if node.Name == "node0" {
						return false
					}
					return true
				},
			},
			retryCount:           2,
			expectedRequestCount: 3, // 2 for node0, 1 for node1
			expectedCreateCount:  1,
			expectedFail:         true,
		},
	}

	for _, item := range table {
		nodes := api.NodeList{}
		for _, machine := range item.machines {
			nodes.Items = append(nodes.Items, *newNode(machine))
		}
		nodeController := NewNodeController(nil, "", item.machines, &api.NodeResources{}, item.fakeNodeHandler, nil, 10, time.Minute,
			util.NewFakeRateLimiter(), testNodeMonitorGracePeriod, testNodeStartupGracePeriod, testNodeMonitorPeriod)
		err := nodeController.RegisterNodes(&nodes, item.retryCount, time.Millisecond)
		if !item.expectedFail && err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if item.expectedFail && err == nil {
			t.Errorf("unexpected non-error")
		}
		if item.fakeNodeHandler.RequestCount != item.expectedRequestCount {
			t.Errorf("expected %v calls, but got %v.", item.expectedRequestCount, item.fakeNodeHandler.RequestCount)
		}
		if len(item.fakeNodeHandler.CreatedNodes) != item.expectedCreateCount {
			t.Errorf("expected %v nodes, but got %v.", item.expectedCreateCount, item.fakeNodeHandler.CreatedNodes)
		}
	}
}

func TestCreateGetStaticNodesWithSpec(t *testing.T) {
	table := []struct {
		machines      []string
		expectedNodes *api.NodeList
	}{
		{
			machines:      []string{},
			expectedNodes: &api.NodeList{},
		},
		{
			machines: []string{"node0"},
			expectedNodes: &api.NodeList{
				Items: []api.Node{
					{
						ObjectMeta: api.ObjectMeta{Name: "node0"},
						Spec: api.NodeSpec{
							ExternalID: "node0",
						},
						Status: api.NodeStatus{
							Capacity: api.ResourceList{
								api.ResourceName(api.ResourceCPU):    resource.MustParse("10"),
								api.ResourceName(api.ResourceMemory): resource.MustParse("10G"),
							},
						},
					},
				},
			},
		},
		{
			machines: []string{"node0", "node1"},
			expectedNodes: &api.NodeList{
				Items: []api.Node{
					{
						ObjectMeta: api.ObjectMeta{Name: "node0"},
						Spec: api.NodeSpec{
							ExternalID: "node0",
						},
						Status: api.NodeStatus{
							Capacity: api.ResourceList{
								api.ResourceName(api.ResourceCPU):    resource.MustParse("10"),
								api.ResourceName(api.ResourceMemory): resource.MustParse("10G"),
							},
						},
					},
					{
						ObjectMeta: api.ObjectMeta{Name: "node1"},
						Spec: api.NodeSpec{
							ExternalID: "node1",
						},
						Status: api.NodeStatus{
							Capacity: api.ResourceList{
								api.ResourceName(api.ResourceCPU):    resource.MustParse("10"),
								api.ResourceName(api.ResourceMemory): resource.MustParse("10G"),
							},
						},
					},
				},
			},
		},
	}

	resources := api.NodeResources{
		Capacity: api.ResourceList{
			api.ResourceName(api.ResourceCPU):    resource.MustParse("10"),
			api.ResourceName(api.ResourceMemory): resource.MustParse("10G"),
		},
	}
	for _, item := range table {
		nodeController := NewNodeController(nil, "", item.machines, &resources, nil, nil, 10, time.Minute,
			util.NewFakeRateLimiter(), testNodeMonitorGracePeriod, testNodeStartupGracePeriod, testNodeMonitorPeriod)
		nodes, err := nodeController.GetStaticNodesWithSpec()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(item.expectedNodes, nodes) {
			t.Errorf("expected node list %+v, got %+v", item.expectedNodes, nodes)
		}
	}
}

func TestCreateGetCloudNodesWithSpec(t *testing.T) {
	resourceList := api.ResourceList{
		api.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
		api.ResourceMemory: *resource.NewQuantity(3000, resource.DecimalSI),
	}

	table := []struct {
		fakeCloud     *fake_cloud.FakeCloud
		machines      []string
		expectedNodes *api.NodeList
	}{
		{
			fakeCloud:     &fake_cloud.FakeCloud{},
			expectedNodes: &api.NodeList{},
		},
		{
			fakeCloud: &fake_cloud.FakeCloud{
				Machines:      []string{"node0"},
				NodeResources: &api.NodeResources{Capacity: resourceList},
			},
			expectedNodes: &api.NodeList{
				Items: []api.Node{
					{
						ObjectMeta: api.ObjectMeta{Name: "node0"},
						Status:     api.NodeStatus{Capacity: resourceList},
					},
				},
			},
		},
		{
			fakeCloud: &fake_cloud.FakeCloud{
				Machines:      []string{"node0", "node1"},
				NodeResources: &api.NodeResources{Capacity: resourceList},
			},
			expectedNodes: &api.NodeList{
				Items: []api.Node{
					{
						ObjectMeta: api.ObjectMeta{Name: "node0"},
						Status:     api.NodeStatus{Capacity: resourceList},
					},
					{
						ObjectMeta: api.ObjectMeta{Name: "node1"},
						Status:     api.NodeStatus{Capacity: resourceList},
					},
				},
			},
		},
	}

	for _, item := range table {
		nodeController := NewNodeController(item.fakeCloud, ".*", nil, &api.NodeResources{}, nil, nil, 10, time.Minute,
			util.NewFakeRateLimiter(), testNodeMonitorGracePeriod, testNodeStartupGracePeriod, testNodeMonitorPeriod)
		nodes, err := nodeController.GetCloudNodesWithSpec()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(item.expectedNodes, nodes) {
			t.Errorf("expected node list %+v, got %+v", item.expectedNodes, nodes)
		}
	}
}

func TestSyncCloudNodes(t *testing.T) {
	table := []struct {
		fakeNodeHandler      *FakeNodeHandler
		fakeCloud            *fake_cloud.FakeCloud
		matchRE              string
		expectedRequestCount int
		expectedNameCreated  []string
		expectedExtIDCreated []string
		expectedAddrsCreated []string
		expectedDeleted      []string
	}{
		{
			// 1 existing node, 1 cloud nodes: do nothing.
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{newNode("node0")},
			},
			fakeCloud: &fake_cloud.FakeCloud{
				Machines: []string{"node0"},
				ExtID: map[string]string{
					"node0": "ext-node0",
					"node1": "ext-node1",
				},
				Addresses: []api.NodeAddress{{Type: api.NodeLegacyHostIP, Address: "1.2.3.4"}},
			},
			matchRE:              ".*",
			expectedRequestCount: 1, // List
			expectedNameCreated:  []string{},
			expectedExtIDCreated: []string{},
			expectedAddrsCreated: []string{},
			expectedDeleted:      []string{},
		},
		{
			// 1 existing node, 2 cloud nodes: create 1.
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{newNode("node0")},
			},
			fakeCloud: &fake_cloud.FakeCloud{
				Machines: []string{"node0", "node1"},
				ExtID: map[string]string{
					"node0": "ext-node0",
					"node1": "ext-node1",
				},
				Addresses: []api.NodeAddress{{Type: api.NodeLegacyHostIP, Address: "1.2.3.4"}},
			},
			matchRE:              ".*",
			expectedRequestCount: 2, // List + Create
			expectedNameCreated:  []string{"node1"},
			expectedExtIDCreated: []string{"ext-node1"},
			expectedAddrsCreated: []string{"1.2.3.4"},
			expectedDeleted:      []string{},
		},
		{
			// 2 existing nodes, 1 cloud node: delete 1.
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{newNode("node0"), newNode("node1")},
			},
			fakeCloud: &fake_cloud.FakeCloud{
				Machines: []string{"node0"},
				ExtID: map[string]string{
					"node0": "ext-node0",
					"node1": "ext-node1",
				},
				Addresses: []api.NodeAddress{{Type: api.NodeLegacyHostIP, Address: "1.2.3.4"}},
			},
			matchRE:              ".*",
			expectedRequestCount: 2, // List + Delete
			expectedNameCreated:  []string{},
			expectedExtIDCreated: []string{},
			expectedAddrsCreated: []string{},
			expectedDeleted:      []string{"node1"},
		},
		{
			// 1 existing node, 3 cloud nodes but only 2 match regex: delete 1.
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{newNode("node0")},
			},
			fakeCloud: &fake_cloud.FakeCloud{
				Machines: []string{"node0", "node1", "fake"},
				ExtID: map[string]string{
					"node0": "ext-node0",
					"node1": "ext-node1",
					"fake":  "ext-fake",
				},
				Addresses: []api.NodeAddress{{Type: api.NodeLegacyHostIP, Address: "1.2.3.4"}},
			},
			matchRE:              "node[0-9]+",
			expectedRequestCount: 2, // List + Create
			expectedNameCreated:  []string{"node1"},
			expectedExtIDCreated: []string{"ext-node1"},
			expectedAddrsCreated: []string{"1.2.3.4"},
			expectedDeleted:      []string{},
		},
	}

	for _, item := range table {
		if item.fakeNodeHandler.Fake == nil {
			item.fakeNodeHandler.Fake = testclient.NewSimpleFake()
		}
		nodeController := NewNodeController(item.fakeCloud, item.matchRE, nil, &api.NodeResources{}, item.fakeNodeHandler, nil, 10, time.Minute,
			util.NewFakeRateLimiter(), testNodeMonitorGracePeriod, testNodeStartupGracePeriod, testNodeMonitorPeriod)
		if err := nodeController.SyncCloudNodes(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if item.fakeNodeHandler.RequestCount != item.expectedRequestCount {
			t.Errorf("expected %v call, but got %v.", item.expectedRequestCount, item.fakeNodeHandler.RequestCount)
		}
		nodes := sortedNodeNames(item.fakeNodeHandler.CreatedNodes)
		if !reflect.DeepEqual(item.expectedNameCreated, nodes) {
			t.Errorf("expected node list %+v, got %+v", item.expectedNameCreated, nodes)
		}
		nodeExtIDs := sortedNodeExternalIDs(item.fakeNodeHandler.CreatedNodes)
		if !reflect.DeepEqual(item.expectedExtIDCreated, nodeExtIDs) {
			t.Errorf("expected node external id list %+v, got %+v", item.expectedExtIDCreated, nodeExtIDs)
		}
		nodeAddrs := sortedNodeAddresses(item.fakeNodeHandler.CreatedNodes)
		if !reflect.DeepEqual(item.expectedAddrsCreated, nodeAddrs) {
			t.Errorf("expected node address list %+v, got %+v", item.expectedAddrsCreated, nodeAddrs)
		}
		nodes = sortedNodeNames(item.fakeNodeHandler.DeletedNodes)
		if !reflect.DeepEqual(item.expectedDeleted, nodes) {
			t.Errorf("expected node list %+v, got %+v", item.expectedDeleted, nodes)
		}
	}
}

func TestSyncCloudNodesEvictPods(t *testing.T) {
	table := []struct {
		fakeNodeHandler      *FakeNodeHandler
		fakeCloud            *fake_cloud.FakeCloud
		matchRE              string
		expectedRequestCount int
		expectedDeleted      []string
		expectedActions      []testclient.FakeAction
	}{
		{
			// No node to delete: do nothing.
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{newNode("node0"), newNode("node1")},
				Fake:     testclient.NewSimpleFake(&api.PodList{Items: []api.Pod{*newPod("pod0", "node0"), *newPod("pod1", "node1")}}),
			},
			fakeCloud: &fake_cloud.FakeCloud{
				Machines: []string{"node0", "node1"},
			},
			matchRE:              ".*",
			expectedRequestCount: 1, // List
			expectedDeleted:      []string{},
			expectedActions:      nil,
		},
		{
			// Delete node1, and pod0 is running on it.
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{newNode("node0"), newNode("node1")},
				Fake:     testclient.NewSimpleFake(&api.PodList{Items: []api.Pod{*newPod("pod0", "node1")}}),
			},
			fakeCloud: &fake_cloud.FakeCloud{
				Machines: []string{"node0"},
			},
			matchRE:              ".*",
			expectedRequestCount: 2, // List + Delete
			expectedDeleted:      []string{"node1"},
			expectedActions:      []testclient.FakeAction{{Action: "list-pods"}, {Action: "delete-pod", Value: "pod0"}},
		},
		{
			// Delete node1, but pod0 is running on node0.
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{newNode("node0"), newNode("node1")},
				Fake:     testclient.NewSimpleFake(&api.PodList{Items: []api.Pod{*newPod("pod0", "node0")}}),
			},
			fakeCloud: &fake_cloud.FakeCloud{
				Machines: []string{"node0"},
			},
			matchRE:              ".*",
			expectedRequestCount: 2, // List + Delete
			expectedDeleted:      []string{"node1"},
			expectedActions:      []testclient.FakeAction{{Action: "list-pods"}},
		},
	}

	for _, item := range table {
		if item.fakeNodeHandler.Fake == nil {
			item.fakeNodeHandler.Fake = testclient.NewSimpleFake()
		}
		nodeController := NewNodeController(item.fakeCloud, item.matchRE, nil, &api.NodeResources{}, item.fakeNodeHandler, nil, 10, time.Minute,
			util.NewFakeRateLimiter(), testNodeMonitorGracePeriod, testNodeStartupGracePeriod, testNodeMonitorPeriod)
		if err := nodeController.SyncCloudNodes(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if item.fakeNodeHandler.RequestCount != item.expectedRequestCount {
			t.Errorf("expected %v call, but got %v.", item.expectedRequestCount, item.fakeNodeHandler.RequestCount)
		}
		nodes := sortedNodeNames(item.fakeNodeHandler.DeletedNodes)
		if !reflect.DeepEqual(item.expectedDeleted, nodes) {
			t.Errorf("expected node list %+v, got %+v", item.expectedDeleted, nodes)
		}
		if !reflect.DeepEqual(item.expectedActions, item.fakeNodeHandler.Actions) {
			t.Errorf("time out waiting for deleting pods, expected %+v, got %+v", item.expectedActions, item.fakeNodeHandler.Actions)
		}
	}
}

func TestPopulateNodeAddresses(t *testing.T) {
	table := []struct {
		nodes             *api.NodeList
		fakeCloud         *fake_cloud.FakeCloud
		expectedFail      bool
		expectedAddresses []api.NodeAddress
	}{
		{
			nodes:     &api.NodeList{Items: []api.Node{*newNode("node0"), *newNode("node1")}},
			fakeCloud: &fake_cloud.FakeCloud{Addresses: []api.NodeAddress{{Type: api.NodeLegacyHostIP, Address: "1.2.3.4"}}},
			expectedAddresses: []api.NodeAddress{
				{Type: api.NodeLegacyHostIP, Address: "1.2.3.4"},
			},
		},
		{
			nodes:             &api.NodeList{Items: []api.Node{*newNode("node0"), *newNode("node1")}},
			fakeCloud:         &fake_cloud.FakeCloud{Err: ErrQueryIPAddress},
			expectedAddresses: nil,
		},
	}

	for _, item := range table {
		nodeController := NewNodeController(item.fakeCloud, ".*", nil, nil, nil, nil, 10, time.Minute,
			util.NewFakeRateLimiter(), testNodeMonitorGracePeriod, testNodeStartupGracePeriod, testNodeMonitorPeriod)
		result, err := nodeController.PopulateAddresses(item.nodes)
		// In case of IP querying error, we should continue.
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		for _, node := range result.Items {
			if !reflect.DeepEqual(item.expectedAddresses, node.Status.Addresses) {
				t.Errorf("expect HostIP %s, got %s", item.expectedAddresses, node.Status.Addresses)
			}
		}
	}
}

func TestMonitorNodeStatusEvictPods(t *testing.T) {
	fakeNow := util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC)
	evictionTimeout := 10 * time.Minute

	table := []struct {
		fakeNodeHandler   *FakeNodeHandler
		timeToPass        time.Duration
		newNodeStatus     api.NodeStatus
		expectedEvictPods bool
		description       string
	}{
		// Node created recently, with no status (happens only at cluster startup).
		{
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{
					{
						ObjectMeta: api.ObjectMeta{
							Name:              "node0",
							CreationTimestamp: fakeNow,
						},
					},
				},
				Fake: testclient.NewSimpleFake(&api.PodList{Items: []api.Pod{*newPod("pod0", "node0")}}),
			},
			timeToPass:        0,
			newNodeStatus:     api.NodeStatus{},
			expectedEvictPods: false,
			description:       "Node created recently, with no status.",
		},
		// Node created long time ago, and kubelet posted NotReady for a short period of time.
		{
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{
					{
						ObjectMeta: api.ObjectMeta{
							Name:              "node0",
							CreationTimestamp: util.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC),
						},
						Status: api.NodeStatus{
							Conditions: []api.NodeCondition{
								{
									Type:               api.NodeReady,
									Status:             api.ConditionFalse,
									LastHeartbeatTime:  util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
									LastTransitionTime: util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
								},
							},
						},
					},
				},
				Fake: testclient.NewSimpleFake(&api.PodList{Items: []api.Pod{*newPod("pod0", "node0")}}),
			},
			timeToPass: evictionTimeout,
			newNodeStatus: api.NodeStatus{
				Conditions: []api.NodeCondition{
					{
						Type:   api.NodeReady,
						Status: api.ConditionFalse,
						// Node status has just been updated, and is NotReady for 10min.
						LastHeartbeatTime:  util.Date(2015, 1, 1, 12, 9, 0, 0, time.UTC),
						LastTransitionTime: util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
					},
				},
			},
			expectedEvictPods: false,
			description:       "Node created long time ago, and kubelet posted NotReady for a short period of time.",
		},
		// Node created long time ago, and kubelet posted NotReady for a long period of time.
		{
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{
					{
						ObjectMeta: api.ObjectMeta{
							Name:              "node0",
							CreationTimestamp: util.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC),
						},
						Status: api.NodeStatus{
							Conditions: []api.NodeCondition{
								{
									Type:               api.NodeReady,
									Status:             api.ConditionFalse,
									LastHeartbeatTime:  util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
									LastTransitionTime: util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
								},
							},
						},
					},
				},
				Fake: testclient.NewSimpleFake(&api.PodList{Items: []api.Pod{*newPod("pod0", "node0")}}),
			},
			timeToPass: time.Hour,
			newNodeStatus: api.NodeStatus{
				Conditions: []api.NodeCondition{
					{
						Type:   api.NodeReady,
						Status: api.ConditionFalse,
						// Node status has just been updated, and is NotReady for 1hr.
						LastHeartbeatTime:  util.Date(2015, 1, 1, 12, 59, 0, 0, time.UTC),
						LastTransitionTime: util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
					},
				},
			},
			expectedEvictPods: true,
			description:       "Node created long time ago, and kubelet posted NotReady for a long period of time.",
		},
		// Node created long time ago, node controller posted Unknown for a short period of time.
		{
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{
					{
						ObjectMeta: api.ObjectMeta{
							Name:              "node0",
							CreationTimestamp: util.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC),
						},
						Status: api.NodeStatus{
							Conditions: []api.NodeCondition{
								{
									Type:               api.NodeReady,
									Status:             api.ConditionUnknown,
									LastHeartbeatTime:  util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
									LastTransitionTime: util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
								},
							},
						},
					},
				},
				Fake: testclient.NewSimpleFake(&api.PodList{Items: []api.Pod{*newPod("pod0", "node0")}}),
			},
			timeToPass: evictionTimeout - testNodeMonitorGracePeriod,
			newNodeStatus: api.NodeStatus{
				Conditions: []api.NodeCondition{
					{
						Type:   api.NodeReady,
						Status: api.ConditionUnknown,
						// Node status was updated by nodecontroller 10min ago
						LastHeartbeatTime:  util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
						LastTransitionTime: util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
					},
				},
			},
			expectedEvictPods: false,
			description:       "Node created long time ago, node controller posted Unknown for a short period of time.",
		},
		// Node created long time ago, node controller posted Unknown for a long period of time.
		{
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{
					{
						ObjectMeta: api.ObjectMeta{
							Name:              "node0",
							CreationTimestamp: util.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC),
						},
						Status: api.NodeStatus{
							Conditions: []api.NodeCondition{
								{
									Type:               api.NodeReady,
									Status:             api.ConditionUnknown,
									LastHeartbeatTime:  util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
									LastTransitionTime: util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
								},
							},
						},
					},
				},
				Fake: testclient.NewSimpleFake(&api.PodList{Items: []api.Pod{*newPod("pod0", "node0")}}),
			},
			timeToPass: 60 * time.Minute,
			newNodeStatus: api.NodeStatus{
				Conditions: []api.NodeCondition{
					{
						Type:   api.NodeReady,
						Status: api.ConditionUnknown,
						// Node status was updated by nodecontroller 1hr ago
						LastHeartbeatTime:  util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
						LastTransitionTime: util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
					},
				},
			},
			expectedEvictPods: true,
			description:       "Node created long time ago, node controller posted Unknown for a long period of time.",
		},
	}

	for _, item := range table {
		nodeController := NewNodeController(nil, "", []string{"node0"}, nil, item.fakeNodeHandler, nil, 10,
			evictionTimeout, util.NewFakeRateLimiter(), testNodeMonitorGracePeriod,
			testNodeStartupGracePeriod, testNodeMonitorPeriod)
		nodeController.now = func() util.Time { return fakeNow }
		if err := nodeController.MonitorNodeStatus(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if item.timeToPass > 0 {
			nodeController.now = func() util.Time { return util.Time{Time: fakeNow.Add(item.timeToPass)} }
			item.fakeNodeHandler.Existing[0].Status = item.newNodeStatus
		}
		if err := nodeController.MonitorNodeStatus(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		podEvicted := false
		for _, action := range item.fakeNodeHandler.Actions {
			if action.Action == "delete-pod" {
				podEvicted = true
			}
		}
		if item.expectedEvictPods != podEvicted {
			t.Errorf("expected pod eviction: %+v, got %+v for %+v", item.expectedEvictPods,
				podEvicted, item.description)
		}
	}
}

func TestMonitorNodeStatusUpdateStatus(t *testing.T) {
	fakeNow := util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC)
	table := []struct {
		fakeNodeHandler      *FakeNodeHandler
		timeToPass           time.Duration
		newNodeStatus        api.NodeStatus
		expectedEvictPods    bool
		expectedRequestCount int
		expectedNodes        []*api.Node
	}{
		// Node created long time ago, without status:
		// Expect Unknown status posted from node controller.
		{
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{
					{
						ObjectMeta: api.ObjectMeta{
							Name:              "node0",
							CreationTimestamp: util.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC),
						},
					},
				},
				Fake: testclient.NewSimpleFake(&api.PodList{Items: []api.Pod{*newPod("pod0", "node0")}}),
			},
			expectedRequestCount: 2, // List+Update
			expectedNodes: []*api.Node{
				{
					ObjectMeta: api.ObjectMeta{
						Name:              "node0",
						CreationTimestamp: util.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC),
					},
					Status: api.NodeStatus{
						Conditions: []api.NodeCondition{
							{
								Type:               api.NodeReady,
								Status:             api.ConditionUnknown,
								Reason:             fmt.Sprintf("Kubelet never posted node status."),
								LastHeartbeatTime:  util.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC),
								LastTransitionTime: fakeNow,
							},
						},
					},
				},
			},
		},
		// Node created recently, without status.
		// Expect no action from node controller (within startup grace period).
		{
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{
					{
						ObjectMeta: api.ObjectMeta{
							Name:              "node0",
							CreationTimestamp: fakeNow,
						},
					},
				},
				Fake: testclient.NewSimpleFake(&api.PodList{Items: []api.Pod{*newPod("pod0", "node0")}}),
			},
			expectedRequestCount: 1, // List
			expectedNodes:        nil,
		},
		// Node created long time ago, with status updated by kubelet exceeds grace period.
		// Expect Unknown status posted from node controller.
		{
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{
					{
						ObjectMeta: api.ObjectMeta{
							Name:              "node0",
							CreationTimestamp: util.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC),
						},
						Status: api.NodeStatus{
							Conditions: []api.NodeCondition{
								{
									Type:   api.NodeReady,
									Status: api.ConditionTrue,
									// Node status hasn't been updated for 1hr.
									LastHeartbeatTime:  util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
									LastTransitionTime: util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
								},
							},
							Capacity: api.ResourceList{
								api.ResourceName(api.ResourceCPU):    resource.MustParse("10"),
								api.ResourceName(api.ResourceMemory): resource.MustParse("10G"),
							},
						},
						Spec: api.NodeSpec{
							ExternalID: "node0",
						},
					},
				},
				Fake: testclient.NewSimpleFake(&api.PodList{Items: []api.Pod{*newPod("pod0", "node0")}}),
			},
			expectedRequestCount: 3, // (List+)List+Update
			timeToPass:           time.Hour,
			newNodeStatus: api.NodeStatus{
				Conditions: []api.NodeCondition{
					{
						Type:   api.NodeReady,
						Status: api.ConditionTrue,
						// Node status hasn't been updated for 1hr.
						LastHeartbeatTime:  util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
						LastTransitionTime: util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
					},
				},
				Capacity: api.ResourceList{
					api.ResourceName(api.ResourceCPU):    resource.MustParse("10"),
					api.ResourceName(api.ResourceMemory): resource.MustParse("10G"),
				},
			},
			expectedNodes: []*api.Node{
				{
					ObjectMeta: api.ObjectMeta{
						Name:              "node0",
						CreationTimestamp: util.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC),
					},
					Status: api.NodeStatus{
						Conditions: []api.NodeCondition{
							{
								Type:               api.NodeReady,
								Status:             api.ConditionUnknown,
								Reason:             fmt.Sprintf("Kubelet stopped posting node status."),
								LastHeartbeatTime:  util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC),
								LastTransitionTime: util.Time{util.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC).Add(time.Hour)},
							},
						},
						Capacity: api.ResourceList{
							api.ResourceName(api.ResourceCPU):    resource.MustParse("10"),
							api.ResourceName(api.ResourceMemory): resource.MustParse("10G"),
						},
					},
					Spec: api.NodeSpec{
						ExternalID: "node0",
					},
				},
			},
		},
		// Node created long time ago, with status updated recently.
		// Expect no action from node controller (within monitor grace period).
		{
			fakeNodeHandler: &FakeNodeHandler{
				Existing: []*api.Node{
					{
						ObjectMeta: api.ObjectMeta{
							Name:              "node0",
							CreationTimestamp: util.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC),
						},
						Status: api.NodeStatus{
							Conditions: []api.NodeCondition{
								{
									Type:   api.NodeReady,
									Status: api.ConditionTrue,
									// Node status has just been updated.
									LastHeartbeatTime:  fakeNow,
									LastTransitionTime: fakeNow,
								},
							},
							Capacity: api.ResourceList{
								api.ResourceName(api.ResourceCPU):    resource.MustParse("10"),
								api.ResourceName(api.ResourceMemory): resource.MustParse("10G"),
							},
						},
						Spec: api.NodeSpec{
							ExternalID: "node0",
						},
					},
				},
				Fake: testclient.NewSimpleFake(&api.PodList{Items: []api.Pod{*newPod("pod0", "node0")}}),
			},
			expectedRequestCount: 1, // List
			expectedNodes:        nil,
		},
	}

	for _, item := range table {
		nodeController := NewNodeController(nil, "", []string{"node0"}, nil, item.fakeNodeHandler, nil, 10, 5*time.Minute, util.NewFakeRateLimiter(),
			testNodeMonitorGracePeriod, testNodeStartupGracePeriod, testNodeMonitorPeriod)
		nodeController.now = func() util.Time { return fakeNow }
		if err := nodeController.MonitorNodeStatus(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if item.timeToPass > 0 {
			nodeController.now = func() util.Time { return util.Time{Time: fakeNow.Add(item.timeToPass)} }
			item.fakeNodeHandler.Existing[0].Status = item.newNodeStatus
			if err := nodeController.MonitorNodeStatus(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}
		if item.expectedRequestCount != item.fakeNodeHandler.RequestCount {
			t.Errorf("expected %v call, but got %v.", item.expectedRequestCount, item.fakeNodeHandler.RequestCount)
		}
		if !api.Semantic.DeepEqual(item.expectedNodes, item.fakeNodeHandler.UpdatedNodes) {
			t.Errorf("expected nodes %+v, got %+v", item.expectedNodes[0],
				item.fakeNodeHandler.UpdatedNodes[0])
		}
	}
}

func newNode(name string) *api.Node {
	return &api.Node{
		ObjectMeta: api.ObjectMeta{Name: name},
		Spec: api.NodeSpec{
			ExternalID: name,
		},
		Status: api.NodeStatus{
			Capacity: api.ResourceList{
				api.ResourceName(api.ResourceCPU):    resource.MustParse("10"),
				api.ResourceName(api.ResourceMemory): resource.MustParse("10G"),
			},
		},
	}
}

func newPod(name, host string) *api.Pod {
	return &api.Pod{ObjectMeta: api.ObjectMeta{Name: name}, Spec: api.PodSpec{Host: host}}
}

func sortedNodeNames(nodes []*api.Node) []string {
	nodeNames := []string{}
	for _, node := range nodes {
		nodeNames = append(nodeNames, node.Name)
	}
	sort.Strings(nodeNames)
	return nodeNames
}

func sortedNodeAddresses(nodes []*api.Node) []string {
	nodeAddresses := []string{}
	for _, node := range nodes {
		for _, addr := range node.Status.Addresses {
			nodeAddresses = append(nodeAddresses, addr.Address)
		}
	}
	sort.Strings(nodeAddresses)
	return nodeAddresses
}

func sortedNodeExternalIDs(nodes []*api.Node) []string {
	nodeExternalIDs := []string{}
	for _, node := range nodes {
		nodeExternalIDs = append(nodeExternalIDs, node.Spec.ExternalID)
	}
	sort.Strings(nodeExternalIDs)
	return nodeExternalIDs
}

func contains(node *api.Node, nodes []*api.Node) bool {
	for i := 0; i < len(nodes); i++ {
		if node.Name == nodes[i].Name {
			return true
		}
	}
	return false
}
