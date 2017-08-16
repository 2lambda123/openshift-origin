package prune

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/golang/glog"
	gonum "github.com/gonum/graph"

	kmeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	deploygraph "github.com/openshift/origin/pkg/deploy/graph/nodes"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
	"github.com/openshift/origin/pkg/util/netutils"
)

// TODO these edges should probably have an `Add***Edges` method in images/graph and be moved there
const (
	// ReferencedImageEdgeKind defines a "strong" edge where the tail is an
	// ImageNode, with strong indicating that the ImageNode tail is not a
	// candidate for pruning.
	ReferencedImageEdgeKind = "ReferencedImage"
	// WeakReferencedImageEdgeKind defines a "weak" edge where the tail is
	// an ImageNode, with weak indicating that this particular edge does
	// not keep an ImageNode from being a candidate for pruning.
	WeakReferencedImageEdgeKind = "WeakReferencedImage"

	// ReferencedImageConfigEdgeKind defines an edge from an ImageStreamNode or an
	// ImageNode to an ImageComponentNode.
	ReferencedImageConfigEdgeKind = "ReferencedImageConfig"

	// ReferencedImageLayerEdgeKind defines an edge from an ImageStreamNode or an
	// ImageNode to an ImageComponentNode.
	ReferencedImageLayerEdgeKind = "ReferencedImageLayer"
)

// pruneAlgorithm contains the various settings to use when evaluating images
// and layers for pruning.
type pruneAlgorithm struct {
	keepYoungerThan    time.Duration
	keepTagRevisions   int
	pruneOverSizeLimit bool
	namespace          string
	allImages          bool
}

// ImageDeleter knows how to remove images from OpenShift.
type ImageDeleter interface {
	// DeleteImage removes the image from OpenShift's storage.
	DeleteImage(image *imageapi.Image) error
}

// ImageStreamDeleter knows how to remove an image reference from an image stream.
type ImageStreamDeleter interface {
	// DeleteImageStream removes all references to the image from the image
	// stream's status.tags. The updated image stream is returned.
	DeleteImageStream(stream *imageapi.ImageStream, image *imageapi.Image, updatedTags []string) (*imageapi.ImageStream, error)
}

// BlobDeleter knows how to delete a blob from the Docker registry.
type BlobDeleter interface {
	// DeleteBlob uses registryClient to ask the registry at registryURL
	// to remove the blob.
	DeleteBlob(ctx context.Context, registryClient *http.Client, registryURL, blob string) error
}

// LayerLinkDeleter knows how to delete a repository layer link from the Docker registry.
type LayerLinkDeleter interface {
	// DeleteLayerLink uses registryClient to ask the registry at registryURL to
	// delete the repository layer link.
	DeleteLayerLink(ctx context.Context, registryClient *http.Client, registryURL, repo, linkName string) error
}

// ManifestDeleter knows how to delete image manifest data for a repository from
// the Docker registry.
type ManifestDeleter interface {
	// DeleteManifest uses registryClient to ask the registry at registryURL to
	// delete the repository's image manifest data.
	DeleteManifest(ctx context.Context, registryClient *http.Client, registryURL, repo, manifest string) error
}

// PrunerOptions contains the fields used to initialize a new Pruner.
type PrunerOptions struct {
	// KeepYoungerThan indicates the minimum age an Image must be to be a
	// candidate for pruning.
	KeepYoungerThan *time.Duration
	// KeepTagRevisions is the minimum number of tag revisions to preserve;
	// revisions older than this value are candidates for pruning.
	KeepTagRevisions *int
	// PruneOverSizeLimit indicates that images exceeding defined limits (openshift.io/Image)
	// will be considered as candidates for pruning.
	PruneOverSizeLimit *bool
	// AllImages considers all images for pruning, not just those pushed directly to the registry.
	// Requires RegistryURL be set.
	AllImages *bool
	// Namespace to be pruned, if specified it should never remove Images.
	Namespace string
	// Images is the entire list of images in OpenShift. An image must be in this
	// list to be a candidate for pruning.
	Images *imageapi.ImageList
	// Streams is the entire list of image streams across all namespaces in the
	// cluster.
	Streams *imageapi.ImageStreamList
	// Pods is the entire list of pods across all namespaces in the cluster.
	Pods *kapi.PodList
	// RCs is the entire list of replication controllers across all namespaces in
	// the cluster.
	RCs *kapi.ReplicationControllerList
	// BCs is the entire list of build configs across all namespaces in the
	// cluster.
	BCs *buildapi.BuildConfigList
	// Builds is the entire list of builds across all namespaces in the cluster.
	Builds *buildapi.BuildList
	// DCs is the entire list of deployment configs across all namespaces in the
	// cluster.
	DCs *deployapi.DeploymentConfigList
	// LimitRanges is a map of LimitRanges across namespaces, being keys in this map.
	LimitRanges map[string][]*kapi.LimitRange
	// DryRun indicates that no changes will be made to the cluster and nothing
	// will be removed.
	DryRun bool
	// RegistryClient is the http.Client to use when contacting the registry.
	RegistryClient *http.Client
	// RegistryURL is the URL for the registry.
	RegistryURL string
	// Allow a fallback to insecure transport when contacting the registry.
	Insecure bool
}

// Pruner knows how to prune istags, images, layers and image configs.
type Pruner interface {
	// Prune uses imagePruner, streamPruner, layerLinkPruner, blobPruner, and
	// manifestPruner to remove images that have been identified as candidates
	// for pruning based on the Pruner's internal pruning algorithm.
	// Please see NewPruner for details on the algorithm.
	Prune(ctx context.Context, imagePruner ImageDeleter, streamPruner ImageStreamDeleter, layerLinkPruner LayerLinkDeleter, blobPruner BlobDeleter, manifestPruner ManifestDeleter) error
}

