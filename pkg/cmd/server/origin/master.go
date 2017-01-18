package origin

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	restful "github.com/emicklei/go-restful"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/natefinch/lumberjack.v2"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kubeapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	v1beta1extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/apiserver"
	kapiserverfilters "k8s.io/kubernetes/pkg/apiserver/filters"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/genericapiserver"
	kgenericfilters "k8s.io/kubernetes/pkg/genericapiserver/filters"
	genericmux "k8s.io/kubernetes/pkg/genericapiserver/mux"
	genericroutes "k8s.io/kubernetes/pkg/genericapiserver/routes"
	"k8s.io/kubernetes/pkg/healthz"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
	"k8s.io/kubernetes/pkg/util/sets"
	utilwait "k8s.io/kubernetes/pkg/util/wait"

	buildclient "github.com/openshift/origin/pkg/build/client"
	buildgenerator "github.com/openshift/origin/pkg/build/generator"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildetcd "github.com/openshift/origin/pkg/build/registry/build/etcd"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildconfigetcd "github.com/openshift/origin/pkg/build/registry/buildconfig/etcd"
	buildlogregistry "github.com/openshift/origin/pkg/build/registry/buildlog"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/generic"
	"github.com/openshift/origin/pkg/build/webhook/github"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	deployconfigregistry "github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	deployconfigetcd "github.com/openshift/origin/pkg/deploy/registry/deployconfig/etcd"
	deploylogregistry "github.com/openshift/origin/pkg/deploy/registry/deploylog"
	deployconfiggenerator "github.com/openshift/origin/pkg/deploy/registry/generator"
	deployconfiginstantiate "github.com/openshift/origin/pkg/deploy/registry/instantiate"
	deployrollback "github.com/openshift/origin/pkg/deploy/registry/rollback"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/image/importer"
	imageimporter "github.com/openshift/origin/pkg/image/importer"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagesecret"
	"github.com/openshift/origin/pkg/image/registry/imagesignature"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimport"
	"github.com/openshift/origin/pkg/image/registry/imagestreammapping"
	"github.com/openshift/origin/pkg/image/registry/imagestreamtag"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/discovery"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	authorizetokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken/etcd"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	clientetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclient/etcd"
	clientauthetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization/etcd"
	projectproxy "github.com/openshift/origin/pkg/project/registry/project/proxy"
	projectrequeststorage "github.com/openshift/origin/pkg/project/registry/projectrequest/delegated"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
	routeetcd "github.com/openshift/origin/pkg/route/registry/route/etcd"
	clusternetworketcd "github.com/openshift/origin/pkg/sdn/registry/clusternetwork/etcd"
	egressnetworkpolicyetcd "github.com/openshift/origin/pkg/sdn/registry/egressnetworkpolicy/etcd"
	hostsubnetetcd "github.com/openshift/origin/pkg/sdn/registry/hostsubnet/etcd"
	netnamespaceetcd "github.com/openshift/origin/pkg/sdn/registry/netnamespace/etcd"
	saoauth "github.com/openshift/origin/pkg/serviceaccounts/oauthclient"
	templateregistry "github.com/openshift/origin/pkg/template/registry"
	templateetcd "github.com/openshift/origin/pkg/template/registry/etcd"
	groupetcd "github.com/openshift/origin/pkg/user/registry/group/etcd"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	identityetcd "github.com/openshift/origin/pkg/user/registry/identity/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
	"github.com/openshift/origin/pkg/version"

	"github.com/openshift/origin/pkg/build/registry/buildclone"
	"github.com/openshift/origin/pkg/build/registry/buildconfiginstantiate"

	appliedclusterresourcequotaregistry "github.com/openshift/origin/pkg/quota/registry/appliedclusterresourcequota"
	clusterresourcequotaregistry "github.com/openshift/origin/pkg/quota/registry/clusterresourcequota"

	"github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/api/v1"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicystorage "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy/etcd"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	clusterpolicybindingstorage "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding/etcd"
	clusterrolestorage "github.com/openshift/origin/pkg/authorization/registry/clusterrole/proxy"
	clusterrolebindingstorage "github.com/openshift/origin/pkg/authorization/registry/clusterrolebinding/proxy"
	"github.com/openshift/origin/pkg/authorization/registry/localresourceaccessreview"
	"github.com/openshift/origin/pkg/authorization/registry/localsubjectaccessreview"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policyetcd "github.com/openshift/origin/pkg/authorization/registry/policy/etcd"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	policybindingetcd "github.com/openshift/origin/pkg/authorization/registry/policybinding/etcd"
	"github.com/openshift/origin/pkg/authorization/registry/resourceaccessreview"
	rolestorage "github.com/openshift/origin/pkg/authorization/registry/role/policybased"
	rolebindingstorage "github.com/openshift/origin/pkg/authorization/registry/rolebinding/policybased"
	rolebindingrestrictionregistry "github.com/openshift/origin/pkg/authorization/registry/rolebindingrestriction"
	"github.com/openshift/origin/pkg/authorization/registry/selfsubjectrulesreview"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/authorization/registry/subjectrulesreview"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
	routeplugin "github.com/openshift/origin/pkg/route/allocation/simple"
	"github.com/openshift/origin/pkg/security/registry/podsecuritypolicyreview"
	"github.com/openshift/origin/pkg/security/registry/podsecuritypolicyselfsubjectreview"
	"github.com/openshift/origin/pkg/security/registry/podsecuritypolicysubjectreview"
	oscc "github.com/openshift/origin/pkg/security/scc"
)

