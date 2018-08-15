// +build !linux

package network

import (
	"fmt"

	kclientset "k8s.io/client-go/kubernetes"
	kinternalclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig"

	networkclient "github.com/openshift/client-go/network/clientset/versioned"
	networkinformers "github.com/openshift/client-go/network/informers/externalversions"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

func NewSDNInterfaces(options configapi.NodeConfig, networkClient networkclient.Interface,
	kubeClientset kclientset.Interface, kubeClient kinternalclientset.Interface,
	internalKubeInformers kinternalinformers.SharedInformerFactory,
	internalNetworkInformers networkinformers.SharedInformerFactory,
	proxyconfig *kubeproxyconfig.KubeProxyConfiguration) (NodeInterface, ProxyInterface, error) {

	return nil, nil, fmt.Errorf("SDN not supported on this platform")
}
