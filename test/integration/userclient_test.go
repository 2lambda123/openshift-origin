// +build integration,!no-etcd

package integration

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kuser "github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"

	"github.com/openshift/origin/pkg/api/latest"
	oapauth "github.com/openshift/origin/pkg/auth/authenticator/password/oauthpassword/registry"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/user"
	"github.com/openshift/origin/pkg/user/api"
	_ "github.com/openshift/origin/pkg/user/api/v1beta1"
	"github.com/openshift/origin/pkg/user/registry/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
	testutil "github.com/openshift/origin/test/util"
)

func init() {
	testutil.RequireEtcd()
}

func TestUserInitialization(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	etcdClient := testutil.NewEtcdClient()
	interfaces, _ := latest.InterfacesFor(latest.Version)
	userRegistry := etcd.New(tools.NewEtcdHelper(etcdClient, interfaces.Codec), user.NewDefaultUserInitStrategy())
	storage := map[string]rest.Storage{
		"userIdentityMappings": useridentitymapping.NewREST(userRegistry),
		"users":                userregistry.NewREST(userRegistry),
	}

	osMux := http.NewServeMux()
	server := httptest.NewServer(osMux)
	defer server.Close()
	handlerContainer := master.NewHandlerContainer(osMux)

	version := &apiserver.APIGroupVersion{
		Root:    "/osapi",
		Version: "v1beta1",

		Storage: storage,
		Codec:   latest.Codec,

		Mapper: latest.RESTMapper,

		Creater: kapi.Scheme,
		Typer:   kapi.Scheme,
		Linker:  interfaces.MetadataAccessor,

		Admit:   admit.NewAlwaysAdmit(),
		Context: kapi.NewRequestContextMapper(),
	}
	if err := version.InstallREST(handlerContainer); err != nil {
		t.Fatalf("unable to install REST: %v", err)
	}

	mapping := api.UserIdentityMapping{
		ObjectMeta: kapi.ObjectMeta{Name: ":test"},
		User: api.User{
			ObjectMeta: kapi.ObjectMeta{Name: ":test"},
		},
		Identity: api.Identity{
			ObjectMeta: kapi.ObjectMeta{Name: ":test"},
			Provider:   "",
			UserName:   "test",
			Extra: map[string]string{
				"name": "Mr. Test",
			},
		},
	}

	client, err := client.New(&kclient.Config{Host: server.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actual, created, err := client.UserIdentityMappings().CreateOrUpdate(&mapping)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Errorf("expected created to be true")
	}

	expectedUser := api.User{
		ObjectMeta: kapi.ObjectMeta{
			Name: ":test",
			// Copy the UID and timestamp from the actual one
			UID:               actual.User.UID,
			CreationTimestamp: actual.User.CreationTimestamp,
		},
		FullName: "Mr. Test",
	}
	// Copy the UID and timestamp from the actual one
	mapping.Identity.UID = actual.Identity.UID
	mapping.Identity.CreationTimestamp = actual.Identity.CreationTimestamp
	expected := &api.UserIdentityMapping{
		ObjectMeta: kapi.ObjectMeta{
			Name: ":test",
			// Copy the UID and timestamp from the actual one
			UID:               actual.UID,
			CreationTimestamp: actual.CreationTimestamp,
		},
		Identity: mapping.Identity,
		User:     expectedUser,
	}
	compareIgnoringSelfLinkAndVersion(t, expected, actual)

	// Make sure uid, name, and creation timestamp get initialized
	if len(actual.UID) == 0 {
		t.Fatalf("Expected UID to be set")
	}
	if len(actual.Name) == 0 {
		t.Fatalf("Expected Name to be set")
	}
	if actual.CreationTimestamp.IsZero() {
		t.Fatalf("Expected CreationTimestamp to be set")
	}
	if len(actual.User.UID) == 0 {
		t.Fatalf("Expected UID to be set")
	}
	if len(actual.User.Name) == 0 {
		t.Fatalf("Expected Name to be set")
	}
	if actual.User.CreationTimestamp.IsZero() {
		t.Fatalf("Expected CreationTimestamp to be set")
	}
	if len(actual.Identity.UID) == 0 {
		t.Fatalf("Expected UID to be set")
	}
	if len(actual.Identity.Name) == 0 {
		t.Fatalf("Expected Name to be set")
	}
	if actual.Identity.CreationTimestamp.IsZero() {
		t.Fatalf("Expected CreationTimestamp to be set")
	}

	user, err := userRegistry.GetUser(expected.User.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	compareIgnoringSelfLinkAndVersion(t, &expected.User, user)

	actualUser, err := client.Users().Get(expectedUser.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	compareIgnoringSelfLinkAndVersion(t, &expected.User, actualUser)
}

type testTokenSource struct {
	Token string
	Err   error
}

func (s *testTokenSource) AuthenticatePassword(username, password string) (string, bool, error) {
	return s.Token, s.Token != "", s.Err
}

func TestUserLookup(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	etcdClient := testutil.NewEtcdClient()
	interfaces, _ := latest.InterfacesFor(latest.Version)
	userRegistry := etcd.New(tools.NewEtcdHelper(etcdClient, interfaces.Codec), user.NewDefaultUserInitStrategy())
	userInfo := &kuser.DefaultInfo{
		Name: ":test",
	}
	contextMapper := kapi.NewRequestContextMapper()

	storage := map[string]rest.Storage{
		"userIdentityMappings": useridentitymapping.NewREST(userRegistry),
		"users":                userregistry.NewREST(userRegistry),
	}

	osMux := http.NewServeMux()
	handlerContainer := master.NewHandlerContainer(osMux)

	version := &apiserver.APIGroupVersion{
		Root:    "/osapi",
		Version: "v1beta1",

		Storage: storage,
		Codec:   latest.Codec,

		Mapper: latest.RESTMapper,

		Creater: kapi.Scheme,
		Typer:   kapi.Scheme,
		Linker:  interfaces.MetadataAccessor,

		Admit:   admit.NewAlwaysAdmit(),
		Context: contextMapper,
	}
	if err := version.InstallREST(handlerContainer); err != nil {
		t.Fatalf("unable to install REST: %v", err)
	}

	// Wrap with authenticator
	authenticatedHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx, ok := contextMapper.Get(req)
		if !ok {
			t.Fatalf("No context on request")
			return
		}
		if err := contextMapper.Update(req, kapi.WithUser(ctx, userInfo)); err != nil {
			t.Fatalf("Could not set user on request")
			return
		}
		osMux.ServeHTTP(w, req)
	})

	// Wrap with contextmapper
	contextHandler, err := kapi.NewRequestContextFilter(contextMapper, authenticatedHandler)
	if err != nil {
		t.Fatalf("Could not create context filter")
	}

	server := httptest.NewServer(contextHandler)

	mapping := api.UserIdentityMapping{
		ObjectMeta: kapi.ObjectMeta{Name: ":test"},
		User: api.User{
			ObjectMeta: kapi.ObjectMeta{Name: ":test"},
		},
		Identity: api.Identity{
			ObjectMeta: kapi.ObjectMeta{Name: ":test"},
			Provider:   "",
			UserName:   "test",
			Extra: map[string]string{
				"name": "Mr. Test",
			},
		},
	}

	client, err := client.New(&kclient.Config{Host: server.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actual, created, err := client.UserIdentityMappings().CreateOrUpdate(&mapping)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		// TODO: t.Errorf("expected created to be true")
	}
	expectedUser := api.User{
		ObjectMeta: kapi.ObjectMeta{
			Name: ":test",
			// Copy the UID and timestamp from the actual one
			UID:               actual.User.UID,
			CreationTimestamp: actual.User.CreationTimestamp,
		},
		FullName: "Mr. Test",
	}
	// Copy the UID and timestamp from the actual one
	mapping.Identity.UID = actual.Identity.UID
	mapping.Identity.CreationTimestamp = actual.Identity.CreationTimestamp
	expected := &api.UserIdentityMapping{
		ObjectMeta: kapi.ObjectMeta{
			Name: ":test",
			// Copy the UID and timestamp from the actual one
			UID:               actual.UID,
			CreationTimestamp: actual.CreationTimestamp,
		},
		Identity: mapping.Identity,
		User:     expectedUser,
	}
	compareIgnoringSelfLinkAndVersion(t, expected, actual)

	// check the user returned by the registry
	user, err := userRegistry.GetUser(expected.User.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	compareIgnoringSelfLinkAndVersion(t, &expected.User, user)

	// check the user returned by the client
	actualUser, err := client.Users().Get(expectedUser.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	compareIgnoringSelfLinkAndVersion(t, &expected.User, actualUser)

	// check the current user
	currentUser, err := client.Users().Get("~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	compareIgnoringSelfLinkAndVersion(t, &expected.User, currentUser)

	// test retrieving user info from a token
	authorizer := oapauth.New(&testTokenSource{Token: "token"}, server.URL, nil)
	info, ok, err := authorizer.AuthenticatePassword("foo", "bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("should have been authenticated")
	}
	if user.Name != info.GetName() || string(user.UID) != info.GetUID() {
		t.Errorf("unexpected user info", info)
	}
}

func compareIgnoringSelfLinkAndVersion(t *testing.T, expected runtime.Object, actual runtime.Object) {
	if actualAccessor, _ := meta.Accessor(actual); actualAccessor != nil {
		actualAccessor.SetSelfLink("")
		actualAccessor.SetResourceVersion("")
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected\n %#v,\n got\n %#v", expected, actual)
	}
}