const (
	openAPIServePath = "/swagger.json"
	// Discovery endpoint for OAuth 2.0 Authorization Server Metadata
	// See IETF Draft:
	// https://tools.ietf.org/html/draft-ietf-oauth-discovery-04#section-2
	oauthMetadataEndpoint = "/.well-known/oauth-authorization-server"
)

var excludedV1Types = sets.NewString()

// Run launches the OpenShift master by creating a kubernetes master, installing
// OpenShift APIs into it and then running it.
func (c *MasterConfig) Run(kc *kubernetes.MasterConfig, assetConfig *AssetConfig) {
	var (
		messages []string
		err      error
	)
	kc.Master.GenericConfig.BuildHandlerChainsFunc, messages, err = c.buildHandlerChain(assetConfig)
	if err != nil {
		glog.Fatalf("Failed to launch master: %v", err)
	}

	kmaster, err := kc.Master.Complete().New()
	if err != nil {
		glog.Fatalf("Failed to launch master: %v", err)
	}

	c.InstallProtectedAPI(kmaster.GenericAPIServer.HandlerContainer)
	messages = append(messages, c.kubernetesAPIMessages(kc)...)

	for _, s := range messages {
		glog.Infof(s, c.Options.ServingInfo.BindAddress)
	}
	go kmaster.GenericAPIServer.PrepareRun().Run(utilwait.NeverStop)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial(c.TLS, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
}

func (c *MasterConfig) RunInProxyMode(proxy *kubernetes.ProxyConfig, assetConfig *AssetConfig) {
	handlerChain, messages, err := c.buildHandlerChain(assetConfig)
	if err != nil {
		glog.Fatalf("Failed to launch master: %v", err)
	}

	// TODO(sttts): create a genericapiserver here
	container := genericmux.NewAPIContainer(http.NewServeMux(), kapi.Codecs)

	// install /api proxy forwarder
	proxyMessages, err := proxy.InstallAPI(container.Container)
	if err != nil {
		glog.Fatalf("Failed to launch master: %v", err)
	}
	messages = append(messages, proxyMessages...)

	// install GenericAPIServer handlers manually, usually done by GenericAPIServer.PrepareRun()
	healthz.InstallHandler(&container.NonSwaggerRoutes, healthz.PingHealthz)

	swaggerConfig := genericapiserver.DefaultSwaggerConfig()
	swaggerConfig.WebServicesUrl = c.Options.MasterPublicURL
	genericroutes.Swagger{Config: swaggerConfig}.Install(container)
	messages = append(messages, fmt.Sprintf("Started Swagger Schema API at %%s%s", swaggerConfig.ApiPath))

	genericroutes.OpenAPI{Config: kubernetes.DefaultOpenAPIConfig()}.Install(container)
	messages = append(messages, fmt.Sprintf("Started OpenAPI Schema at %%s%s", openAPIServePath))

	// install origin handlers
	c.InstallProtectedAPI(container)

	// TODO(sttts): split cmd/server/kubernetes config generation into generic and master-specific
	// until then: create ad-hoc config
	genericConfig := genericapiserver.NewConfig()
	genericConfig.RequestContextMapper = c.RequestContextMapper
	genericConfig.LegacyAPIGroupPrefixes = kubernetes.LegacyAPIGroupPrefixes
	genericConfig.MaxRequestsInFlight = c.Options.ServingInfo.MaxRequestsInFlight

	secureHandler, _ := handlerChain(container.ServeMux, genericConfig)
	c.serve(secureHandler, messages)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial(c.TLS, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
}

type sortedGroupVersions []unversioned.GroupVersion

func (s sortedGroupVersions) Len() int           { return len(s) }
func (s sortedGroupVersions) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedGroupVersions) Less(i, j int) bool { return s[i].Group < s[j].Group }

func (c *MasterConfig) kubernetesAPIMessages(kc *kubernetes.MasterConfig) []string {
	var messages []string

	// v1 has to be printed separately since it's served from different endpoint than groups
	if configapi.HasKubernetesAPIVersion(*c.Options.KubernetesMasterConfig, kubeapiv1.SchemeGroupVersion) {
		messages = append(messages, fmt.Sprintf("Started Kubernetes API at %%s%s", genericapiserver.DefaultLegacyAPIPrefix))
	}
	versions := registered.EnabledVersions()
	sort.Sort(sortedGroupVersions(versions))
	for _, ver := range versions {
		if ver.String() == "v1" {
			// skip legacy v1 as we handle that above
			continue
		}
		if configapi.HasKubernetesAPIVersion(*c.Options.KubernetesMasterConfig, ver) {
			messages = append(messages, fmt.Sprintf("Started Kubernetes API %s at %%s%s", ver.String(), genericapiserver.APIGroupPrefix))
		}
	}

	messages = append(messages, fmt.Sprintf("Started Swagger Schema API at %%s%s", kc.Master.GenericConfig.SwaggerConfig.ApiPath))
	messages = append(messages, fmt.Sprintf("Started OpenAPI Schema at %%s%s", openAPIServePath))

	return messages
}

func (c *MasterConfig) buildHandlerChain(assetConfig *AssetConfig) (func(http.Handler, *genericapiserver.Config) (secure, insecure http.Handler), []string, error) {
	var messages []string
	if c.Options.OAuthConfig != nil {
		messages = append(messages, fmt.Sprintf("Started OAuth2 API at %%s%s", OpenShiftOAuthAPIPrefix))
	}

	if assetConfig != nil {
		publicURL, err := url.Parse(assetConfig.Options.PublicURL)
		if err != nil {
			return nil, nil, err
		}
		messages = append(messages, fmt.Sprintf("Started Web Console %%s%s", publicURL.Path))
	}

	// TODO(sttts): resync with upstream handler chain and re-use upstream filters as much as possible
	return func(apiHandler http.Handler, kc *genericapiserver.Config) (secure, insecure http.Handler) {
		attributeGetter := kapiserverfilters.NewRequestAttributeGetter(c.RequestContextMapper)

		handler := c.versionSkewFilter(apiHandler, c.getRequestContextMapper())
		handler = c.authorizationFilter(handler)
		handler = c.impersonationFilter(handler)

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
			handler = kapiserverfilters.WithAudit(handler, attributeGetter, writer)
		}
		handler = authenticationHandlerFilter(handler, c.Authenticator, c.getRequestContextMapper())
		handler = namespacingFilter(handler, c.getRequestContextMapper())
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

		handler = kgenericfilters.WithCORS(handler, c.Options.CORSAllowedOrigins, nil, nil, nil, "true")
		handler = kgenericfilters.WithPanicRecovery(handler, c.RequestContextMapper)
		handler = kgenericfilters.WithTimeoutForNonLongRunningRequests(handler, kc.LongRunningFunc)
		// TODO: MaxRequestsInFlight should be subdivided by intent, type of behavior, and speed of
		// execution - updates vs reads, long reads vs short reads, fat reads vs skinny reads.
		// NOTE: read vs. write is implemented in Kube 1.6+
		handler = kgenericfilters.WithMaxInFlightLimit(handler, kc.MaxRequestsInFlight, kc.LongRunningFunc)
		handler = kapiserverfilters.WithRequestInfo(handler, genericapiserver.NewRequestInfoResolver(kc), kc.RequestContextMapper)
		handler = kapi.WithRequestContext(handler, kc.RequestContextMapper)

		return handler, nil
	}, messages, nil
}

