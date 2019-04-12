package build

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/containers/image/signature"
	toml "github.com/pelletier/go-toml"
	"k8s.io/klog"

	"github.com/openshift/origin/pkg/build/buildapihelpers"
	metrics "github.com/openshift/origin/pkg/build/metrics/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	ktypedclient "k8s.io/client-go/kubernetes/typed/core/v1"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	buildv1 "github.com/openshift/api/build/v1"
	configv1 "github.com/openshift/api/config/v1"
	imagev1 "github.com/openshift/api/image/v1"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned"
	buildv1informer "github.com/openshift/client-go/build/informers/externalversions/build/v1"
	buildv1lister "github.com/openshift/client-go/build/listers/build/v1"
	configv1informer "github.com/openshift/client-go/config/informers/externalversions/config/v1"
	configv1lister "github.com/openshift/client-go/config/listers/config/v1"
	imagev1informer "github.com/openshift/client-go/image/informers/externalversions/image/v1"
	imagev1lister "github.com/openshift/client-go/image/listers/image/v1"
	"github.com/openshift/origin/pkg/api/imagereferencemutators"
	"github.com/openshift/origin/pkg/build/buildscheme"
	buildmanualclient "github.com/openshift/origin/pkg/build/client"
	builddefaults "github.com/openshift/origin/pkg/build/controller/build/defaults"
	buildoverrides "github.com/openshift/origin/pkg/build/controller/build/overrides"
	"github.com/openshift/origin/pkg/build/controller/common"
	"github.com/openshift/origin/pkg/build/controller/policy"
	"github.com/openshift/origin/pkg/build/controller/strategy"
	buildutil "github.com/openshift/origin/pkg/build/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageutil "github.com/openshift/origin/pkg/image/util"
)

const (
	maxRetries = 15

	// maxExcerptLength is the maximum length of the LogSnippet on a build.
	maxExcerptLength = 5
)

// resourceTriggerQueue tracks a set of resource keys to trigger when another object changes.
type resourceTriggerQueue struct {
	lock  sync.Mutex
	queue map[string][]string
}

// newResourceTriggerQueue creates a resourceTriggerQueue.
func newResourceTriggerQueue() *resourceTriggerQueue {
	return &resourceTriggerQueue{
		queue: make(map[string][]string),
	}
}

// Add ensures resource will be returned the next time any of on are invoked
// on Pop().
func (q *resourceTriggerQueue) Add(resource string, on []string) {
	q.lock.Lock()
	defer q.lock.Unlock()
	for _, key := range on {
		q.queue[key] = append(q.queue[key], resource)
	}
}

func (q *resourceTriggerQueue) Remove(resource string, on []string) {
	q.lock.Lock()
	defer q.lock.Unlock()
	for _, key := range on {
		resources := q.queue[key]
		newResources := make([]string, 0, len(resources))
		for _, existing := range resources {
			if existing == resource {
				continue
			}
			newResources = append(newResources, existing)
		}
		q.queue[key] = newResources
	}
}

func (q *resourceTriggerQueue) Pop(key string) []string {
	q.lock.Lock()
	defer q.lock.Unlock()
	resources := q.queue[key]
	delete(q.queue, key)
	return resources
}

type registryList struct {
	Registries []string `toml:"registries"`
}

type registries struct {
	Search   registryList `toml:"search"`
	Insecure registryList `toml:"insecure"`
	Block    registryList `toml:"block"`
}

type tomlConfig struct {
	Registries registries `toml:"registries"`
}

// BuildController watches builds and synchronizes them with their
// corresponding build pods. It is also responsible for resolving image
// stream references in the Build to docker images prior to invoking the pod.
// The build controller late binds image stream references so that users can
// create a build config before they create the image stream (or before
// an image is pushed to a tag) which allows actions to converge. It also
// allows multiple Build objects to directly reference images created by another
// Build, acting as a simple dependency graph for a logical multi-image build
// that reuses many individual Builds.
//
// Like other controllers that do "on behalf of" image resolution, the controller
// resolves the reference which allows users to see what image ID corresponds to a tag
// simply by requesting resolution. This is consistent with other image policy in the
// system (image tag references in deployments, triggers, and image policy). The only
// leaked image information is the ID which is considered acceptable. Secrets are also
// resolved, allowing a user in the same namespace to in theory infer the presence of
// a secret or make it usable by a build - but this is identical to our existing model
// where a service account determines access to secrets used in pods.
type BuildController struct {
	buildPatcher                buildmanualclient.BuildPatcher
	buildLister                 buildv1lister.BuildLister
	buildConfigGetter           buildv1lister.BuildConfigLister
	buildDeleter                buildmanualclient.BuildDeleter
	buildControllerConfigLister configv1lister.BuildLister
	imageConfigLister           configv1lister.ImageLister
	podClient                   ktypedclient.PodsGetter
	configMapClient             ktypedclient.ConfigMapsGetter
	kubeClient                  kubernetes.Interface

	buildQueue            workqueue.RateLimitingInterface
	podQueue              workqueue.RateLimitingInterface
	imageStreamQueue      *resourceTriggerQueue
	buildConfigQueue      workqueue.RateLimitingInterface
	controllerConfigQueue workqueue.RateLimitingInterface

	buildStore                    buildv1lister.BuildLister
	secretStore                   v1lister.SecretLister
	podStore                      v1lister.PodLister
	imageStreamStore              imagev1lister.ImageStreamLister
	openShiftConfigConfigMapStore v1lister.ConfigMapLister

	podInformer   cache.SharedIndexInformer
	buildInformer cache.SharedIndexInformer

	buildStoreSynced                 cache.InformerSynced
	buildControllerConfigStoreSynced cache.InformerSynced
	imageConfigStoreSynced           cache.InformerSynced
	podStoreSynced                   cache.InformerSynced
	secretStoreSynced                cache.InformerSynced
	imageStreamStoreSynced           cache.InformerSynced
	configMapStoreSynced             cache.InformerSynced

	runPolicies              []policy.RunPolicy
	createStrategy           buildPodCreationStrategy
	buildDefaults            builddefaults.BuildDefaults
	buildOverrides           buildoverrides.BuildOverrides
	internalRegistryHostname string

	recorder                record.EventRecorder
	registryConfData        string
	signaturePolicyData     string
	additionalTrustedCAData map[string]string
}

// BuildControllerParams is the set of parameters needed to
// create a new BuildController
type BuildControllerParams struct {
	BuildInformer                    buildv1informer.BuildInformer
	BuildConfigInformer              buildv1informer.BuildConfigInformer
	BuildControllerConfigInformer    configv1informer.BuildInformer
	ImageConfigInformer              configv1informer.ImageInformer
	ImageStreamInformer              imagev1informer.ImageStreamInformer
	PodInformer                      kubeinformers.PodInformer
	SecretInformer                   kubeinformers.SecretInformer
	OpenshiftConfigConfigMapInformer kubeinformers.ConfigMapInformer
	KubeClient                       kubernetes.Interface
	BuildClient                      buildv1client.Interface
	DockerBuildStrategy              *strategy.DockerBuildStrategy
	SourceBuildStrategy              *strategy.SourceBuildStrategy
	CustomBuildStrategy              *strategy.CustomBuildStrategy
	BuildDefaults                    builddefaults.BuildDefaults
	BuildOverrides                   buildoverrides.BuildOverrides
	InternalRegistryHostname         string
}

// NewBuildController creates a new BuildController.
func NewBuildController(params *BuildControllerParams) *BuildController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&ktypedclient.EventSinkImpl{Interface: params.KubeClient.CoreV1().Events("")})

	buildClient := buildmanualclient.NewClientBuildClient(params.BuildClient)
	buildLister := params.BuildInformer.Lister()
	buildConfigGetter := params.BuildConfigInformer.Lister()
	c := &BuildController{
		buildPatcher:                  buildClient,
		buildLister:                   buildLister,
		buildConfigGetter:             buildConfigGetter,
		buildDeleter:                  buildClient,
		buildControllerConfigLister:   params.BuildControllerConfigInformer.Lister(),
		imageConfigLister:             params.ImageConfigInformer.Lister(),
		secretStore:                   params.SecretInformer.Lister(),
		podClient:                     params.KubeClient.CoreV1(),
		configMapClient:               params.KubeClient.CoreV1(),
		openShiftConfigConfigMapStore: params.OpenshiftConfigConfigMapInformer.Lister(),
		kubeClient:                    params.KubeClient,
		podInformer:                   params.PodInformer.Informer(),
		podStore:                      params.PodInformer.Lister(),
		buildInformer:                 params.BuildInformer.Informer(),
		buildStore:                    params.BuildInformer.Lister(),
		imageStreamStore:              params.ImageStreamInformer.Lister(),
		createStrategy: &typeBasedFactoryStrategy{
			dockerBuildStrategy: params.DockerBuildStrategy,
			sourceBuildStrategy: params.SourceBuildStrategy,
			customBuildStrategy: params.CustomBuildStrategy,
		},
		buildDefaults:            params.BuildDefaults,
		buildOverrides:           params.BuildOverrides,
		internalRegistryHostname: params.InternalRegistryHostname,

		buildQueue:            workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		podQueue:              workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		imageStreamQueue:      newResourceTriggerQueue(),
		buildConfigQueue:      workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		controllerConfigQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		recorder:    eventBroadcaster.NewRecorder(buildscheme.EncoderScheme, corev1.EventSource{Component: "build-controller"}),
		runPolicies: policy.GetAllRunPolicies(buildLister, buildClient),
	}

	c.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.podAdded,
		UpdateFunc: c.podUpdated,
		DeleteFunc: c.podDeleted,
	})
	c.buildInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.buildAdded,
		UpdateFunc: c.buildUpdated,
		DeleteFunc: c.buildDeleted,
	})
	params.ImageStreamInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.imageStreamAdded,
		UpdateFunc: c.imageStreamUpdated,
	})
	params.BuildControllerConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.buildControllerConfigAdded,
		UpdateFunc: c.buildControllerConfigUpdated,
		DeleteFunc: c.buildControllerConfigDeleted,
	})
	params.ImageConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.imageConfigAdded,
		UpdateFunc: c.imageConfigUpdated,
		DeleteFunc: c.imageConfigDeleted,
	})
	params.OpenshiftConfigConfigMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.configMapAdded,
		UpdateFunc: c.configMapUpdated,
		DeleteFunc: c.configMapDeleted,
	})

	c.buildStoreSynced = c.buildInformer.HasSynced
	c.podStoreSynced = c.podInformer.HasSynced
	c.secretStoreSynced = params.SecretInformer.Informer().HasSynced
	c.imageStreamStoreSynced = params.ImageStreamInformer.Informer().HasSynced
	c.buildControllerConfigStoreSynced = params.BuildControllerConfigInformer.Informer().HasSynced
	c.imageConfigStoreSynced = params.ImageConfigInformer.Informer().HasSynced
	c.configMapStoreSynced = params.OpenshiftConfigConfigMapInformer.Informer().HasSynced

	return c
}

