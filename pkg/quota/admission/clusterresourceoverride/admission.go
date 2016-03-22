package clusterresourceoverride

import (
	"fmt"
	"io"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	"github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api/validation"
	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/plugin/pkg/admission/limitranger"

	"github.com/golang/glog"
	"speter.net/go/exp/math/dec/inf"
)

const (
	clusterResourceOverrideAnnotation = "quota.openshift.io/cluster-resource-override-enabled"
	cpuBaseScaleFactor                = 1000.0 / (1024.0 * 1024.0 * 1024.0) // 1000 milliCores per 1GiB
)

var (
	zeroDec = inf.NewDec(0, 0)
)

func init() {
	admission.RegisterPlugin(api.PluginName, func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
		return newClusterResourceOverride(client, config)
	})
}

type internalConfig struct {
	limitCPUToMemoryRatio     *inf.Dec
	cpuRequestToLimitRatio    *inf.Dec
	memoryRequestToLimitRatio *inf.Dec
}
type clusterResourceOverridePlugin struct {
	*admission.Handler
	config       *internalConfig
	ProjectCache *cache.ProjectCache
	LimitRanger  admission.Interface
}

var _ = oadmission.WantsProjectCache(&clusterResourceOverridePlugin{})
var _ = oadmission.Validator(&clusterResourceOverridePlugin{})

// newClusterResourceOverride returns an admission controller for containers that
// configurably overrides container resource request/limits
func newClusterResourceOverride(client clientset.Interface, config io.Reader) (admission.Interface, error) {
	parsed, err := ReadConfig(config)
	if err != nil {
		glog.V(5).Infof("%s admission controller loaded with error: (%T) %[2]v", api.PluginName, err)
		return nil, err
	}
	if errs := validation.Validate(parsed); len(errs) > 0 {
		return nil, errs.ToAggregate()
	}
	glog.V(5).Infof("%s admission controller loaded with config: %v", api.PluginName, parsed)
	var internal *internalConfig
	if parsed != nil {
		internal = &internalConfig{
			limitCPUToMemoryRatio:     inf.NewDec(parsed.LimitCPUToMemoryPercent, 2),
			cpuRequestToLimitRatio:    inf.NewDec(parsed.CPURequestToLimitPercent, 2),
			memoryRequestToLimitRatio: inf.NewDec(parsed.MemoryRequestToLimitPercent, 2),
		}
	}

	lrActions := &wrappedLimitRangerActions{
		internalLimitRangerActions: &limitranger.DefaultLimitRangerActions{},
	}
	limitRanger, err := limitranger.NewLimitRanger(client, lrActions)
	if err != nil {
		return nil, err
	}

	return &clusterResourceOverridePlugin{
		Handler:     admission.NewHandler(admission.Create),
		config:      internal,
		LimitRanger: limitRanger,
	}, nil
}

func (a *clusterResourceOverridePlugin) SetProjectCache(projectCache *cache.ProjectCache) {
	a.ProjectCache = projectCache
}

