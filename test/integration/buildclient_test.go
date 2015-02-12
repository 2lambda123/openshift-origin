// +build integration,!no-etcd

package integration

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildcontrollerfactory "github.com/openshift/origin/pkg/build/controller/factory"
	buildstrategy "github.com/openshift/origin/pkg/build/controller/strategy"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildetcd "github.com/openshift/origin/pkg/build/registry/etcd"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/github"
	osclient "github.com/openshift/origin/pkg/client"
	imageetcd "github.com/openshift/origin/pkg/image/registry/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
)

func init() {
	requireEtcd()
}

func TestListBuilds(t *testing.T) {

	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()

	builds, err := openshift.Client.Builds(testNamespace).List(labels.Everything(), labels.Everything())
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if len(builds.Items) != 0 {
		t.Errorf("Expected no builds, got %#v", builds.Items)
	}
}

func TestCreateBuild(t *testing.T) {

	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()
	build := mockBuild()

	expected, err := openshift.Client.Builds(testNamespace).Create(build)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if expected.Name == "" {
		t.Errorf("Unexpected empty build Name %v", expected)
	}

	builds, err := openshift.Client.Builds(testNamespace).List(labels.Everything(), labels.Everything())
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if len(builds.Items) != 1 {
		t.Errorf("Expected one build, got %#v", builds.Items)
	}
}

func TestDeleteBuild(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()
	build := mockBuild()

	actual, err := openshift.Client.Builds(testNamespace).Create(build)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := openshift.Client.Builds(testNamespace).Delete(actual.Name); err != nil {
		t.Fatalf("Unxpected error: %v", err)
	}
}

func TestWatchBuilds(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()
	build := mockBuild()

	watch, err := openshift.Client.Builds(testNamespace).Watch(labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer watch.Stop()

	expected, err := openshift.Client.Builds(testNamespace).Create(build)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	event := <-watch.ResultChan()
	actual := event.Object.(*buildapi.Build)

	if e, a := expected.Name, actual.Name; e != a {
		t.Errorf("Expected build Name %s, got %s", e, a)
	}
}

func mockBuild() *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Labels: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
		},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://my.docker/build",
				},
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					ContextDir: "context",
				},
			},
			Output: buildapi.BuildOutput{
				DockerImageReference: "namespace/builtimage",
			},
		},
	}
}

type testBuildOpenshift struct {
	Client   *osclient.Client
	server   *httptest.Server
	whPrefix string
	stop     chan struct{}
	lock     sync.Mutex
}

func NewTestBuildOpenshift(t *testing.T) *testBuildOpenshift {
	openshift := &testBuildOpenshift{
		stop: make(chan struct{}),
	}

	openshift.lock.Lock()
	defer openshift.lock.Unlock()
	etcdClient := newEtcdClient()
	etcdHelper, _ := master.NewEtcdHelper(etcdClient, klatest.Version)

	osMux := http.NewServeMux()
	openshift.server = httptest.NewServer(osMux)

	kubeClient := client.NewOrDie(&client.Config{Host: openshift.server.URL, Version: klatest.Version})
	osClient := osclient.NewOrDie(&client.Config{Host: openshift.server.URL, Version: latest.Version})

	openshift.Client = osClient

	kubeletClient, err := kclient.NewKubeletClient(&kclient.KubeletConfig{Port: 10250})
	if err != nil {
		t.Fatalf("Unable to configure Kubelet client: %v", err)
	}

	handlerContainer := master.NewHandlerContainer(osMux)

	_ = master.New(&master.Config{
		Client:           kubeClient,
		EtcdHelper:       etcdHelper,
		KubeletClient:    kubeletClient,
		APIPrefix:        "/api",
		AdmissionControl: admit.NewAlwaysAdmit(),
		RestfulContainer: handlerContainer,
	})

	interfaces, _ := latest.InterfacesFor(latest.Version)

	buildEtcd := buildetcd.New(etcdHelper)
	imageEtcd := imageetcd.New(etcdHelper, imageetcd.DefaultRegistryFunc(func() (string, bool) { return "registry:3000", true }))

	storage := map[string]apiserver.RESTStorage{
		"builds":            buildregistry.NewREST(buildEtcd),
		"buildConfigs":      buildconfigregistry.NewREST(buildEtcd),
		"imageRepositories": imagerepository.NewREST(imageEtcd),
	}

	osPrefix := "/osapi/v1beta1"
	apiserver.NewAPIGroupVersion(storage, latest.Codec, osPrefix, interfaces.MetadataAccessor, admit.NewAlwaysAdmit(), latest.RESTMapper).InstallREST(handlerContainer, "/osapi", "v1beta1")

	openshift.whPrefix = osPrefix + "/buildConfigHooks/"
	osMux.Handle(openshift.whPrefix, http.StripPrefix(openshift.whPrefix,
		webhook.NewController(buildclient.NewOSClientBuildConfigClient(osClient), buildclient.NewOSClientBuildClient(osClient), osClient.ImageRepositories(kapi.NamespaceAll).(osclient.ImageRepositoryNamespaceGetter), map[string]webhook.Plugin{
			"github": github.New(),
		})))

	factory := buildcontrollerfactory.BuildControllerFactory{
		OSClient:     osClient,
		KubeClient:   kubeClient,
		BuildUpdater: buildclient.NewOSClientBuildClient(osClient),
		DockerBuildStrategy: &buildstrategy.DockerBuildStrategy{
			Image:          "test-docker-builder",
			UseLocalImages: false,
			Codec:          latest.Codec,
		},
		STIBuildStrategy: &buildstrategy.STIBuildStrategy{
			Image:                "test-sti-builder",
			TempDirectoryCreator: buildstrategy.STITempDirectoryCreator,
			UseLocalImages:       false,
			Codec:                latest.Codec,
		},
		Stop: openshift.stop,
	}

	factory.Create().Run()

	return openshift
}

func (t *testBuildOpenshift) Close() {
	t.lock.Lock()
	defer t.lock.Unlock()
	time.Sleep(100 * time.Millisecond)
	close(t.stop)
	t.server.CloseClientConnections()
	t.server.Close()
}