func (c *MasterConfig) RunHealth() {
	apiContainer := genericmux.NewAPIContainer(http.NewServeMux(), kapi.Codecs)

	healthz.InstallHandler(&apiContainer.NonSwaggerRoutes, healthz.PingHealthz)
	initReadinessCheckRoute(apiContainer, "/healthz/ready", func() bool { return true })
	initMetricsRoute(apiContainer, "/metrics")

	c.serve(apiContainer.ServeMux, []string{"Started health checks at %s"})
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

func (c *MasterConfig) InstallProtectedAPI(apiContainer *genericmux.APIContainer) ([]string, error) {
	// initialize OpenShift API
	storage := c.GetRestStorage()

	messages := []string{}
	legacyAPIVersions := []string{}
	currentAPIVersions := []string{}

	if configapi.HasOpenShiftAPILevel(c.Options, v1.SchemeGroupVersion.Version) {
		if err := c.apiLegacyV1(storage).InstallREST(apiContainer.Container); err != nil {
			glog.Fatalf("Unable to initialize v1 API: %v", err)
		}
		messages = append(messages, fmt.Sprintf("Started Origin API at %%s%s/%s", api.Prefix, v1.SchemeGroupVersion.Version))
		currentAPIVersions = append(currentAPIVersions, v1.SchemeGroupVersion.Version)
	}

	// fix API doc string
	for _, service := range apiContainer.Container.RegisteredWebServices() {
		if service.RootPath() == api.Prefix+"/"+v1.SchemeGroupVersion.Version {
			service.Doc("OpenShift REST API, version v1").ApiVersion("v1")
		}
	}

	// The old API prefix must continue to return 200 (with an empty versions
	// list) for backwards compatibility, even though we won't service any other
	// requests through the route. Take care when considering whether to delete
	// this route.
	initAPIVersionRoute(apiContainer, api.LegacyPrefix, legacyAPIVersions...)
	initAPIVersionRoute(apiContainer, api.Prefix, currentAPIVersions...)

	initControllerRoutes(apiContainer, "/controllers", c.Options.Controllers != configapi.ControllersDisabled, c.ControllerPlug)
	// TODO(sttts): use upstream healthz checks for the /healthz/ready route
	initReadinessCheckRoute(apiContainer, "/healthz/ready", c.ProjectAuthorizationCache.ReadyForAccess)
	// TODO(sttts): use upstream version route
	initVersionRoute(apiContainer.Container, "/version/openshift")

	// Set up OAuth metadata only if we are configured to use OAuth
	if c.Options.OAuthConfig != nil {
		initOAuthAuthorizationServerMetadataRoute(apiContainer, oauthMetadataEndpoint, c.Options.OAuthConfig.MasterPublicURL)
	}

	return messages, nil
}

// initVersionRoute initializes an HTTP endpoint for the server's version information.
func initVersionRoute(container *restful.Container, path string) {
	// Build version info once
	versionInfo, err := json.MarshalIndent(version.Get(), "", "  ")
	if err != nil {
		glog.Errorf("Unable to initialize version route: %v", err)
		return
	}

	// Set up a service to return the git code version.
	ws := new(restful.WebService)
	ws.Path(path)
	ws.Doc("git code version from which this is built")
	ws.Route(
		ws.GET("/").To(func(_ *restful.Request, resp *restful.Response) {
			writeJSON(resp, versionInfo)
		}).
			Doc("get the code version").
			Operation("getCodeVersion").
			Produces(restful.MIME_JSON))

	container.Add(ws)
}

func writeJSON(resp *restful.Response, json []byte) {
	resp.ResponseWriter.Header().Set("Content-Type", "application/json")
	resp.ResponseWriter.WriteHeader(http.StatusOK)
	resp.ResponseWriter.Write(json)
}

// initOAuthAuthorizationServerMetadataRoute initializes an HTTP endpoint for OAuth 2.0 Authorization Server Metadata discovery
// https://tools.ietf.org/id/draft-ietf-oauth-discovery-04.html#rfc.section.2
// masterPublicURL should be internally and externally routable to allow all users to discover this information
func initOAuthAuthorizationServerMetadataRoute(apiContainer *genericmux.APIContainer, path, masterPublicURL string) {
	// Build OAuth metadata once
	metadata, err := json.MarshalIndent(discovery.Get(masterPublicURL, OpenShiftOAuthAuthorizeURL(masterPublicURL), OpenShiftOAuthTokenURL(masterPublicURL)), "", "  ")
	if err != nil {
		glog.Errorf("Unable to initialize OAuth authorization server metadata route: %v", err)
		return
	}

	// Create temporary container because we only have a mux for secret routes
	secretContainer := restful.NewContainer()
	secretContainer.ServeMux = apiContainer.SecretRoutes.(*http.ServeMux) // we know it's a *http.ServeMux. In kube 1.6, the type will actually be correct.

	// Set up a service to return the OAuth metadata.
	ws := new(restful.WebService)
	ws.Path(path)
	ws.Doc("OAuth 2.0 Authorization Server Metadata")
	ws.Route(
		ws.GET("/").To(func(_ *restful.Request, resp *restful.Response) {
			writeJSON(resp, metadata)
		}).
			Doc("get the server's OAuth 2.0 Authorization Server Metadata").
			Operation("getOAuthAuthorizationServerMetadata").
			Produces(restful.MIME_JSON))

	secretContainer.Add(ws)
}

func (c *MasterConfig) GetRestStorage() map[string]rest.Storage {
	//TODO/REBASE use something other than c.KubeClientset
	nodeConnectionInfoGetter, err := kubeletclient.NewNodeConnectionInfoGetter(c.KubeClientset().Core().Nodes(), *c.KubeletClientConfig)
	if err != nil {
		glog.Fatalf("Unable to configure the node connection info getter: %v", err)
	}

	// TODO: allow the system CAs and the local CAs to be joined together.
	importTransport, err := restclient.TransportFor(&restclient.Config{})
	if err != nil {
		glog.Fatalf("Unable to configure a default transport for importing: %v", err)
	}
	insecureImportTransport, err := restclient.TransportFor(&restclient.Config{Insecure: true})
	if err != nil {
		glog.Fatalf("Unable to configure a default transport for importing: %v", err)
	}

	buildStorage, buildDetailsStorage, err := buildetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	buildRegistry := buildregistry.NewRegistry(buildStorage)

	buildConfigStorage, err := buildconfigetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	buildConfigRegistry := buildconfigregistry.NewRegistry(buildConfigStorage)

	deployConfigStorage, deployConfigStatusStorage, deployConfigScaleStorage, err := deployconfigetcd.NewREST(c.RESTOptionsGetter)

	dcInstantiateOriginClient, dcInstantiateKubeClient := c.DeploymentConfigInstantiateClients()
	dcInstantiateStorage := deployconfiginstantiate.NewREST(
		*deployConfigStorage.Store,
		dcInstantiateOriginClient,
		dcInstantiateKubeClient,
		c.ExternalVersionCodec,
		c.AdmissionControl,
	)

	checkStorageErr(err)
	deployConfigRegistry := deployconfigregistry.NewRegistry(deployConfigStorage)

	routeAllocator := c.RouteAllocator()

	routeStorage, routeStatusStorage, err := routeetcd.NewREST(c.RESTOptionsGetter, routeAllocator)
	checkStorageErr(err)

	hostSubnetStorage, err := hostsubnetetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	netNamespaceStorage, err := netnamespaceetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	clusterNetworkStorage, err := clusternetworketcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	egressNetworkPolicyStorage, err := egressnetworkpolicyetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)

	userStorage, err := useretcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	userRegistry := userregistry.NewRegistry(userStorage)
	identityStorage, err := identityetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	identityRegistry := identityregistry.NewRegistry(identityStorage)
	userIdentityMappingStorage := useridentitymapping.NewREST(userRegistry, identityRegistry)
	groupStorage, err := groupetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)

	policyStorage, err := policyetcd.NewStorage(c.RESTOptionsGetter)
	checkStorageErr(err)
	policyRegistry := policyregistry.NewRegistry(policyStorage)
	policyBindingStorage, err := policybindingetcd.NewStorage(c.RESTOptionsGetter)
	checkStorageErr(err)
	policyBindingRegistry := policybindingregistry.NewRegistry(policyBindingStorage)

	clusterPolicyStorage, err := clusterpolicystorage.NewStorage(c.RESTOptionsGetter)
	checkStorageErr(err)
	clusterPolicyRegistry := clusterpolicyregistry.NewRegistry(clusterPolicyStorage)
	clusterPolicyBindingStorage, err := clusterpolicybindingstorage.NewStorage(c.RESTOptionsGetter)
	checkStorageErr(err)
	clusterPolicyBindingRegistry := clusterpolicybindingregistry.NewRegistry(clusterPolicyBindingStorage)

	selfSubjectRulesReviewStorage := selfsubjectrulesreview.NewREST(c.RuleResolver, c.Informers.ClusterPolicies().Lister().ClusterPolicies())
	subjectRulesReviewStorage := subjectrulesreview.NewREST(c.RuleResolver, c.Informers.ClusterPolicies().Lister().ClusterPolicies())

	roleStorage := rolestorage.NewVirtualStorage(policyRegistry, c.RuleResolver, nil, authorizationapi.Resource("role"))
	roleBindingStorage := rolebindingstorage.NewVirtualStorage(policyBindingRegistry, c.RuleResolver, nil, authorizationapi.Resource("rolebinding"))
	clusterRoleStorage := clusterrolestorage.NewClusterRoleStorage(clusterPolicyRegistry, clusterPolicyBindingRegistry, c.RuleResolver)
	clusterRoleBindingStorage := clusterrolebindingstorage.NewClusterRoleBindingStorage(clusterPolicyRegistry, clusterPolicyBindingRegistry, c.RuleResolver)

	subjectAccessReviewStorage := subjectaccessreview.NewREST(c.Authorizer)
	subjectAccessReviewRegistry := subjectaccessreview.NewRegistry(subjectAccessReviewStorage)
	localSubjectAccessReviewStorage := localsubjectaccessreview.NewREST(subjectAccessReviewRegistry)
	resourceAccessReviewStorage := resourceaccessreview.NewREST(c.Authorizer)
	resourceAccessReviewRegistry := resourceaccessreview.NewRegistry(resourceAccessReviewStorage)
	localResourceAccessReviewStorage := localresourceaccessreview.NewREST(resourceAccessReviewRegistry)

	podSecurityPolicyReviewStorage := podsecuritypolicyreview.NewREST(oscc.NewDefaultSCCMatcher(c.Informers.SecurityContextConstraints().Lister()), c.Informers.KubernetesInformers().ServiceAccounts().Lister(), c.PrivilegedLoopbackKubernetesClientset)
	podSecurityPolicySubjectStorage := podsecuritypolicysubjectreview.NewREST(oscc.NewDefaultSCCMatcher(c.Informers.SecurityContextConstraints().Lister()), c.PrivilegedLoopbackKubernetesClientset)
	podSecurityPolicySelfSubjectReviewStorage := podsecuritypolicyselfsubjectreview.NewREST(oscc.NewDefaultSCCMatcher(c.Informers.SecurityContextConstraints().Lister()), c.PrivilegedLoopbackKubernetesClientset)

	imageStorage, err := imageetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	imageRegistry := image.NewRegistry(imageStorage)
	imageSignatureStorage := imagesignature.NewREST(c.PrivilegedLoopbackOpenShiftClient.Images())
	imageStreamSecretsStorage := imagesecret.NewREST(c.ImageStreamSecretClient())
	imageStreamStorage, imageStreamStatusStorage, internalImageStreamStorage, err := imagestreametcd.NewREST(c.RESTOptionsGetter, c.RegistryNameFn, subjectAccessReviewRegistry, c.LimitVerifier)
	checkStorageErr(err)
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatusStorage, internalImageStreamStorage)
	imageStreamMappingStorage := imagestreammapping.NewREST(imageRegistry, imageStreamRegistry, c.RegistryNameFn)
	imageStreamTagStorage := imagestreamtag.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamTagRegistry := imagestreamtag.NewRegistry(imageStreamTagStorage)
	importerCache, err := imageimporter.NewImageStreamLayerCache(imageimporter.DefaultImageStreamLayerCacheSize)
	checkStorageErr(err)
	importerFn := func(r importer.RepositoryRetriever) imageimporter.Interface {
		return imageimporter.NewImageStreamImporter(r, c.Options.ImagePolicyConfig.MaxImagesBulkImportedPerRepository, flowcontrol.NewTokenBucketRateLimiter(2.0, 3), &importerCache)
	}
	importerDockerClientFn := func() dockerregistry.Client {
		return dockerregistry.NewClient(20*time.Second, false)
	}
	imageStreamImportStorage := imagestreamimport.NewREST(importerFn, imageStreamRegistry, internalImageStreamStorage, imageStorage, c.ImageStreamImportSecretClient(), importTransport, insecureImportTransport, importerDockerClientFn)
	imageStreamImageStorage := imagestreamimage.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamImageRegistry := imagestreamimage.NewRegistry(imageStreamImageStorage)

	buildGenerator := &buildgenerator.BuildGenerator{
		Client: buildgenerator.Client{
			GetBuildConfigFunc:      buildConfigRegistry.GetBuildConfig,
			UpdateBuildConfigFunc:   buildConfigRegistry.UpdateBuildConfig,
			GetBuildFunc:            buildRegistry.GetBuild,
			CreateBuildFunc:         buildRegistry.CreateBuild,
			GetImageStreamFunc:      imageStreamRegistry.GetImageStream,
			GetImageStreamImageFunc: imageStreamImageRegistry.GetImageStreamImage,
			GetImageStreamTagFunc:   imageStreamTagRegistry.GetImageStreamTag,
		},
		ServiceAccounts: c.KubeClientset(),
		Secrets:         c.KubeClientset(),
	}

	// TODO: with sharding, this needs to be changed
	deployConfigGenerator := &deployconfiggenerator.DeploymentConfigGenerator{
		Client: deployconfiggenerator.Client{
			DCFn:   deployConfigRegistry.GetDeploymentConfig,
			ISFn:   imageStreamRegistry.GetImageStream,
			LISFn2: imageStreamRegistry.ListImageStreams,
		},
	}
	configClient, kclient := c.DeploymentConfigClients()
	deployRollbackClient := deployrollback.Client{
		DCFn: deployConfigRegistry.GetDeploymentConfig,
		RCFn: clientDeploymentInterface{kclient}.GetDeployment,
		GRFn: deployrollback.NewRollbackGenerator().GenerateRollback,
	}
	deployConfigRollbackStorage := deployrollback.NewREST(configClient, kclient, c.ExternalVersionCodec)

	projectStorage := projectproxy.NewREST(c.PrivilegedLoopbackKubernetesClientset.Core().Namespaces(), c.ProjectAuthorizationCache, c.ProjectAuthorizationCache, c.ProjectCache)

	namespace, templateName, err := configapi.ParseNamespaceAndName(c.Options.ProjectConfig.ProjectRequestTemplate)
	if err != nil {
		glog.Errorf("Error parsing project request template value: %v", err)
		// we can continue on, the storage that gets created will be valid, it simply won't work properly.  There's no reason to kill the master
	}
	projectRequestStorage := projectrequeststorage.NewREST(c.Options.ProjectConfig.ProjectRequestMessage, namespace, templateName, c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClientset, c.Informers.PolicyBindings().Lister())

	bcClient := c.BuildConfigWebHookClient()
	buildConfigWebHooks := buildconfigregistry.NewWebHookREST(
		buildConfigRegistry,
		buildclient.NewOSClientBuildConfigInstantiatorClient(bcClient),
		map[string]webhook.Plugin{
			"generic": generic.New(),
			"github":  github.New(),
		},
	)

	clientStorage, err := clientetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)
	clientRegistry := clientregistry.NewRegistry(clientStorage)

	// If OAuth is disabled, set the strategy to Deny
	saAccountGrantMethod := oauthapi.GrantHandlerDeny
	if c.Options.OAuthConfig != nil {
		// Otherwise, take the value provided in master-config.yaml
		saAccountGrantMethod = oauthapi.GrantHandlerType(c.Options.OAuthConfig.GrantConfig.ServiceAccountMethod)
	}

	osClient, kubeClient := c.OAuthServerClients()
	combinedOAuthClientGetter := saoauth.NewServiceAccountOAuthClientGetter(kubeClient.Core(), kubeClient.Core(), osClient, clientRegistry, saAccountGrantMethod)
	authorizeTokenStorage, err := authorizetokenetcd.NewREST(c.RESTOptionsGetter, combinedOAuthClientGetter)
	checkStorageErr(err)
	accessTokenStorage, err := accesstokenetcd.NewREST(c.RESTOptionsGetter, combinedOAuthClientGetter)
	checkStorageErr(err)
	clientAuthorizationStorage, err := clientauthetcd.NewREST(c.RESTOptionsGetter, combinedOAuthClientGetter)
	checkStorageErr(err)

	templateStorage, err := templateetcd.NewREST(c.RESTOptionsGetter)
	checkStorageErr(err)

	storage := map[string]rest.Storage{
		"images":               imageStorage,
		"imagesignatures":      imageSignatureStorage,
		"imageStreams/secrets": imageStreamSecretsStorage,
		"imageStreams":         imageStreamStorage,
		"imageStreams/status":  imageStreamStatusStorage,
		"imageStreamImports":   imageStreamImportStorage,
		"imageStreamImages":    imageStreamImageStorage,
		"imageStreamMappings":  imageStreamMappingStorage,
		"imageStreamTags":      imageStreamTagStorage,

		"deploymentConfigs":             deployConfigStorage,
		"deploymentConfigs/scale":       deployConfigScaleStorage,
		"deploymentConfigs/status":      deployConfigStatusStorage,
		"deploymentConfigs/rollback":    deployConfigRollbackStorage,
		"deploymentConfigs/log":         deploylogregistry.NewREST(configClient, kclient, c.DeploymentLogClient(), nodeConnectionInfoGetter),
		"deploymentConfigs/instantiate": dcInstantiateStorage,

		// TODO: Deprecate these
		"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, c.ExternalVersionCodec),
		"deploymentConfigRollbacks": deployrollback.NewDeprecatedREST(deployRollbackClient, c.ExternalVersionCodec),

		"processedTemplates": templateregistry.NewREST(),
		"templates":          templateStorage,

		"routes":        routeStorage,
		"routes/status": routeStatusStorage,

		"projects":        projectStorage,
		"projectRequests": projectRequestStorage,

		"hostSubnets":           hostSubnetStorage,
		"netNamespaces":         netNamespaceStorage,
		"clusterNetworks":       clusterNetworkStorage,
		"egressNetworkPolicies": egressNetworkPolicyStorage,

		"users":                userStorage,
		"groups":               groupStorage,
		"identities":           identityStorage,
		"userIdentityMappings": userIdentityMappingStorage,

		"oAuthAuthorizeTokens":      authorizeTokenStorage,
		"oAuthAccessTokens":         accessTokenStorage,
		"oAuthClients":              clientStorage,
		"oAuthClientAuthorizations": clientAuthorizationStorage,

		"resourceAccessReviews":      resourceAccessReviewStorage,
		"subjectAccessReviews":       subjectAccessReviewStorage,
		"localSubjectAccessReviews":  localSubjectAccessReviewStorage,
		"localResourceAccessReviews": localResourceAccessReviewStorage,
		"selfSubjectRulesReviews":    selfSubjectRulesReviewStorage,
		"subjectRulesReviews":        subjectRulesReviewStorage,

		"podSecurityPolicyReviews":            podSecurityPolicyReviewStorage,
		"podSecurityPolicySubjectReviews":     podSecurityPolicySubjectStorage,
		"podSecurityPolicySelfSubjectReviews": podSecurityPolicySelfSubjectReviewStorage,

		"policies":       policyStorage,
		"policyBindings": policyBindingStorage,
		"roles":          roleStorage,
		"roleBindings":   roleBindingStorage,

		"clusterPolicies":       clusterPolicyStorage,
		"clusterPolicyBindings": clusterPolicyBindingStorage,
		"clusterRoleBindings":   clusterRoleBindingStorage,
		"clusterRoles":          clusterRoleStorage,

		"clusterResourceQuotas":        restInPeace(clusterresourcequotaregistry.NewStorage(c.RESTOptionsGetter)),
		"clusterResourceQuotas/status": updateInPeace(clusterresourcequotaregistry.NewStatusStorage(c.RESTOptionsGetter)),
		"appliedClusterResourceQuotas": appliedclusterresourcequotaregistry.NewREST(
			c.ClusterQuotaMappingController.GetClusterQuotaMapper(), c.Informers.ClusterResourceQuotas().Lister(), c.Informers.KubernetesInformers().Namespaces().Lister()),

		"roleBindingRestrictions": restInPeace(rolebindingrestrictionregistry.NewStorage(c.RESTOptionsGetter)),
	}

	if configapi.IsBuildEnabled(&c.Options) {
		storage["builds"] = buildStorage
		storage["builds/clone"] = buildclone.NewStorage(buildGenerator)
		storage["builds/log"] = buildlogregistry.NewREST(buildStorage, buildStorage, c.BuildLogClient().Core(), nodeConnectionInfoGetter)
		storage["builds/details"] = buildDetailsStorage

		storage["buildConfigs"] = buildConfigStorage
		storage["buildConfigs/webhooks"] = buildConfigWebHooks
		storage["buildConfigs/instantiate"] = buildconfiginstantiate.NewStorage(buildGenerator)
		storage["buildConfigs/instantiatebinary"] = buildconfiginstantiate.NewBinaryStorage(buildGenerator, buildStorage, c.BuildLogClient(), nodeConnectionInfoGetter)
	}

	return storage
}

