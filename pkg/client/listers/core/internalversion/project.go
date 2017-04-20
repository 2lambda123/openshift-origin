// This file was automatically generated by lister-gen

package internalversion

import (
	api "github.com/openshift/origin/pkg/project/api"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ProjectLister helps list Projects.
type ProjectLister interface {
	// List lists all Projects in the indexer.
	List(selector labels.Selector) (ret []*api.Project, err error)
	// Get retrieves the Project from the index for a given name.
	Get(name string) (*api.Project, error)
	ProjectListerExpansion
}

// projectLister implements the ProjectLister interface.
type projectLister struct {
	indexer cache.Indexer
}

// NewProjectLister returns a new ProjectLister.
func NewProjectLister(indexer cache.Indexer) ProjectLister {
	return &projectLister{indexer: indexer}
}

// List lists all Projects in the indexer.
func (s *projectLister) List(selector labels.Selector) (ret []*api.Project, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*api.Project))
	})
	return ret, err
}

// Get retrieves the Project from the index for a given name.
func (s *projectLister) Get(name string) (*api.Project, error) {
	key := &api.Project{ObjectMeta: v1.ObjectMeta{Name: name}}
	obj, exists, err := s.indexer.Get(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(api.Resource("project"), name)
	}
	return obj.(*api.Project), nil
}
