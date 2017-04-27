package restoptions

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/storage/storagebackend/factory"

	"github.com/golang/glog"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
)

// UseConfiguredCacheSize indicates that the configured cache size should be used
const UseConfiguredCacheSize = -1

// configRESTOptionsGetter provides RESTOptions based on a provided config
type configRESTOptionsGetter struct {
	masterOptions configapi.MasterConfig

	restOptionsLock sync.Mutex
	restOptionsMap  map[schema.GroupResource]generic.RESTOptions

	storageFactory        serverstorage.StorageFactory
	defaultResourceConfig *serverstorage.ResourceConfig

	cacheEnabled     bool
	defaultCacheSize int
	cacheSizes       map[schema.GroupResource]int
	quorumResources  map[schema.GroupResource]struct{}
}

// NewConfigGetter returns a restoptions.Getter implemented using information from the provided master config.
// By default, the etcd watch cache is enabled with a size of 1000 per resource type.
// TODO: this class should either not need to know about configapi.MasterConfig, or not be in pkg/util
func NewConfigGetter(masterOptions configapi.MasterConfig, defaultResourceConfig *serverstorage.ResourceConfig, resourcePrefixOverrides map[schema.GroupResource]string, enforcedStorageVersions []schema.GroupVersionResource, quorumResources map[schema.GroupResource]struct{}) (Getter, error) {
	apiserverOptions, err := kubernetes.BuildKubeAPIserverOptions(masterOptions)
	if err != nil {
		return nil, err
	}
	storageFactory, err := kubernetes.BuildStorageFactory(masterOptions, apiserverOptions, enforcedStorageVersions)
	if err != nil {
		return nil, err
	}
	storageFactory.DefaultResourcePrefixes = resourcePrefixOverrides
	storageFactory.StorageConfig.Prefix = masterOptions.EtcdStorageConfig.OpenShiftStoragePrefix

	// TODO: refactor vendor/k8s.io/kubernetes/pkg/registry/cachesize to remove our custom cache size code
	errs := []error{}
	cacheSizes := map[schema.GroupResource]int{}
	for _, c := range apiserverOptions.GenericServerRunOptions.WatchCacheSizes {
		tokens := strings.Split(c, "#")
		if len(tokens) != 2 {
			errs = append(errs, fmt.Errorf("invalid watch cache size value '%s', expecting <resource>#<size> format (e.g. builds#100)", c))
			continue
		}

		resource := schema.ParseGroupResource(tokens[0])

		size, err := strconv.Atoi(tokens[1])
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid watch cache size value '%s': %v", c, err))
			continue
		}
		cacheSizes[resource] = size
	}
	if len(errs) > 0 {
		return nil, kerrors.NewAggregate(errs)
	}

	return &configRESTOptionsGetter{
		masterOptions:         masterOptions,
		cacheEnabled:          apiserverOptions.Etcd.EnableWatchCache,
		defaultCacheSize:      1000,
		cacheSizes:            cacheSizes,
		restOptionsMap:        map[schema.GroupResource]generic.RESTOptions{},
		defaultResourceConfig: defaultResourceConfig,
		quorumResources:       quorumResources,
		storageFactory:        storageFactory,
	}, nil
}

func (g *configRESTOptionsGetter) GetRESTOptions(resource schema.GroupResource) (generic.RESTOptions, error) {
	g.restOptionsLock.Lock()
	defer g.restOptionsLock.Unlock()
	if resourceOptions, ok := g.restOptionsMap[resource]; ok {
		return resourceOptions, nil
	}

	config, err := g.storageFactory.NewConfig(resource)
	if err != nil {
		return generic.RESTOptions{}, err
	}

	if _, ok := g.quorumResources[resource]; ok {
		config.Quorum = true
	}

	configuredCacheSize, specified := g.cacheSizes[resource]
	if !specified || configuredCacheSize < 0 {
		configuredCacheSize = g.defaultCacheSize
	}

	decorator := func(
		copier runtime.ObjectCopier,
		storageConfig *storagebackend.Config,
		requestedSize int,
		objectType runtime.Object,
		resourcePrefix string,
		keyFunc func(obj runtime.Object) (string, error),
		newListFn func() runtime.Object,
		getAttrsFunc storage.AttrFunc,
		triggerFn storage.TriggerPublisherFunc,
	) (storage.Interface, factory.DestroyFunc) {
		capacity := requestedSize
		if capacity == UseConfiguredCacheSize {
			capacity = configuredCacheSize
		}

		if capacity == 0 || !g.cacheEnabled {
			glog.V(5).Infof("using uncached watch storage for %s", resource.String())
			return generic.UndecoratedStorage(copier, storageConfig, capacity, objectType, resourcePrefix, keyFunc, newListFn, getAttrsFunc, triggerFn)
		}

		glog.V(5).Infof("using watch cache storage (capacity=%d) for %s %#v", capacity, resource.String(), storageConfig)
		return registry.StorageWithCacher(copier, storageConfig, capacity, objectType, resourcePrefix, keyFunc, newListFn, getAttrsFunc, triggerFn)
	}

	resourceOptions := generic.RESTOptions{
		StorageConfig:           config,
		Decorator:               decorator,
		DeleteCollectionWorkers: 1,     // TODO(rebase): use upstream value here from Etcd options?
		EnableGarbageCollection: false, // TODO(rebase): use upstream value here from Etcd options?
		ResourcePrefix:          g.storageFactory.ResourcePrefix(resource),
	}
	g.restOptionsMap[resource] = resourceOptions

	return resourceOptions, nil
}
