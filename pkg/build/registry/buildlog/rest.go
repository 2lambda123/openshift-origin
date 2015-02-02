package buildlog

import (
	"fmt"
	"net/url"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/build"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	BuildRegistry build.Registry
	PodControl    PodControlInterface
}

type PodControlInterface interface {
	getPod(namespace, name string) (*kapi.Pod, error)
}

type RealPodControl struct {
	podsNamspacer kclient.PodsNamespacer
}

func (r RealPodControl) getPod(namespace, name string) (*kapi.Pod, error) {
	return r.podsNamspacer.Pods(namespace).Get(name)
}

// NewREST creates a new REST for BuildLog
// Takes build registry and pod client to get necessary attributes to assemble
// URL to which the request shall be redirected in order to get build logs.
func NewREST(b build.Registry, pn kclient.PodsNamespacer) apiserver.RESTStorage {
	return &REST{
		BuildRegistry: b,
		PodControl:    RealPodControl{pn},
	}
}

// Redirector implementation
func (r *REST) ResourceLocation(ctx kapi.Context, id string) (string, error) {
	build, err := r.BuildRegistry.GetBuild(ctx, id)
	if err != nil {
		return "", errors.NewFieldNotFound("Build", id)
	}

	// TODO: these must be status errors, not field errors
	// TODO: choose a more appropriate "try again later" status code, like 202
	if build.PodRef == nil || len(build.PodRef.Name) == 0 || len(build.PodRef.Namespace) == 0 {
		return "", errors.NewFieldRequired("Build.podRef", build.PodRef)
	}

	pod, err := r.PodControl.getPod(build.PodRef.Namespace, build.PodRef.Name)
	if err != nil {
		return "", errors.NewFieldNotFound("Build.podRef", build.PodRef)
	}

	buildPodName := build.PodRef.Name
	buildPodHost := pod.Status.Host
	buildPodNamespace := pod.Namespace
	// Build will take place only in one container
	buildContainerName := pod.Spec.Containers[0].Name
	location := &url.URL{
		Scheme: kubernetes.NodeScheme,
		Host:   fmt.Sprintf("%s:%d", buildPodHost, kubernetes.NodePort),
		Path:   fmt.Sprintf("/containerLogs/%s/%s/%s", buildPodNamespace, buildPodName, buildContainerName),
	}

	// Pod in which build take place can't be in the Pending or Unknown phase,
	// cause no containers are present in the Pod in those phases.
	if pod.Status.Phase == kapi.PodPending || pod.Status.Phase == kapi.PodUnknown {
		return "", errors.NewFieldInvalid("Pod.Status", pod.Status.Phase, "must be Running, Succeeded or Failed")
	}

	switch build.Status {
	case api.BuildStatusRunning:
		location.RawQuery = url.Values{"follow": []string{"1"}}.Encode()
	case api.BuildStatusComplete, api.BuildStatusFailed:
		// Do not follow the Complete and Failed logs as the streaming already finished.
	default:
		return "", errors.NewFieldInvalid("build.Status", build.Status, "must be Running, Complete or Failed")
	}

	if err != nil {
		return "", err
	}
	return location.String(), nil
}

func (r *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	return nil, fmt.Errorf("BuildLog can't be retrieved")
}

func (r *REST) New() runtime.Object {
	return &api.BuildLog{}
}