// Run begins watching and syncing.
func (bc *BuildController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer bc.buildQueue.ShutDown()
	defer bc.podQueue.ShutDown()
	defer bc.buildConfigQueue.ShutDown()
	defer bc.controllerConfigQueue.ShutDown()

	// Wait for the controller stores to sync before starting any work in this controller.
	if !cache.WaitForCacheSync(stopCh,
		bc.buildStoreSynced,
		bc.podStoreSynced,
		bc.secretStoreSynced,
		bc.imageStreamStoreSynced,
		bc.configMapStoreSynced) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	// Integration tests currently do not support cache sync for operator-installed custom resource definitions
	if os.Getenv("OS_INTEGRATION_TEST") != "true" {
		if !cache.WaitForCacheSync(stopCh,
			bc.buildControllerConfigStoreSynced,
			bc.imageConfigStoreSynced) {
			utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
			return
		}
	}

	// Sync build controller config on start up.
	if errs := bc.handleControllerConfig(); len(errs) > 0 {
		utilruntime.HandleError(fmt.Errorf("errors syncing build controller config: %v", errs))
	}

	klog.Infof("Starting build controller")

	// Only need one worker to manage config
	go wait.Until(bc.controllerConfigWorker, time.Second, stopCh)

	for i := 0; i < workers; i++ {
		go wait.Until(bc.buildWorker, time.Second, stopCh)
	}

	for i := 0; i < workers; i++ {
		go wait.Until(bc.buildConfigWorker, time.Second, stopCh)
	}

	for i := 0; i < workers; i++ {
		go wait.Until(bc.podWorker, time.Second, stopCh)
	}

	metrics.IntializeMetricsCollector(bc.buildLister)

	<-stopCh
	klog.Infof("Shutting down build controller")
}

func (bc *BuildController) podWorker() {
	for {
		if quit := bc.podWork(); quit {
			return
		}
	}
}

func (bc *BuildController) podWork() bool {
	obj, quit := bc.podQueue.Get()
	if quit {
		return true
	}
	defer bc.podQueue.Done(obj)
	pod := obj.(*corev1.Pod)
	err := bc.handlePod(pod)
	bc.handleQueueError(err, pod, bc.podQueue)
	return false
}

func (bc *BuildController) buildWorker() {
	for {
		if quit := bc.buildWork(); quit {
			return
		}
	}
}

// buildWork gets the next build from the buildQueue and invokes handleBuild on it
func (bc *BuildController) buildWork() bool {
	key, quit := bc.buildQueue.Get()
	if quit {
		return true
	}

	defer bc.buildQueue.Done(key)

	build, err := bc.getBuildByKey(key.(string))
	if err != nil {
		bc.handleQueueError(err, key, bc.buildQueue)
		return false
	}
	if build == nil {
		return false
	}

	err = bc.handleBuild(build)
	bc.handleQueueError(err, key, bc.buildConfigQueue)
	return false
}

func (bc *BuildController) buildConfigWorker() {
	for {
		if quit := bc.buildConfigWork(); quit {
			return
		}
	}
}

// buildConfigWork gets the next build config from the buildConfigQueue and invokes handleBuildConfig on it
func (bc *BuildController) buildConfigWork() bool {
	key, quit := bc.buildConfigQueue.Get()
	if quit {
		return true
	}
	defer bc.buildConfigQueue.Done(key)

	namespace, name, err := parseBuildConfigKey(key.(string))
	if err != nil {
		utilruntime.HandleError(err)
		return false
	}

	err = bc.handleBuildConfig(namespace, name)
	bc.handleQueueError(err, key, bc.buildConfigQueue)
	return false
}

func parseBuildConfigKey(key string) (string, string, error) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid build config key: %s", key)
	}
	return parts[0], parts[1], nil
}

func (bc *BuildController) handlePod(pod *corev1.Pod) error {
	var update *buildUpdate
	var err, updateErr error
	var build *buildv1.Build

	build, err = bc.getBuildByKey(resourceName(pod.Namespace, getBuildName(pod)))
	if err != nil {
		return err
	}
	if shouldIgnore(build) {
		return nil
	}

	switch {
	case shouldCancel(build):
		update, err = bc.cancelBuild(build)
	case build.Status.Phase == buildv1.BuildPhaseNew:
		update, err = bc.postPodCreateSetupCfgMapSecretAnnotation(build, pod)
	default:
		klog.V(0).Infof("GGM have non new build %#v for presumably added pod %#v", build, pod)
		return nil
	}
	if update != nil && !update.isEmpty() {
		updateErr = bc.updateBuild(build, update, pod)
	}
	if err != nil {
		return err
	}
	if updateErr != nil {
		return updateErr
	}
	return nil
}

// handleBuild retrieves the build's corresponding pod and calls the appropriate
// handle function based on the build's current state. Each handler returns a buildUpdate
// object that includes any updates that need to be made on the build.
func (bc *BuildController) handleBuild(build *buildv1.Build) error {

	// If pipeline build, handle pruning.
	if build.Spec.Strategy.JenkinsPipelineStrategy != nil {
		if buildutil.IsBuildComplete(build) {
			if err := common.HandleBuildPruning(buildutil.ConfigNameForBuild(build), build.Namespace, bc.buildLister, bc.buildConfigGetter, bc.buildDeleter); err != nil {
				utilruntime.HandleError(fmt.Errorf("failed to prune builds for %s/%s: %v", build.Namespace, build.Name, err))
			}
		}
	}

	if shouldIgnore(build) {
		return nil
	}

	klog.V(4).Infof("Handling build %s", buildDesc(build))

	pod, podErr := bc.podStore.Pods(build.Namespace).Get(buildapihelpers.GetBuildPodName(build))

	// Technically the only error that is returned from retrieving the pod is the
	// NotFound error so this check should not be needed, but leaving here in case
	// that changes in the future.
	if podErr != nil && !errors.IsNotFound(podErr) {
		return podErr
	}

	var update *buildUpdate
	var err, updateErr error

	switch {
	case shouldCancel(build):
		update, err = bc.cancelBuild(build)
	case build.Status.Phase == buildv1.BuildPhaseNew:
		update, err = bc.handleNewBuild(build, pod)
	case build.Status.Phase == buildv1.BuildPhasePending,
		build.Status.Phase == buildv1.BuildPhaseRunning:
		update, err = bc.handleActiveBuild(build, pod)
	case buildutil.IsBuildComplete(build):
		update, err = bc.handleCompletedBuild(build, pod)
	}
	if update != nil && !update.isEmpty() {
		updateErr = bc.updateBuild(build, update, pod)
	}
	if err != nil {
		return err
	}
	if updateErr != nil {
		return updateErr
	}
	return nil
}

// shouldIgnore returns true if a build should be ignored by the controller.
// These include pipeline builds as well as builds that are in a terminal state.
// However if the build is either complete or failed and its completion timestamp
// has not been set, then it returns false so that the build's completion timestamp
// gets updated.
func shouldIgnore(build *buildv1.Build) bool {
	// If pipeline build, do nothing.
	// These builds are processed/updated/etc by the jenkins sync plugin
	if build.Spec.Strategy.JenkinsPipelineStrategy != nil {
		klog.V(4).Infof("Ignoring build %s with jenkins pipeline strategy", buildDesc(build))
		return true
	}

	// If a build is in a terminal state, ignore it; unless it is in a succeeded or failed
	// state and its completion time or logsnippet is not set, then we should at least attempt to set its
	// completion time and logsnippet if possible because the build pod may have put the build in
	// this state and it would have not set the completion timestamp or logsnippet data.
	if buildutil.IsBuildComplete(build) {
		switch build.Status.Phase {
		case buildv1.BuildPhaseComplete:
			if build.Status.CompletionTimestamp == nil {
				return false
			}
		case buildv1.BuildPhaseFailed:
			if build.Status.CompletionTimestamp == nil || len(build.Status.LogSnippet) == 0 {
				return false
			}
		}
		klog.V(4).Infof("Ignoring build %s in completed state", buildDesc(build))
		return true
	}

	return false
}

// shouldCancel returns true if a build is active and its cancellation flag is set
func shouldCancel(build *buildv1.Build) bool {
	return !buildutil.IsBuildComplete(build) && build.Status.Cancelled
}

// cancelBuild deletes a build pod and returns an update to mark the build as cancelled
func (bc *BuildController) cancelBuild(build *buildv1.Build) (*buildUpdate, error) {
	klog.V(4).Infof("Cancelling build %s", buildDesc(build))

	podName := buildapihelpers.GetBuildPodName(build)
	err := bc.podClient.Pods(build.Namespace).Delete(podName, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("could not delete build pod %s/%s to cancel build %s: %v", build.Namespace, podName, buildDesc(build), err)
	}

	return transitionToPhase(buildv1.BuildPhaseCancelled, buildv1.StatusReasonCancelledBuild, buildutil.StatusMessageCancelledBuild), nil
}

