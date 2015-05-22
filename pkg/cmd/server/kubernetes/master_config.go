package kubernetes

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/authorizer"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
)

// MasterConfig defines the required values to start a Kubernetes master
type MasterConfig struct {
	MasterIP    net.IP
	MasterPort  int
	MasterCount int

	NodeHosts []string
	PortalNet *net.IPNet

	RequestContextMapper kapi.RequestContextMapper

	EtcdHelper          tools.EtcdHelper
	KubeClient          *kclient.Client
	KubeletClientConfig *kclient.KubeletConfig

	Authorizer       authorizer.Authorizer
	AdmissionControl admission.Interface

	SchedulerConfigFile string
}

func BuildKubernetesMasterConfig(options configapi.MasterConfig, requestContextMapper kapi.RequestContextMapper, kubeClient *kclient.Client) (*MasterConfig, error) {
	if options.KubernetesMasterConfig == nil {
		return nil, errors.New("insufficient information to build KubernetesMasterConfig")
	}

	// Connect and setup etcd interfaces
	etcdClient, err := etcd.GetAndTestEtcdClient(options.EtcdClientInfo)
	if err != nil {
		return nil, err
	}
	ketcdHelper, err := master.NewEtcdHelper(etcdClient, options.EtcdStorageConfig.KubernetesStorageVersion, options.EtcdStorageConfig.KubernetesStoragePrefix)
	if err != nil {
		return nil, fmt.Errorf("Error setting up Kubernetes server storage: %v", err)
	}

	kubeletClientConfig := configapi.GetKubeletClientConfig(options)

	portalNet := net.IPNet(flagtypes.DefaultIPNet(options.KubernetesMasterConfig.ServicesSubnet))

	// in-order list of plug-ins that should intercept admission decisions
	// TODO: Push node environment support to upstream in future
	// TODO: JTL: update serviceaccount admission plugin to limit secrets to the ones held by the serviceaccount
	admissionControlPluginNames := []string{"NamespaceExists", "NamespaceLifecycle", "OriginPodNodeEnvironment", "LimitRanger", "ServiceAccount", "ResourceQuota"}
	admissionController := admission.NewFromPlugins(kubeClient, admissionControlPluginNames, "")

	_, portString, err := net.SplitHostPort(options.ServingInfo.BindAddress)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, err
	}

	kmaster := &MasterConfig{
		MasterIP:             net.ParseIP(options.KubernetesMasterConfig.MasterIP),
		MasterPort:           port,
		MasterCount:          options.KubernetesMasterConfig.MasterCount,
		NodeHosts:            options.KubernetesMasterConfig.StaticNodeNames,
		PortalNet:            &portalNet,
		RequestContextMapper: requestContextMapper,
		EtcdHelper:           ketcdHelper,
		KubeClient:           kubeClient,
		KubeletClientConfig:  kubeletClientConfig,
		Authorizer:           apiserver.NewAlwaysAllowAuthorizer(),
		AdmissionControl:     admissionController,
		SchedulerConfigFile:  options.KubernetesMasterConfig.SchedulerConfigFile,
	}

	return kmaster, nil
}