// pruner is an object that knows how to prune a data set
type pruner struct {
	g              graph.Graph
	algorithm      pruneAlgorithm
	registryPinger registryPinger
	registryClient *http.Client
	registryURL    string
}

var _ Pruner = &pruner{}

// registryPinger performs a health check against a registry.
type registryPinger interface {
	// ping performs a health check against registry.
	ping(registry string) error
}

// defaultRegistryPinger implements registryPinger.
type defaultRegistryPinger struct {
	client   *http.Client
	insecure bool
}

func (drp *defaultRegistryPinger) ping(registry string) error {
	healthCheck := func(proto, registry string) error {
		// TODO: `/healthz` route is deprecated by `/`; remove it in future versions
		healthResponse, err := drp.client.Get(fmt.Sprintf("%s://%s/healthz", proto, registry))
		if err != nil {
			return err
		}
		defer healthResponse.Body.Close()

		if healthResponse.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status: %s", healthResponse.Status)
		}

		return nil
	}

	var errs []error
	protos := make([]string, 0, 2)
	protos = append(protos, "https")
	if drp.insecure || netutils.IsPrivateAddress(registry) {
		protos = append(protos, "http")
	}
	for _, proto := range protos {
		glog.V(4).Infof("Trying %s for %s", proto, registry)
		err := healthCheck(proto, registry)
		if err == nil {
			return nil
		}
		errs = append(errs, err)
		glog.V(4).Infof("Error with %s for %s: %v", proto, registry, err)
	}

	return kerrors.NewAggregate(errs)
}

// dryRunRegistryPinger implements registryPinger.
type dryRunRegistryPinger struct {
}

func (*dryRunRegistryPinger) ping(registry string) error {
	return nil
}

// NewPruner creates a Pruner.
//
// Images younger than keepYoungerThan and images referenced by image streams
// and/or pods younger than keepYoungerThan are preserved. All other images are
// candidates for pruning. For example, if keepYoungerThan is 60m, and an
// ImageStream is only 59 minutes old, none of the images it references are
// eligible for pruning.
//
// keepTagRevisions is the number of revisions per tag in an image stream's
// status.tags that are preserved and ineligible for pruning. Any revision older
// than keepTagRevisions is eligible for pruning.
//
// pruneOverSizeLimit is a boolean flag speyfing that all images exceeding limits
// defined in their namespace will be considered for pruning. Important to note is
// the fact that this flag does not work in any combination with the keep* flags.
//
// images, streams, pods, rcs, bcs, builds, and dcs are the resources used to run
// the pruning algorithm. These should be the full list for each type from the
// cluster; otherwise, the pruning algorithm might result in incorrect
// calculations and premature pruning.
//
// The ImageDeleter performs the following logic:
//
// remove any image that was created at least *n* minutes ago and is *not*
// currently referenced by:
//
// - any pod created less than *n* minutes ago
// - any image stream created less than *n* minutes ago
// - any running pods
// - any pending pods
// - any replication controllers
// - any deployment configs
// - any build configs
// - any builds
// - the n most recent tag revisions in an image stream's status.tags
//
// including only images with the annotation openshift.io/image.managed=true
// unless allImages is true.
//
// When removing an image, remove all references to the image from all
// ImageStreams having a reference to the image in `status.tags`.
//
// Also automatically remove any image layer that is no longer referenced by any
// images.
func NewPruner(options PrunerOptions) Pruner {
	glog.V(1).Infof("Creating image pruner with keepYoungerThan=%v, keepTagRevisions=%s, pruneOverSizeLimit=%s, allImages=%s",
		options.KeepYoungerThan, getValue(options.KeepTagRevisions), getValue(options.PruneOverSizeLimit), getValue(options.AllImages))

	algorithm := pruneAlgorithm{}
	if options.KeepYoungerThan != nil {
		algorithm.keepYoungerThan = *options.KeepYoungerThan
	}
	if options.KeepTagRevisions != nil {
		algorithm.keepTagRevisions = *options.KeepTagRevisions
	}
	if options.PruneOverSizeLimit != nil {
		algorithm.pruneOverSizeLimit = *options.PruneOverSizeLimit
	}
	algorithm.allImages = true
	if options.AllImages != nil {
		algorithm.allImages = *options.AllImages
	}
	algorithm.namespace = options.Namespace

	g := graph.New()
	addImagesToGraph(g, options.Images, algorithm)
	addImageStreamsToGraph(g, options.Streams, options.LimitRanges, algorithm)
	addPodsToGraph(g, options.Pods, algorithm)
	addReplicationControllersToGraph(g, options.RCs)
	addBuildConfigsToGraph(g, options.BCs)
	addBuildsToGraph(g, options.Builds)
	addDeploymentConfigsToGraph(g, options.DCs)

	var rp registryPinger
	if options.DryRun {
		rp = &dryRunRegistryPinger{}
	} else {
		rp = &defaultRegistryPinger{
			client:   options.RegistryClient,
			insecure: options.Insecure,
		}
	}

	return &pruner{
		g:              g,
		algorithm:      algorithm,
		registryPinger: rp,
		registryClient: options.RegistryClient,
		registryURL:    options.RegistryURL,
	}
}

func getValue(option interface{}) string {
	if v := reflect.ValueOf(option); !v.IsNil() {
		return fmt.Sprintf("%v", v.Elem())
	}
	return "<nil>"
}

