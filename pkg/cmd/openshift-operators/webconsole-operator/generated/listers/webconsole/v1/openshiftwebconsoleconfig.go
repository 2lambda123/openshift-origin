// This file was automatically generated by lister-gen

package v1

import (
	v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/apis/webconsole/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// OpenShiftWebConsoleConfigLister helps list OpenShiftWebConsoleConfigs.
type OpenShiftWebConsoleConfigLister interface {
	// List lists all OpenShiftWebConsoleConfigs in the indexer.
	List(selector labels.Selector) (ret []*v1.OpenShiftWebConsoleConfig, err error)
	// Get retrieves the OpenShiftWebConsoleConfig from the index for a given name.
	Get(name string) (*v1.OpenShiftWebConsoleConfig, error)
	OpenShiftWebConsoleConfigListerExpansion
}

// openShiftWebConsoleConfigLister implements the OpenShiftWebConsoleConfigLister interface.
type openShiftWebConsoleConfigLister struct {
	indexer cache.Indexer
}

// NewOpenShiftWebConsoleConfigLister returns a new OpenShiftWebConsoleConfigLister.
func NewOpenShiftWebConsoleConfigLister(indexer cache.Indexer) OpenShiftWebConsoleConfigLister {
	return &openShiftWebConsoleConfigLister{indexer: indexer}
}

// List lists all OpenShiftWebConsoleConfigs in the indexer.
func (s *openShiftWebConsoleConfigLister) List(selector labels.Selector) (ret []*v1.OpenShiftWebConsoleConfig, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.OpenShiftWebConsoleConfig))
	})
	return ret, err
}

// Get retrieves the OpenShiftWebConsoleConfig from the index for a given name.
func (s *openShiftWebConsoleConfigLister) Get(name string) (*v1.OpenShiftWebConsoleConfig, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("openshiftwebconsoleconfig"), name)
	}
	return obj.(*v1.OpenShiftWebConsoleConfig), nil
}