// handleNewBuild will check whether policy allows running the new build and if so, creates a pod
// for the build and returns an update to move it to the Pending phase
func (bc *BuildController) handleNewBuild(build *buildv1.Build, pod *corev1.Pod) (*buildUpdate, error) {
	if pod != nil {
		bc.recorder.Eventf(build, corev1.EventTypeWarning, "PossibleDuplicateLeader", "Pod already exists: %s/%s", pod.Namespace, pod.Name)
		// We're in phase New and a build pod already exists.
		// now it used to be, if the pod has an
		// owner reference to the build, we take that to mean that we created
		// the pod but failed to update the build object afterwards.

		// However, now we only update the build as part of this controller's attempt to
		// create the pod if the pod creation failed, including IsAlreadyExists
		// Both result in the status.reason/message fields getting set and now should set the phase
		// to buildv1.BuildPhaseError instead of the new phase
		// Then we update the build push secret, phase, output ref, and pod annotation as needed
		// once we get the pod added event

		// So if we get here, either
		// a) one those setup errors occurred, and we failed to patch update the build, in which case
		// we now log an error in the build controller log and attempt to generate an event
		// b) a duplicate leader scenario is in play, and we have multiple creaters of pods / writers
		// of builds
		// c) duplicate events / relist ?!?!

		if strategy.HasOwnerReference(pod, build) {
			return bc.handleActiveBuild(build, pod)
		}
		// If a pod was not created by the current build, move the build to
		// error.
		return transitionToPhase(buildv1.BuildPhaseError, buildv1.StatusReasonBuildPodExists, buildutil.StatusMessageBuildPodExists), nil
	}

	runPolicy := policy.ForBuild(build, bc.runPolicies)
	if runPolicy == nil {
		return nil, fmt.Errorf("unable to determine build policy for %s", buildDesc(build))
	}

	// The runPolicy decides whether to execute this build or not.
	if run, err := runPolicy.IsRunnable(build); err != nil || !run {
		return nil, err
	}

	return bc.createBuildPod(build)
}

// createPodSpec creates a pod spec for the given build, with all references already resolved.
func (bc *BuildController) createPodSpec(build *buildv1.Build, includeAdditionalCA bool) (*corev1.Pod, error) {
	if build.Spec.Output.To != nil {
		build.Status.OutputDockerImageReference = build.Spec.Output.To.Name
	}

	// ensure the build object the pod sees starts with a clean set of reasons/messages,
	// rather than inheriting the potential "invalidoutputreference" message from the current
	// build state.  Otherwise when the pod attempts to update the build (e.g. with the git
	// revision information), it will re-assert the stale reason/message.
	build.Status.Reason = ""
	build.Status.Message = ""

	// Invoke the strategy to create a build pod.
	podSpec, err := bc.createStrategy.CreateBuildPod(build, bc.additionalTrustedCAData, bc.internalRegistryHostname)
	if err != nil {
		if strategy.IsFatal(err) {
			return nil, &strategy.FatalError{Reason: fmt.Sprintf("failed to create a build pod spec for build %s/%s: %v", build.Namespace, build.Name, err)}
		}
		return nil, fmt.Errorf("failed to create a build pod spec for build %s/%s: %v", build.Namespace, build.Name, err)
	}
	if err := bc.buildDefaults.ApplyDefaults(podSpec); err != nil {
		return nil, fmt.Errorf("failed to apply build defaults for build %s/%s: %v", build.Namespace, build.Name, err)
	}
	if err := bc.buildOverrides.ApplyOverrides(podSpec); err != nil {
		return nil, fmt.Errorf("failed to apply build overrides for build %s/%s: %v", build.Namespace, build.Name, err)
	}

	// Handle resolving ValueFrom references in build environment variables
	if err := common.ResolveValueFrom(podSpec, bc.kubeClient); err != nil {
		return nil, err
	}
	return podSpec, nil
}

// resolveImageSecretAsReference returns a LocalObjectReference to a secret that should
// be able to push/pull at the image location.
// Note that we are using controller level permissions to resolve the secret,
// meaning users could theoretically define a build that references an imagestream they cannot
// see, and 1) get the docker image reference of that imagestream and 2) a reference to a secret
// associated with a service account that can push to that location.  However they still cannot view the secret,
// and ability to use a service account implies access to its secrets, so this is considered safe.
// Furthermore it's necessary to enable triggered builds since a triggered build is not "requested"
// by a particular user, so there are no user permissions to validate against in that case.
func (bc *BuildController) resolveImageSecretAsReference(build *buildv1.Build, imagename string) (*corev1.LocalObjectReference, error) {
	serviceAccount := build.Spec.ServiceAccount
	if len(serviceAccount) == 0 {
		serviceAccount = buildutil.BuilderServiceAccountName
	}
	builderSecrets, err := buildutil.FetchServiceAccountSecrets(bc.kubeClient.CoreV1(), build.Namespace, serviceAccount)
	if err != nil {
		return nil, fmt.Errorf("Error getting push/pull secrets for service account %s/%s: %v", build.Namespace, serviceAccount, err)
	}
	var secret *corev1.LocalObjectReference
	if len(imagename) != 0 {
		secret = buildutil.FindDockerSecretAsReference(builderSecrets, imagename)
	}
	if secret == nil {
		klog.V(4).Infof("build %s is referencing an unknown image, will attempt to use the default secret for the service account", build.Name)
		dockerSecretExists := false
		for _, builderSecret := range builderSecrets {
			if builderSecret.Type == corev1.SecretTypeDockercfg || builderSecret.Type == corev1.SecretTypeDockerConfigJson {
				dockerSecretExists = true
				secret = &corev1.LocalObjectReference{Name: builderSecret.Name}
				break
			}
		}
		// If there are no docker secrets associated w/ the service account, return an error so the build
		// will be retried.  The secrets will be created shortly.
		if !dockerSecretExists {
			return nil, fmt.Errorf("No docker secrets associated with build service account %s", serviceAccount)
		}
		klog.V(4).Infof("No secrets found for pushing or pulling image named %s for build, using default: %s %s/%s", imagename, build.Namespace, build.Name, secret.Name)
	}
	return secret, nil
}

// resourceName creates a string that can be used to uniquely key the provided resource.
func resourceName(namespace, name string) string {
	return namespace + "/" + name
}

var (
	// errInvalidImageReferences is a marker error for when a build contains invalid object
	// reference names.
	errInvalidImageReferences = fmt.Errorf("one or more image references were invalid")
	// errNoIntegratedRegistry is a marker error for when the output image points to a registry
	// that cannot be resolved.
	errNoIntegratedRegistry = fmt.Errorf("the integrated registry is not configured")
)

// unresolvedImageStreamReferences finds all image stream references in the provided
// mutator that need to be resolved prior to the resource being accepted and returns
// them as an array of "namespace/name" strings. If any references are invalid, an error
// is returned.
func unresolvedImageStreamReferences(m imagereferencemutators.ImageReferenceMutator, defaultNamespace string) ([]string, error) {
	var streams []string
	fn := func(ref *corev1.ObjectReference) error {
		switch ref.Kind {
		case "ImageStreamImage":
			namespace := ref.Namespace
			if len(namespace) == 0 {
				namespace = defaultNamespace
			}
			name, _, ok := imageapi.SplitImageStreamImage(ref.Name)
			if !ok {
				return errInvalidImageReferences
			}
			streams = append(streams, resourceName(namespace, name))
		case "ImageStreamTag":
			namespace := ref.Namespace
			if len(namespace) == 0 {
				namespace = defaultNamespace
			}
			name, _, ok := imageapi.SplitImageStreamTag(ref.Name)
			if !ok {
				return errInvalidImageReferences
			}
			streams = append(streams, resourceName(namespace, name))
		}
		return nil
	}
	errs := m.Mutate(fn)
	if len(errs) > 0 {
		return nil, errInvalidImageReferences
	}
	return streams, nil
}

// resolveImageStreamLocation transforms the provided reference into a string pointing to the integrated registry,
// or returns an error.
func resolveImageStreamLocation(ref *corev1.ObjectReference, lister imagev1lister.ImageStreamLister, defaultNamespace string) (string, error) {
	namespace := ref.Namespace
	if len(namespace) == 0 {
		namespace = defaultNamespace
	}

	var (
		name string
		tag  string
	)
	switch ref.Kind {
	case "ImageStreamImage":
		var ok bool
		name, _, ok = imageapi.SplitImageStreamImage(ref.Name)
		if !ok {
			return "", errInvalidImageReferences
		}
		// for backwards compatibility, image stream images will be resolved to the :latest tag
		tag = imageapi.DefaultImageTag
	case "ImageStreamTag":
		var ok bool
		name, tag, ok = imageapi.SplitImageStreamTag(ref.Name)
		if !ok {
			return "", errInvalidImageReferences
		}
	case "ImageStream":
		name = ref.Name
	}

	stream, err := lister.ImageStreams(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return "", err
		}
		return "", fmt.Errorf("the referenced output image stream %s/%s could not be found: %v", namespace, name, err)
	}

	// TODO: this check will not work if the admin installs the registry without restarting the controller, because
	// only a relist from the API server will correct the empty value here (no watch events are sent)
	if len(stream.Status.DockerImageRepository) == 0 {
		return "", errNoIntegratedRegistry
	}

	repo, err := imageapi.ParseDockerImageReference(stream.Status.DockerImageRepository)
	if err != nil {
		return "", fmt.Errorf("the referenced output image stream does not represent a valid reference name: %v", err)
	}
	repo.ID = ""
	repo.Tag = tag
	return repo.Exact(), nil
}

func resolveImageStreamImage(ref *corev1.ObjectReference, lister imagev1lister.ImageStreamLister, defaultNamespace string) (*corev1.ObjectReference, error) {
	namespace := ref.Namespace
	if len(namespace) == 0 {
		namespace = defaultNamespace
	}
	name, imageID, ok := imageapi.SplitImageStreamImage(ref.Name)
	if !ok {
		return nil, errInvalidImageReferences
	}
	stream, err := lister.ImageStreams(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("the referenced image stream %s/%s could not be found: %v", namespace, name, err)
	}
	event, err := imageutil.ResolveImageID(stream, imageID)
	if err != nil {
		return nil, err
	}
	if len(event.DockerImageReference) == 0 {
		return nil, fmt.Errorf("the referenced image stream image %s/%s does not have a pull spec", namespace, ref.Name)
	}
	return &corev1.ObjectReference{Kind: "DockerImage", Name: event.DockerImageReference}, nil
}

func resolveImageStreamTag(ref *corev1.ObjectReference, lister imagev1lister.ImageStreamLister, defaultNamespace string) (*corev1.ObjectReference, error) {
	namespace := ref.Namespace
	if len(namespace) == 0 {
		namespace = defaultNamespace
	}
	name, tag, ok := imageapi.SplitImageStreamTag(ref.Name)
	if !ok {
		return nil, errInvalidImageReferences
	}
	stream, err := lister.ImageStreams(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("the referenced image stream %s/%s could not be found: %v", namespace, name, err)
	}
	if newRef, ok := imageutil.ResolveLatestTaggedImage(stream, tag); ok {
		return &corev1.ObjectReference{Kind: "DockerImage", Name: newRef}, nil
	}
	return nil, fmt.Errorf("the referenced image stream tag %s/%s does not exist", namespace, ref.Name)
}