func checkStorageErr(err error) {
	if err != nil {
		glog.Fatalf("Error building REST storage: %v", err)
	}
}

// initAPIVersionRoute initializes the osapi endpoint to behave similar to the upstream api endpoint
func initAPIVersionRoute(apiContainer *genericmux.APIContainer, prefix string, versions ...string) {
	versionHandler := apiserver.APIVersionHandler(kapi.Codecs, func(req *restful.Request) *unversioned.APIVersions {
		apiVersionsForDiscovery := unversioned.APIVersions{
			// TODO: ServerAddressByClientCIDRs: s.getServerAddressByClientCIDRs(req.Request),
			Versions: versions,
		}
		return &apiVersionsForDiscovery
	})
	ws := new(restful.WebService).
		Path(prefix).
		Doc("list supported server API versions")
	ws.Route(ws.GET("/").To(versionHandler).
		Doc("list supported server API versions").
		Produces(restful.MIME_JSON).
		Consumes(restful.MIME_JSON).
		Operation("get" + strings.Title(prefix[1:]) + "Version"))
	apiContainer.Add(ws)
}

// initReadinessCheckRoute initializes an HTTP endpoint for readiness checking
func initReadinessCheckRoute(apiContainer *genericmux.APIContainer, path string, readyFunc func() bool) {
	ws := new(restful.WebService).
		Path(path).
		Doc("return the readiness state of the master")
	ws.Route(ws.GET("/").To(func(req *restful.Request, resp *restful.Response) {
		if readyFunc() {
			resp.ResponseWriter.WriteHeader(http.StatusOK)
			resp.ResponseWriter.Write([]byte("ok"))

		} else {
			resp.ResponseWriter.WriteHeader(http.StatusServiceUnavailable)
		}
	}).Doc("return the readiness state of the master").
		Returns(http.StatusOK, "if the master is ready", nil).
		Returns(http.StatusServiceUnavailable, "if the master is not ready", nil).
		Produces(restful.MIME_JSON))

	apiContainer.Add(ws)
}