// addImagesToGraph adds all images to the graph that belong to one of the
// registries in the algorithm and are at least as old as the minimum age
// threshold as specified by the algorithm. It also adds all the images' layers
// to the graph.
func addImagesToGraph(g graph.Graph, images *imageapi.ImageList, algorithm pruneAlgorithm) {
	for i := range images.Items {
		image := &images.Items[i]

		glog.V(4).Infof("Examining image %q", image.Name)

		if !algorithm.allImages {
			if image.Annotations[imageapi.ManagedByOpenShiftAnnotation] != "true" {
				glog.V(4).Infof("Image %q with DockerImageReference %q belongs to an external registry - skipping", image.Name, image.DockerImageReference)
				continue
			}
		}

		age := metav1.Now().Sub(image.CreationTimestamp.Time)
		if !algorithm.pruneOverSizeLimit && age < algorithm.keepYoungerThan {
			glog.V(4).Infof("Image %q is younger than minimum pruning age, skipping (age=%v)", image.Name, age)
			continue
		}

		glog.V(4).Infof("Adding image %q to graph", image.Name)
		imageNode := imagegraph.EnsureImageNode(g, image)

		if image.DockerImageManifestMediaType == schema2.MediaTypeManifest && len(image.DockerImageMetadata.ID) > 0 {
			configName := image.DockerImageMetadata.ID
			glog.V(4).Infof("Adding image config %q to graph", configName)
			configNode := imagegraph.EnsureImageComponentConfigNode(g, configName)
			g.AddEdge(imageNode, configNode, ReferencedImageConfigEdgeKind)
		}

		for _, layer := range image.DockerImageLayers {
			glog.V(4).Infof("Adding image layer %q to graph", layer.Name)
			layerNode := imagegraph.EnsureImageComponentLayerNode(g, layer.Name)
			g.AddEdge(imageNode, layerNode, ReferencedImageLayerEdgeKind)
		}
	}
}

// addImageStreamsToGraph adds all the streams to the graph. The most recent n
// image revisions for a tag will be preserved, where n is specified by the
// algorithm's keepTagRevisions. Image revisions older than n are candidates
// for pruning if the image stream's age is at least as old as the minimum
// threshold in algorithm.  Otherwise, if the image stream is younger than the
// threshold, all image revisions for that stream are ineligible for pruning.
// If pruneOverSizeLimit flag is set to true, above does not matter, instead
// all images size is checked against LimitRanges defined in that same namespace,
// and whenever its size exceeds the smallest limit in that namespace, it will be
// considered a candidate for pruning.
//
// addImageStreamsToGraph also adds references from each stream to all the
// layers it references (via each image a stream references).
func addImageStreamsToGraph(g graph.Graph, streams *imageapi.ImageStreamList, limits map[string][]*kapi.LimitRange, algorithm pruneAlgorithm) {
	for i := range streams.Items {
		stream := &streams.Items[i]

		glog.V(4).Infof("Examining ImageStream %s", getName(stream))

		// use a weak reference for old image revisions by default
		oldImageRevisionReferenceKind := WeakReferencedImageEdgeKind

		age := metav1.Now().Sub(stream.CreationTimestamp.Time)
		if !algorithm.pruneOverSizeLimit && age < algorithm.keepYoungerThan {
			// stream's age is below threshold - use a strong reference for old image revisions instead
			oldImageRevisionReferenceKind = ReferencedImageEdgeKind
		}

		glog.V(4).Infof("Adding ImageStream %s to graph", getName(stream))
		isNode := imagegraph.EnsureImageStreamNode(g, stream)
		imageStreamNode := isNode.(*imagegraph.ImageStreamNode)

		for tag, history := range stream.Status.Tags {
			for i := range history.Items {
				n := imagegraph.FindImage(g, history.Items[i].Image)
				if n == nil {
					glog.V(2).Infof("Unable to find image %q in graph (from tag=%q, revision=%d, dockerImageReference=%s) - skipping",
						history.Items[i].Image, tag, i, history.Items[i].DockerImageReference)
					continue
				}
				imageNode := n.(*imagegraph.ImageNode)

				kind := oldImageRevisionReferenceKind
				if algorithm.pruneOverSizeLimit {
					if exceedsLimits(stream, imageNode.Image, limits) {
						kind = WeakReferencedImageEdgeKind
					} else {
						kind = ReferencedImageEdgeKind
					}
				} else {
					if i < algorithm.keepTagRevisions {
						kind = ReferencedImageEdgeKind
					}
				}

				glog.V(4).Infof("Checking for existing strong reference from stream %s to image %s", getName(stream), imageNode.Image.Name)
				if edge := g.Edge(imageStreamNode, imageNode); edge != nil && g.EdgeKinds(edge).Has(ReferencedImageEdgeKind) {
					glog.V(4).Infof("Strong reference found")
					continue
				}

				glog.V(4).Infof("Adding edge (kind=%s) from %q to %q", kind, imageStreamNode.UniqueName(), imageNode.UniqueName())
				g.AddEdge(imageStreamNode, imageNode, kind)

				glog.V(4).Infof("Adding stream->(layer|config) references")
				// add stream -> layer references so we can prune them later
				for _, s := range g.From(imageNode) {
					cn, ok := s.(*imagegraph.ImageComponentNode)
					if !ok {
						continue
					}

					glog.V(4).Infof("Adding reference from stream %s to %s", getName(stream), cn.Describe())
					if cn.Type == imagegraph.ImageComponentTypeConfig {
						g.AddEdge(imageStreamNode, s, ReferencedImageConfigEdgeKind)
					} else {
						g.AddEdge(imageStreamNode, s, ReferencedImageLayerEdgeKind)
					}
				}
			}
		}
	}
}