// resolveOutputDockerImageReference updates the output spec to a docker image reference.
func (bc *BuildController) resolveOutputDockerImageReference(build *buildv1.Build) error {
	ref := build.Spec.Output.To
	if ref == nil || ref.Name == "" {
		return nil
	}

	switch ref.Kind {
	case "ImageStream", "ImageStreamTag":
		newRef, err := resolveImageStreamLocation(ref, bc.imageStreamStore, build.Namespace)
		if err != nil {
			return err
		}
		*ref = corev1.ObjectReference{Kind: "DockerImage", Name: newRef}
		return nil
	default:
		return nil
	}
}

// resolveImageReferences resolves references to Docker images computed from the build.Spec. It will update
// the output spec as well if it has not already been updated.
func (bc *BuildController) resolveImageReferences(build *buildv1.Build, update *buildUpdate) error {
	m := imagereferencemutators.NewBuildMutator(build)

	// get a list of all unresolved references to add to the cache
	streams, err := unresolvedImageStreamReferences(m, build.Namespace)
	if err != nil {
		return err
	}
	if len(streams) == 0 {
		klog.V(5).Infof("Build %s contains no unresolved image references", build.Name)
		return nil
	}

	// build references are level driven, so we queue here to ensure we get notified if
	// we are racing against updates in the image stream store
	buildKey := resourceName(build.Namespace, build.Name)
	bc.imageStreamQueue.Add(buildKey, streams)

	// resolve the output image reference
	if err := bc.resolveOutputDockerImageReference(build); err != nil {
		// If we cannot resolve the output reference, the output image stream
		// may not yet exist. The build should remain in the new state and show the
		// reason that it is still in the new state.
		update.setReason(buildv1.StatusReasonInvalidOutputReference)
		update.setMessage(buildutil.StatusMessageInvalidOutputRef)
		if err == errNoIntegratedRegistry {
			e := fmt.Errorf("an image stream cannot be used as build output because the integrated Docker registry is not configured")
			bc.recorder.Eventf(build, corev1.EventTypeWarning, "InvalidOutput", "Error starting build: %v", e)
		}
		return err
	}
	// resolve the remaining references
	errs := m.Mutate(func(ref *corev1.ObjectReference) error {
		switch ref.Kind {
		case "ImageStreamImage":
			newRef, err := resolveImageStreamImage(ref, bc.imageStreamStore, build.Namespace)
			if err != nil {
				return err
			}
			*ref = *newRef
		case "ImageStreamTag":
			newRef, err := resolveImageStreamTag(ref, bc.imageStreamStore, build.Namespace)
			if err != nil {
				return err
			}
			*ref = *newRef
		}
		return nil
	})

	if len(errs) > 0 {
		update.setReason(buildv1.StatusReasonInvalidImageReference)
		update.setMessage(buildutil.StatusMessageInvalidImageRef)
		return errs.ToAggregate()
	}
	// we have resolved all images, and will not need any further notifications
	bc.imageStreamQueue.Remove(buildKey, streams)
	return nil
}

func (bc *BuildController) prePodCreateSetup(build *buildv1.Build) (*buildv1.Build, *buildUpdate, error) {
	update := &buildUpdate{}

	// image reference resolution requires a copy of the build
	// TODO: Rename this to buildCopy
	build = build.DeepCopy()

	// Resolve all Docker image references to valid values.
	if err := bc.resolveImageReferences(build, update); err != nil {
		// if we're waiting for an image stream to exist, we will get an update via the
		// trigger, and thus don't need to be requeued.
		if hasError(err, errors.IsNotFound, field.NewErrorTypeMatcher(field.ErrorTypeNotFound)) {
			return build, update, nil
		}
		return build, update, err
	}

	// Set the pushSecret that will be needed by the build to push the image to the registry
	// at the end of the build.
	pushSecret := build.Spec.Output.PushSecret
	// Only look up a push secret if the user hasn't explicitly provided one.
	if build.Spec.Output.PushSecret == nil && build.Spec.Output.To != nil && len(build.Spec.Output.To.Name) > 0 {
		var err error
		pushSecret, err = bc.resolveImageSecretAsReference(build, build.Spec.Output.To.Name)
		if err != nil {
			transitionToPhase(buildv1.BuildPhaseError, buildv1.StatusReasonCannotRetrieveServiceAccount, buildutil.StatusMessageCannotRetrieveServiceAccount)
			return build, update, err
		}
	}
	build.Spec.Output.PushSecret = pushSecret

	// Set the pullSecret that will be needed by the build to pull the base/builder image.
	var pullSecret *corev1.LocalObjectReference
	var imageName string
	switch {
	case build.Spec.Strategy.SourceStrategy != nil:
		pullSecret = build.Spec.Strategy.SourceStrategy.PullSecret
		imageName = build.Spec.Strategy.SourceStrategy.From.Name
	case build.Spec.Strategy.DockerStrategy != nil:
		pullSecret = build.Spec.Strategy.DockerStrategy.PullSecret
		if build.Spec.Strategy.DockerStrategy.From != nil {
			imageName = build.Spec.Strategy.DockerStrategy.From.Name
		}
	case build.Spec.Strategy.CustomStrategy != nil:
		pullSecret = build.Spec.Strategy.CustomStrategy.PullSecret
		imageName = build.Spec.Strategy.CustomStrategy.From.Name
	}

	// Only look up a pull secret if the user hasn't explicitly provided one
	// if we don't know what image they are referencing, we'll end up using the
	// docker secret associated w/ the build's service account.
	if pullSecret == nil {
		var err error
		pullSecret, err = bc.resolveImageSecretAsReference(build, imageName)
		if err != nil {
			update = transitionToPhase(buildv1.BuildPhaseError, buildv1.StatusReasonCannotRetrieveServiceAccount, buildutil.StatusMessageCannotRetrieveServiceAccount)
			return build, update, err
		}
		if pullSecret != nil {
			switch {
			case build.Spec.Strategy.SourceStrategy != nil:
				build.Spec.Strategy.SourceStrategy.PullSecret = pullSecret
			case build.Spec.Strategy.DockerStrategy != nil:
				build.Spec.Strategy.DockerStrategy.PullSecret = pullSecret
			case build.Spec.Strategy.CustomStrategy != nil:
				build.Spec.Strategy.CustomStrategy.PullSecret = pullSecret
			}
		}
	}

	// look up the secrets needed to pull any source input images.
	for i, s := range build.Spec.Source.Images {
		if s.PullSecret != nil {
			continue
		}
		imageInputPullSecret, err := bc.resolveImageSecretAsReference(build, s.From.Name)
		if err != nil {
			update = transitionToPhase(buildv1.BuildPhaseError, buildv1.StatusReasonCannotRetrieveServiceAccount, buildutil.StatusMessageCannotRetrieveServiceAccount)
			return build, update, err
		}
		build.Spec.Source.Images[i].PullSecret = imageInputPullSecret
	}

	if build.Spec.Strategy.CustomStrategy != nil {
		buildutil.UpdateCustomImageEnv(build.Spec.Strategy.CustomStrategy, build.Spec.Strategy.CustomStrategy.From.Name)
	}

	return build, update, nil
}

