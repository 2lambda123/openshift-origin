package imagestreamtag

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/api/latest"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
)

var testDefaultRegistry = imagestream.DefaultRegistryFunc(func() (string, bool) { return "defaultregistry:5000", true })

type fakeSubjectAccessReviewRegistry struct {
}

var _ subjectaccessreview.Registry = &fakeSubjectAccessReviewRegistry{}

func (f *fakeSubjectAccessReviewRegistry) CreateSubjectAccessReview(ctx kapi.Context, subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	return nil, nil
}

type fakeUser struct {
}

var _ user.Info = &fakeUser{}

func (u *fakeUser) GetName() string {
	return "user"
}

func (u *fakeUser) GetUID() string {
	return "uid"
}

func (u *fakeUser) GetGroups() []string {
	return []string{"group1"}
}

func setup(t *testing.T) (*tools.FakeEtcdClient, tools.EtcdHelper, *REST) {
	fakeEtcdClient := tools.NewFakeEtcdClient(t)
	fakeEtcdClient.TestIndex = true
	helper := tools.NewEtcdHelper(fakeEtcdClient, latest.Codec)
	imageStorage := imageetcd.NewREST(helper)
	imageRegistry := image.NewRegistry(imageStorage)
	imageStreamStorage, imageStreamStatus := imagestreametcd.NewREST(helper, testDefaultRegistry, &fakeSubjectAccessReviewRegistry{})
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatus)
	storage := NewREST(imageRegistry, imageStreamRegistry)
	return fakeEtcdClient, helper, storage
}

type statusError interface {
	Status() kapi.Status
}

func TestNameAndTag(t *testing.T) {
	tests := map[string]struct {
		id           string
		expectedName string
		expectedTag  string
		expectError  bool
	}{
		"empty id": {
			id:          "",
			expectError: true,
		},
		"missing semicolon": {
			id:          "hello",
			expectError: true,
		},
		"too many semicolons": {
			id:          "a:b:c",
			expectError: true,
		},
		"empty name": {
			id:          ":tag",
			expectError: true,
		},
		"empty tag": {
			id:          "name",
			expectError: true,
		},
		"happy path": {
			id:           "name:tag",
			expectError:  false,
			expectedName: "name",
			expectedTag:  "tag",
		},
	}

	for description, testCase := range tests {
		name, tag, err := nameAndTag(testCase.id)
		gotError := err != nil
		if e, a := testCase.expectError, gotError; e != a {
			t.Fatalf("%s: expected err: %t, got: %t: %s", description, e, a, err)
		}
		if err != nil {
			continue
		}
		if e, a := testCase.expectedName, name; e != a {
			t.Errorf("%s: name: expected %q, got %q", description, e, a)
		}
		if e, a := testCase.expectedTag, tag; e != a {
			t.Errorf("%s: tag: expected %q, got %q", description, e, a)
		}
	}
}