// initMetricsRoute initializes an HTTP endpoint for metrics.
func initMetricsRoute(apiContainer *genericmux.APIContainer, path string) {
	ws := new(restful.WebService).
		Path(path).
		Doc("return metrics for this process")
	h := prometheus.Handler()
	ws.Route(ws.GET("/").To(func(req *restful.Request, resp *restful.Response) {
		h.ServeHTTP(resp.ResponseWriter, req.Request)
	}).Doc("return metrics for this process").
		Returns(http.StatusOK, "if metrics are available", nil).
		Produces("text/plain"))

	apiContainer.Add(ws)
}

func (c *MasterConfig) defaultAPIGroupVersion() *apiserver.APIGroupVersion {
	var restMapper meta.MultiRESTMapper
	seenGroups := sets.String{}
	for _, gv := range registered.EnabledVersions() {
		if seenGroups.Has(gv.Group) {
			continue
		}
		seenGroups.Insert(gv.Group)

		groupMeta, err := registered.Group(gv.Group)
		if err != nil {
			continue
		}
		restMapper = meta.MultiRESTMapper(append(restMapper, groupMeta.RESTMapper))
	}

	statusMapper := meta.NewDefaultRESTMapper([]unversioned.GroupVersion{kubeapiv1.SchemeGroupVersion}, registered.GroupOrDie(kapi.GroupName).InterfacesFor)
	statusMapper.Add(kubeapiv1.SchemeGroupVersion.WithKind("Status"), meta.RESTScopeRoot)
	restMapper = meta.MultiRESTMapper(append(restMapper, statusMapper))

	return &apiserver.APIGroupVersion{
		Root: api.Prefix,

		Mapper: restMapper,

		Creater:   kapi.Scheme,
		Typer:     kapi.Scheme,
		Convertor: kapi.Scheme,
		Copier:    kapi.Scheme,
		Linker:    registered.GroupOrDie("").SelfLinker,

		Admit:                       c.AdmissionControl,
		Context:                     c.getRequestContextMapper(),
		SubresourceGroupVersionKind: map[string]unversioned.GroupVersionKind{},
	}
}