// createBuildPod creates a new pod to run a build
func (bc *BuildController) createBuildPod(build *buildv1.Build) (*buildUpdate, error) {
	build, update, err := bc.prePodCreateSetup(build)
	if err != nil {
		return update, err
	}
	if !update.isEmpty() {
		return update, nil
	}

	// Indicate if the pod spec should mount the additional trusted CAs
	includeAdditionalCA := false
	if len(bc.additionalTrustedCAData) > 0 {
		includeAdditionalCA = true
	}

	// Create the build pod spec
	buildPod, err := bc.createPodSpec(build, includeAdditionalCA)
	if err != nil {
		switch err.(type) {
		case common.ErrEnvVarResolver:
			update = transitionToPhase(buildv1.BuildPhaseError, buildv1.StatusReasonUnresolvableEnvironmentVariable, fmt.Sprintf("%v, %v",
				buildutil.StatusMessageUnresolvableEnvironmentVariable, err.Error()))
		default:
			update = transitionToPhase(buildv1.BuildPhaseError, buildv1.StatusReasonCannotCreateBuildPodSpec, buildutil.StatusMessageCannotCreateBuildPodSpec)
		}
		// If an error occurred when creating the pod spec, it likely means
		// that the build is something we don't understand. For example, it could
		// have a strategy that we don't recognize. It will remain in New state
		// and be updated with the reason that it is still in New

		// The error will be logged, but will not be returned to the caller
		// to be retried. The reason is that there's really no external factor
		// that could cause the pod spec creation to fail; therefore no reason
		// to immediately retry processing the build.
		//
		// A scenario where this would happen is that we've introduced a
		// new build strategy in the master, but the old version of the controller
		// is still running. We don't want the old controller to move the
		// build to the error phase and we don't want it to keep actively retrying.
		utilruntime.HandleError(err)
		bc.recorder.Eventf(build, corev1.EventTypeWarning, "FailedCreate", "Pod spec error: %s/%s: %s", build.Namespace, build.Name, err.Error())
		return update, nil
	}

	klog.V(4).Infof("Pod %s/%s for build %s is about to be created", build.Namespace, buildPod.Name, buildDesc(build))
	_, err = bc.podClient.Pods(build.Namespace).Create(buildPod)
	if err != nil && !errors.IsAlreadyExists(err) {
		// Log an event if the pod is not created (most likely due to quota denial).
		bc.recorder.Eventf(build, corev1.EventTypeWarning, "FailedCreate", "Error creating build pod: %v", err)
		update = transitionToPhase(buildv1.BuildPhaseError, buildv1.StatusReasonCannotCreateBuildPod, buildutil.StatusMessageCannotCreateBuildPod)
		return update, fmt.Errorf("failed to create build pod: %v", err)

	} else if err != nil {
		bc.recorder.Eventf(build, corev1.EventTypeWarning, "FailedCreate", "Pod already exists: %s/%s", buildPod.Namespace, buildPod.Name)
		klog.V(4).Infof("Build pod %s/%s for build %s already exists", build.Namespace, buildPod.Name, buildDesc(build))

		// If the existing pod was not created by this build, switch to the
		// Error state.
		existingPod, err := bc.podClient.Pods(build.Namespace).Get(buildPod.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if !strategy.HasOwnerReference(existingPod, build) {
			klog.V(4).Infof("Did not recognise pod %s/%s as belonging to build %s", build.Namespace, buildPod.Name, buildDesc(build))
			update = transitionToPhase(buildv1.BuildPhaseError, buildv1.StatusReasonBuildPodExists, buildutil.StatusMessageBuildPodExists)
			return update, nil
		}
		klog.V(4).Infof("Recognised pod %s/%s as belonging to build %s", build.Namespace, buildPod.Name, buildDesc(build))

	} else {
		klog.V(4).Infof("Created pod %s/%s for build %s", build.Namespace, buildPod.Name, buildDesc(build))
	}

	// so only update the build if an error occurs
	return nil, nil
}

func (bc *BuildController) postPodCreateSetupCfgMapSecretAnnotation(build *buildv1.Build, pod *corev1.Pod) (*buildUpdate, error) {
	build, update, err := bc.prePodCreateSetup(build)
	if err != nil {
		return update, err
	}

	// Check if the existing pod has the CA ConfigMap properly attached
	hasCAMap, err := bc.findOwnedConfigMap(pod, build.Namespace, buildapihelpers.GetBuildCAConfigMapName(build))
	if err != nil {
		return update, fmt.Errorf("could not find certificate authority for build: %v", err)
	}
	if !hasCAMap {
		// Create the CA ConfigMap to mount certificate authorities to the build pod
		// update is only changed by this method if an error occurs
		update, err := bc.createBuildCAConfigMap(build, pod, update)
		if err != nil {
			return update, err
		}

	}
	hasRegistryConf, err := bc.findOwnedConfigMap(pod, build.Namespace, buildapihelpers.GetBuildSystemConfigMapName(build))
	if err != nil {
		return update, fmt.Errorf("could not find registry config for build: %v", err)
	}
	if !hasRegistryConf {
		// Create the registry config ConfigMap to mount the registry configuration into the build pod
		// update is only changed by this method if an error occurs
		update, err = bc.createBuildSystemConfConfigMap(build, pod, update)
		if err != nil {
			//FYI this used to return nil, err for some reason
			return update, err
		}
	}

	// Only look up a push secret if the user hasn't explicitly provided one.
	if build.Spec.Output.PushSecret == nil && build.Spec.Output.To != nil && len(build.Spec.Output.To.Name) > 0 {
		var err error
		pushSecret, err := bc.resolveImageSecretAsReference(build, build.Spec.Output.To.Name)
		if err != nil {
			update = transitionToPhase(buildv1.BuildPhaseError, buildv1.StatusReasonCannotRetrieveServiceAccount, buildutil.StatusMessageCannotRetrieveServiceAccount)
			return update, err
		}
		if pushSecret != nil {
			update.setPushSecret(*pushSecret)
		}
	}

	update = transitionToPhase(buildv1.BuildPhasePending, "", "")

	update.setPodNameAnnotation(pod.Name)
	if build.Spec.Output.To != nil {
		update.setOutputRef(build.Spec.Output.To.Name)
	}

	return update, nil
}

// handleActiveBuild handles a build in either pending or running state
func (bc *BuildController) handleActiveBuild(build *buildv1.Build, pod *corev1.Pod) (*buildUpdate, error) {
	if pod == nil {
		pod = bc.findMissingPod(build)
		if pod == nil {
			klog.V(4).Infof("Failed to find the build pod for build %s. Moving it to Error state", buildDesc(build))
			return transitionToPhase(buildv1.BuildPhaseError, buildv1.StatusReasonBuildPodDeleted, buildutil.StatusMessageBuildPodDeleted), nil
		}
	}

	podPhase := pod.Status.Phase
	var update *buildUpdate
	// Pods don't report running until initcontainers are done, but from a build's perspective
	// the pod is running as soon as the first init container has run.
	if build.Status.Phase == buildv1.BuildPhasePending || build.Status.Phase == buildv1.BuildPhaseNew {
		for _, initContainer := range pod.Status.InitContainerStatuses {
			if initContainer.Name == strategy.GitCloneContainer && initContainer.State.Running != nil {
				podPhase = corev1.PodRunning
			}
		}
	}
	switch podPhase {
	case corev1.PodPending:
		if build.Status.Phase != buildv1.BuildPhasePending {
			update = transitionToPhase(buildv1.BuildPhasePending, "", "")
		}
		if secret := build.Spec.Output.PushSecret; secret != nil && build.Status.Reason != buildv1.StatusReasonMissingPushSecret {
			if _, err := bc.secretStore.Secrets(build.Namespace).Get(secret.Name); err != nil && errors.IsNotFound(err) {
				klog.V(4).Infof("Setting reason for pending build to %q due to missing secret for %s", build.Status.Reason, buildDesc(build))
				update = transitionToPhase(buildv1.BuildPhasePending, buildv1.StatusReasonMissingPushSecret, buildutil.StatusMessageMissingPushSecret)
			}
		}
	case corev1.PodRunning:
		if build.Status.Phase != buildv1.BuildPhaseRunning {
			update = transitionToPhase(buildv1.BuildPhaseRunning, "", "")
			if pod.Status.StartTime != nil {
				update.setStartTime(*pod.Status.StartTime)
			}
		}
	case corev1.PodSucceeded:
		if build.Status.Phase != buildv1.BuildPhaseComplete {
			update = transitionToPhase(buildv1.BuildPhaseComplete, "", "")
		}
		if len(pod.Status.ContainerStatuses) == 0 {
			// no containers in the pod means something went terribly wrong, so the build
			// should be set to an error state
			klog.V(2).Infof("Setting build %s to error state because its pod has no containers", buildDesc(build))
			update = transitionToPhase(buildv1.BuildPhaseError, buildv1.StatusReasonNoBuildContainerStatus,
				buildutil.StatusMessageNoBuildContainerStatus)
		} else {
			for _, info := range pod.Status.ContainerStatuses {
				if info.State.Terminated != nil && info.State.Terminated.ExitCode != 0 {
					klog.V(2).Infof("Setting build %s to error state because a container in its pod has non-zero exit code", buildDesc(build))
					update = transitionToPhase(buildv1.BuildPhaseError, buildv1.StatusReasonFailedContainer, buildutil.StatusMessageFailedContainer)
					break
				}
			}
		}
	case corev1.PodFailed:
		if isOOMKilled(pod) {
			update = transitionToPhase(buildv1.BuildPhaseFailed, buildv1.StatusReasonOutOfMemoryKilled, buildutil.StatusMessageOutOfMemoryKilled)
		} else if isPodEvicted(pod) {
			// Use the pod status message to report why the build pod was evicted.
			update = transitionToPhase(buildv1.BuildPhaseFailed, buildv1.StatusReasonBuildPodEvicted, pod.Status.Message)
		} else if build.Status.Phase != buildv1.BuildPhaseFailed {
			// If a DeletionTimestamp has been set, it means that the pod will
			// soon be deleted. The build should be transitioned to the Error phase.
			if pod.DeletionTimestamp != nil {
				update = transitionToPhase(buildv1.BuildPhaseError, buildv1.StatusReasonBuildPodDeleted, buildutil.StatusMessageBuildPodDeleted)
			} else {
				update = transitionToPhase(buildv1.BuildPhaseFailed, buildv1.StatusReasonGenericBuildFailed, buildutil.StatusMessageGenericBuildFailed)
			}
		}
	}
	return update, nil
}

func isOOMKilled(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	if pod.Status.Reason == "OOMKilled" {
		return true
	}
	for _, c := range pod.Status.InitContainerStatuses {
		terminated := c.State.Terminated
		if terminated != nil && terminated.Reason == "OOMKilled" {
			return true
		}
	}
	for _, c := range pod.Status.ContainerStatuses {
		terminated := c.State.Terminated
		if terminated != nil && terminated.Reason == "OOMKilled" {
			return true
		}
	}
	return false
}

func isPodEvicted(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	if pod.Status.Reason == "Evicted" {
		return true
	}
	return false
}

// handleCompletedBuild will only be called on builds that are already in a terminal phase.  It is used to setup the
// completion timestamp and failure logsnippet as needed.
func (bc *BuildController) handleCompletedBuild(build *buildv1.Build, pod *corev1.Pod) (*buildUpdate, error) {

	update := &buildUpdate{}
	if isOOMKilled(pod) {
		update = transitionToPhase(buildv1.BuildPhaseFailed, buildv1.StatusReasonOutOfMemoryKilled, buildutil.StatusMessageOutOfMemoryKilled)
	}
	setBuildCompletionData(build, pod, update)

	return update, nil
}

// updateBuild is the single place where any update to a build is done in the build controller.
// It will check that the update is valid, peform any necessary processing such as calling HandleBuildCompletion,
// and apply the buildUpdate object as a patch.
func (bc *BuildController) updateBuild(build *buildv1.Build, update *buildUpdate, pod *corev1.Pod) error {

	stateTransition := false
	// Check whether we are transitioning to a different build phase
	if update.phase != nil && (*update.phase) != build.Status.Phase {
		stateTransition = true
	} else if build.Status.Phase == buildv1.BuildPhaseFailed && update.completionTime != nil {
		// Treat a failed->failed update as a state transition when the completionTime is getting
		// updated. This will cause an event to be emitted and completion processing to trigger.
		// We get into this state when the pod updates the phase through the build/details subresource.
		// The phase, reason, and message are set, but no event has been emitted about the failure,
		// and the policy has not been given a chance to start the next build if one is waiting to
		// start.
		update.setPhase(buildv1.BuildPhaseFailed)
		stateTransition = true
	}

	if stateTransition {
		// Make sure that the transition is valid
		if !isValidTransition(build.Status.Phase, *update.phase) {
			return fmt.Errorf("invalid phase transition %s -> %s", buildDesc(build), *update.phase)
		}

		// Log that we are updating build status
		reasonText := ""
		if update.reason != nil && *update.reason != "" {
			reasonText = fmt.Sprintf(" ( %s )", *update.reason)
		}

		// Update build completion timestamp if transitioning to a terminal phase
		if buildutil.IsTerminalPhase(*update.phase) {
			setBuildCompletionData(build, pod, update)
		}
		klog.V(4).Infof("Updating build %s -> %s%s", buildDesc(build), *update.phase, reasonText)
	}

	// Ensure that a pod name annotation has been set on the build if a pod is available
	if update.podNameAnnotation == nil && !common.HasBuildPodNameAnnotation(build) && pod != nil {
		update.setPodNameAnnotation(pod.Name)
	}

	patchedBuild, err := bc.patchBuild(build, update)
	if err != nil {
		logStr := fmt.Sprintf("error patching build %s with update %#v:  %v", resourceName(build.Namespace, build.Name), update, err)
		klog.Errorf(logStr)
		bc.recorder.Eventf(build, corev1.EventTypeWarning, "BuildPatchFailure", logStr)
		return err
	}

	// Emit events and handle build completion if transitioned to a terminal phase
	if stateTransition {
		switch *update.phase {
		case buildv1.BuildPhaseRunning:
			bc.recorder.Eventf(patchedBuild, corev1.EventTypeNormal, buildutil.BuildStartedEventReason,
				fmt.Sprintf(buildutil.BuildStartedEventMessage,
					patchedBuild.Namespace, patchedBuild.Name))
		case buildv1.BuildPhaseCancelled:
			bc.recorder.Eventf(patchedBuild, corev1.EventTypeNormal, buildutil.BuildCancelledEventReason,
				fmt.Sprintf(buildutil.BuildCancelledEventMessage, patchedBuild.Namespace, patchedBuild.Name))
		case buildv1.BuildPhaseComplete:
			bc.recorder.Eventf(patchedBuild, corev1.EventTypeNormal, buildutil.BuildCompletedEventReason,
				fmt.Sprintf(buildutil.BuildCompletedEventMessage, patchedBuild.Namespace, patchedBuild.Name))
		case buildv1.BuildPhaseError,
			buildv1.BuildPhaseFailed:
			bc.recorder.Eventf(patchedBuild, corev1.EventTypeNormal, buildutil.BuildFailedEventReason, fmt.Sprintf(buildutil.BuildFailedEventMessage,
				patchedBuild.Namespace, patchedBuild.Name))
		}
		if buildutil.IsTerminalPhase(*update.phase) {
			bc.handleBuildCompletion(patchedBuild)
		}
	}
	return nil
}

func (bc *BuildController) handleBuildCompletion(build *buildv1.Build) {
	bcName := buildutil.ConfigNameForBuild(build)
	bc.enqueueBuildConfig(build.Namespace, bcName)
	if err := common.HandleBuildPruning(bcName, build.Namespace, bc.buildLister, bc.buildConfigGetter, bc.buildDeleter); err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to prune builds for %s/%s: %v", build.Namespace, build.Name, err))
	}
}

