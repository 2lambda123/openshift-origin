package kubernetes

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	proxyoptions "k8s.io/kubernetes/cmd/kube-proxy/app/options"
	kubeletoptions "k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/fake"
	"k8s.io/kubernetes/pkg/kubelet/rkt"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/util"
	utilconfig "k8s.io/kubernetes/pkg/util/config"
	"k8s.io/kubernetes/pkg/util/diff"
)

func TestKubeletDefaults(t *testing.T) {
	defaults := kubeletoptions.NewKubeletServer()

	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in BuildKubernetesNodeConfig(), update this expected default to match the new upstream defaults
	expectedDefaults := &kubeletoptions.KubeletServer{
		AuthPath:   util.NewStringFlag(""),
		KubeConfig: util.NewStringFlag("/var/lib/kubelet/kubeconfig"),

		KubeletConfiguration: componentconfig.KubeletConfiguration{
			Address:                     "0.0.0.0", // overridden
			AllowPrivileged:             false,     // overridden
			CAdvisorPort:                4194,      // disabled
			VolumeStatsAggPeriod:        unversioned.Duration{Duration: time.Minute},
			CertDirectory:               "/var/run/kubernetes",
			CgroupRoot:                  "",
			ClusterDNS:                  "", // overridden
			ClusterDomain:               "", // overridden
			ConfigureCBR0:               false,
			ContainerRuntime:            "docker",
			Containerized:               false, // overridden based on OPENSHIFT_CONTAINERIZED
			CPUCFSQuota:                 true,  // forced to true
			DockerExecHandlerName:       "native",
			DockerEndpoint:              "unix:///var/run/docker.sock",
			EventBurst:                  10,
			EventRecordQPS:              5.0,
			EnableCustomMetrics:         false,
			EnableDebuggingHandlers:     true,
			EnableServer:                true,
			EvictionHard:                "memory.available<100Mi",
			FileCheckFrequency:          unversioned.Duration{Duration: 20 * time.Second}, // overridden
			HealthzBindAddress:          "127.0.0.1",                                      // disabled
			HealthzPort:                 10248,                                            // disabled
			HostNetworkSources:          []string{"*"},                                    // overridden
			HostPIDSources:              []string{"*"},                                    // overridden
			HostIPCSources:              []string{"*"},                                    // overridden
			HTTPCheckFrequency:          unversioned.Duration{Duration: 20 * time.Second}, // disabled
			ImageMinimumGCAge:           unversioned.Duration{Duration: 120 * time.Second},
			ImageGCHighThresholdPercent: 90,
			ImageGCLowThresholdPercent:  80,
			IPTablesMasqueradeBit:       14,
			IPTablesDropBit:             15,
			LowDiskSpaceThresholdMB:     256,
			MakeIPTablesUtilChains:      true,
			MasterServiceNamespace:      "default",
			MaxContainerCount:           -1,
			MaxPerPodContainerCount:     1,
			MaxOpenFiles:                1000000,
			MaxPods:                     110, // overridden
			MinimumGCAge:                unversioned.Duration{},
			NetworkPluginDir:            "",
			NetworkPluginName:           "", // overridden
			NonMasqueradeCIDR:           "10.0.0.0/8",
			VolumePluginDir:             "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/",
			NodeStatusUpdateFrequency:   unversioned.Duration{Duration: 10 * time.Second},
			NodeLabels:                  nil,
			OOMScoreAdj:                 -999,
			LockFilePath:                "",
			PodInfraContainerImage:      "gcr.io/google_containers/pause-amd64:3.0", // overridden
			Port:                           10250, // overridden
			ReadOnlyPort:                   10255, // disabled
			RegisterNode:                   true,
			RegisterSchedulable:            true,
			RegistryBurst:                  10,
			RegistryPullQPS:                5.0,
			ResolverConfig:                 kubetypes.ResolvConfDefault,
			KubeletCgroups:                 "",
			RktAPIEndpoint:                 rkt.DefaultRktAPIServiceEndpoint,
			RktPath:                        "",
			RktStage1Image:                 "",
			RootDirectory:                  "/var/lib/kubelet", // overridden
			RuntimeCgroups:                 "",
			SerializeImagePulls:            true,
			StreamingConnectionIdleTimeout: unversioned.Duration{Duration: 4 * time.Hour},
			SyncFrequency:                  unversioned.Duration{Duration: 1 * time.Minute},
			SystemCgroups:                  "",
			TLSCertFile:                    "", // overridden to prevent cert generation
			TLSPrivateKeyFile:              "", // overridden to prevent cert generation
			ReconcileCIDR:                  true,
			KubeAPIQPS:                     5.0,
			KubeAPIBurst:                   10,
			ExperimentalFlannelOverlay:     false,
			OutOfDiskTransitionFrequency:   unversioned.Duration{Duration: 5 * time.Minute},
			HairpinMode:                    "promiscuous-bridge",
			BabysitDaemons:                 false,
			SeccompProfileRoot:             "",
			CloudProvider:                  "auto-detect",
			RuntimeRequestTimeout:          unversioned.Duration{Duration: 2 * time.Minute},
			ContentType:                    "application/vnd.kubernetes.protobuf",
			EnableControllerAttachDetach:   true,

			EvictionPressureTransitionPeriod: unversioned.Duration{Duration: 5 * time.Minute},

			SystemReserved: utilconfig.ConfigurationMap{},
			KubeReserved:   utilconfig.ConfigurationMap{},
		},
	}

	if !reflect.DeepEqual(defaults, expectedDefaults) {
		t.Logf("expected defaults, actual defaults: \n%s", diff.ObjectReflectDiff(expectedDefaults, defaults))
		t.Errorf("Got different defaults than expected, adjust in BuildKubernetesNodeConfig and update expectedDefaults")
	}
}

