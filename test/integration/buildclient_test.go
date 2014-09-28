// +build integration,!no-etcd

package integration

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	// "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/version"

	"github.com/openshift/origin/pkg/build"
	"github.com/openshift/origin/pkg/build/api"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	osclient "github.com/openshift/origin/pkg/client"
)

func init() {
	requireEtcd()
}

func TestBuildClient(t *testing.T) {
	deleteAllEtcdKeys()
	etcdClient := newEtcdClient()
	m := master.New(&master.Config{
		EtcdServers: etcdClient.GetCluster(),
	})
	osMux := http.NewServeMux()
	storage := map[string]apiserver.RESTStorage{
		"builds":       buildregistry.NewStorage(build.NewEtcdRegistry(etcdClient)),
		"buildConfigs": buildconfigregistry.NewStorage(build.NewEtcdRegistry(etcdClient)),
	}
	apiserver.NewAPIGroup(m.API_v1beta1()).InstallREST(osMux, "/api/v1beta1")
	apiserver.NewAPIGroup(storage, runtime.Codec).InstallREST(osMux, "/osapi/v1beta1")
	apiserver.InstallSupport(osMux)

	s := httptest.NewServer(osMux)

	kubeclient := client.NewOrDie(s.URL, nil)
	osclient, _ := osclient.New(s.URL, nil)

	info, err := kubeclient.ServerVersion()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := version.Get(), *info; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %#v, got %#v", e, a)
	}

	builds, err := osclient.ListBuilds(labels.Everything())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(builds.Items) != 0 {
		t.Errorf("expected no builds, got %#v", builds)
	}

	// get a validation error
	build := &api.Build{
		Labels: map[string]string{
			"label1": "value1",
			"label2": "value2",
		},
		Input: api.BuildInput{
			Type:         api.DockerBuildType,
			SourceURI:    "http://my.docker/build",
			ImageTag:     "namespace/builtimage",
			BuilderImage: "anImage",
		},
	}
	got, err := osclient.CreateBuild(build)
	if err == nil {
		t.Fatalf("unexpected non-error: %v", err)
	}

	// get a created build
	build.Input.BuilderImage = ""
	got, err = osclient.CreateBuild(build)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID == "" {
		t.Errorf("unexpected empty build ID %v", got)
	}

	// get a list of builds
	builds, err = osclient.ListBuilds(labels.Everything())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(builds.Items) != 1 {
		t.Errorf("expected one build, got %#v", builds)
	}
	actual := builds.Items[0]
	if actual.ID != got.ID {
		t.Errorf("expected build %#v, got %#v", got, actual)
	}
	if actual.Status != api.BuildNew {
		t.Errorf("expected build status to be BuildNew, got %s", actual.Status)
	}

	// delete a build
	err = osclient.DeleteBuild(got.ID)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	builds, err = osclient.ListBuilds(labels.Everything())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(builds.Items) != 0 {
		t.Errorf("expected no builds, got %#v", builds)
	}
}
