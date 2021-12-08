package disruption

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/openshift/origin/pkg/monitor"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

type BackendSampler interface {
	GetDisruptionBackendName() string
	GetLocator() string
	SetHost(host string)
	GetURL() (string, error)
	SetEventRecorder(recorder events.EventRecorder)
	StartEndpointMonitoring(ctx context.Context, m *monitor.Monitor) error
}

type BackendDisruptionUpgradeTest interface {
	upgrades.Test
	DisplayName() string
}

func NewBackendDisruptionTest(testName string, backend BackendSampler) *backendDisruptionTest {
	return &backendDisruptionTest{
		testName:             testName,
		backend:              backend,
		getAllowedDisruption: NoDisruption,
	}
}

func (t *backendDisruptionTest) WithAllowedDisruption(allowedDisruptionFn AllowedDisruptionFunc) *backendDisruptionTest {
	t.getAllowedDisruption = allowedDisruptionFn
	return t
}

type SetupFunc func(f *framework.Framework, backendSampler BackendSampler) error

func (t *backendDisruptionTest) WithPreSetup(preSetup SetupFunc) *backendDisruptionTest {
	t.preSetup = preSetup
	return t
}

type TearDownFunc func(f *framework.Framework) error

func (t *backendDisruptionTest) WithPostTeardown(postTearDown TearDownFunc) *backendDisruptionTest {
	t.postTearDown = postTearDown
	return t
}

func NoDisruption(f *framework.Framework, totalDuration time.Duration) (*time.Duration, error) {
	zero := 0 * time.Second
	return &zero, nil
}

type AllowedDisruptionFunc func(f *framework.Framework, totalDuration time.Duration) (*time.Duration, error)

// availableTest tests that route frontends are available before, during, and
// after a cluster upgrade.
type backendDisruptionTest struct {
	// testName is the name to show in unit.
	testName string
	// backend describes a route that should be monitored.
	backend              BackendSampler
	getAllowedDisruption AllowedDisruptionFunc

	preSetup     SetupFunc
	postTearDown TearDownFunc
}

func (t *backendDisruptionTest) Name() string { return t.backend.GetDisruptionBackendName() }
func (t *backendDisruptionTest) DisplayName() string {
	return t.testName
}

// Setup looks up the host of the route specified by the backendSampler and updates
// the backendSampler with the route's host.
func (t *backendDisruptionTest) Setup(f *framework.Framework) {
	if t.preSetup != nil {
		framework.ExpectNoError(t.preSetup(f, t.backend))
	}

	url, err := t.backend.GetURL()
	framework.ExpectNoError(err)
	if len(url) == 0 {
		framework.Failf("backend has no URL: %v", t.backend.GetLocator())
	}
}

// Test runs a connectivity check to a route.
func (t *backendDisruptionTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	stopCh := make(chan struct{})
	defer close(stopCh)
	newBroadcaster := events.NewBroadcaster(&events.EventSinkImpl{Interface: f.ClientSet.EventsV1()})
	t.backend.SetEventRecorder(newBroadcaster.NewRecorder(scheme.Scheme, "openshift.io/"+t.backend.GetDisruptionBackendName()))
	newBroadcaster.StartRecordingToSink(stopCh)

	ginkgo.By(fmt.Sprintf("continuously hitting backend: %s", t.backend.GetLocator()))

	ctx, cancel := context.WithCancel(context.Background())
	m := monitor.NewMonitorWithInterval(1 * time.Second)
	err := t.backend.StartEndpointMonitoring(ctx, m)
	framework.ExpectNoError(err, fmt.Sprintf("unable to monitor: %s", t.backend.GetLocator()))

	start := time.Now()
	m.StartSampling(ctx)

	// Wait to ensure the route is still available after the test ends.
	<-done
	ginkgo.By(fmt.Sprintf("waiting for any post disruption failures: %s", t.backend.GetLocator()))
	time.Sleep(30 * time.Second)
	cancel()
	end := time.Now()

	allowedDisruption, err := t.getAllowedDisruption(f, end.Sub(start))
	framework.ExpectNoError(err)

	ExpectNoDisruptionForDuration(
		f,
		*allowedDisruption,
		end.Sub(start),
		m.Intervals(time.Time{}, time.Time{}),
		fmt.Sprintf("%s was unreachable during disruption", t.backend.GetLocator()),
	)
}

// Teardown cleans up any remaining resources.
func (t *backendDisruptionTest) Teardown(f *framework.Framework) {
	if t.postTearDown != nil {
		framework.ExpectNoError(t.postTearDown(f))
	}
}