func ReadConfig(configFile io.Reader) (*api.ClusterResourceOverrideConfig, error) {
	obj, err := configlatest.ReadYAML(configFile)
	if err != nil {
		glog.V(5).Infof("%s error reading config: %v", api.PluginName, err)
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	config, ok := obj.(*api.ClusterResourceOverrideConfig)
	if !ok {
		return nil, fmt.Errorf("unexpected config object: %#v", obj)
	}
	glog.V(5).Infof("%s config is: %v", api.PluginName, config)
	return config, nil
}

func (a *clusterResourceOverridePlugin) Validate() error {
	if a.ProjectCache == nil {
		return fmt.Errorf("%s did not get a project cache", api.PluginName)
	}
	return nil
}

// TODO this will need to update when we have pod requests/limits
func (a *clusterResourceOverridePlugin) Admit(attr admission.Attributes) error {
	glog.V(6).Infof("%s admission controller is invoked", api.PluginName)
	if a.config == nil || !internalSupportsAttributes(attr) {
		return nil // not applicable
	}
	pod, ok := attr.GetObject().(*kapi.Pod)
	if !ok {
		return admission.NewForbidden(attr, fmt.Errorf("unexpected object: %#v", attr.GetObject()))
	}
	glog.V(5).Infof("%s is looking at creating pod %s in project %s", api.PluginName, pod.Name, attr.GetNamespace())

	// allow annotations on project to override
	if ns, err := a.ProjectCache.GetNamespace(attr.GetNamespace()); err != nil {
		glog.Warningf("%s got an error retrieving namespace: %v", api.PluginName, err)
		return admission.NewForbidden(attr, err) // this should not happen though
	} else {
		projectEnabledPlugin, exists := ns.Annotations[clusterResourceOverrideAnnotation]
		if exists && projectEnabledPlugin != "true" {
			glog.V(5).Infof("%s is disabled for project %s", api.PluginName, attr.GetNamespace())
			return nil // disabled for this project, do nothing
		}
	}

	// Reuse LimitRanger logic to apply limit/req defaults from the project. Ignore validation
	// errors, assume that LimitRanger will run after this plugin to validate.
	glog.V(5).Infof("%s: initial pod limits are: %#v", api.PluginName, pod.Spec.Containers[0].Resources)
	if err := a.LimitRanger.Admit(attr); err != nil {
		glog.V(5).Infof("%s: error from LimitRanger: %#v", api.PluginName, err)
	}
	glog.V(5).Infof("%s: pod limits after LimitRanger are: %#v", api.PluginName, pod.Spec.Containers[0].Resources)
	for _, container := range pod.Spec.Containers {
		resources := container.Resources
		memLimit, memFound := resources.Limits[kapi.ResourceMemory]
		if memFound && a.config.memoryRequestToLimitRatio.Cmp(zeroDec) != 0 {
			resources.Requests[kapi.ResourceMemory] = resource.Quantity{
				Amount: multiply(memLimit.Amount, a.config.memoryRequestToLimitRatio),
				Format: resource.BinarySI,
			}
		}
		if memFound && a.config.limitCPUToMemoryRatio.Cmp(zeroDec) != 0 {
			resources.Limits[kapi.ResourceCPU] = resource.Quantity{
				// float math is necessary here as there is no way to create an inf.Dec to represent cpuBaseScaleFactor < 0.001
				Amount: multiply(inf.NewDec(int64(float64(memLimit.Value())*cpuBaseScaleFactor), 3), a.config.limitCPUToMemoryRatio),
				Format: resource.DecimalSI,
			}
		}
		cpuLimit, cpuFound := resources.Limits[kapi.ResourceCPU]
		if cpuFound && a.config.cpuRequestToLimitRatio.Cmp(zeroDec) != 0 {
			resources.Requests[kapi.ResourceCPU] = resource.Quantity{
				Amount: multiply(cpuLimit.Amount, a.config.cpuRequestToLimitRatio),
				Format: resource.DecimalSI,
			}
		}
	}
	glog.V(5).Infof("%s: pod limits after overrides are: %#v", api.PluginName, pod.Spec.Containers[0].Resources)
	return nil
}

func multiply(x *inf.Dec, y *inf.Dec) *inf.Dec {
	return inf.NewDec(0, 0).Mul(x, y)
}

// wrappedLimitRangerActions wraps the default LimitRangerActions.
type wrappedLimitRangerActions struct {
	internalLimitRangerActions limitranger.LimitRangerActions
}

// wrappedLimitRangerActions implements the LimitRangerActions interface
var _ limitranger.LimitRangerActions = &wrappedLimitRangerActions{}

// Limit is a pluggable function to enforce limits on the object.  This wraps the default
// implementation to always return success so that all defaults will be applied.  Validation will
// occur after the overrides.  Implements the LimitRangerActions interface.
func (lr *wrappedLimitRangerActions) Limit(limitRange *kapi.LimitRange, kind string, obj runtime.Object) error {
	lr.internalLimitRangerActions.Limit(limitRange, kind, obj)
	return nil
}

// SupportsLimit provides a check to see if the limitRange is applicable.
// Implements the LimitRangerActions interface.
func (lr *wrappedLimitRangerActions) SupportsLimit(limitRange *kapi.LimitRange) bool {
	return lr.internalLimitRangerActions.SupportsLimit(limitRange)
}

// SupportsAttributes is a helper that returns true if the resource is supported by the plugin.
// Implements the LimitRangerActions interface.
func (lr *wrappedLimitRangerActions) SupportsAttributes(attr admission.Attributes) bool {
	return internalSupportsAttributes(attr)
}

// internalSupportsAttributes is factored out for usage outside the actions object.
func internalSupportsAttributes(attr admission.Attributes) bool {
	if attr.GetResource() != kapi.Resource("pods") || attr.GetSubresource() != "" {
		return false // not applicable
	}
	return true
}
