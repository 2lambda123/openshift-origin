package originpolymorphichelpers

import (
	"k8s.io/apimachinery/pkg/api/meta"
	kinternalclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	deploymentcmd "github.com/openshift/origin/pkg/oc/cli/deploymentconfigs"
)

func NewHistoryViewerFn(delegate polymorphichelpers.HistoryViewerFunc) polymorphichelpers.HistoryViewerFunc {
	return func(restClientGetter genericclioptions.RESTClientGetter, mapping *meta.RESTMapping) (kubectl.HistoryViewer, error) {
		if appsapi.Kind("DeploymentConfig") == mapping.GroupVersionKind.GroupKind() {
			config, err := restClientGetter.ToRESTConfig()
			if err != nil {
				return nil, err
			}
			coreClient, err := kinternalclient.NewForConfig(config)
			if err != nil {
				return nil, err
			}

			return deploymentcmd.NewDeploymentConfigHistoryViewer(coreClient), nil
		}
		return delegate(restClientGetter, mapping)
	}
}
