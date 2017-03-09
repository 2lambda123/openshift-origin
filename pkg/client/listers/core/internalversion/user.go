// This file was automatically generated by lister-gen with arguments: --input-dirs=[github.com/openshift/origin/pkg/authorization/api,github.com/openshift/origin/pkg/authorization/api/v1,github.com/openshift/origin/pkg/build/api,github.com/openshift/origin/pkg/build/api/v1,github.com/openshift/origin/pkg/deploy/api,github.com/openshift/origin/pkg/deploy/api/v1,github.com/openshift/origin/pkg/image/api,github.com/openshift/origin/pkg/image/api/v1,github.com/openshift/origin/pkg/oauth/api,github.com/openshift/origin/pkg/oauth/api/v1,github.com/openshift/origin/pkg/project/api,github.com/openshift/origin/pkg/project/api/v1,github.com/openshift/origin/pkg/quota/api,github.com/openshift/origin/pkg/quota/api/v1,github.com/openshift/origin/pkg/route/api,github.com/openshift/origin/pkg/route/api/v1,github.com/openshift/origin/pkg/sdn/api,github.com/openshift/origin/pkg/sdn/api/v1,github.com/openshift/origin/pkg/template/api,github.com/openshift/origin/pkg/template/api/v1,github.com/openshift/origin/pkg/user/api,github.com/openshift/origin/pkg/user/api/v1] --logtostderr=true

package internalversion

import (
	api "github.com/openshift/origin/pkg/user/api"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// UserLister helps list Users.
type UserLister interface {
	// List lists all Users in the indexer.
	List(selector labels.Selector) (ret []*api.User, err error)
	// Users returns an object that can list and get Users.
	Users(namespace string) UserNamespaceLister
	UserListerExpansion
}

// userLister implements the UserLister interface.
type userLister struct {
	indexer cache.Indexer
}

// NewUserLister returns a new UserLister.
func NewUserLister(indexer cache.Indexer) UserLister {
	return &userLister{indexer: indexer}
}

// List lists all Users in the indexer.
func (s *userLister) List(selector labels.Selector) (ret []*api.User, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*api.User))
	})
	return ret, err
}

// Users returns an object that can list and get Users.
func (s *userLister) Users(namespace string) UserNamespaceLister {
	return userNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// UserNamespaceLister helps list and get Users.
type UserNamespaceLister interface {
	// List lists all Users in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*api.User, err error)
	// Get retrieves the User from the indexer for a given namespace and name.
	Get(name string) (*api.User, error)
	UserNamespaceListerExpansion
}

// userNamespaceLister implements the UserNamespaceLister
// interface.
type userNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Users in the indexer for a given namespace.
func (s userNamespaceLister) List(selector labels.Selector) (ret []*api.User, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*api.User))
	})
	return ret, err
}

// Get retrieves the User from the indexer for a given namespace and name.
func (s userNamespaceLister) Get(name string) (*api.User, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(api.Resource("user"), name)
	}
	return obj.(*api.User), nil
}
