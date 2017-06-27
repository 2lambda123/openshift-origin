package origin

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/golang/glog"
	"gopkg.in/natefinch/lumberjack.v2"

	utilwait "k8s.io/apimachinery/pkg/util/wait"
	apifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	apiserver "k8s.io/apiserver/pkg/server"
	apiserverfilters "k8s.io/apiserver/pkg/server/filters"
	"k8s.io/apiserver/pkg/server/healthz"
	genericmux "k8s.io/apiserver/pkg/server/mux"
	genericroutes "k8s.io/apiserver/pkg/server/routes"
	kubeapiserver "k8s.io/kubernetes/pkg/master"
	kcorestorage "k8s.io/kubernetes/pkg/registry/core/rest"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	serverhandlers "github.com/openshift/origin/pkg/cmd/server/handlers"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	routeplugin "github.com/openshift/origin/pkg/route/allocation/simple"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
	sccstorage "github.com/openshift/origin/pkg/security/registry/securitycontextconstraints/etcd"
)

func (c *MasterConfig) newOpenshiftAPIConfig(kubeAPIServerConfig apiserver.Config) (*OpenshiftAPIConfig, error) {
	// sccStorage must use the upstream RESTOptionsGetter to be in the correct location
	// this probably creates a duplicate cache, but there are not very many SCCs, so live with it to avoid further linkage
	sccStorage := sccstorage.NewREST(kubeAPIServerConfig.RESTOptionsGetter)

	// make a shallow copy to let us twiddle a few things
	// most of the config actually remains the same.  We only need to mess with a couple items
	genericConfig := kubeAPIServerConfig
	// TODO try to stop special casing these.  We should all agree on them.
	genericConfig.AdmissionControl = c.AdmissionControl
	genericConfig.RESTOptionsGetter = c.RESTOptionsGetter
	genericConfig.Authenticator = c.Authenticator
	genericConfig.Authorizer = c.Authorizer
	genericConfig.RequestContextMapper = c.RequestContextMapper

	ret := &OpenshiftAPIConfig{
		GenericConfig: &genericConfig,

		KubeClientExternal:                 c.PrivilegedLoopbackKubernetesClientsetExternal,
		KubeClientInternal:                 c.PrivilegedLoopbackKubernetesClientsetInternal,
		KubeletClientConfig:                c.KubeletClientConfig,
		KubeInternalInformers:              c.InternalKubeInformers,
		AuthorizationInformers:             c.AuthorizationInformers,
		QuotaInformers:                     c.QuotaInformers,
		SecurityInformers:                  c.SecurityInformers,
		DeprecatedOpenshiftClient:          c.PrivilegedLoopbackOpenShiftClient,
		RuleResolver:                       c.RuleResolver,
		SubjectLocator:                     c.SubjectLocator,
		LimitVerifier:                      c.LimitVerifier,
		RegistryNameFn:                     c.RegistryNameFn,
		AllowedRegistriesForImport:         c.Options.ImagePolicyConfig.AllowedRegistriesForImport,
		MaxImagesBulkImportedPerRepository: c.Options.ImagePolicyConfig.MaxImagesBulkImportedPerRepository,
		RouteAllocator:                     c.RouteAllocator(),
		ProjectAuthorizationCache:          c.ProjectAuthorizationCache,
		ProjectCache:                       c.ProjectCache,
		ProjectRequestTemplate:             c.Options.ProjectConfig.ProjectRequestTemplate,
		ProjectRequestMessage:              c.Options.ProjectConfig.ProjectRequestMessage,
		EnableBuilds:                       configapi.IsBuildEnabled(&c.Options),
		EnableTemplateServiceBroker:        c.Options.TemplateServiceBrokerConfig != nil,
		ClusterQuotaMappingController:      c.ClusterQuotaMappingController,
		SCCStorage:                         sccStorage,
	}
	if c.Options.OAuthConfig != nil {
		ret.ServiceAccountMethod = c.Options.OAuthConfig.GrantConfig.ServiceAccountMethod
	}

	return ret, ret.Validate()
}