// apiLegacyV1 returns the resources and codec for API version v1.
func (c *MasterConfig) apiLegacyV1(all map[string]rest.Storage) *apiserver.APIGroupVersion {
	storage := make(map[string]rest.Storage)
	for k, v := range all {
		if excludedV1Types.Has(k) {
			continue
		}
		storage[strings.ToLower(k)] = v
	}
	version := c.defaultAPIGroupVersion()
	version.Storage = storage
	version.GroupVersion = v1.SchemeGroupVersion
	version.Serializer = kapi.Codecs
	version.ParameterCodec = runtime.NewParameterCodec(kapi.Scheme)
	version.SubresourceGroupVersionKind["deploymentconfigs/scale"] = v1beta1extensions.SchemeGroupVersion.WithKind("Scale")
	return version
}

// getRequestContextMapper returns a mapper from requests to contexts, initializing it if needed
func (c *MasterConfig) getRequestContextMapper() kapi.RequestContextMapper {
	if c.RequestContextMapper == nil {
		c.RequestContextMapper = kapi.NewRequestContextMapper()
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

type clientDeploymentInterface struct {
	KubeClient kclientset.Interface
}

// GetDeployment returns the deployment with the provided context and name
func (c clientDeploymentInterface) GetDeployment(ctx kapi.Context, name string) (*kapi.ReplicationController, error) {
	return c.KubeClient.Core().ReplicationControllers(kapi.NamespaceValue(ctx)).Get(name)
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