// exceedsLimits checks if given image exceeds LimitRanges defined in ImageStream's namespace.
func exceedsLimits(is *imageapi.ImageStream, image *imageapi.Image, limits map[string][]*kapi.LimitRange) bool {
	limitRanges, ok := limits[is.Namespace]
	if !ok || len(limitRanges) == 0 {
		return false
	}

	imageSize := resource.NewQuantity(image.DockerImageMetadata.Size, resource.BinarySI)
	for _, limitRange := range limitRanges {
		if limitRange == nil {
			continue
		}
		for _, limit := range limitRange.Spec.Limits {
			if limit.Type != imageapi.LimitTypeImage {
				continue
			}

			limitQuantity, ok := limit.Max[kapi.ResourceStorage]
			if !ok {
				continue
			}
			if limitQuantity.Cmp(*imageSize) < 0 {
				// image size is larger than the permitted limit range max size
				glog.V(4).Infof("Image %s in stream %s exceeds limit %s: %v vs %v",
					image.Name, getName(is), limitRange.Name, *imageSize, limitQuantity)
				return true
			}
		}
	}
	return false
}

// addPodsToGraph adds pods to the graph.
//
// Edges are added to the graph from each pod to the images specified by that
// pod's list of containers, as long as the image is managed by OpenShift.
func addPodsToGraph(g graph.Graph, pods *kapi.PodList, algorithm pruneAlgorithm) {
	for i := range pods.Items {
		pod := &pods.Items[i]

		glog.V(4).Infof("Examining pod %s", getName(pod))

		// A pod is only *excluded* from being added to the graph if its phase is not
		// pending or running. Additionally, it has to be at least as old as the minimum
		// age threshold defined by the algorithm.
		if pod.Status.Phase != kapi.PodRunning && pod.Status.Phase != kapi.PodPending {
			age := metav1.Now().Sub(pod.CreationTimestamp.Time)
			if age >= algorithm.keepYoungerThan {
				glog.V(4).Infof("Pod %s is not running nor pending and age exceeds keepYoungerThan (%v) - skipping", getName(pod), age)
				continue
			}
		}

		glog.V(4).Infof("Adding pod %s to graph", getName(pod))
		podNode := kubegraph.EnsurePodNode(g, pod)

		addPodSpecToGraph(g, &pod.Spec, podNode)
	}
}

// Edges are added to the graph from each predecessor (pod or replication
// controller) to the images specified by the pod spec's list of containers, as
// long as the image is managed by OpenShift.
func addPodSpecToGraph(g graph.Graph, spec *kapi.PodSpec, predecessor gonum.Node) {
	for j := range spec.Containers {
		container := spec.Containers[j]

		glog.V(4).Infof("Examining container image %q", container.Image)

		ref, err := imageapi.ParseDockerImageReference(container.Image)
		if err != nil {
			glog.V(2).Infof("Unable to parse DockerImageReference %q: %v - skipping", container.Image, err)
			continue
		}

		if len(ref.ID) == 0 {
			glog.V(4).Infof("%q has no image ID", container.Image)
			continue
		}

		imageNode := imagegraph.FindImage(g, ref.ID)
		if imageNode == nil {
			glog.V(2).Infof("Unable to find image %q in the graph - skipping", ref.ID)
			continue
		}

		glog.V(4).Infof("Adding edge from pod to image")
		g.AddEdge(predecessor, imageNode, ReferencedImageEdgeKind)
	}
}

// addReplicationControllersToGraph adds replication controllers to the graph.
//
// Edges are added to the graph from each replication controller to the images
// specified by its pod spec's list of containers, as long as the image is
// managed by OpenShift.
func addReplicationControllersToGraph(g graph.Graph, rcs *kapi.ReplicationControllerList) {
	for i := range rcs.Items {
		rc := &rcs.Items[i]
		glog.V(4).Infof("Examining replication controller %s", getName(rc))
		rcNode := kubegraph.EnsureReplicationControllerNode(g, rc)
		addPodSpecToGraph(g, &rc.Spec.Template.Spec, rcNode)
	}
}

// addDeploymentConfigsToGraph adds deployment configs to the graph.
//
// Edges are added to the graph from each deployment config to the images
// specified by its pod spec's list of containers, as long as the image is
// managed by OpenShift.
func addDeploymentConfigsToGraph(g graph.Graph, dcs *deployapi.DeploymentConfigList) {
	for i := range dcs.Items {
		dc := &dcs.Items[i]
		glog.V(4).Infof("Examining DeploymentConfig %s", getName(dc))
		dcNode := deploygraph.EnsureDeploymentConfigNode(g, dc)
		addPodSpecToGraph(g, &dc.Spec.Template.Spec, dcNode)
	}
}

// addBuildConfigsToGraph adds build configs to the graph.
//
// Edges are added to the graph from each build config to the image specified by its strategy.from.
func addBuildConfigsToGraph(g graph.Graph, bcs *buildapi.BuildConfigList) {
	for i := range bcs.Items {
		bc := &bcs.Items[i]
		glog.V(4).Infof("Examining BuildConfig %s", getName(bc))
		bcNode := buildgraph.EnsureBuildConfigNode(g, bc)
		addBuildStrategyImageReferencesToGraph(g, bc.Spec.Strategy, bcNode)
	}
}

// addBuildsToGraph adds builds to the graph.
//
// Edges are added to the graph from each build to the image specified by its strategy.from.
func addBuildsToGraph(g graph.Graph, builds *buildapi.BuildList) {
	for i := range builds.Items {
		build := &builds.Items[i]
		glog.V(4).Infof("Examining build %s", getName(build))
		buildNode := buildgraph.EnsureBuildNode(g, build)
		addBuildStrategyImageReferencesToGraph(g, build.Spec.Strategy, buildNode)
	}
}

