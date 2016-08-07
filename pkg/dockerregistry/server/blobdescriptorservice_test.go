package server

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/docker/distribution"
	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/api/v2"
	registryauth "github.com/docker/distribution/registry/auth"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/handlers"
	"github.com/docker/distribution/registry/middleware/registry"
	"github.com/docker/distribution/registry/storage"
	"github.com/docker/distribution/testutil"

	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/client/testclient"
	imagetest "github.com/openshift/origin/pkg/image/admission/testutil"
)

// TestBlobDescriptorServiceIsApplied ensures that blobDescriptorService middleware gets applied.
// It relies on the fact that blobDescriptorService requires higher levels to set repository object on given
// context. If the object isn't given, its method will err out.
func TestBlobDescriptorServiceIsApplied(t *testing.T) {
	ctx := context.Background()

	// don't do any authorization check
	installFakeAccessController(t)
	m := fakeBlobDescriptorService(t)
	// to make other unit tests working
	defer m.changeUnsetRepository(false)

	testImage := newImageForManifest(t, "user/app", sampleImageManifestSchema1, true)
	testImageStream := testNewImageStreamObject("user", "app", "latest", testImage.Name)
	client := &testclient.Fake{}
	client.AddReactor("get", "imagestreams", imagetest.GetFakeImageStreamGetHandler(t, *testImageStream))
	client.AddReactor("get", "images", getFakeImageGetHandler(t, *testImage))

	// TODO: get rid of those nasty global vars
	backupRegistryClient := DefaultRegistryClient
	DefaultRegistryClient = makeFakeRegistryClient(client, ktestclient.NewSimpleFake())
	defer func() {
		// set it back once this test finishes to make other unit tests working
		DefaultRegistryClient = backupRegistryClient
	}()

	app := handlers.NewApp(ctx, &configuration.Configuration{
		Loglevel: "debug",
		Auth: map[string]configuration.Parameters{
			fakeAuthorizerName: {"realm": fakeAuthorizerName},
		},
		Storage: configuration.Storage{
			"inmemory": configuration.Parameters{},
			"cache": configuration.Parameters{
				"blobdescriptor": "inmemory",
			},
			"delete": configuration.Parameters{
				"enabled": true,
			},
		},
		Middleware: map[string][]configuration.Middleware{
			"registry":   {{Name: "openshift"}},
			"repository": {{Name: "openshift"}},
			"storage":    {{Name: "openshift"}},
		},
	})
	server := httptest.NewServer(app)
	router := v2.Router()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("error parsing server url: %v", err)
	}
	os.Setenv("DOCKER_REGISTRY_URL", serverURL.Host)

	desc := uploadTestBlob(t, serverURL, "user/app")

	for _, tc := range []struct {
		name                      string
		method                    string
		endpoint                  string
		vars                      []string
		unsetRepository           bool
		expectedStatus            int
		expectedMethodInvocations map[string]int
	}{
		{
			name:     "get blob with repository unset",
			method:   http.MethodGet,
			endpoint: v2.RouteNameBlob,
			vars: []string{
				"name", "user/app",
				"digest", desc.Digest.String(),
			},
			unsetRepository:           true,
			expectedStatus:            http.StatusInternalServerError,
			expectedMethodInvocations: map[string]int{"Stat": 1},
		},

		{
			name:     "get blob",
			method:   http.MethodGet,
			endpoint: v2.RouteNameBlob,
			vars: []string{
				"name", "user/app",
				"digest", desc.Digest.String(),
			},
			expectedStatus:            http.StatusOK,
			expectedMethodInvocations: map[string]int{"Stat": 2},
		},

		{
			name:     "stat blob with repository unset",
			method:   http.MethodHead,
			endpoint: v2.RouteNameBlob,
			vars: []string{
				"name", "user/app",
				"digest", desc.Digest.String(),
			},
			unsetRepository:           true,
			expectedStatus:            http.StatusInternalServerError,
			expectedMethodInvocations: map[string]int{"Stat": 1},
		},

		{
			name:     "stat blob",
			method:   http.MethodHead,
			endpoint: v2.RouteNameBlob,
			vars: []string{
				"name", "user/app",
				"digest", desc.Digest.String(),
			},
			expectedStatus:            http.StatusOK,
			expectedMethodInvocations: map[string]int{"Stat": 3},
		},

		{
			name:     "delete blob with repository unset",
			method:   http.MethodDelete,
			endpoint: v2.RouteNameBlob,
			vars: []string{
				"name", "user/app",
				"digest", desc.Digest.String(),
			},
			unsetRepository:           true,
			expectedStatus:            http.StatusInternalServerError,
			expectedMethodInvocations: map[string]int{"Stat": 1},
		},

		{
			name:     "delete blob",
			method:   http.MethodDelete,
			endpoint: v2.RouteNameBlob,
			vars: []string{
				"name", "user/app",
				"digest", desc.Digest.String(),
			},
			expectedStatus:            http.StatusAccepted,
			expectedMethodInvocations: map[string]int{"Stat": 1, "Clear": 1},
		},

		{
			// this is expected to succeed because we don't check local links (the manifest is retrieved from
			// etcd)
			name:     "get manifest with repository unset",
			method:   http.MethodGet,
			endpoint: v2.RouteNameManifest,
			vars: []string{
				"name", "user/app",
				"reference", "latest",
			},
			unsetRepository: true,
			expectedStatus:  http.StatusOK,
			//expectedMethodInvocations: map[string]int{"Stat": 2},
		},

		{
			name:     "get manifest",
			method:   http.MethodGet,
			endpoint: v2.RouteNameManifest,
			vars: []string{
				"name", "user/app",
				"reference", "latest",
			},
			expectedStatus: http.StatusOK,
			//expectedMethodInvocations: map[string]int{"Stat": 0},
		},

		{
			// this is expected to succeed because we don't check local links (the manifest is retrieved from
			// etcd)
			name:     "delete manifest with repository unset",
			method:   http.MethodDelete,
			endpoint: v2.RouteNameManifest,
			vars: []string{
				"name", "user/app",
				"reference", testImage.Name,
			},
			unsetRepository:           true,
			expectedStatus:            http.StatusInternalServerError,
			expectedMethodInvocations: map[string]int{"Stat": 1},
		},

		{
			name:     "delete manifest",
			method:   http.MethodDelete,
			endpoint: v2.RouteNameManifest,
			vars: []string{
				"name", "user/app",
				"reference", testImage.Name,
			},
			expectedStatus:            http.StatusNotFound,
			expectedMethodInvocations: map[string]int{"Stat": 1},
		},
	} {
		m.clearStats()
		m.changeUnsetRepository(tc.unsetRepository)

		route := router.GetRoute(tc.endpoint).Host(serverURL.Host)
		u, err := route.URL(tc.vars...)
		if err != nil {
			t.Errorf("[%s] failed to build route: %v", tc.name, err)
			continue
		}

		req, err := http.NewRequest(tc.method, u.String(), nil)
		if err != nil {
			t.Errorf("[%s] failed to make request: %v", tc.name, err)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("[%s] failed to do the request: %v", tc.name, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != tc.expectedStatus {
			t.Errorf("[%s] unexpected status code: %v != %v", tc.name, resp.StatusCode, tc.expectedStatus)
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			content, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("[%s] failed to read body: %v", tc.name, err)
			} else if len(content) > 0 {
				errs := errcode.Errors{}
				err := errs.UnmarshalJSON(content)
				if err != nil {
					t.Logf("[%s] failed to parse body as error: %v", tc.name, err)
					t.Logf("[%s] received body: %v", tc.name, string(content))
				} else {
					t.Logf("[%s] received errors: %#+v", tc.name, errs)
				}
			}
		}

		stats := m.getStats()
		for method, exp := range tc.expectedMethodInvocations {
			invoked := stats[method]
			if invoked != exp {
				t.Errorf("[%s] unexpected number of infocations of method %q: %v != %v", tc.name, method, invoked, exp)
			}
		}
		for method, invoked := range stats {
			if _, ok := tc.expectedMethodInvocations[method]; !ok {
				t.Errorf("[%s] unexpected method %q invoked %d times", tc.name, method, invoked)
			}
		}
	}
}