func (bc *BuildController) enqueueBuildConfig(ns, name string) {
	key := resourceName(ns, name)
	bc.buildConfigQueue.Add(key)
}

func (bc *BuildController) handleBuildConfig(bcNamespace string, bcName string) error {
	klog.V(4).Infof("Handling build config %s/%s", bcNamespace, bcName)
	nextBuilds, hasRunningBuilds, err := policy.GetNextConfigBuild(bc.buildLister, bcNamespace, bcName)
	if err != nil {
		klog.V(2).Infof("Error getting next builds for %s/%s: %v", bcNamespace, bcName, err)
		return err
	}
	klog.V(5).Infof("Build config %s/%s: has %d next builds, is running builds: %v", bcNamespace, bcName, len(nextBuilds), hasRunningBuilds)
	if hasRunningBuilds {
		klog.V(4).Infof("Build config %s/%s has running builds, will retry", bcNamespace, bcName)
		return fmt.Errorf("build config %s/%s has running builds and cannot run more builds", bcNamespace, bcName)
	}
	if len(nextBuilds) == 0 {
		klog.V(4).Infof("Build config %s/%s has no builds to run next, will retry", bcNamespace, bcName)
		return fmt.Errorf("build config %s/%s has no builds to run next", bcNamespace, bcName)
	}

	// Enqueue any builds to build next
	for _, build := range nextBuilds {
		klog.V(5).Infof("Queueing next build for build config %s/%s: %s", bcNamespace, bcName, build.Name)
		bc.enqueueBuild(build)
	}
	return nil
}
func createBuildPatch(older, newer *buildv1.Build) ([]byte, error) {
	newerJSON, err := runtime.Encode(buildscheme.Encoder, newer)
	if err != nil {
		return nil, fmt.Errorf("error encoding newer: %v", err)
	}
	olderJSON, err := runtime.Encode(buildscheme.Encoder, older)
	if err != nil {
		return nil, fmt.Errorf("error encoding older: %v", err)
	}
	patch, err := strategicpatch.CreateTwoWayMergePatch(olderJSON, newerJSON, &buildv1.Build{})
	if err != nil {
		return nil, fmt.Errorf("error creating a strategic patch: %v", err)
	}
	return patch, nil
}

// patchBuild generates a patch for the given build and buildUpdate
// and applies that patch using the REST client
func (bc *BuildController) patchBuild(build *buildv1.Build, update *buildUpdate) (*buildv1.Build, error) {
	// Create a patch using the buildUpdate object
	updatedBuild := build.DeepCopy()
	update.apply(updatedBuild)

	patch, err := createBuildPatch(build, updatedBuild)
	if err != nil {
		return nil, fmt.Errorf("failed to create a build patch: %v", err)
	}

	klog.V(5).Infof("Patching build %s with %v", buildDesc(build), update)
	return bc.buildPatcher.Patch(build.Namespace, build.Name, patch)
}

// findMissingPod uses the REST client directly to determine if a pod exists or not.
// It is called when a corresponding pod for a build is not found in the cache.
func (bc *BuildController) findMissingPod(build *buildv1.Build) *corev1.Pod {
	// Make one last attempt to fetch the pod using the REST client
	pod, err := bc.podClient.Pods(build.Namespace).Get(buildapihelpers.GetBuildPodName(build), metav1.GetOptions{})
	if err == nil {
		klog.V(2).Infof("Found missing pod for build %s by using direct client.", buildDesc(build))
		return pod
	}
	return nil
}

// getBuildByKey looks up a build by key in the buildInformer cache
func (bc *BuildController) getBuildByKey(key string) (*buildv1.Build, error) {
	obj, exists, err := bc.buildInformer.GetIndexer().GetByKey(key)
	if err != nil {
		klog.V(2).Infof("Unable to retrieve build %q from store: %v", key, err)
		return nil, err
	}
	if !exists {
		klog.V(2).Infof("Build %q has been deleted", key)
		return nil, nil
	}

	return obj.(*buildv1.Build), nil
}

// podUpdated gets called by the pod informer event handler whenever a pod
// is updated or there is a relist of pods
func (bc *BuildController) podUpdated(old, cur interface{}) {
	// A periodic relist will send update events for all known pods.
	curPod := cur.(*corev1.Pod)
	oldPod := old.(*corev1.Pod)
	// The old and new ResourceVersion will be the same in a relist of pods.
	// Here we ignore pod relists because we already listen to build relists.
	if curPod.ResourceVersion == oldPod.ResourceVersion {
		return
	}
	if isBuildPod(curPod) {
		klog.V(0).Infof("GGM podUpdated enqueing build %s for pod %s old status %#v new status %#v", resourceName(curPod.Namespace, getBuildName(curPod)), curPod.Name, oldPod.Status, curPod.Status)
		bc.enqueueBuildForPod(curPod)
	}
}

func (bc *BuildController) podAdded(obj interface{}) {
	pod := obj.(*corev1.Pod)
	if isBuildPod(pod) {
		klog.V(0).Infof("GGM podAdded build %s for pod %s status %#v", resourceName(pod.Namespace, getBuildName(pod)), pod.Name, pod.Status)
		bc.enqueueAddedPod(pod)
	}
}

// podDeleted gets called by the pod informer event handler whenever a pod
// is deleted
func (bc *BuildController) podDeleted(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone: %+v", obj))
			return
		}
		pod, ok = tombstone.Obj.(*corev1.Pod)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a pod: %+v", obj))
			return
		}
	}
	if isBuildPod(pod) {
		bc.enqueueBuildForPod(pod)
	}
}

// buildAdded is called by the build informer event handler whenever a build
// is created
func (bc *BuildController) buildAdded(obj interface{}) {
	build := obj.(*buildv1.Build)
	klog.V(0).Infof("GGM build %s added %#v", resourceName(build.Namespace, build.Name), build.Status)
	bc.enqueueBuild(build)
}

// buildUpdated is called by the build informer event handler whenever a build
// is updated or there is a relist of builds
func (bc *BuildController) buildUpdated(old, cur interface{}) {
	oldbuild := old.(*buildv1.Build)
	newbuild := cur.(*buildv1.Build)
	klog.V(0).Infof("GGM build %s update old status %#v new status %#v", resourceName(newbuild.Namespace, newbuild.Name), oldbuild.Status, newbuild.Status)
	bc.enqueueBuild(newbuild)
}

// buildDeleted is called by the build informer event handler whenever a build
// is deleted
func (bc *BuildController) buildDeleted(obj interface{}) {
	build, ok := obj.(*buildv1.Build)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone: %+v", obj))
			return
		}
		build, ok = tombstone.Obj.(*buildv1.Build)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a pod: %+v", obj))
			return
		}
	}
	// If the build was not in a complete state, poke the buildconfig to run the next build
	if !buildutil.IsBuildComplete(build) {
		bcName := buildutil.ConfigNameForBuild(build)
		bc.enqueueBuildConfig(build.Namespace, bcName)
	}
}

// enqueueBuild adds the given build to the buildQueue.
func (bc *BuildController) enqueueBuild(build *buildv1.Build) {
	key := resourceName(build.Namespace, build.Name)
	bc.buildQueue.Add(key)
}

// enqueueBuildForPod adds the build corresponding to the given pod to the controller
// buildQueue. If a build is not found for the pod, then an error is logged.
func (bc *BuildController) enqueueBuildForPod(pod *corev1.Pod) {
	bc.buildQueue.Add(resourceName(pod.Namespace, getBuildName(pod)))
}

func (bc *BuildController) enqueueAddedPod(pod *corev1.Pod) {
	bc.podQueue.Add(pod)
}

// imageStreamAdded queues any builds that have registered themselves for this image stream.
// Because builds are level driven when resolving images, we are not concerned with duplicate
// build events.
func (bc *BuildController) imageStreamAdded(obj interface{}) {
	stream := obj.(*imagev1.ImageStream)
	for _, buildKey := range bc.imageStreamQueue.Pop(resourceName(stream.Namespace, stream.Name)) {
		bc.buildQueue.Add(buildKey)
	}
}

// imageStreamUpdated queues any builds that have registered themselves for the image stream.
func (bc *BuildController) imageStreamUpdated(old, cur interface{}) {
	bc.imageStreamAdded(cur)
}

