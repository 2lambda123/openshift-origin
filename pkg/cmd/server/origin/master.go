package origin

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/golang/glog"

	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/internalversion"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	apiserver "k8s.io/apiserver/pkg/server"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	kubeapiserver "k8s.io/kubernetes/pkg/master"

	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver/configprocessing"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	"github.com/openshift/origin/pkg/cmd/server/origin/legacyconfigprocessing"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	kapiserveroptions "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
)

func (c *MasterConfig) newOpenshiftAPIConfig(kubeAPIServerConfig apiserver.Config) (*openshiftapiserver.OpenshiftAPIConfig, error) {
	// make a shallow copy to let us twiddle a few things
	// most of the config actually remains the same.  We only need to mess with a couple items
	genericConfig := kubeAPIServerConfig
	var err error
	genericConfig.RESTOptionsGetter, err = legacyconfigprocessing.NewRESTOptionsGetter(c.Options.KubernetesMasterConfig.APIServerArguments, c.Options.EtcdClientInfo, c.Options.EtcdStorageConfig.OpenShiftStoragePrefix)
	if err != nil {
		return nil, err
	}

	var caData []byte
	if len(c.Options.ImagePolicyConfig.AdditionalTrustedCA) != 0 {
		glog.V(2).Infof("Image import using additional CA path: %s", c.Options.ImagePolicyConfig.AdditionalTrustedCA)
		var err error
		caData, err = ioutil.ReadFile(c.Options.ImagePolicyConfig.AdditionalTrustedCA)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA bundle %s for image importing: %v", c.Options.ImagePolicyConfig.AdditionalTrustedCA, err)
		}
	}

	routeAllocator, err := configprocessing.RouteAllocator(c.Options.RoutingConfig.Subdomain)
	if err != nil {
		return nil, err
	}

	allowedRegistriesForImport := openshiftcontrolplanev1.AllowedRegistries{}
	if c.Options.ImagePolicyConfig.AllowedRegistriesForImport != nil {
		for _, curr := range *c.Options.ImagePolicyConfig.AllowedRegistriesForImport {
			allowedRegistriesForImport = append(allowedRegistriesForImport, openshiftcontrolplanev1.RegistryLocation{
				Insecure:   curr.Insecure,
				DomainName: curr.DomainName,
			})
		}
	}

	ret := &openshiftapiserver.OpenshiftAPIConfig{
		GenericConfig: &apiserver.RecommendedConfig{Config: genericConfig},
		ExtraConfig: openshiftapiserver.OpenshiftAPIExtraConfig{
			InformerStart:                      c.InformerStart,
			KubeAPIServerClientConfig:          &c.PrivilegedLoopbackClientConfig,
			KubeInternalInformers:              c.InternalKubeInformers,
			KubeInformers:                      c.ClientGoKubeInformers,
			QuotaInformers:                     c.QuotaInformers,
			SecurityInformers:                  c.SecurityInformers,
			RuleResolver:                       c.RuleResolver,
			SubjectLocator:                     c.SubjectLocator,
			LimitVerifier:                      c.LimitVerifier,
			RegistryHostnameRetriever:          c.RegistryHostnameRetriever,
			AllowedRegistriesForImport:         allowedRegistriesForImport,
			MaxImagesBulkImportedPerRepository: c.Options.ImagePolicyConfig.MaxImagesBulkImportedPerRepository,
			AdditionalTrustedCA:                caData,
			RouteAllocator:                     routeAllocator,
			ProjectAuthorizationCache:          c.ProjectAuthorizationCache,
			ProjectCache:                       c.ProjectCache,
			ProjectRequestTemplate:             c.Options.ProjectConfig.ProjectRequestTemplate,
			ProjectRequestMessage:              c.Options.ProjectConfig.ProjectRequestMessage,
			ClusterQuotaMappingController:      c.ClusterQuotaMappingController,
			RESTMapper:                         c.RESTMapper,
		},
	}
	if c.Options.OAuthConfig != nil {
		ret.ExtraConfig.ServiceAccountMethod = string(c.Options.OAuthConfig.GrantConfig.ServiceAccountMethod)
	}

	return ret, ret.ExtraConfig.Validate()
}

