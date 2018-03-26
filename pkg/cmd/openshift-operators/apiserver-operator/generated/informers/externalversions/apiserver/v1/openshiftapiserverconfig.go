// This file was automatically generated by informer-gen

package v1

import (
	apiserver_v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/apis/apiserver/v1"
	versioned "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/generated/clientset/versioned"
	internalinterfaces "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/generated/informers/externalversions/internalinterfaces"
	v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/generated/listers/apiserver/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
	time "time"
)

// OpenShiftAPIServerConfigInformer provides access to a shared informer and lister for
// OpenShiftAPIServerConfigs.
type OpenShiftAPIServerConfigInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.OpenShiftAPIServerConfigLister
}

type openShiftAPIServerConfigInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewOpenShiftAPIServerConfigInformer constructs a new informer for OpenShiftAPIServerConfig type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewOpenShiftAPIServerConfigInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredOpenShiftAPIServerConfigInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredOpenShiftAPIServerConfigInformer constructs a new informer for OpenShiftAPIServerConfig type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredOpenShiftAPIServerConfigInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ApiserverV1().OpenShiftAPIServerConfigs().List(options)
			},
			WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ApiserverV1().OpenShiftAPIServerConfigs().Watch(options)
			},
		},
		&apiserver_v1.OpenShiftAPIServerConfig{},
		resyncPeriod,
		indexers,
	)
}

func (f *openShiftAPIServerConfigInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredOpenShiftAPIServerConfigInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *openShiftAPIServerConfigInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&apiserver_v1.OpenShiftAPIServerConfig{}, f.defaultInformer)
}

func (f *openShiftAPIServerConfigInformer) Lister() v1.OpenShiftAPIServerConfigLister {
	return v1.NewOpenShiftAPIServerConfigLister(f.Informer().GetIndexer())
}
