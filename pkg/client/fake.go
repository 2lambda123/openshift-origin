package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	userapi "github.com/openshift/origin/pkg/user/api"
)

type FakeAction struct {
	Action string
	Value  interface{}
}

// Fake implements Interface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the method you want to test easier.
type Fake struct {
	// Fake by default keeps a simple list of the methods that have been called.
	Actions []FakeAction
}

func (c *Fake) CreateBuild(build *buildapi.Build) (*buildapi.Build, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-build"})
	return &buildapi.Build{}, nil
}

func (c *Fake) ListBuilds(selector labels.Selector) (*buildapi.BuildList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-builds"})
	return &buildapi.BuildList{}, nil
}

func (c *Fake) UpdateBuild(build *buildapi.Build) (*buildapi.Build, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-build"})
	return &buildapi.Build{}, nil
}

func (c *Fake) DeleteBuild(id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-build", Value: id})
	return nil
}

func (c *Fake) CreateBuildConfig(config *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-buildconfig"})
	return &buildapi.BuildConfig{}, nil
}

func (c *Fake) ListBuildConfigs(selector labels.Selector) (*buildapi.BuildConfigList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-buildconfig"})
	return &buildapi.BuildConfigList{}, nil
}

func (c *Fake) GetBuildConfig(id string) (*buildapi.BuildConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-buildconfig", Value: id})
	return &buildapi.BuildConfig{}, nil
}

func (c *Fake) UpdateBuildConfig(config *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-buildconfig"})
	return &buildapi.BuildConfig{}, nil
}

func (c *Fake) DeleteBuildConfig(id string) error {
	c.Actions = append(c.Actions, FakeAction{Action: "delete-buildconfig", Value: id})
	return nil
}

func (c *Fake) ListImages(selector labels.Selector) (*imageapi.ImageList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-images"})
	return &imageapi.ImageList{}, nil
}

func (c *Fake) GetImage(id string) (*imageapi.Image, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-image", Value: id})
	return &imageapi.Image{}, nil
}

func (c *Fake) CreateImage(image *imageapi.Image) (*imageapi.Image, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-image"})
	return &imageapi.Image{}, nil
}

func (c *Fake) ListImageRepositories(selector labels.Selector) (*imageapi.ImageRepositoryList, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "list-imagerepositories"})
	return &imageapi.ImageRepositoryList{}, nil
}

func (c *Fake) GetImageRepository(id string) (*imageapi.ImageRepository, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-imagerepository", Value: id})
	return &imageapi.ImageRepository{}, nil
}

func (c *Fake) WatchImageRepositories(field, label labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "watch-imagerepositories"})
	return nil, nil
}

func (c *Fake) CreateImageRepository(repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "create-imagerepository"})
	return &imageapi.ImageRepository{}, nil
}

func (c *Fake) UpdateImageRepository(repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "update-imagerepository"})
	return &imageapi.ImageRepository{}, nil
}

func (c *Fake) CreateImageRepositoryMapping(mapping *imageapi.ImageRepositoryMapping) error {
	c.Actions = append(c.Actions, FakeAction{Action: "create-imagerepositorymapping"})
	return nil
}

func (c *Fake) GetUser(id string) (*userapi.User, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "get-user", Value: id})
	return &userapi.User{}, nil
}

func (c *Fake) CreateOrUpdateUserIdentityMapping(mapping *userapi.UserIdentityMapping) (*userapi.UserIdentityMapping, bool, error) {
	c.Actions = append(c.Actions, FakeAction{Action: "createorupdate-useridentitymapping"})
	return nil, false, nil
}