// addBuildStrategyImageReferencesToGraph ads references from the build strategy's parent node to the image
// the build strategy references.
//
// Edges are added to the graph from each predecessor (build or build config)
// to the image specified by strategy.from, as long as the image is managed by
// OpenShift.
func addBuildStrategyImageReferencesToGraph(g graph.Graph, strategy buildapi.BuildStrategy, predecessor gonum.Node) {
	from := buildutil.GetInputReference(strategy)
	if from == nil {
		glog.V(4).Infof("Unable to determine 'from' reference - skipping")
		return
	}

	glog.V(4).Infof("Examining build strategy with from: %#v", from)

	var imageID string

	switch from.Kind {
	case "ImageStreamImage":
		_, id, err := imageapi.ParseImageStreamImageName(from.Name)
		if err != nil {
			glog.V(2).Infof("Error parsing ImageStreamImage name %q: %v - skipping", from.Name, err)
			return
		}
		imageID = id
	case "DockerImage":
		ref, err := imageapi.ParseDockerImageReference(from.Name)
		if err != nil {
			glog.V(2).Infof("Error parsing DockerImage name %q: %v - skipping", from.Name, err)
			return
		}
		imageID = ref.ID
	default:
		return
	}

	glog.V(4).Infof("Looking for image %q in graph", imageID)
	imageNode := imagegraph.FindImage(g, imageID)
	if imageNode == nil {
		glog.V(4).Infof("Unable to find image %q in graph - skipping", imageID)
		return
	}

	glog.V(4).Infof("Adding edge from %v to %v", predecessor, imageNode)
	g.AddEdge(predecessor, imageNode, ReferencedImageEdgeKind)
}

// getImageNodes returns only nodes of type ImageNode.
func getImageNodes(nodes []gonum.Node) []*imagegraph.ImageNode {
	ret := []*imagegraph.ImageNode{}
	for i := range nodes {
		if node, ok := nodes[i].(*imagegraph.ImageNode); ok {
			ret = append(ret, node)
		}
	}
	return ret
}

// getImageStreamNodes returns only nodes of type ImageStreamNode.
func getImageStreamNodes(nodes []gonum.Node) []*imagegraph.ImageStreamNode {
	ret := []*imagegraph.ImageStreamNode{}
	for i := range nodes {
		if node, ok := nodes[i].(*imagegraph.ImageStreamNode); ok {
			ret = append(ret, node)
		}
	}
	return ret
}

// edgeKind returns true if the edge from "from" to "to" is of the desired kind.
func edgeKind(g graph.Graph, from, to gonum.Node, desiredKind string) bool {
	edge := g.Edge(from, to)
	kinds := g.EdgeKinds(edge)
	return kinds.Has(desiredKind)
}

// imageIsPrunable returns true iff the image node only has weak references
// from its predecessors to it. A weak reference to an image is a reference
// from an image stream to an image where the image is not the current image
// for a tag and the image stream is at least as old as the minimum pruning
// age.
func imageIsPrunable(g graph.Graph, imageNode *imagegraph.ImageNode) bool {
	for _, n := range g.To(imageNode) {
		glog.V(4).Infof("Examining predecessor %#v", n)
		if edgeKind(g, n, imageNode, ReferencedImageEdgeKind) {
			glog.V(4).Infof("Strong reference detected")
			return false
		}
	}

	return true
}

// calculatePrunableImages returns the list of prunable images and a
// graph.NodeSet containing the image node IDs.
func calculatePrunableImages(g graph.Graph, imageNodes []*imagegraph.ImageNode) ([]*imagegraph.ImageNode, graph.NodeSet) {
	prunable := []*imagegraph.ImageNode{}
	ids := make(graph.NodeSet)

	for _, imageNode := range imageNodes {
		glog.V(4).Infof("Examining image %q", imageNode.Image.Name)

		if imageIsPrunable(g, imageNode) {
			glog.V(4).Infof("Image %q is prunable", imageNode.Image.Name)
			prunable = append(prunable, imageNode)
			ids.Add(imageNode.ID())
		}
	}

	return prunable, ids
}

// subgraphWithoutPrunableImages creates a subgraph from g with prunable image
// nodes excluded.
func subgraphWithoutPrunableImages(g graph.Graph, prunableImageIDs graph.NodeSet) graph.Graph {
	return g.Subgraph(
		func(g graph.Interface, node gonum.Node) bool {
			return !prunableImageIDs.Has(node.ID())
		},
		func(g graph.Interface, from, to gonum.Node, edgeKinds sets.String) bool {
			if prunableImageIDs.Has(from.ID()) {
				return false
			}
			if prunableImageIDs.Has(to.ID()) {
				return false
			}
			return true
		},
	)
}

// calculatePrunableImageComponents returns the list of prunable image components.
func calculatePrunableImageComponents(g graph.Graph) []*imagegraph.ImageComponentNode {
	components := []*imagegraph.ImageComponentNode{}
	nodes := g.Nodes()

	for i := range nodes {
		cn, ok := nodes[i].(*imagegraph.ImageComponentNode)
		if !ok {
			continue
		}

		glog.V(4).Infof("Examining %v", cn)
		if imageComponentIsPrunable(g, cn) {
			glog.V(4).Infof("%v is prunable", cn)
			components = append(components, cn)
		}
	}

	return components
}