func (c *MasterConfig) withAPIExtensions(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerOptions *kapiserveroptions.ServerRunOptions, kubeAPIServerConfig apiserver.Config) (apiserver.DelegationTarget, apiextensionsinformers.SharedInformerFactory, error) {
	apiExtensionsConfig, err := createAPIExtensionsConfig(kubeAPIServerConfig, c.ClientGoKubeInformers, kubeAPIServerOptions)
	if err != nil {
		return nil, nil, err
	}
	apiExtensionsServer, err := createAPIExtensionsServer(apiExtensionsConfig, delegateAPIServer)
	if err != nil {
		return nil, nil, err
	}
	return apiExtensionsServer.GenericAPIServer, apiExtensionsServer.Informers, nil
}

func (c *MasterConfig) withNonAPIRoutes(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerConfig apiserver.Config) (apiserver.DelegationTarget, error) {
	openshiftNonAPIConfig, err := legacyconfigprocessing.NewOpenshiftNonAPIConfig(&kubeAPIServerConfig, c.ClientGoKubeInformers, c.Options.OAuthConfig, c.Options.AuthConfig)
	if err != nil {
		return nil, err
	}
	openshiftNonAPIServer, err := openshiftNonAPIConfig.Complete().New(delegateAPIServer)
	if err != nil {
		return nil, err
	}
	return openshiftNonAPIServer.GenericAPIServer, nil
}

func (c *MasterConfig) withOpenshiftAPI(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerConfig apiserver.Config) (*apiserver.GenericAPIServer, error) {
	openshiftAPIServerConfig, err := c.newOpenshiftAPIConfig(kubeAPIServerConfig)
	if err != nil {
		return nil, err
	}

	openshiftAPIServer, err := openshiftAPIServerConfig.Complete().New(delegateAPIServer, true)
	if err != nil {
		return nil, err
	}
	// this sets up the openapi endpoints
	preparedOpenshiftAPIServer := openshiftAPIServer.GenericAPIServer.PrepareRun()

	return preparedOpenshiftAPIServer.GenericAPIServer, nil
}

func (c *MasterConfig) withKubeAPI(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerConfig kubeapiserver.Config) (apiserver.DelegationTarget, error) {
	var err error
	if err != nil {
		return nil, err
	}
	kubeAPIServer, err := kubeAPIServerConfig.Complete(c.ClientGoKubeInformers).New(delegateAPIServer)
	if err != nil {
		return nil, err
	}
	// this sets up the openapi endpoints
	preparedKubeAPIServer := kubeAPIServer.GenericAPIServer.PrepareRun()

	// this remains here and separate so that you can check both kube and openshift levels
	// TODO make this is a proxy at some point
	openshiftapiserver.AddOpenshiftVersionRoute(kubeAPIServer.GenericAPIServer.Handler.GoRestfulContainer, "/version/openshift")

	return preparedKubeAPIServer.GenericAPIServer, nil
}

func (c *MasterConfig) withAggregator(delegateAPIServer apiserver.DelegationTarget, kubeAPIServerOptions *kapiserveroptions.ServerRunOptions, kubeAPIServerConfig apiserver.Config, apiExtensionsInformers apiextensionsinformers.SharedInformerFactory) (*aggregatorapiserver.APIAggregator, error) {
	aggregatorConfig, err := createAggregatorConfig(
		kubeAPIServerConfig,
		kubeAPIServerOptions,
		c.ClientGoKubeInformers,
		aggregatorapiserver.NewClusterIPServiceResolver(c.ClientGoKubeInformers.Core().V1().Services().Lister()),
		utilnet.SetTransportDefaults(&http.Transport{}),
	)
	if err != nil {
		return nil, err
	}
	aggregatorServer, err := createAggregatorServer(aggregatorConfig, delegateAPIServer, apiExtensionsInformers)
	if err != nil {
		// we don't need special handling for innerStopCh because the aggregator server doesn't create any go routines
		return nil, err
	}

	return aggregatorServer, nil
}

