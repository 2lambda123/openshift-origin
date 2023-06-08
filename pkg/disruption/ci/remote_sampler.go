package ci

import (
	"context"
	"sync"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/kubernetes/test/e2e/framework"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
)

type testRemoteFactory struct {
	dependency dependency
	err        error
}

// RemoteSampler has the machinery to start disruption monitor in the cluster
type RemoteSampler struct {
	lock   sync.Mutex
	cancel context.CancelFunc
}

func (bs *RemoteSampler) GetTargetServerName() string {
	return ""
}

func (bs *RemoteSampler) GetLoadBalancerType() string {
	return ""
}

func (bs *RemoteSampler) GetConnectionType() monitorapi.BackendConnectionType {
	return ""
}

func (bs *RemoteSampler) GetProtocol() string {
	return ""
}

func (bs *RemoteSampler) GetDisruptionBackendName() string {
	return ""
}

func (bs *RemoteSampler) GetLocator() string {
	return ""
}

func (bs *RemoteSampler) GetURL() (string, error) {
	return "", nil
}

func (bs *RemoteSampler) RunEndpointMonitoring(ctx context.Context, m backenddisruption.Recorder, eventRecorder events.EventRecorder) error {
	ctx, cancel := context.WithCancel(ctx)
	bs.lock.Lock()
	bs.cancel = cancel
	bs.lock.Unlock()

	if eventRecorder == nil {
		fakeEventRecorder := events.NewFakeRecorder(100)
		// discard the events
		go func() {
			for {
				select {
				case <-fakeEventRecorder.Events:
				case <-ctx.Done():
					return
				}
			}
		}()
		eventRecorder = fakeEventRecorder
	}

	framework.Logf("InClusterDisruptionTest: starting in-cluster monitors")
	<-ctx.Done()

	framework.Logf("InClusterDisruptionTest: Run has completed")

	return nil
}

func (bs *RemoteSampler) StartEndpointMonitoring(ctx context.Context, m backenddisruption.Recorder, eventRecorder events.EventRecorder) error {
	return nil
}

func (bs *RemoteSampler) Stop() {
	bs.lock.Lock()
	cancel := bs.cancel
	bs.lock.Unlock()

	if cancel != nil {
		cancel()
	}
}

// NewInClusterMonitorTestFactory returns a shared disruption test factory that uses
// the given rest Config object to create new disruption test instances.
func NewInClusterMonitorTestFactory(config *rest.Config) Factory {
	return &testRemoteFactory{
		dependency: &restConfigDependency{
			config: config,
		},
	}
}

func (b *testRemoteFactory) New(c TestConfiguration) (Sampler, error) {
	if b.err != nil {
		return nil, b.err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &RemoteSampler{}, nil
}