type testBlobDescriptorManager struct {
	mu              sync.Mutex
	stats           map[string]int
	unsetRepository bool
}

func (m *testBlobDescriptorManager) clearStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for k := range m.stats {
		delete(m.stats, k)
	}
}

func (m *testBlobDescriptorManager) methodInvoked(methodName string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	newCount := m.stats[methodName] + 1
	m.stats[methodName] = newCount

	return newCount
}

// unsetRepository returns true if the testBlobDescriptorService should unset repository from context before
// passing down the call
func (m *testBlobDescriptorManager) getUnsetRepository() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.unsetRepository
}

// changeUnsetRepository allows to configure whether the testBlobDescriptorService should unset repository
// from context before passing down the call
func (m *testBlobDescriptorManager) changeUnsetRepository(unset bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.unsetRepository = unset
}

func (m *testBlobDescriptorManager) getStats() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := make(map[string]int)
	for k, v := range m.stats {
		stats[k] = v
	}
	return stats
}

// fakeBlobDescriptorService installs a fake blob descriptor on top of blobDescriptorService that collects
// stats of method invocations. unsetRepository commands the controller to remove repository object from
// context passed down to blobDescriptorService if true.
func fakeBlobDescriptorService(t *testing.T) *testBlobDescriptorManager {
	m := &testBlobDescriptorManager{
		stats: make(map[string]int),
	}
	middleware.RegisterOptions(storage.BlobDescriptorServiceFactory(&testBlobDescriptorServiceFactory{t: t, m: m}))
	return m
}