// pruneStreams removes references from all image streams' status.tags entries
// to prunable images, invoking streamPruner.DeleteImageStream for each updated
// stream.
func pruneStreams(g graph.Graph, imageNodes []*imagegraph.ImageNode, streamPruner ImageStreamDeleter) []error {
	errs := []error{}

	glog.V(4).Infof("Removing pruned image references from streams")
	for _, imageNode := range imageNodes {
		for _, n := range g.To(imageNode) {
			streamNode, ok := n.(*imagegraph.ImageStreamNode)
			if !ok {
				continue
			}

			stream := streamNode.ImageStream
			updatedTags := sets.NewString()

			glog.V(4).Infof("Checking if ImageStream %s has references to image %s in status.tags", getName(stream), imageNode.Image.Name)

			for tag, history := range stream.Status.Tags {
				glog.V(4).Infof("Checking tag %q", tag)

				newHistory := imageapi.TagEventList{}

				for i, tagEvent := range history.Items {
					glog.V(4).Infof("Checking tag event %d with image %q", i, tagEvent.Image)

					if tagEvent.Image != imageNode.Image.Name {
						glog.V(4).Infof("Tag event doesn't match deleted image - keeping")
						newHistory.Items = append(newHistory.Items, tagEvent)
					} else {
						glog.V(4).Infof("Tag event matches deleted image - removing reference")
						updatedTags.Insert(tag)
					}
				}
				if len(newHistory.Items) == 0 {
					glog.V(4).Infof("Removing tag %q from status.tags of ImageStream %s", tag, getName(stream))
					delete(stream.Status.Tags, tag)
				} else {
					stream.Status.Tags[tag] = newHistory
				}
			}

			updatedStream, err := streamPruner.DeleteImageStream(stream, imageNode.Image, updatedTags.List())
			if err != nil {
				errs = append(errs, fmt.Errorf("error removing image %s from stream %s: %v",
					imageNode.Image.Name, getName(stream), err))
				continue
			}

			streamNode.ImageStream = updatedStream
		}
	}
	glog.V(4).Infof("Done removing pruned image references from streams")
	return errs
}

// pruneImages invokes imagePruner.DeleteImage with each image that is prunable.
func pruneImages(g graph.Graph, imageNodes []*imagegraph.ImageNode, imagePruner ImageDeleter) []error {
	errs := []error{}

	for _, imageNode := range imageNodes {
		if err := imagePruner.DeleteImage(imageNode.Image); err != nil {
			errs = append(errs, fmt.Errorf("error removing image %q: %v", imageNode.Image.Name, err))
		}
	}

	return errs
}

// order younger images before older
type imgByAge []*imageapi.Image

func (ba imgByAge) Len() int      { return len(ba) }
func (ba imgByAge) Swap(i, j int) { ba[i], ba[j] = ba[j], ba[i] }
func (ba imgByAge) Less(i, j int) bool {
	return ba[i].CreationTimestamp.After(ba[j].CreationTimestamp.Time)
}

// order younger image stream before older
type isByAge []*imagegraph.ImageStreamNode

func (ba isByAge) Len() int      { return len(ba) }
func (ba isByAge) Swap(i, j int) { ba[i], ba[j] = ba[j], ba[i] }
func (ba isByAge) Less(i, j int) bool {
	return ba[i].ImageStream.CreationTimestamp.After(ba[j].ImageStream.CreationTimestamp.Time)
}

func (p *pruner) determineRegistry(imageNodes []*imagegraph.ImageNode, isNodes []*imagegraph.ImageStreamNode) (string, error) {
	if len(p.registryURL) > 0 {
		return p.registryURL, nil
	}

	var pullSpec string
	var managedImages []*imageapi.Image

	// 1st try to determine registry url from a pull spec of the youngest managed image
	for _, node := range imageNodes {
		if node.Image.Annotations[imageapi.ManagedByOpenShiftAnnotation] != "true" {
			continue
		}
		managedImages = append(managedImages, node.Image)
	}
	// be sure to pick up the newest managed image which should have an up to date information
	sort.Sort(imgByAge(managedImages))

	if len(managedImages) > 0 {
		pullSpec = managedImages[0].DockerImageReference
	} else {
		// 2nd try to get the pull spec from any image stream
		// Sorting by creation timestamp may not get us up to date info. Modification time would be much
		// better if there were such an attribute.
		sort.Sort(isByAge(isNodes))
		for _, node := range isNodes {
			if len(node.ImageStream.Status.DockerImageRepository) == 0 {
				continue
			}
			pullSpec = node.ImageStream.Status.DockerImageRepository
		}
	}

	if len(pullSpec) == 0 {
		return "", fmt.Errorf("no managed image found")
	}

	ref, err := imageapi.ParseDockerImageReference(pullSpec)
	if err != nil {
		return "", fmt.Errorf("unable to parse %q: %v", pullSpec, err)
	}

	if len(ref.Registry) == 0 {
		return "", fmt.Errorf("%s does not include a registry", pullSpec)
	}

	return ref.Registry, nil
}

