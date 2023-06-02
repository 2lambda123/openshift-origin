package controlplane

import (
	"context"
	"fmt"
	"github.com/openshift/origin/pkg/disruption/backend"
	"time"

	disruptionci "github.com/openshift/origin/pkg/disruption/ci"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"k8s.io/client-go/rest"
)

func StartAllAPIMonitoring(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	if err := startKubeAPIMonitoringWithNewConnections(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startKubeAPIMonitoringWithNewConnectionsAgainstAPICache(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOpenShiftAPIMonitoringWithNewConnections(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOpenShiftAPIMonitoringWithNewConnectionsAgainstAPICache(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOAuthAPIMonitoringWithNewConnections(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOAuthAPIMonitoringWithNewConnectionsAgainstAPICache(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startKubeAPIMonitoringWithConnectionReuse(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startKubeAPIMonitoringWithConnectionReuseAgainstAPICache(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOpenShiftAPIMonitoringWithConnectionReuse(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOpenShiftAPIMonitoringWithConnectionReuseAgainstAPICache(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOAuthAPIMonitoringWithConnectionReuse(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOAuthAPIMonitoringWithConnectionReuseAgainstAPICache(ctx, m, clusterConfig); err != nil {
		return err
	}

	factory := disruptionci.NewDisruptionTestFactory(clusterConfig)
	if err := startKubeAPIMonitoringWithNewConnectionsHTTP2(ctx, m, factory); err != nil {
		return err
	}
	if err := startKubeAPIMonitoringWithConnectionReuseHTTP2(ctx, m, factory); err != nil {
		return err
	}
	if err := startKubeAPIMonitoringWithNewConnectionsHTTP1(ctx, m, factory); err != nil {
		return err
	}
	if err := startKubeAPIMonitoringWithConnectionReuseHTTP1(ctx, m, factory); err != nil {
		return err
	}
	if err := startOpenShiftAPIMonitoringWithNewConnectionsHTTP2(ctx, m, factory); err != nil {
		return err
	}
	if err := startOpenShiftAPIMonitoringWithConnectionReuseHTTP2(ctx, m, factory); err != nil {
		return err
	}
	return nil
}

func startKubeAPIMonitoringWithNewConnectionsHTTP2(ctx context.Context, m monitor.Recorder, factory disruptionci.Factory) error {
	backendSampler, err := createKubeAPIMonitoringWithNewConnectionsHTTP2(factory)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startKubeAPIMonitoringWithConnectionReuseHTTP2(ctx context.Context, m monitor.Recorder, factory disruptionci.Factory) error {
	backendSampler, err := createKubeAPIMonitoringWithConnectionReuseHTTP2(factory)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startKubeAPIMonitoringWithNewConnectionsHTTP1(ctx context.Context, m monitor.Recorder, factory disruptionci.Factory) error {
	backendSampler, err := createKubeAPIMonitoringWithNewConnectionsHTTP1(factory)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startKubeAPIMonitoringWithConnectionReuseHTTP1(ctx context.Context, m monitor.Recorder, factory disruptionci.Factory) error {
	backendSampler, err := createKubeAPIMonitoringWithConnectionReuseHTTP1(factory)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOpenShiftAPIMonitoringWithNewConnectionsHTTP2(ctx context.Context, m monitor.Recorder, factory disruptionci.Factory) error {
	backendSampler, err := createOpenShiftAPIMonitoringWithNewConnectionsHTTP2(factory)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOpenShiftAPIMonitoringWithConnectionReuseHTTP2(ctx context.Context, m monitor.Recorder, factory disruptionci.Factory) error {
	backendSampler, err := createOpenShiftAPIMonitoringWithConnectionReuseHTTP2(factory)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startKubeAPIMonitoringWithNewConnections(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createKubeAPIMonitoringWithNewConnections(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startKubeAPIMonitoringWithNewConnectionsAgainstAPICache(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createKubeAPIMonitoringWithNewConnectionsAgainstAPICache(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOpenShiftAPIMonitoringWithNewConnections(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOpenShiftAPIMonitoringWithNewConnections(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOpenShiftAPIMonitoringWithNewConnectionsAgainstAPICache(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOpenShiftAPIMonitoringWithNewConnectionsAgainstAPICache(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOAuthAPIMonitoringWithNewConnections(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOAuthAPIMonitoringWithNewConnections(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOAuthAPIMonitoringWithNewConnectionsAgainstAPICache(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOAuthAPIMonitoringWithNewConnectionsAgainstAPICache(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startKubeAPIMonitoringWithConnectionReuse(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createKubeAPIMonitoringWithConnectionReuse(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startKubeAPIMonitoringWithConnectionReuseAgainstAPICache(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createKubeAPIMonitoringWithConnectionReuseAgainstAPICache(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOpenShiftAPIMonitoringWithConnectionReuse(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOpenShiftAPIMonitoringWithConnectionReuse(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOpenShiftAPIMonitoringWithConnectionReuseAgainstAPICache(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOpenShiftAPIMonitoringWithConnectionReuseAgainstAPICache(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOAuthAPIMonitoringWithConnectionReuse(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOAuthAPIMonitoringWithConnectionReuse(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOAuthAPIMonitoringWithConnectionReuseAgainstAPICache(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOAuthAPIMonitoringWithConnectionReuseAgainstAPICache(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func createKubeAPIMonitoringWithNewConnections(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	return createAPIServerBackendSampler(clusterConfig, "kube-api", "/api/v1/namespaces/default", monitorapi.NewConnectionType)
}

func createKubeAPIMonitoringWithNewConnectionsAgainstAPICache(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.
	return createAPIServerBackendSampler(clusterConfig, "cache-kube-api", "/api/v1/namespaces/default?resourceVersion=0", monitorapi.NewConnectionType)
}

func createOpenShiftAPIMonitoringWithNewConnections(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// this request should never 404, but should be empty/small
	return createAPIServerBackendSampler(clusterConfig, "openshift-api", "/apis/image.openshift.io/v1/namespaces/default/imagestreams", monitorapi.NewConnectionType)
}

func createOpenShiftAPIMonitoringWithNewConnectionsAgainstAPICache(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.
	// this request should never 404, but should be empty/small
	return createAPIServerBackendSampler(clusterConfig, "cache-openshift-api", "/apis/image.openshift.io/v1/namespaces/default/imagestreams?resourceVersion=0", monitorapi.NewConnectionType)
}

func createOAuthAPIMonitoringWithNewConnections(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// this should be relatively small and should not ever 404
	return createAPIServerBackendSampler(clusterConfig, "oauth-api", "/apis/oauth.openshift.io/v1/oauthclients", monitorapi.NewConnectionType)
}

func createOAuthAPIMonitoringWithNewConnectionsAgainstAPICache(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.
	// this should be relatively small and should not ever 404
	return createAPIServerBackendSampler(clusterConfig, "cache-oauth-api", "/apis/oauth.openshift.io/v1/oauthclients?resourceVersion=0", monitorapi.NewConnectionType)
}

func createKubeAPIMonitoringWithConnectionReuse(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// default gets auto-created, so this should always exist
	return createAPIServerBackendSampler(clusterConfig, "kube-api", "/api/v1/namespaces/default", monitorapi.ReusedConnectionType)
}

func createKubeAPIMonitoringWithConnectionReuseAgainstAPICache(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.
	// default gets auto-created, so this should always exist
	return createAPIServerBackendSampler(clusterConfig, "cache-kube-api", "/api/v1/namespaces/default?resourceVersion=0", monitorapi.ReusedConnectionType)
}

func createOpenShiftAPIMonitoringWithConnectionReuse(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// this request should never 404, but should be empty/small
	return createAPIServerBackendSampler(clusterConfig, "openshift-api", "/apis/image.openshift.io/v1/namespaces/default/imagestreams", monitorapi.ReusedConnectionType)
}

func createOpenShiftAPIMonitoringWithConnectionReuseAgainstAPICache(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.
	// this request should never 404, but should be empty/small
	return createAPIServerBackendSampler(clusterConfig, "cache-openshift-api", "/apis/image.openshift.io/v1/namespaces/default/imagestreams?resourceVersion=0", monitorapi.ReusedConnectionType)
}

func createOAuthAPIMonitoringWithConnectionReuse(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// this should be relatively small and should not ever 404
	return createAPIServerBackendSampler(clusterConfig, "oauth-api", "/apis/oauth.openshift.io/v1/oauthclients", monitorapi.ReusedConnectionType)
}

func createOAuthAPIMonitoringWithConnectionReuseAgainstAPICache(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.
	// this should be relatively small and should not ever 404
	return createAPIServerBackendSampler(clusterConfig, "cache-oauth-api", "/apis/oauth.openshift.io/v1/oauthclients?resourceVersion=0", monitorapi.ReusedConnectionType)
}

func createKubeAPIMonitoringWithNewConnectionsHTTP2(factory disruptionci.Factory) (*disruptionci.BackendSampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.KubeAPIServer,
			LoadBalancerType: backend.ExternalLoadBalancerType,
			ConnectionType:   monitorapi.NewConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         "/api/v1/namespaces/default",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
}

func createKubeAPIMonitoringWithConnectionReuseHTTP2(factory disruptionci.Factory) (*disruptionci.BackendSampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.KubeAPIServer,
			LoadBalancerType: backend.ExternalLoadBalancerType,
			ConnectionType:   monitorapi.ReusedConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         "/api/v1/namespaces/default",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
}

func createKubeAPIMonitoringWithNewConnectionsHTTP1(factory disruptionci.Factory) (*disruptionci.BackendSampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.KubeAPIServer,
			LoadBalancerType: backend.ExternalLoadBalancerType,
			ConnectionType:   monitorapi.NewConnectionType,
			Protocol:         backend.ProtocolHTTP1,
		},
		Path:                         "/api/v1/namespaces/default",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
}

func createKubeAPIMonitoringWithConnectionReuseHTTP1(factory disruptionci.Factory) (*disruptionci.BackendSampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.KubeAPIServer,
			LoadBalancerType: backend.ExternalLoadBalancerType,
			ConnectionType:   monitorapi.ReusedConnectionType,
			Protocol:         backend.ProtocolHTTP1,
		},
		Path:                         "/api/v1/namespaces/default",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
}

func createOpenShiftAPIMonitoringWithNewConnectionsHTTP2(factory disruptionci.Factory) (*disruptionci.BackendSampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.OpenShiftAPIServer,
			LoadBalancerType: backend.ExternalLoadBalancerType,
			ConnectionType:   monitorapi.NewConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         "/apis/image.openshift.io/v1/namespaces/default/imagestreams",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
}

func createOpenShiftAPIMonitoringWithConnectionReuseHTTP2(factory disruptionci.Factory) (*disruptionci.BackendSampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.OpenShiftAPIServer,
			LoadBalancerType: backend.ExternalLoadBalancerType,
			ConnectionType:   monitorapi.ReusedConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         "/apis/image.openshift.io/v1/namespaces/default/imagestreams",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
}

func createAPIServerBackendSampler(clusterConfig *rest.Config, disruptionBackendName, url string, connectionType monitorapi.BackendConnectionType) (*backenddisruption.BackendSampler, error) {
	// default gets auto-created, so this should always exist
	backendSampler, err := backenddisruption.NewAPIServerBackend(clusterConfig, disruptionBackendName, url, connectionType)
	if err != nil {
		return nil, err
	}
	backendSampler = backendSampler.WithUserAgent(fmt.Sprintf("openshift-external-backend-sampler-%s-%s", connectionType, disruptionBackendName))

	return backendSampler, nil
}