func TestGetImageStreamTag(t *testing.T) {
	tests := map[string]struct {
		image           *api.Image
		repo            *api.ImageStream
		expectError     bool
		errorTargetKind string
		errorTargetID   string
	}{
		"happy path": {
			image: &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"},
			repo: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							Annotations: map[string]string{
								"color": "blue",
								"size":  "large",
							},
						},
					},
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"latest": {Items: []api.TagEvent{{DockerImageReference: "test", Image: "10"}}},
					},
				},
			},
		},
		"image = ''": {
			repo: &api.ImageStream{Status: api.ImageStreamStatus{
				Tags: map[string]api.TagEventList{
					"latest": {Items: []api.TagEvent{{DockerImageReference: "test", Image: ""}}},
				},
			}},
			expectError:     true,
			errorTargetKind: "imageStreamTag",
			errorTargetID:   "test:latest",
		},
		"missing image": {
			repo: &api.ImageStream{Status: api.ImageStreamStatus{
				Tags: map[string]api.TagEventList{
					"latest": {Items: []api.TagEvent{{DockerImageReference: "test", Image: "10"}}},
				},
			}},
			expectError:     true,
			errorTargetKind: "image",
			errorTargetID:   "10",
		},
		"missing repo": {
			expectError:     true,
			errorTargetKind: "imageStream",
			errorTargetID:   "test",
		},
		"missing tag": {
			image: &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"},
			repo: &api.ImageStream{Status: api.ImageStreamStatus{
				Tags: map[string]api.TagEventList{
					"other": {Items: []api.TagEvent{{DockerImageReference: "test", Image: "10"}}},
				},
			}},
			expectError:     true,
			errorTargetKind: "imageStreamTag",
			errorTargetID:   "test:latest",
		},
	}

	for name, testCase := range tests {
		fakeEtcdClient, _, storage := setup(t)

		if testCase.image != nil {
			fakeEtcdClient.Data["/images/"+testCase.image.Name] = tools.EtcdResponseWithError{
				R: &etcd.Response{
					Node: &etcd.Node{
						Value:         runtime.EncodeOrDie(latest.Codec, testCase.image),
						ModifiedIndex: 1,
					},
				},
			}
		} else {
			fakeEtcdClient.Data["/images/10"] = tools.EtcdResponseWithError{
				R: &etcd.Response{
					Node: nil,
				},
				E: tools.EtcdErrorNotFound,
			}
		}

		if testCase.repo != nil {
			fakeEtcdClient.Data["/imageRepositories/default/test"] = tools.EtcdResponseWithError{
				R: &etcd.Response{
					Node: &etcd.Node{
						Value:         runtime.EncodeOrDie(latest.Codec, testCase.repo),
						ModifiedIndex: 1,
					},
				},
			}
		} else {
			fakeEtcdClient.Data["/imageRepositories/default/test"] = tools.EtcdResponseWithError{
				R: &etcd.Response{
					Node: nil,
				},
				E: tools.EtcdErrorNotFound,
			}
		}

		obj, err := storage.Get(kapi.NewDefaultContext(), "test:latest")
		gotErr := err != nil
		if e, a := testCase.expectError, gotErr; e != a {
			t.Fatalf("%s: Expected err=%v: got %v: %v", name, e, a, err)
		}
		if testCase.expectError {
			if !errors.IsNotFound(err) {
				t.Fatalf("%s: unexpected error type: %v", name, err)
			}
			status := err.(statusError).Status()
			if status.Details.Kind != testCase.errorTargetKind || status.Details.ID != testCase.errorTargetID {
				t.Errorf("%s: unexpected status: %#v", name, status)
			}
		} else {
			actual := obj.(*api.ImageStreamTag)
			if e, a := "default", actual.Namespace; e != a {
				t.Errorf("%s: namespace: expected %v, got %v", name, e, a)
			}
			if e, a := "test:latest", actual.Name; e != a {
				t.Errorf("%s: name: expected %v, got %v", name, e, a)
			}
			if e, a := map[string]string{"size": "large", "color": "blue"}, actual.Annotations; !reflect.DeepEqual(e, a) {
				t.Errorf("%s: annotations: expected %v, got %v", name, e, a)
			}
		}
	}
}

func TestDeleteImageStreamTag(t *testing.T) {
	tests := map[string]struct {
		repo        *api.ImageStream
		expectError bool
	}{
		"repo not found": {
			expectError: true,
		},
		"nil tag map": {
			repo: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
			},
			expectError: true,
		},
		"missing tag": {
			repo: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"other": {
							From: &kapi.ObjectReference{
								Kind: "ImageStreamTag",
								Name: "test:foo",
							},
						},
					},
				},
			},
			expectError: true,
		},
		"happy path": {
			repo: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"another": {
							From: &kapi.ObjectReference{
								Kind: "ImageStreamTag",
								Name: "test:foo",
							},
						},
						"latest": {
							From: &kapi.ObjectReference{
								Kind: "ImageStreamTag",
								Name: "test:bar",
							},
						},
					},
				},
				Status: api.ImageStreamStatus{
					DockerImageRepository: "registry.default.local/default/test",
					Tags: map[string]api.TagEventList{
						"another": {
							Items: []api.TagEvent{
								{
									DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
								},
							},
						},
						"foo": {
							Items: []api.TagEvent{
								{
									DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
								},
							},
						},
						"latest": {
							Items: []api.TagEvent{
								{
									DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
								},
							},
						},
						"bar": {
							Items: []api.TagEvent{
								{
									DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
								},
							},
						},
					},
				},
			},
		},
	}

	for name, testCase := range tests {
		fakeEtcdClient, helper, storage := setup(t)
		if testCase.repo != nil {
			fakeEtcdClient.Data["/imageRepositories/default/test"] = tools.EtcdResponseWithError{
				R: &etcd.Response{
					Node: &etcd.Node{
						Value:         runtime.EncodeOrDie(latest.Codec, testCase.repo),
						ModifiedIndex: 1,
					},
				},
			}
		} else {
			fakeEtcdClient.Data["/imageRepositories/default/test"] = tools.EtcdResponseWithError{
				R: &etcd.Response{
					Node: nil,
				},
				E: tools.EtcdErrorNotFound,
			}
		}

		ctx := kapi.WithUser(kapi.NewDefaultContext(), &fakeUser{})
		obj, err := storage.Delete(ctx, "test:latest")
		gotError := err != nil
		if e, a := testCase.expectError, gotError; e != a {
			t.Fatalf("%s: expectError=%t, gotError=%t: %s", name, e, a, err)
		}
		if testCase.expectError {
			continue
		}

		if obj == nil {
			t.Fatalf("%s: unexpected nil response", name)
		}
		expectedStatus := &kapi.Status{Status: kapi.StatusSuccess}
		if e, a := expectedStatus, obj; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: expected %#v, got %#v", name, e, a)
		}

		updatedRepo := &api.ImageStream{}
		if err := helper.ExtractObj("/imageRepositories/default/test", updatedRepo, false); err != nil {
			t.Fatalf("%s: error retrieving updated repo: %s", name, err)
		}
		expected := map[string]api.TagReference{
			"another": {
				From: &kapi.ObjectReference{
					Kind: "ImageStreamTag",
					Name: "test:foo",
				},
			},
		}
		if e, a := expected, updatedRepo.Spec.Tags; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: tags: expected %v, got %v", name, e, a)
		}
	}
}