type testBlobDescriptorServiceFactory struct {
	t *testing.T
	m *testBlobDescriptorManager
}

func (bf *testBlobDescriptorServiceFactory) BlobAccessController(svc distribution.BlobDescriptorService) distribution.BlobDescriptorService {
	if _, ok := svc.(*blobDescriptorService); !ok {
		svc = (&blobDescriptorServiceFactory{}).BlobAccessController(svc)
	}
	return &testBlobDescriptorService{BlobDescriptorService: svc, t: bf.t, m: bf.m}
}

type testBlobDescriptorService struct {
	distribution.BlobDescriptorService
	t *testing.T
	m *testBlobDescriptorManager
}

func (bs *testBlobDescriptorService) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	bs.m.methodInvoked("Stat")
	if bs.m.getUnsetRepository() {
		bs.t.Logf("unsetting repository from the context")
		ctx = WithRepository(ctx, nil)
	}

	return bs.BlobDescriptorService.Stat(ctx, dgst)
}
func (bs *testBlobDescriptorService) Clear(ctx context.Context, dgst digest.Digest) error {
	bs.m.methodInvoked("Clear")
	if bs.m.getUnsetRepository() {
		bs.t.Logf("unsetting repository from the context")
		ctx = WithRepository(ctx, nil)
	}
	return bs.BlobDescriptorService.Clear(ctx, dgst)
}

// uploadTestBlob generates a random tar file and uploads it to the given repository.
func uploadTestBlob(t *testing.T, serverURL *url.URL, repoName string) distribution.Descriptor {
	rs, ds, err := testutil.CreateRandomTarFile()
	if err != nil {
		t.Fatalf("unexpected error generating test layer file: %v", err)
	}
	dgst := digest.Digest(ds)

	ctx := context.Background()
	ref, err := reference.ParseNamed(repoName)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("server url: %s", serverURL.String())
	repo, err := client.NewRepository(ctx, ref, serverURL.String(), nil)
	if err != nil {
		t.Fatalf("failed to get repository %q: %v", repoName, err)
	}
	blobs := repo.Blobs(ctx)
	wr, err := blobs.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n, err := io.Copy(wr, rs)
	if err != nil {
		t.Fatalf("unexpected error copying to upload: %v", err)
	}
	desc, err := wr.Commit(ctx, distribution.Descriptor{Digest: dgst})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("uploaded generated layer of size %d with digest %q\n", n, dgst.String())

	return desc
}

const fakeAuthorizerName = "fake"

// installFakeAccessController installs an authorizer that allows access anywhere to anybody.
func installFakeAccessController(t *testing.T) {
	registryauth.Register(fakeAuthorizerName, registryauth.InitFunc(
		func(options map[string]interface{}) (registryauth.AccessController, error) {
			return &fakeAccessController{t: t}, nil
		}))
}

type fakeAccessController struct {
	t *testing.T
}

var _ registryauth.AccessController = &fakeAccessController{}

func (f *fakeAccessController) Authorized(ctx context.Context, access ...registryauth.Access) (context.Context, error) {
	for _, access := range access {
		f.t.Logf("fake authorizer: authorizing access to %s:%s:%s", access.Resource.Type, access.Resource.Name, access.Action)
	}

	ctx = WithAuthPerformed(ctx)
	return ctx, nil
}

func makeFakeRegistryClient(client osclient.Interface, kClient kclient.Interface) RegistryClient {
	return &fakeRegistryClient{
		client:  client,
		kClient: kClient,
	}
}

type fakeRegistryClient struct {
	client  osclient.Interface
	kClient kclient.Interface
}

func (f *fakeRegistryClient) Clients() (osclient.Interface, kclient.Interface, error) {
	return f.client, f.kClient, nil
}
func (f *fakeRegistryClient) SafeClientConfig() restclient.Config {
	return (&registryClient{}).SafeClientConfig()
}