func (bc *BuildController) handleQueueError(err error, key interface{}, queue workqueue.RateLimitingInterface) {
	if err == nil {
		queue.Forget(key)
		return
	}

	if strategy.IsFatal(err) {
		klog.V(2).Infof("Will not retry fatal error for key %v: %v", key, err)
		queue.Forget(key)
		return
	}

	if bc.buildQueue.NumRequeues(key) < maxRetries {
		klog.V(4).Infof("Retrying key %v: %v", key, err)
		queue.AddRateLimited(key)
		return
	}

	klog.V(2).Infof("Giving up retrying %v: %v", key, err)
	queue.Forget(key)

}

// handleBuildError is called by the main work loop to check the return of calling handleBuild.
// If an error occurred, then the key is re-added to the buildQueue unless it has been retried too many
// times.
func (bc *BuildController) handleBuildError(err error, key interface{}) {
	if err == nil {
		bc.buildQueue.Forget(key)
		return
	}

	if strategy.IsFatal(err) {
		klog.V(2).Infof("Will not retry fatal error for key %v: %v", key, err)
		bc.buildQueue.Forget(key)
		return
	}

	if bc.buildQueue.NumRequeues(key) < maxRetries {
		klog.V(4).Infof("Retrying key %v: %v", key, err)
		bc.buildQueue.AddRateLimited(key)
		return
	}

	klog.V(2).Infof("Giving up retrying %v: %v", key, err)
	bc.buildQueue.Forget(key)
}

// handleBuildConfigError is called by the buildConfig work loop to check the return of calling handleBuildConfig.
// If an error occurred, then the key is re-added to the buildConfigQueue unless it has been retried too many
// times.
func (bc *BuildController) handleBuildConfigError(err error, key interface{}) {
	if err == nil {
		bc.buildConfigQueue.Forget(key)
		return
	}

	if bc.buildConfigQueue.NumRequeues(key) < maxRetries {
		klog.V(4).Infof("Retrying key %v: %v", key, err)
		bc.buildConfigQueue.AddRateLimited(key)
		return
	}

	klog.V(2).Infof("Giving up retrying %v: %v", key, err)
	bc.buildConfigQueue.Forget(key)
}

// createBuildCAConfigMap creates a ConfigMap containing certificate authorities used by the build pod.
func (bc *BuildController) createBuildCAConfigMap(build *buildv1.Build, buildPod *corev1.Pod, update *buildUpdate) (*buildUpdate, error) {
	configMapSpec := bc.createBuildCAConfigMapSpec(build, buildPod)
	configMap, err := bc.configMapClient.ConfigMaps(buildPod.Namespace).Create(configMapSpec)
	if err != nil {
		bc.recorder.Eventf(build, corev1.EventTypeWarning, "FailedCreate", "Error creating build certificate authority configMap: %v", err)
		update = transitionToPhase(buildv1.BuildPhaseError, "CannotCreateCAConfigMap", buildutil.StatusMessageCannotCreateCAConfigMap)
		return update, fmt.Errorf("failed to create build certificate authority configMap: %v", err)
	}
	klog.V(4).Infof("Created certificate authority configMap %s/%s for build %s", build.Namespace, configMap.Name, buildDesc(build))
	return update, nil
}

// createBuildCAConfigMapSpec creates a ConfigMap template to hold certificate authorities
// used by the build pod.
// The returned ConfigMap has an owner reference to the provided pod, ensuring proper
// garbage collection.
func (bc *BuildController) createBuildCAConfigMapSpec(build *buildv1.Build, buildPod *corev1.Pod) *corev1.ConfigMap {
	// Create a ConfigMap with the `inject-cabundle` annotation, which tells the service cert signing controller
	// to add the service-ca.crt entry to this map.
	// See https://github.com/openshift/service-serving-cert-signer
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: buildapihelpers.GetBuildCAConfigMapName(build),
			OwnerReferences: []metav1.OwnerReference{
				makeBuildPodOwnerRef(buildPod),
			},
			Annotations: map[string]string{
				"service.alpha.openshift.io/inject-cabundle": "true",
			},
		},
		Data: bc.additionalTrustedCAData,
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	return cm
}

// findOwnedConfigMap finds the ConfigMap with the given name and namespace, and owned by the provided pod.
func (bc *BuildController) findOwnedConfigMap(owner *corev1.Pod, namespace string, name string) (bool, error) {
	cm, err := bc.configMapClient.ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if hasRef := hasBuildPodOwnerRef(owner, cm); !hasRef {
		return true, fmt.Errorf("configMap %s/%s is not owned by build pod %s/%s", cm.Namespace, cm.Name, owner.Namespace, owner.Name)
	}
	return true, nil
}

func (bc *BuildController) createBuildSystemConfConfigMap(build *buildv1.Build, buildPod *corev1.Pod, update *buildUpdate) (*buildUpdate, error) {
	configMapSpec := bc.createBuildSystemConfigMapSpec(build, buildPod)
	configMap, err := bc.configMapClient.ConfigMaps(build.Namespace).Create(configMapSpec)
	if err != nil {
		bc.recorder.Eventf(build, corev1.EventTypeWarning, "FailedCreate", "Error creating build system config configMap: %v", err)
		update = transitionToPhase(buildv1.BuildPhaseError, "CannotCreateBuildSysConfigMap", buildutil.StatusMessageCannotCreateBuildSysConfigMap)
		return update, fmt.Errorf("failed to create build system config configMap: %v", err)
	}
	klog.V(4).Infof("Created build system config configMap %s/%s for build %s", build.Namespace, configMap.Name, buildDesc(build))
	return update, nil
}

func (bc *BuildController) createBuildSystemConfigMapSpec(build *buildv1.Build, buildPod *corev1.Pod) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: buildapihelpers.GetBuildSystemConfigMapName(build),
			OwnerReferences: []metav1.OwnerReference{
				makeBuildPodOwnerRef(buildPod),
			},
		},
		Data: make(map[string]string),
	}
	if len(bc.registryConfData) > 0 {
		cm.Data[buildutil.RegistryConfKey] = bc.registryConfData
	}
	if len(bc.signaturePolicyData) > 0 {
		cm.Data[buildutil.SignaturePolicyKey] = bc.signaturePolicyData
	}
	return cm
}

func (bc *BuildController) controllerConfigWorker() {
	for {
		if quit := bc.controllerConfigWork(); quit {
			return
		}
	}
}

func (bc *BuildController) controllerConfigWork() bool {
	key, quit := bc.controllerConfigQueue.Get()
	if quit {
		return true
	}
	defer bc.controllerConfigQueue.Done(key)

	var err error
	errs := bc.handleControllerConfig()
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = fmt.Sprintf("%v", e)
		}
		err = fmt.Errorf("failed to apply build controller config: %v", msgs)
	}
	bc.handleControllerConfigError(err, key)
	return false
}

// handleControllerConfig synchronizes the build controller config in the cluster
// `*.config.openshift.io` instances and any referenced data in the `openshift-config` namespace.
func (bc *BuildController) handleControllerConfig() []error {
	configErrs := bc.readClusterImageConfig()
	err := bc.readClusterBuildControllerConfig()
	if err != nil {
		configErrs = append(configErrs, err)
	}
	return configErrs
}

// readClusterBuildControllerConfig synchronizes the build controller config
// in `builds.config.openshift.io/cluster`
func (bc *BuildController) readClusterBuildControllerConfig() error {
	buildConfig, err := bc.buildControllerConfigLister.Get("cluster")
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if buildConfig == nil {
		bc.buildDefaults.DefaultProxy = nil
		return nil
	}

	if klog.V(5) {
		configJSON, _ := json.Marshal(buildConfig)
		if configJSON != nil {
			klog.Infof("build controller config: %s", string(configJSON))
		}
	}
	bc.buildDefaults.DefaultProxy = buildConfig.Spec.BuildDefaults.DefaultProxy

	return nil
}

// readClusterImageConfig synchronizes the cluster image configuration in
// `image.config.openshift.io/cluster`, including the additional certificate authorities
// that are saved in the `openshift-config` namespace.
// This also generates the container runtime configuration data used by the build pods.
func (bc *BuildController) readClusterImageConfig() []error {
	configErrs := []error{}
	// Get additional CAs from the image config
	imageConfig, err := bc.imageConfigLister.Get("cluster")
	if err != nil && !errors.IsNotFound(err) {
		configErrs = append(configErrs, err)
		return configErrs
	} else if imageConfig == nil {
		bc.additionalTrustedCAData = nil
		bc.registryConfData = ""
		bc.signaturePolicyData = ""
		return configErrs
	}

	if klog.V(5) {
		configJSON, _ := json.Marshal(imageConfig)
		if configJSON != nil {
			klog.Infof("image config: %s", string(configJSON))
		}
	}

	additionalCAs, err := bc.getAdditionalTrustedCAData(imageConfig)
	if err != nil {
		configErrs = append(configErrs, err)
	} else {
		bc.additionalTrustedCAData = additionalCAs
	}

	registriesTOML, regErr := bc.createBuildRegistriesConfigData(imageConfig)
	if regErr != nil {
		configErrs = append(configErrs, regErr)
	} else {
		bc.registryConfData = registriesTOML
	}

	signatureJSON, sigErr := bc.createBuildSignaturePolicyData(imageConfig)
	if sigErr != nil {
		configErrs = append(configErrs, sigErr)
	} else {
		bc.signaturePolicyData = signatureJSON
	}

	return configErrs
}

func (bc *BuildController) getAdditionalTrustedCAData(config *configv1.Image) (map[string]string, error) {
	if len(config.Spec.AdditionalTrustedCA.Name) == 0 {
		klog.V(4).Info("additional certificate authorities for builds not specified")
		return nil, nil
	}
	additionalCA, err := bc.openShiftConfigConfigMapStore.ConfigMaps("openshift-config").Get(config.Spec.AdditionalTrustedCA.Name)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if additionalCA == nil {
		klog.V(3).Infof("configMap reference %s/%s with additional certificate authorities for builds not found",
			"openshift-config",
			config.Spec.AdditionalTrustedCA.Name)
		return nil, nil
	}
	if klog.V(5) {
		keys := make([]string, len(additionalCA.Data))
		i := 0
		for key := range additionalCA.Data {
			keys[i] = key
			i++
		}
		klog.Infof("found certificate authorities for hosts %s", keys)
	}
	return additionalCA.Data, nil
}