func TestProxyConfig(t *testing.T) {
	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in buildKubeProxyConfig(), update this expected default to match the new upstream defaults
	oomScoreAdj := int32(-999)
	ipTablesMasqueratebit := int32(14)
	expectedDefaultConfig := &proxyoptions.ProxyServerConfig{
		KubeProxyConfiguration: componentconfig.KubeProxyConfiguration{
			BindAddress:        "0.0.0.0",
			ClusterCIDR:        "",
			HealthzPort:        10249,         // disabled
			HealthzBindAddress: "127.0.0.1",   // disabled
			OOMScoreAdj:        &oomScoreAdj,  // disabled
			ResourceContainer:  "/kube-proxy", // disabled
			IPTablesSyncPeriod: unversioned.Duration{Duration: 30 * time.Second},
			// from k8s.io/kubernetes/cmd/kube-proxy/app/options/options.go
			// defaults to 14.
			IPTablesMasqueradeBit:          &ipTablesMasqueratebit,
			UDPIdleTimeout:                 unversioned.Duration{Duration: 250 * time.Millisecond},
			ConntrackMaxPerCore:            32 * 1024,
			ConntrackTCPEstablishedTimeout: unversioned.Duration{Duration: 86400 * time.Second}, // 1 day (1/5 default)
		},
		ConfigSyncPeriod: 15 * time.Minute,
		KubeAPIQPS:       5.0,
		KubeAPIBurst:     10,
		ContentType:      "application/vnd.kubernetes.protobuf",
	}

	actualDefaultConfig := proxyoptions.NewProxyConfig()

	if !reflect.DeepEqual(expectedDefaultConfig, actualDefaultConfig) {
		t.Errorf("Default kube proxy config has changed. Adjust buildKubeProxyConfig() as needed to disable or make use of additions.")
		t.Logf("Difference %s", diff.ObjectReflectDiff(expectedDefaultConfig, actualDefaultConfig))
	}
}

var fakeCloudProviderName = "fake"

func init() {
	cloudprovider.RegisterCloudProvider(fakeCloudProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		return &fake.FakeCloud{}, nil
	})
}