// Run identifies images eligible for pruning, invoking imagePruner for each image, and then it identifies
// image configs and layers  eligible for pruning, invoking layerLinkPruner for each registry URL that has
// layers or configs that can be pruned.
func (p *pruner) Prune(
	ctx context.Context,
	imagePruner ImageDeleter,
	streamPruner ImageStreamDeleter,
	layerLinkPruner LayerLinkDeleter,
	blobPruner BlobDeleter,
	manifestPruner ManifestDeleter,
) error {
	allNodes := p.g.Nodes()

	imageNodes := getImageNodes(allNodes)
	if len(imageNodes) == 0 {
		return nil
	}

	registryURL, err := p.determineRegistry(imageNodes, getImageStreamNodes(allNodes))
	if err != nil {
		return fmt.Errorf("unable to determine registry: %v", err)
	}
	glog.V(1).Infof("Using registry: %s", registryURL)

	if err := p.registryPinger.ping(registryURL); err != nil {
		return fmt.Errorf("error communicating with registry %s: %v", registryURL, err)
	}

	prunableImageNodes, prunableImageIDs := calculatePrunableImages(p.g, imageNodes)

	errs := []error{}
	errs = append(errs, pruneStreams(p.g, prunableImageNodes, streamPruner)...)
	// if namespace is specified prune only ImageStreams and nothing more
	if len(p.algorithm.namespace) > 0 {
		return kerrors.NewAggregate(errs)
	}

	graphWithoutPrunableImages := subgraphWithoutPrunableImages(p.g, prunableImageIDs)
	prunableComponents := calculatePrunableImageComponents(graphWithoutPrunableImages)
	errs = append(errs, pruneImageComponents(ctx, p.g, p.registryClient, registryURL, prunableComponents, layerLinkPruner)...)
	errs = append(errs, pruneBlobs(ctx, p.g, p.registryClient, registryURL, prunableComponents, blobPruner)...)
	errs = append(errs, pruneManifests(ctx, p.g, p.registryClient, registryURL, prunableImageNodes, manifestPruner)...)

	if len(errs) > 0 {
		// If we had any errors removing image references from image streams or deleting
		// layers, blobs, or manifest data from the registry, stop here and don't
		// delete any images. This way, you can rerun prune and retry things that failed.
		return kerrors.NewAggregate(errs)
	}

	errs = append(errs, pruneImages(p.g, prunableImageNodes, imagePruner)...)
	return kerrors.NewAggregate(errs)
}

// imageComponentIsPrunable returns true if the image component is not referenced by any images.
func imageComponentIsPrunable(g graph.Graph, cn *imagegraph.ImageComponentNode) bool {
	for _, predecessor := range g.To(cn) {
		glog.V(4).Infof("Examining predecessor %#v of image config %v", predecessor, cn)
		if g.Kind(predecessor) == imagegraph.ImageNodeKind {
			glog.V(4).Infof("Config %v has an image predecessor", cn)
			return false
		}
	}

	return true
}

// streamReferencingImageComponent returns a list of ImageStreamNodes that reference a
// given ImageComponentNode.
func streamsReferencingImageComponent(g graph.Graph, cn *imagegraph.ImageComponentNode) []*imagegraph.ImageStreamNode {
	ret := []*imagegraph.ImageStreamNode{}
	for _, predecessor := range g.To(cn) {
		if g.Kind(predecessor) != imagegraph.ImageStreamNodeKind {
			continue
		}
		ret = append(ret, predecessor.(*imagegraph.ImageStreamNode))
	}

	return ret
}

// pruneImageComponents invokes layerLinkDeleter.DeleteLayerLink for each repository layer link to
// be deleted from the registry.
func pruneImageComponents(
	ctx context.Context,
	g graph.Graph,
	registryClient *http.Client,
	registryURL string,
	imageComponents []*imagegraph.ImageComponentNode,
	layerLinkDeleter LayerLinkDeleter,
) []error {
	errs := []error{}

	for _, cn := range imageComponents {
		// get streams that reference config
		streamNodes := streamsReferencingImageComponent(g, cn)

		for _, streamNode := range streamNodes {
			stream := streamNode.ImageStream
			streamName := fmt.Sprintf("%s/%s", stream.Namespace, stream.Name)

			glog.V(4).Infof("Pruning registry=%q, repo=%q, %s", registryURL, streamName, cn.Describe())
			if err := layerLinkDeleter.DeleteLayerLink(ctx, registryClient, registryURL, streamName, cn.Component); err != nil {
				errs = append(errs, fmt.Errorf("error pruning layer link %s in repo %q: %v", cn.Component, streamName, err))
			}
		}
	}

	return errs
}

// pruneBlobs invokes blobPruner.DeleteBlob for each blob to be deleted from the
// registry.
func pruneBlobs(
	ctx context.Context,
	g graph.Graph,
	registryClient *http.Client,
	registryURL string,
	componentNodes []*imagegraph.ImageComponentNode,
	blobPruner BlobDeleter,
) []error {
	errs := []error{}

	for _, cn := range componentNodes {
		if err := blobPruner.DeleteBlob(ctx, registryClient, registryURL, cn.Component); err != nil {
			errs = append(errs, fmt.Errorf("error removing blob from registry %s: blob %q: %v",
				registryURL, cn.Component, err))
		}
	}

	return errs
}

// pruneManifests invokes manifestPruner.DeleteManifest for each repository
// manifest to be deleted from the registry.
func pruneManifests(ctx context.Context, g graph.Graph, registryClient *http.Client, registryURL string, imageNodes []*imagegraph.ImageNode, manifestPruner ManifestDeleter) []error {
	errs := []error{}

	for _, imageNode := range imageNodes {
		for _, n := range g.To(imageNode) {
			streamNode, ok := n.(*imagegraph.ImageStreamNode)
			if !ok {
				continue
			}

			stream := streamNode.ImageStream
			repoName := fmt.Sprintf("%s/%s", stream.Namespace, stream.Name)

			glog.V(4).Infof("Pruning manifest for registry %q, repo %q, image %q", registryURL, repoName, imageNode.Image.Name)
			if err := manifestPruner.DeleteManifest(ctx, registryClient, registryURL, repoName, imageNode.Image.Name); err != nil {
				errs = append(errs, fmt.Errorf("error pruning manifest for registry %q, repo %q, image %q: %v", registryURL, repoName, imageNode.Image.Name, err))
			}
		}
	}

	return errs
}

// imageDeleter removes an image from OpenShift.
type imageDeleter struct {
	images client.ImageInterface
}

var _ ImageDeleter = &imageDeleter{}

