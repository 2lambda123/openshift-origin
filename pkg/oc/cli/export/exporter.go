package export

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	osautil "github.com/openshift/origin/pkg/oc/lib/serviceaccount"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

var ErrExportOmit = fmt.Errorf("object is omitted")

type Exporter interface {
	AddExportOptions(*pflag.FlagSet)
	Export(obj runtime.Object, exact bool) error
}

type DefaultExporter struct{}

func (e *DefaultExporter) AddExportOptions(flags *pflag.FlagSet) {
}

func exportObjectMeta(objMeta metav1.Object, exact bool) {
	objMeta.SetUID("")
	if !exact {
		objMeta.SetNamespace("")
	}
	objMeta.SetCreationTimestamp(metav1.Time{})
	objMeta.SetDeletionTimestamp(nil)
	objMeta.SetResourceVersion("")
	objMeta.SetSelfLink("")
	if len(objMeta.GetGenerateName()) > 0 && !exact {
		objMeta.SetName("")
	}
}

func (e *DefaultExporter) Export(obj runtime.Object, exact bool) error {
	if meta, err := meta.Accessor(obj); err == nil {
		exportObjectMeta(meta, exact)
	} else {
		glog.V(4).Infof("Object of type %v does not have ObjectMeta: %v", reflect.TypeOf(obj), err)
	}

	switch t := obj.(type) {
	case *kapi.Endpoints:
	case *kapi.ResourceQuota:
		t.Status = kapi.ResourceQuotaStatus{}
	case *kapi.LimitRange:
	// TODO: this needs to be fixed
	//  limitrange.Strategy.PrepareForCreate(obj)
	case *kapi.Node:
		if exact {
			return nil
		}
		// Nodes are the only resources that allow direct status edits, therefore
		// we clear that without exact so that the node value can be reused.
		t.Status = kapi.NodeStatus{}
	case *kapi.Namespace:
	case *kapi.PersistentVolumeClaim:
		t.Status = kapi.PersistentVolumeClaimStatus{}
	case *kapi.PersistentVolume:
	case *kapi.ReplicationController:
		t.Status = kapi.ReplicationControllerStatus{}
	case *kapi.Pod:
		t.Status = kapi.PodStatus{}
	case *kapi.PodTemplate:
	case *kapi.Service:
		// TODO: service does not yet have a strategy
		t.Status = kapi.ServiceStatus{}
		if exact {
			return nil
		}
		if t.Spec.ClusterIP != kapi.ClusterIPNone {
			t.Spec.ClusterIP = ""
		}
		if t.Spec.Type == kapi.ServiceTypeNodePort {
			for i := range t.Spec.Ports {
				t.Spec.Ports[i].NodePort = 0
			}
		}
	case *kapi.Secret:
		if exact {
			return nil
		}
		// secrets that are tied to the UID of a service account cannot be exported anyway
		if t.Type == kapi.SecretTypeServiceAccountToken || len(t.Annotations[kapi.ServiceAccountUIDKey]) > 0 {
			return ErrExportOmit
		}
	case *kapi.ServiceAccount:
		if exact {
			return nil
		}

		dockercfgSecretPrefix := osautil.GetDockercfgSecretNamePrefix(t.Name)
		newImagePullSecrets := []kapi.LocalObjectReference{}
		for _, secretRef := range t.ImagePullSecrets {
			if strings.HasPrefix(secretRef.Name, dockercfgSecretPrefix) {
				continue
			}
			newImagePullSecrets = append(newImagePullSecrets, secretRef)
		}
		t.ImagePullSecrets = newImagePullSecrets

		tokenSecretPrefix := osautil.GetTokenSecretNamePrefix(t.Name)
		newMountableSecrets := []kapi.ObjectReference{}
		for _, secretRef := range t.Secrets {
			if strings.HasPrefix(secretRef.Name, dockercfgSecretPrefix) ||
				strings.HasPrefix(secretRef.Name, tokenSecretPrefix) {
				continue
			}
			newMountableSecrets = append(newMountableSecrets, secretRef)
		}
		t.Secrets = newMountableSecrets

	case *appsapi.DeploymentConfig:
		t.Status = appsapi.DeploymentConfigStatus{}

	case *buildapi.BuildConfig:
		t.Status = buildapi.BuildConfigStatus{}

		for i := range t.Spec.Triggers {
			if p := t.Spec.Triggers[i].ImageChange; p != nil {
				p.LastTriggeredImageID = ""
			}
		}
	case *buildapi.Build:
		// TODO: should be handled by prepare for create
		t.Status.Duration = 0
		t.Status.Phase = buildapi.BuildPhaseNew
		t.Status.StartTimestamp = nil
		t.Status.CompletionTimestamp = nil
		if exact {
			return nil
		}
		if t.Status.Config != nil {
			t.Status.Config = &kapi.ObjectReference{Name: t.Status.Config.Name}
		}
	case *routeapi.Route:
	case *imageapi.Image:
	case *imageapi.ImageStream:
		if exact {
			return nil
		}
		// if we point to a docker image repository upstream, copy only the spec tags
		if len(t.Spec.DockerImageRepository) > 0 {
			t.Status = imageapi.ImageStreamStatus{}
			break
		}
		// create an image stream that mirrors (each spec tag points to the remote image stream)
		if len(t.Status.DockerImageRepository) > 0 {
			ref, err := imageapi.ParseDockerImageReference(t.Status.DockerImageRepository)
			if err != nil {
				return err
			}
			newSpec := imageapi.ImageStreamSpec{
				Tags: map[string]imageapi.TagReference{},
			}
			for name, tag := range t.Status.Tags {
				if len(tag.Items) > 0 {
					// copy annotations
					existing := t.Spec.Tags[name]
					// point directly to that registry
					ref.Tag = name
					existing.From = &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: ref.String(),
					}
					newSpec.Tags[name] = existing
				}
			}
			for name, ref := range t.Spec.Tags {
				if _, ok := t.Status.Tags[name]; ok {
					continue
				}
				// TODO: potentially trim some of these
				newSpec.Tags[name] = ref
			}
			t.Spec = newSpec
			t.Status = imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{},
			}
			break
		}

		// otherwise, try to snapshot the most recent image as spec items
		newSpec := imageapi.ImageStreamSpec{
			Tags: map[string]imageapi.TagReference{},
		}
		for name, tag := range t.Status.Tags {
			if len(tag.Items) > 0 {
				// copy annotations
				existing := t.Spec.Tags[name]
				existing.From = &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: tag.Items[0].DockerImageReference,
				}
				newSpec.Tags[name] = existing
			}
		}
		t.Spec = newSpec
		t.Status = imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{},
		}

	case *imageapi.ImageStreamTag:
		exportObjectMeta(&t.Image.ObjectMeta, exact)
	case *imageapi.ImageStreamImage:
		exportObjectMeta(&t.Image.ObjectMeta, exact)

	default:
		glog.V(4).Infof("No export strategy defined for objects of type %v", reflect.TypeOf(obj))
	}
	return nil
}
