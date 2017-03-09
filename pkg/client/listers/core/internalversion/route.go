// This file was automatically generated by lister-gen with arguments: --input-dirs=[github.com/openshift/origin/pkg/authorization/api,github.com/openshift/origin/pkg/authorization/api/v1,github.com/openshift/origin/pkg/build/api,github.com/openshift/origin/pkg/build/api/v1,github.com/openshift/origin/pkg/deploy/api,github.com/openshift/origin/pkg/deploy/api/v1,github.com/openshift/origin/pkg/image/api,github.com/openshift/origin/pkg/image/api/v1,github.com/openshift/origin/pkg/oauth/api,github.com/openshift/origin/pkg/oauth/api/v1,github.com/openshift/origin/pkg/project/api,github.com/openshift/origin/pkg/project/api/v1,github.com/openshift/origin/pkg/quota/api,github.com/openshift/origin/pkg/quota/api/v1,github.com/openshift/origin/pkg/route/api,github.com/openshift/origin/pkg/route/api/v1,github.com/openshift/origin/pkg/sdn/api,github.com/openshift/origin/pkg/sdn/api/v1,github.com/openshift/origin/pkg/template/api,github.com/openshift/origin/pkg/template/api/v1,github.com/openshift/origin/pkg/user/api,github.com/openshift/origin/pkg/user/api/v1] --logtostderr=true

package internalversion

import (
	api "github.com/openshift/origin/pkg/route/api"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// RouteLister helps list Routes.
type RouteLister interface {
	// List lists all Routes in the indexer.
	List(selector labels.Selector) (ret []*api.Route, err error)
	// Routes returns an object that can list and get Routes.
	Routes(namespace string) RouteNamespaceLister
	RouteListerExpansion
}

// routeLister implements the RouteLister interface.
type routeLister struct {
	indexer cache.Indexer
}

// NewRouteLister returns a new RouteLister.
func NewRouteLister(indexer cache.Indexer) RouteLister {
	return &routeLister{indexer: indexer}
}

// List lists all Routes in the indexer.
func (s *routeLister) List(selector labels.Selector) (ret []*api.Route, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*api.Route))
	})
	return ret, err
}

// Routes returns an object that can list and get Routes.
func (s *routeLister) Routes(namespace string) RouteNamespaceLister {
	return routeNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// RouteNamespaceLister helps list and get Routes.
type RouteNamespaceLister interface {
	// List lists all Routes in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*api.Route, err error)
	// Get retrieves the Route from the indexer for a given namespace and name.
	Get(name string) (*api.Route, error)
	RouteNamespaceListerExpansion
}

// routeNamespaceLister implements the RouteNamespaceLister
// interface.
type routeNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Routes in the indexer for a given namespace.
func (s routeNamespaceLister) List(selector labels.Selector) (ret []*api.Route, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*api.Route))
	})
	return ret, err
}

// Get retrieves the Route from the indexer for a given namespace and name.
func (s routeNamespaceLister) Get(name string) (*api.Route, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(api.Resource("route"), name)
	}
	return obj.(*api.Route), nil
}