// NewImageDeleter creates a new imageDeleter.
func NewImageDeleter(images client.ImageInterface) ImageDeleter {
	return &imageDeleter{
		images: images,
	}
}

func (p *imageDeleter) DeleteImage(image *imageapi.Image) error {
	glog.V(4).Infof("Deleting image %q", image.Name)
	return p.images.Delete(image.Name)
}

// imageStreamDeleter updates an image stream in OpenShift.
type imageStreamDeleter struct {
	streams client.ImageStreamsNamespacer
}

var _ ImageStreamDeleter = &imageStreamDeleter{}

// NewImageStreamDeleter creates a new imageStreamDeleter.
func NewImageStreamDeleter(streams client.ImageStreamsNamespacer) ImageStreamDeleter {
	return &imageStreamDeleter{
		streams: streams,
	}
}

func (p *imageStreamDeleter) DeleteImageStream(stream *imageapi.ImageStream, image *imageapi.Image, updatedTags []string) (*imageapi.ImageStream, error) {
	glog.V(4).Infof("Updating ImageStream %s", getName(stream))
	glog.V(5).Infof("Updated stream: %#v", stream)
	return p.streams.ImageStreams(stream.Namespace).UpdateStatus(stream)
}

// deleteFromRegistry uses registryClient to send a DELETE request to the
// provided url. It attempts an https request first; if that fails, it fails
// back to http.
func deleteFromRegistry(ctx context.Context, registryClient *http.Client, url string) error {
	deleteFunc := func(proto, url string) error {
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			return err
		}
		req = req.WithContext(ctx)

		glog.V(4).Infof("Sending request to registry")
		resp, err := registryClient.Do(req)
		if err != nil {
			if proto != "https" && strings.Contains(err.Error(), "malformed HTTP response") {
				return fmt.Errorf("%v.\n* Are you trying to connect to a TLS-enabled registry without TLS?", err)
			}
			return err
		}
		defer resp.Body.Close()

		// TODO: investigate why we're getting non-existent layers, for now we're logging
		// them out and continue working
		if resp.StatusCode == http.StatusNotFound {
			glog.Warningf("Unable to prune layer %s, returned %v", url, resp.Status)
			return nil
		}
		// non-2xx/3xx response doesn't cause an error, so we need to check for it
		// manually and return it to caller
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
			return fmt.Errorf(resp.Status)
		}
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
			glog.V(1).Infof("Unexpected status code in response: %d", resp.StatusCode)
			var response errcode.Errors
			decoder := json.NewDecoder(resp.Body)
			if err := decoder.Decode(&response); err != nil {
				return err
			}
			glog.V(1).Infof("Response: %#v", response)
			return &response
		}

		return nil
	}

	var err error
	for _, proto := range []string{"https", "http"} {
		glog.V(4).Infof("Trying %s for %s", proto, url)
		err = deleteFunc(proto, fmt.Sprintf("%s://%s", proto, url))
		if err == nil {
			return nil
		}

		if _, ok := err.(*errcode.Errors); ok {
			// we got a response back from the registry, so return it
			return err
		}

		// we didn't get a success or a errcode.Errors response back from the registry
		glog.V(4).Infof("Error with %s for %s: %v", proto, url, err)
	}
	return err
}

// layerLinkDeleter removes a repository layer link from the registry.
type layerLinkDeleter struct{}

var _ LayerLinkDeleter = &layerLinkDeleter{}

// NewLayerLinkDeleter creates a new layerLinkDeleter.
func NewLayerLinkDeleter() LayerLinkDeleter {
	return &layerLinkDeleter{}
}

func (p *layerLinkDeleter) DeleteLayerLink(ctx context.Context, registryClient *http.Client, registryURL, repoName, linkName string) error {
	glog.V(4).Infof("Deleting layer link from registry %q: repo %q, layer link %q", registryURL, repoName, linkName)
	return deleteFromRegistry(ctx, registryClient, fmt.Sprintf("%s/v2/%s/blobs/%s", registryURL, repoName, linkName))
}

// blobDeleter removes a blob from the registry.
type blobDeleter struct{}

var _ BlobDeleter = &blobDeleter{}

// NewBlobDeleter creates a new blobDeleter.
func NewBlobDeleter() BlobDeleter {
	return &blobDeleter{}
}

func (p *blobDeleter) DeleteBlob(ctx context.Context, registryClient *http.Client, registryURL, blob string) error {
	glog.V(4).Infof("Deleting blob from registry %q: blob %q", registryURL, blob)
	return deleteFromRegistry(ctx, registryClient, fmt.Sprintf("%s/admin/blobs/%s", registryURL, blob))
}

// manifestDeleter deletes repository manifest data from the registry.
type manifestDeleter struct{}

var _ ManifestDeleter = &manifestDeleter{}

// NewManifestDeleter creates a new manifestDeleter.
func NewManifestDeleter() ManifestDeleter {
	return &manifestDeleter{}
}

func (p *manifestDeleter) DeleteManifest(ctx context.Context, registryClient *http.Client, registryURL, repoName, manifest string) error {
	glog.V(4).Infof("Deleting manifest from registry %q: repo %q, manifest %q", registryURL, repoName, manifest)
	return deleteFromRegistry(ctx, registryClient, fmt.Sprintf("%s/v2/%s/manifests/%s", registryURL, repoName, manifest))
}

func getName(obj runtime.Object) string {
	accessor, err := kmeta.Accessor(obj)
	if err != nil {
		glog.V(4).Infof("Error getting accessor for %#v", obj)
		return "<unknown>"
	}
	return fmt.Sprintf("%s/%s", accessor.GetNamespace(), accessor.GetName())
}