func (c *MasterConfig) newOpenshiftNonAPIConfig(kubeAPIServerConfig apiserver.Config) *OpenshiftNonAPIConfig {
	ret := &OpenshiftNonAPIConfig{
		GenericConfig:               &kubeAPIServerConfig,
		EnableControllers:           c.Options.Controllers != configapi.ControllersDisabled,
		ControllerPlug:              c.ControllerPlug,
		EnableOAuth:                 c.Options.OAuthConfig != nil,
		KubeClientInternal:          c.PrivilegedLoopbackKubernetesClientsetInternal,
		EnableTemplateServiceBroker: c.Options.TemplateServiceBrokerConfig != nil,
		TemplateInformers:           c.TemplateInformers,
	}
	if c.Options.OAuthConfig != nil {
		ret.MasterPublicURL = c.Options.OAuthConfig.MasterPublicURL
	}
	if c.Options.TemplateServiceBrokerConfig != nil {
		ret.TemplateNamespaces = c.Options.TemplateServiceBrokerConfig.TemplateNamespaces
	}

	return ret
}

// Run launches the OpenShift master by creating a kubernetes master, installing
// OpenShift APIs into it and then running it.
func (c *MasterConfig) Run(kubeAPIServerConfig *kubeapiserver.Config, assetConfig *AssetConfig, stopCh <-chan struct{}) {
	openshiftNonAPIConfig := c.newOpenshiftNonAPIConfig(*kubeAPIServerConfig.GenericConfig)
	openshiftNonAPIServer, err := openshiftNonAPIConfig.Complete().New(apiserver.EmptyDelegate, stopCh)
	if err != nil {
		glog.Fatalf("Failed to launch master: %v", err)
	}

	openshiftAPIServerConfig, err := c.newOpenshiftAPIConfig(*kubeAPIServerConfig.GenericConfig)
	if err != nil {
		glog.Fatalf("Failed to launch master: %v", err)
	}
	// TODO this is eventually where we end up, with the openshift server completely discrete from the kube one
	// but this only works *AFTER* we commit to the aggregator.  Right now the aggregator is optional, so we have
	// to install ourselves in the kubeapiserver
	// openshiftAPIServer, err := openshiftAPIServerConfig.Complete().New(openshiftNonAPIServer.GenericAPIServer, stopCh)
	// if err != nil {
	// 	glog.Fatalf("Failed to launch master: %v", err)
	// }
	// // this sets up the openapi endpoints
	// preparedOpenshiftAPIServer := openshiftAPIServer.GenericAPIServer.PrepareRun()

	// TODO move out of this function to somewhere we build the kubeAPIServerConfig
	kubeAPIServerConfig.GenericConfig.BuildHandlerChainFunc, err = c.buildHandlerChain(assetConfig)
	if err != nil {
		glog.Fatalf("Failed to launch master: %v", err)
	}
	// We need to add an openshift type to the kube's core storage until at least 3.8.  This does that by using a patch we carry.
	kcorestorage.LegacyStorageMutatorFn = sccstorage.AddSCC(openshiftAPIServerConfig.SCCStorage)
	kubeAPIServer, err := kubeAPIServerConfig.Complete().New(openshiftNonAPIServer.GenericAPIServer)
	if err != nil {
		glog.Fatalf("Failed to launch master: %v", err)
	}
	// TODO this goes away in 3.7 after we commit to the aggregator always being on (even if its just in local mode).
	// this is installing the openshift APIs into the kubeapiserver
	// ok, this is a big side-effect.  Openshift APIs run a different admission chain (always have), but since
	// we're going through a "normal" API installation in the wrong server, we need to switch the admission chain
	// *only while we're installing these APIs*.  There are tests that make sure this works and doesn't drop
	// plugins and we'll remove it once we're aggregating
	kubeAPIServer.GenericAPIServer.SetAdmission(openshiftAPIServerConfig.GenericConfig.AdmissionControl)
	installAPIs(openshiftAPIServerConfig, kubeAPIServer.GenericAPIServer)
	kubeAPIServer.GenericAPIServer.SetAdmission(kubeAPIServerConfig.GenericConfig.AdmissionControl)

	// this sets up the openapi endpoints
	preparedKubeAPIServer := kubeAPIServer.GenericAPIServer.PrepareRun()

	// presence of the key indicates whether or not to enable the aggregator
	if len(c.Options.AggregatorConfig.ProxyClientInfo.KeyFile) == 0 {
		go preparedKubeAPIServer.Run(utilwait.NeverStop)

		// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
		cmdutil.WaitForSuccessfulDial(c.TLS, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
		return
	}

	aggregatorConfig, err := c.createAggregatorConfig(*kubeAPIServerConfig.GenericConfig)
	if err != nil {
		glog.Fatalf("Failed to create aggregator config: %v", err)
	}
	aggregatorServer, err := createAggregatorServer(aggregatorConfig, preparedKubeAPIServer.GenericAPIServer, c.InternalKubeInformers, stopCh)
	if err != nil {
		// we don't need special handling for innerStopCh because the aggregator server doesn't create any go routines
		glog.Fatalf("Failed to create aggregator server: %v", err)
	}
	go aggregatorServer.GenericAPIServer.PrepareRun().Run(stopCh)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial(c.TLS, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
}

func (c *MasterConfig) buildHandlerChain(assetConfig *AssetConfig) (func(http.Handler, *apiserver.Config) (secure http.Handler), error) {
	if c.Options.OAuthConfig != nil {
		glog.Infof("Starting OAuth2 API at %s", OpenShiftOAuthAPIPrefix)
	}

	if assetConfig != nil {
		publicURL, err := url.Parse(assetConfig.Options.PublicURL)
		if err != nil {
			return nil, err
		}
		glog.Infof("Starting Web Console %s", publicURL.Path)
	}

	// TODO(sttts): resync with upstream handler chain and re-use upstream filters as much as possible
	return func(apiHandler http.Handler, kc *apiserver.Config) (secure http.Handler) {
		contextMapper := c.getRequestContextMapper()

		handler := c.versionSkewFilter(apiHandler, contextMapper)
		handler = serverhandlers.AuthorizationFilter(handler, c.Authorizer, c.AuthorizationAttributeBuilder, contextMapper)
		handler = serverhandlers.ImpersonationFilter(handler, c.Authorizer, c.GroupCache, contextMapper)

		// audit handler must comes before the impersonationFilter to read the original user
		if c.Options.AuditConfig.Enabled {
			var writer io.Writer
			if len(c.Options.AuditConfig.AuditFilePath) > 0 {
				writer = &lumberjack.Logger{
					Filename:   c.Options.AuditConfig.AuditFilePath,
					MaxAge:     c.Options.AuditConfig.MaximumFileRetentionDays,
					MaxBackups: c.Options.AuditConfig.MaximumRetainedFiles,
					MaxSize:    c.Options.AuditConfig.MaximumFileSizeMegabytes,
				}
			} else {
				// backwards compatible writer to regular log
				writer = cmdutil.NewGLogWriterV(0)
			}
			handler = apifilters.WithAudit(handler, contextMapper, writer)
		}
		handler = serverhandlers.AuthenticationHandlerFilter(handler, c.Authenticator, contextMapper)
		handler = namespacingFilter(handler, contextMapper)
		handler = cacheControlFilter(handler, "no-store") // protected endpoints should not be cached

		if c.Options.OAuthConfig != nil {
			authConfig, err := BuildAuthConfig(c)
			if err != nil {
				glog.Fatalf("Failed to setup OAuth2: %v", err)
			}
			handler, err = authConfig.WithOAuth(handler)
			if err != nil {
				glog.Fatalf("Failed to setup OAuth2: %v", err)
			}
		}

		handler, err := assetConfig.WithAssets(handler)
		if err != nil {
			glog.Fatalf("Failed to setup serving of assets: %v", err)
		}

		// skip authz/n for the index handler
		handler = WithPatternsHandler(handler, apiHandler, "/", "")

		if c.WebConsoleEnabled() {
			handler = WithAssetServerRedirect(handler, c.Options.AssetConfig.PublicURL)
		}

		handler = apiserverfilters.WithCORS(handler, c.Options.CORSAllowedOrigins, nil, nil, nil, "true")
		handler = apiserverfilters.WithPanicRecovery(handler)
		handler = apiserverfilters.WithTimeoutForNonLongRunningRequests(handler, contextMapper, kc.LongRunningFunc)
		// TODO: MaxRequestsInFlight should be subdivided by intent, type of behavior, and speed of
		// execution - updates vs reads, long reads vs short reads, fat reads vs skinny reads.
		// NOTE: read vs. write is implemented in Kube 1.6+
		handler = apiserverfilters.WithMaxInFlightLimit(handler, kc.MaxRequestsInFlight, kc.MaxMutatingRequestsInFlight, contextMapper, kc.LongRunningFunc)
		handler = apifilters.WithRequestInfo(handler, apiserver.NewRequestInfoResolver(kc), contextMapper)
		handler = apirequest.WithRequestContext(handler, contextMapper)

		return handler
	}, nil
}

func (c *MasterConfig) RunHealth() error {
	postGoRestfulMux := genericmux.NewPathRecorderMux("master-healthz")

	healthz.InstallHandler(postGoRestfulMux, healthz.PingHealthz)
	initReadinessCheckRoute(postGoRestfulMux, "/healthz/ready", func() bool { return true })
	genericroutes.Profiling{}.Install(postGoRestfulMux)
	genericroutes.MetricsWithReset{}.Install(postGoRestfulMux)

	// we use direct bypass to allow readiness and health to work regardless of the master health
	authz := serverhandlers.NewBypassAuthorizer(c.Authorizer, "/healthz", "/healthz/ready")
	contextMapper := c.getRequestContextMapper()
	handler := serverhandlers.AuthorizationFilter(postGoRestfulMux, authz, c.AuthorizationAttributeBuilder, contextMapper)
	handler = serverhandlers.AuthenticationHandlerFilter(handler, c.Authenticator, contextMapper)
	handler = apiserverfilters.WithPanicRecovery(handler)
	handler = apifilters.WithRequestInfo(handler, apiserver.NewRequestInfoResolver(&apiserver.Config{}), contextMapper)
	handler = apirequest.WithRequestContext(handler, contextMapper)

	c.serve(handler, []string{"Started health checks at %s"})
	return nil
}

// serve starts serving the provided http.Handler using security settings derived from the MasterConfig
func (c *MasterConfig) serve(handler http.Handler, messages []string) {
	timeout := c.Options.ServingInfo.RequestTimeoutSeconds
	if timeout == -1 {
		timeout = 0
	}

	server := &http.Server{
		Addr:           c.Options.ServingInfo.BindAddress,
		Handler:        handler,
		ReadTimeout:    time.Duration(timeout) * time.Second,
		WriteTimeout:   time.Duration(timeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go utilwait.Forever(func() {
		for _, s := range messages {
			glog.Infof(s, c.Options.ServingInfo.BindAddress)
		}
		if c.TLS {
			extraCerts, err := configapi.GetNamedCertificateMap(c.Options.ServingInfo.NamedCertificates)
			if err != nil {
				glog.Fatal(err)
			}
			server.TLSConfig = crypto.SecureTLSConfig(&tls.Config{
				// Populate PeerCertificates in requests, but don't reject connections without certificates
				// This allows certificates to be validated by authenticators, while still allowing other auth types
				ClientAuth: tls.RequestClientCert,
				ClientCAs:  c.ClientCAs,
				// Set SNI certificate func
				GetCertificate: cmdutil.GetCertificateFunc(extraCerts),
				MinVersion:     crypto.TLSVersionOrDie(c.Options.ServingInfo.MinTLSVersion),
				CipherSuites:   crypto.CipherSuitesOrDie(c.Options.ServingInfo.CipherSuites),
			})
			glog.Fatal(cmdutil.ListenAndServeTLS(server, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.ServerCert.CertFile, c.Options.ServingInfo.ServerCert.KeyFile))
		} else {
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)
}

// InitializeObjects ensures objects in Kubernetes and etcd are properly populated.
// Requires a Kube client to be established and that etcd be started.
func (c *MasterConfig) InitializeObjects() {
	// Create required policy rules if needed
	c.ensureComponentAuthorizationRules()
	// Ensure the default SCCs are created
	c.ensureDefaultSecurityContextConstraints()
	// Bind default roles for service accounts in the default namespace if needed
	c.ensureDefaultNamespaceServiceAccountRoles()
	// Create the infra namespace
	c.ensureOpenShiftInfraNamespace()
	// Create the shared resource namespace
	c.ensureOpenShiftSharedResourcesNamespace()
}

// getRequestContextMapper returns a mapper from requests to contexts, initializing it if needed
func (c *MasterConfig) getRequestContextMapper() apirequest.RequestContextMapper {
	if c.RequestContextMapper == nil {
		c.RequestContextMapper = apirequest.NewRequestContextMapper()
	}
	return c.RequestContextMapper
}

// RouteAllocator returns a route allocation controller.
func (c *MasterConfig) RouteAllocator() *routeallocationcontroller.RouteAllocationController {
	osclient, kclient := c.RouteAllocatorClients()
	factory := routeallocationcontroller.RouteAllocationControllerFactory{
		OSClient:   osclient,
		KubeClient: kclient,
	}

	plugin, err := routeplugin.NewSimpleAllocationPlugin(c.Options.RoutingConfig.Subdomain)
	if err != nil {
		glog.Fatalf("Route plugin initialization failed: %v", err)
	}

	return factory.Create(plugin)
}

// env returns an environment variable, or the defaultValue if it is not set.
func env(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	return val
}

func WithPatternsHandler(handler http.Handler, patternHandler http.Handler, patterns ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for _, p := range patterns {
			if req.URL.Path == p {
				patternHandler.ServeHTTP(w, req)
				return
			}
		}
		handler.ServeHTTP(w, req)
	})
}