func (bc *BuildController) createBuildRegistriesConfigData(config *configv1.Image) (string, error) {
	registriesConfig := config.Spec.RegistrySources
	if len(registriesConfig.InsecureRegistries) == 0 {
		klog.V(4).Info("using default insecure registry settings for builds")
		return "", nil
	}
	configObj := tomlConfig{
		Registries: registries{
			// docker.io must be the only entry in the registry search list
			// See https://github.com/openshift/builder/pull/40
			Search: registryList{
				Registries: []string{"docker.io"},
			},
			Insecure: registryList{
				Registries: registriesConfig.InsecureRegistries,
			},
		},
	}

	configTOML, err := toml.Marshal(configObj)
	if err != nil {
		return "", err
	}
	if len(configTOML) == 0 {
		klog.V(4).Info("using default insecure registry settings for builds")
		return "", nil
	}
	klog.V(4).Info("overrode insecure registry settings for builds")
	klog.V(5).Infof("generated registries.conf for build pods: \n%s", string(configTOML))
	return string(configTOML), nil
}

func (bc *BuildController) createBuildSignaturePolicyData(config *configv1.Image) (string, error) {
	registriesConfig := config.Spec.RegistrySources
	if len(registriesConfig.AllowedRegistries) == 0 && len(registriesConfig.BlockedRegistries) == 0 {
		klog.V(4).Info("allowing builds to pull images from all registries")
		return "", nil
	}
	if len(registriesConfig.AllowedRegistries) != 0 && len(registriesConfig.BlockedRegistries) != 0 {
		return "", fmt.Errorf("invalid registries config: only one of AllowedRegistries or BlockedRegistries may be specified")
	}
	policyObj := &signature.Policy{}
	transportScopes := make(signature.PolicyTransportScopes)
	if len(registriesConfig.AllowedRegistries) > 0 {
		klog.V(4).Infof("only allowing image pulls from %s for builds", registriesConfig.AllowedRegistries)
		policyObj.Default = signature.PolicyRequirements{
			signature.NewPRReject(),
		}
		for _, registry := range registriesConfig.AllowedRegistries {
			transportScopes[registry] = signature.PolicyRequirements{
				signature.NewPRInsecureAcceptAnything(),
			}
		}
	}
	if len(registriesConfig.BlockedRegistries) > 0 {
		klog.V(4).Infof("blocking image pulls from %s for builds", registriesConfig.BlockedRegistries)
		policyObj.Default = signature.PolicyRequirements{
			signature.NewPRInsecureAcceptAnything(),
		}
		for _, registry := range registriesConfig.BlockedRegistries {
			transportScopes[registry] = signature.PolicyRequirements{
				signature.NewPRReject(),
			}
		}
	}

	// Policies for image pull/push are set on a per-transport basis.
	// This list will need to be updated if addtitional transports are used by the build pod.
	// The following transports are currently available in openshift builds:
	//
	// 1. docker: a docker v2 registry (docker.io, quay.io, internal registry, etc.)
	// 2. atomic: an ImageStreamTag reference - deprecated
	//
	// See man skopeo(1) for the full list of supported transports.
	policyObj.Transports = map[string]signature.PolicyTransportScopes{
		"atomic": transportScopes,
		"docker": transportScopes,
	}

	policyJSON, err := json.Marshal(policyObj)
	if err != nil {
		return "", err
	}
	if len(policyJSON) == 0 {
		return "", nil
	}
	klog.V(5).Infof("generated policy.json for build pods: \n%s", string(policyJSON))
	return string(policyJSON), err
}

func (bc *BuildController) handleControllerConfigError(err error, key interface{}) {
	if err != nil {
		utilruntime.HandleError(err)
	}
	bc.controllerConfigQueue.Forget(key)
}

func (bc *BuildController) buildControllerConfigAdded(obj interface{}) {
	bc.controllerConfigChanged()
}

func (bc *BuildController) buildControllerConfigUpdated(old, cur interface{}) {
	bc.controllerConfigChanged()
}

func (bc *BuildController) buildControllerConfigDeleted(obj interface{}) {
	bc.controllerConfigChanged()
}

func (bc *BuildController) imageConfigAdded(obj interface{}) {
	bc.controllerConfigChanged()
}

func (bc *BuildController) imageConfigUpdated(old, cur interface{}) {
	bc.controllerConfigChanged()
}

func (bc *BuildController) imageConfigDeleted(obj interface{}) {
	bc.controllerConfigChanged()
}

func (bc *BuildController) controllerConfigChanged() {
	bc.controllerConfigQueue.Add("openshift-config")
}

func (bc *BuildController) configMapAdded(obj interface{}) {
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("object was not a configMap: %+v", obj))
		return
	}
	if configMap == nil {
		return
	}
	config, err := bc.buildControllerConfigLister.Get("cluster")
	if err != nil && !errors.IsNotFound(err) {
		utilruntime.HandleError(fmt.Errorf("could not get cluster build controller config: %v", err))
		return
	}
	if config == nil {
		return
	}
	if configMap.Name != config.Spec.AdditionalTrustedCA.Name {
		return
	}
	bc.controllerConfigChanged()
}

func (bc *BuildController) configMapUpdated(old, curr interface{}) {
	configMap, ok := curr.(*corev1.ConfigMap)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("object was not a configMap: %+v", curr))
		return
	}
	if configMap == nil {
		return
	}
	config, err := bc.buildControllerConfigLister.Get("cluster")
	if err != nil && !errors.IsNotFound(err) {
		utilruntime.HandleError(fmt.Errorf("could not get cluster build controller config: %v", err))
		return
	}
	if config == nil {
		return
	}
	if configMap.Name != config.Spec.AdditionalTrustedCA.Name {
		return
	}
	bc.controllerConfigChanged()
}

func (bc *BuildController) configMapDeleted(obj interface{}) {
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone: %+v", obj))
			return
		}
		configMap, ok = tombstone.Obj.(*corev1.ConfigMap)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a configMap: %+v", obj))
			return
		}
	}
	config, err := bc.buildControllerConfigLister.Get("cluster")
	if err != nil && !errors.IsNotFound(err) {
		utilruntime.HandleError(fmt.Errorf("could not get cluster build controller config: %v", err))
		return
	}
	if config == nil {
		return
	}
	if configMap.Name != config.Spec.AdditionalTrustedCA.Name {
		return
	}
	bc.controllerConfigChanged()
}

// isBuildPod returns true if the given pod is a build pod
func isBuildPod(pod *corev1.Pod) bool {
	return len(getBuildName(pod)) > 0
}

// buildDesc is a utility to format the namespace/name and phase of a build
// for errors and logging
func buildDesc(build *buildv1.Build) string {
	return fmt.Sprintf("%s/%s (%s)", build.Namespace, build.Name, build.Status.Phase)
}

// transitionToPhase returns a buildUpdate object to transition a build to a new
// phase with the given reason and message
func transitionToPhase(phase buildv1.BuildPhase, reason buildv1.StatusReason, message string) *buildUpdate {
	update := &buildUpdate{}
	update.setPhase(phase)
	update.setReason(reason)
	update.setMessage(message)
	return update
}

// isValidTransition returns true if the given phase transition is valid
func isValidTransition(from, to buildv1.BuildPhase) bool {
	if from == to {
		return true
	}

	switch {
	case buildutil.IsTerminalPhase(from):
		return false
	case from == buildv1.BuildPhasePending:
		switch to {
		case buildv1.BuildPhaseNew:
			return false
		}
	case from == buildv1.BuildPhaseRunning:
		switch to {
		case buildv1.BuildPhaseNew,
			buildv1.BuildPhasePending:
			return false
		}
	}

	return true
}

// setBuildCompletionData sets the build completion time and duration as well as the start time
// if not already set on the given buildUpdate object.  It also sets the log tail data
// if applicable.
func setBuildCompletionData(build *buildv1.Build, pod *corev1.Pod, update *buildUpdate) {
	now := metav1.Now()

	startTime := build.Status.StartTimestamp
	if startTime == nil {
		if pod != nil {
			startTime = pod.Status.StartTime
		}

		if startTime == nil {
			startTime = &now
		}
		update.setStartTime(*startTime)
	}
	if build.Status.CompletionTimestamp == nil {
		update.setCompletionTime(now)
		update.setDuration(now.Rfc3339Copy().Time.Sub(startTime.Rfc3339Copy().Time))
	}

	if (build.Status.Phase == buildv1.BuildPhaseFailed || (update.phase != nil && *update.phase == buildv1.BuildPhaseFailed)) && len(build.Status.LogSnippet) == 0 &&
		pod != nil && len(pod.Status.ContainerStatuses) != 0 && pod.Status.ContainerStatuses[0].State.Terminated != nil {
		msg := pod.Status.ContainerStatuses[0].State.Terminated.Message
		if len(msg) != 0 {
			parts := strings.Split(strings.TrimRight(msg, "\n"), "\n")

			excerptLength := maxExcerptLength
			if len(parts) < maxExcerptLength {
				excerptLength = len(parts)
			}
			excerpt := parts[len(parts)-excerptLength:]
			for i, line := range excerpt {
				if len(line) > 120 {
					excerpt[i] = line[:58] + "..." + line[len(line)-59:]
				}
			}
			msg = strings.Join(excerpt, "\n")
			update.setLogSnippet(msg)
		}
	}

}

// hasError returns true if any error (aggregate or no) matches any of the
// provided functions.
func hasError(err error, fns ...utilerrors.Matcher) bool {
	if err == nil {
		return false
	}
	if agg, ok := err.(utilerrors.Aggregate); ok {
		for _, err := range agg.Errors() {
			if hasError(err, fns...) {
				return true
			}
		}
		return false
	}
	for _, fn := range fns {
		if fn(err) {
			return true
		}
	}
	return false
}

// getBuildName returns name of the build pod.
func getBuildName(pod metav1.Object) string {
	if pod == nil {
		return ""
	}
	return pod.GetAnnotations()[buildutil.BuildAnnotation]
}

func makeBuildPodOwnerRef(buildPod *corev1.Pod) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       buildPod.Name,
		UID:        buildPod.UID,
	}
}

func hasBuildPodOwnerRef(buildPod *corev1.Pod, caMap *corev1.ConfigMap) bool {
	ref := makeBuildPodOwnerRef(buildPod)
	for _, owner := range caMap.OwnerReferences {
		if reflect.DeepEqual(ref, owner) {
			return true
		}
	}
	return false
}
