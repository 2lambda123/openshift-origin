package project

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/project/api"
	"github.com/openshift/origin/pkg/project/api/validation"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewStorage returns a new REST.
func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{registry}
}

// New returns a new Project for use with Create and Update.
func (s *REST) New() runtime.Object {
	return &api.Project{}
}

func (*REST) NewList() runtime.Object {
	return &api.Project{}
}

// List retrieves a list of Projects that match selector.
func (s *REST) List(ctx kapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	projects, err := s.registry.ListProjects(ctx, selector)
	if err != nil {
		return nil, err
	}

	return projects, nil
}

// Get retrieves an Project by id.
func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	project, err := s.registry.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}
	return project, nil
}

// Create registers the given Project.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	project, ok := obj.(*api.Project)
	if !ok {
		return nil, fmt.Errorf("not a project: %#v", obj)
	}

	// BETA 1: A project will reserve a namespace == project.Name
	// When Namespace As Kind lands upstream, we will change this so Project project has an ObjectReference to Namespace
	// Or Project will just be a virtual wrapper on Namespace and not its own object
	project.Namespace = project.Name

	kapi.FillObjectMetaSystemFields(ctx, &project.ObjectMeta)

	// TODO set an id if not provided?, set a Namespace attribute if not provided?

	if errs := validation.ValidateProject(project); len(errs) > 0 {
		return nil, errors.NewInvalid("project", project.Name, errs)
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		if err := s.registry.CreateProject(ctx, project); err != nil {
			return nil, err
		}
		return s.Get(ctx, project.Name)
	}), nil
}

// Update is not supported for Projects, as they are immutable.
func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	// TODO handle update of display name, labels, etc.
	return nil, fmt.Errorf("Projects may not be changed.")
}

// Delete asynchronously deletes a Project specified by its id.
func (s *REST) Delete(ctx kapi.Context, id string) (<-chan apiserver.RESTResult, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return &kapi.Status{Status: kapi.StatusSuccess}, s.registry.DeleteProject(ctx, id)
	}), nil
}
