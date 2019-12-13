package app

import (
	"k8s.io/apiserver/pkg/admission"
	genericapiserver "k8s.io/apiserver/pkg/server"
	clientgoinformers "k8s.io/client-go/informers"
	"k8s.io/kubernetes/openshift-kube-apiserver/openshiftkubeapiserver"
	"k8s.io/kubernetes/pkg/master"
)

var OpenShiftKubeAPIServerConfigPatch openshiftkubeapiserver.KubeAPIServerConfigFunc = nil

type KubeAPIServerServerFunc func(server *master.Master) error

func PatchKubeAPIServerConfig(config *genericapiserver.Config, versionedInformers clientgoinformers.SharedInformerFactory, pluginInitializers *[]admission.PluginInitializer) error {
	if OpenShiftKubeAPIServerConfigPatch == nil {
		return nil
	}

	return OpenShiftKubeAPIServerConfigPatch(config, versionedInformers, pluginInitializers)
}