func TestBuildCloudProviderFake(t *testing.T) {
	server := kubeletoptions.NewKubeletServer()
	server.CloudProvider = fakeCloudProviderName

	cloud, err := buildCloudProvider(server)
	if err != nil {
		t.Errorf("buildCloudProvider failed: %v", err)
	}
	if cloud == nil {
		t.Errorf("buildCloudProvider returned nil cloud provider")
	} else {
		if cloud.ProviderName() != fakeCloudProviderName {
			t.Errorf("Invalid cloud provider returned, expected %q, got %q", fakeCloudProviderName, cloud.ProviderName())
		}
	}
}

func TestBuildCloudProviderNone(t *testing.T) {
	server := kubeletoptions.NewKubeletServer()
	server.CloudProvider = ""
	cloud, err := buildCloudProvider(server)
	if err != nil {
		t.Errorf("buildCloudProvider failed: %v", err)
	}
	if cloud != nil {
		t.Errorf("buildCloudProvider returned cloud provider %q where nil was expected", cloud.ProviderName())
	}
}

func TestBuildCloudProviderError(t *testing.T) {
	server := kubeletoptions.NewKubeletServer()
	server.CloudProvider = "unknown-provider-name"
	cloud, err := buildCloudProvider(server)
	if err == nil {
		t.Errorf("buildCloudProvider returned no error when one was expected")
	}
	if cloud != nil {
		t.Errorf("buildCloudProvider returned cloud provider %q where nil was expected", cloud.ProviderName())
	}
}

var masterKubeConfigText = `apiVersion: v1
clusters:
- cluster:
    server: http://localhost:8080
  name: unit-test
contexts:
- context:
    cluster: unit-test
    user: ""
  name: unit-test
current-context: unit-test
kind: Config
preferences: {}
users: []`

func TestBuildKubernetesNodeConfig_CloudProvider(t *testing.T) {
	tests := []struct {
		name           string
		cloudProvider  string
		expectProvider bool
	}{
		{"no provider set", "", false},
		{"fake provider", fakeCloudProviderName, true},
	}

	for _, test := range tests {
		testDir, err := ioutil.TempDir("", "test-build-kube-node-config")
		if err != nil {
			t.Fatalf("%s: error creating test temp dir: %v", test.name, err)
		}
		defer os.RemoveAll(testDir)

		err = ioutil.WriteFile(filepath.Join(testDir, "masterKubeConfig"), []byte(masterKubeConfigText), 0600)
		if err != nil {
			t.Fatalf("%s: error writing master kube config: %v", test.name, err)
		}

		enableProxy := false
		enableDNS := false
		options := configapi.NodeConfig{
			// the following items are required just to have BuildKubernetesNodeConfig
			// return without error
			MasterKubeConfig: filepath.Join(testDir, "masterKubeConfig"),
			ServingInfo: configapi.ServingInfo{
				BindAddress: "0.0.0.0:12345",
				ServerCert: configapi.CertInfo{
					// fill in something here (even references to bogus files) to keep the
					// kubelet code from trying to generate self signed certs in
					// /var/run/kubernetes, which will get a permission denied error
					CertFile: "foo",
					KeyFile:  "bar",
				},
			},
			IPTablesSyncPeriod: "15s",
			AuthConfig: configapi.NodeAuthConfig{
				AuthenticationCacheTTL: "60s",
				AuthorizationCacheTTL:  "60s",
			},

			// actual test items
			KubeletArguments: configapi.ExtendedArguments{
				"cloud-provider": []string{test.cloudProvider},
			},
		}
		nc, err := BuildKubernetesNodeConfig(options, enableProxy, enableDNS)
		if err != nil {
			t.Error(err)
			continue
		}
		if test.expectProvider {
			if nc.KubeletDeps.Cloud == nil {
				t.Errorf("%s: expected cloud provider to be set but it was nil", test.name)
			} else if e, a := fakeCloudProviderName, nc.KubeletDeps.Cloud.ProviderName(); e != a {
				t.Errorf("%s: expected cloud provider %q, got %q", test.name, e, a)
			}
		} else if nc.KubeletDeps.Cloud != nil {
			t.Errorf("%s: expected no provider but got %q", test.name, nc.KubeletDeps.Cloud.ProviderName())
		}
	}
}