func TestWatch(t *testing.T) {
	fakeEtcdClient, helper, storage := setup(t)

	myImage := api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: "myimage",
		},
		DockerImageReference: "default/foo@myimage",
	}

	helper.SetObj("/images/myimage", &myImage, &myImage, 0)

	myImage2 := api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: "myimage2",
		},
		DockerImageReference: "default/foo@myimage2",
	}

	helper.SetObj("/images/myimage2", &myImage2, &myImage2, 0)

	stream := api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "default",
			Name:      "foo",
		},
		Status: api.ImageStreamStatus{
			Tags: map[string]api.TagEventList{
				"latest": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "default/foo:latest",
							Image:                "myimage",
						},
					},
				},
			},
		},
	}

	helper.SetObj("/imageRepositories/default/foo", &stream, &stream, 0)

	watching, err := storage.Watch(kapi.NewDefaultContext(), labels.Everything(), fields.SelectorFromSet(fields.Set{"name": "foo:latest"}), "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fakeEtcdClient.WaitForWatchCompletion()

	// simulate watch event for default/foo
	glog.Infof("Simulating watch event for default/foo")
	fakeEtcdClient.WatchResponse <- &etcd.Response{
		Action: "set",
		Node: &etcd.Node{
			Value: runtime.EncodeOrDie(latest.Codec, &stream),
		},
	}

	expected := api.ImageStreamTag{
		Image: api.Image{
			ObjectMeta: kapi.ObjectMeta{
				Namespace:       "default",
				Name:            "foo:latest",
				ResourceVersion: "1",
			},
			DockerImageReference:       "default/foo@myimage",
			DockerImageMetadataVersion: "1.0",
		},
	}

	// wait for watch event
	glog.Infof("Waiting for watch event")
	select {
	case event, ok := <-watching.ResultChan():
		if !ok {
			t.Errorf("watching channel should be open")
		}
		if e, a := &expected, event.Object; !reflect.DeepEqual(e, a) {
			t.Errorf("unexpected watch object, diff=%s", util.ObjectDiff(e, a))
		}
	}

	// simulate watch event for default/foo, but no change to :latest
	glog.Infof("Simulating watch event for default/foo, no change to :latest")
	stream.Status.Tags["other"] = api.TagEventList{
		Items: []api.TagEvent{
			{
				DockerImageReference: "default/foo@someimage",
				Image:                "someimage",
			},
		},
	}
	helper.SetObj("/imageRepositories/default/foo", &stream, &stream, 0)
	fakeEtcdClient.WatchResponse <- &etcd.Response{
		Action: "set",
		Node: &etcd.Node{
			Value: runtime.EncodeOrDie(latest.Codec, &stream),
		},
	}

	// wait for watch event
	glog.Infof("Waiting for watch event")
	select {
	case event, ok := <-watching.ResultChan():
		if !ok {
			t.Errorf("watching channel should be open")
		}
		if e, a := &expected, event.Object; !reflect.DeepEqual(e, a) {
			t.Errorf("unexpected watch object, diff=%s", util.ObjectDiff(e, a))
		}
	}

	// simulate watch event for default/foo, but changing :latest
	glog.Infof("Simulating watch event for default/foo, change :latest")
	history := stream.Status.Tags["latest"]
	event := &history.Items[0]
	event.DockerImageReference = "default/foo@myimage2"
	event.Image = "myimage2"
	stream.Status.Tags["latest"] = history
	helper.SetObj("/imageRepositories/default/foo", &stream, &stream, 0)
	fakeEtcdClient.WatchResponse <- &etcd.Response{
		Action: "set",
		Node: &etcd.Node{
			Value: runtime.EncodeOrDie(latest.Codec, &stream),
		},
	}

	expected2 := api.ImageStreamTag{
		Image: api.Image{
			ObjectMeta: kapi.ObjectMeta{
				Namespace:       "default",
				Name:            "foo:latest",
				ResourceVersion: "2",
			},
			DockerImageReference:       "default/foo@myimage2",
			DockerImageMetadataVersion: "1.0",
		},
	}
	// wait for watch event
	glog.Infof("Waiting for watch event")
	select {
	case event, ok := <-watching.ResultChan():
		if !ok {
			t.Errorf("watching channel should be open")
		}
		if e, a := &expected2, event.Object; !reflect.DeepEqual(e, a) {
			t.Errorf("unexpected watch object, diff=%s", util.ObjectDiff(e, a))
		}
	}

	watching.Stop()
}