// Run launches the OpenShift master by creating a kubernetes master, installing
// OpenShift APIs into it and then running it. The returned channel can be waited on to gracefully shutdown the API server.
// TODO this method only exists to support the old openshift start path.  It should be removed a little ways into 3.10.
func (c *MasterConfig) Run(stopCh <-chan struct{}) (<-chan struct{}, error) {
	var err error
	var apiExtensionsInformers apiextensionsinformers.SharedInformerFactory
	var delegateAPIServer apiserver.DelegationTarget
	var extraPostStartHooks map[string]apiserver.PostStartHookFunc

	c.kubeAPIServerConfig.GenericConfig.BuildHandlerChainFunc, extraPostStartHooks, err = legacyconfigprocessing.BuildHandlerChain(
		c.kubeAPIServerConfig.GenericConfig, c.ClientGoKubeInformers,
		c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile, c.Options.OAuthConfig, c.Options.PolicyConfig.UserAgentMatchingConfig)
	if err != nil {
		return nil, err
	}

	kubeAPIServerOptions, err := kubernetes.BuildKubeAPIserverOptions(c.Options)
	if err != nil {
		return nil, err
	}

	delegateAPIServer = apiserver.NewEmptyDelegate()
	delegateAPIServer, apiExtensionsInformers, err = c.withAPIExtensions(delegateAPIServer, kubeAPIServerOptions, *c.kubeAPIServerConfig.GenericConfig)
	if err != nil {
		return nil, err
	}
	delegateAPIServer, err = c.withNonAPIRoutes(delegateAPIServer, *c.kubeAPIServerConfig.GenericConfig)
	if err != nil {
		return nil, err
	}
	delegateAPIServer, err = c.withOpenshiftAPI(delegateAPIServer, *c.kubeAPIServerConfig.GenericConfig)
	if err != nil {
		return nil, err
	}
	delegateAPIServer, err = c.withKubeAPI(delegateAPIServer, *c.kubeAPIServerConfig)
	if err != nil {
		return nil, err
	}
	aggregatedAPIServer, err := c.withAggregator(delegateAPIServer, kubeAPIServerOptions, *c.kubeAPIServerConfig.GenericConfig, apiExtensionsInformers)
	if err != nil {
		return nil, err
	}

	// Start the audit backend before any request comes in. This means we cannot turn it into a
	// post start hook because without calling Backend.Run the Backend.ProcessEvents call might block.
	if c.AuditBackend != nil {
		if err := c.AuditBackend.Run(stopCh); err != nil {
			return nil, fmt.Errorf("failed to run the audit backend: %v", err)
		}
	}

	if GRPCThreadLimit > 0 {
		if err := aggregatedAPIServer.GenericAPIServer.AddHealthzChecks(NewGRPCStuckThreads()); err != nil {
			return nil, err
		}
		// We start a separate gofunc that will panic for us because nothing is watching healthz at the moment.
		PanicOnGRPCStuckThreads(10*time.Second, stopCh)
	}

	// add post-start hooks
	for name, fn := range c.additionalPostStartHooks {
		aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie(name, fn)
	}
	for name, fn := range extraPostStartHooks {
		aggregatedAPIServer.GenericAPIServer.AddPostStartHookOrDie(name, fn)
	}

	shutdownCh := make(chan struct{})
	go func() {
		defer close(shutdownCh)
		aggregatedAPIServer.GenericAPIServer.PrepareRun().Run(stopCh)
	}()

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	return shutdownCh, cmdutil.WaitForSuccessfulDial(true, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
}
