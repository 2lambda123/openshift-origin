package origin

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/elazarl/go-bindata-assetfs"
	"github.com/emicklei/go-restful"
	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kmaster "github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
	"github.com/openshift/origin/pkg/assets"
	"github.com/openshift/origin/pkg/auth/authenticator"
	authcontext "github.com/openshift/origin/pkg/auth/context"
	authfilter "github.com/openshift/origin/pkg/auth/handlers"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildcontrollerfactory "github.com/openshift/origin/pkg/build/controller/factory"
	buildstrategy "github.com/openshift/origin/pkg/build/controller/strategy"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildlogregistry "github.com/openshift/origin/pkg/build/registry/buildlog"
	buildetcd "github.com/openshift/origin/pkg/build/registry/etcd"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/generic"
	"github.com/openshift/origin/pkg/build/webhook/github"
	osclient "github.com/openshift/origin/pkg/client"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deploycontrollerfactory "github.com/openshift/origin/pkg/deploy/controller/factory"
	deployconfiggenerator "github.com/openshift/origin/pkg/deploy/generator"
	deployregistry "github.com/openshift/origin/pkg/deploy/registry/deploy"
	deployconfigregistry "github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	deployetcd "github.com/openshift/origin/pkg/deploy/registry/etcd"
	deployrollback "github.com/openshift/origin/pkg/deploy/rollback"
	imageetcd "github.com/openshift/origin/pkg/image/registry/etcd"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorymapping"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorytag"
	accesstokenregistry "github.com/openshift/origin/pkg/oauth/registry/accesstoken"
	authorizetokenregistry "github.com/openshift/origin/pkg/oauth/registry/authorizetoken"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/client"
	clientauthorizationregistry "github.com/openshift/origin/pkg/oauth/registry/clientauthorization"
	oauthetcd "github.com/openshift/origin/pkg/oauth/registry/etcd"
	projectetcd "github.com/openshift/origin/pkg/project/registry/etcd"
	projectregistry "github.com/openshift/origin/pkg/project/registry/project"
	routeetcd "github.com/openshift/origin/pkg/route/registry/etcd"
	routeregistry "github.com/openshift/origin/pkg/route/registry/route"
	"github.com/openshift/origin/pkg/service"
	templateregistry "github.com/openshift/origin/pkg/template/registry"
	"github.com/openshift/origin/pkg/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"
	"github.com/openshift/origin/pkg/version"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	authorizationetcd "github.com/openshift/origin/pkg/authorization/registry/etcd"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	roleregistry "github.com/openshift/origin/pkg/authorization/registry/role"
	rolebindingregistry "github.com/openshift/origin/pkg/authorization/registry/rolebinding"
)

const (
	OpenShiftAPIPrefix        = "/osapi"
	OpenShiftAPIPrefixV1Beta1 = OpenShiftAPIPrefix + "/v1beta1"
)

// MasterConfig defines the required parameters for starting the OpenShift master
type MasterConfig struct {
	// host:port to bind master to
	MasterBindAddr string
	// host:port to bind asset server to
	AssetBindAddr string
	// url to access the master API on within the cluster
	MasterAddr string
	// url to access kubernetes API on within the cluster
	KubernetesAddr string
	// external clients may need to access APIs at different addresses than internal components do
	MasterPublicAddr     string
	KubernetesPublicAddr string
	AssetPublicAddr      string

	TLS bool

	CORSAllowedOrigins            []*regexp.Regexp
	Authenticator                 authenticator.Request
	Authorizer                    authorizer.Authorizer
	AuthorizationAttributeBuilder authorizer.AuthorizationAttributeBuilder
	// RequestsToUsers is used by both authentication and authorization.  This is a shared, in-memory map, so they must use exactly the same instance
	RequestsToUsers              *authcontext.RequestContextMap
	MasterAuthorizationNamespace string

	EtcdHelper tools.EtcdHelper

	AdmissionControl admission.Interface

	MasterCertFile string
	MasterKeyFile  string
	AssetCertFile  string
	AssetKeyFile   string

	// kubeClient is the client used to call Kubernetes APIs from system components, built from KubeClientConfig.
	// It should only be accessed via the *Client() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically for that component.
	kubeClient *kclient.Client
	// KubeClientConfig is the client configuration used to call Kubernetes APIs from system components.
	// To apply different access control to a system component, create a client config specifically for that component.
	KubeClientConfig kclient.Config

	// osClient is the client used to call OpenShift APIs from system components, built from OSClientConfig.
	// It should only be accessed via the *Client() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically for that component.
	osClient *osclient.Client
	// OSClientConfig is the client configuration used to call OpenShift APIs from system components
	// To apply different access control to a system component, create a client config specifically for that component.
	OSClientConfig kclient.Config

	// DeployerOSClientConfig is the client configuration used to call OpenShift APIs from launched deployer pods
	DeployerOSClientConfig kclient.Config
}

// APIInstaller installs additional API components into this server
type APIInstaller interface {
	// Returns an array of strings describing what was installed
	InstallAPI(*restful.Container) []string
}

func (c *MasterConfig) BuildClients() {
	kubeClient, err := kclient.New(&c.KubeClientConfig)
	if err != nil {
		glog.Fatalf("Unable to configure client: %v", err)
	}
	c.kubeClient = kubeClient

	osclient, err := osclient.New(&c.OSClientConfig)
	if err != nil {
		glog.Fatalf("Unable to configure client: %v", err)
	}
	c.osClient = osclient
}

func (c *MasterConfig) KubeClient() *kclient.Client {
	return c.kubeClient
}
func (c *MasterConfig) DeploymentClient() *kclient.Client {
	return c.kubeClient
}
func (c *MasterConfig) BuildLogClient() *kclient.Client {
	return c.kubeClient
}
func (c *MasterConfig) WebHookClient() *osclient.Client {
	return c.osClient
}
func (c *MasterConfig) BuildControllerClients() (*osclient.Client, *kclient.Client) {
	return c.osClient, c.kubeClient
}
func (c *MasterConfig) ImageChangeControllerClient() *osclient.Client {
	return c.osClient
}
func (c *MasterConfig) DeploymentControllerClients() (*osclient.Client, *kclient.Client) {
	return c.osClient, c.kubeClient
}

// DeployerClientConfig returns the client configuration a Deployer instance launched in a pod
// should use when making API calls.
func (c *MasterConfig) DeployerClientConfig() *kclient.Config {
	return &c.DeployerOSClientConfig
}

func (c *MasterConfig) DeploymentConfigControllerClients() (*osclient.Client, *kclient.Client) {
	return c.osClient, c.kubeClient
}
func (c *MasterConfig) DeploymentConfigChangeControllerClients() (*osclient.Client, *kclient.Client) {
	return c.osClient, c.kubeClient
}
func (c *MasterConfig) DeploymentImageChangeControllerClient() *osclient.Client {
	return c.osClient
}

// EnsureCORSAllowedOrigins takes a string list of origins and attempts to covert them to CORS origin
// regexes, or exits if it cannot.
func (c *MasterConfig) EnsureCORSAllowedOrigins(origins []string) {
	if len(origins) > 0 {
		allowedOriginRegexps, err := util.CompileRegexps(util.StringList(origins))
		if err != nil {
			glog.Fatalf("Invalid CORS allowed origin, --corsAllowedOrigins flag was set to %v - %v", strings.Join(origins, ","), err)
		}
		c.CORSAllowedOrigins = allowedOriginRegexps
	}
}

func (c *MasterConfig) InstallAPI(container *restful.Container) []string {
	defaultRegistry := env("OPENSHIFT_DEFAULT_REGISTRY", "${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT}")
	svcCache := service.NewServiceResolverCache(c.KubeClient().Services(api.NamespaceDefault).Get)
	defaultRegistryFunc, err := svcCache.Defer(defaultRegistry)
	if err != nil {
		glog.Fatalf("OPENSHIFT_DEFAULT_REGISTRY variable is invalid %q: %v", defaultRegistry, err)
	}

	buildEtcd := buildetcd.New(c.EtcdHelper)
	imageEtcd := imageetcd.New(c.EtcdHelper, imageetcd.DefaultRegistryFunc(defaultRegistryFunc))
	deployEtcd := deployetcd.New(c.EtcdHelper)
	routeEtcd := routeetcd.New(c.EtcdHelper)
	projectEtcd := projectetcd.New(c.EtcdHelper)
	userEtcd := useretcd.New(c.EtcdHelper, user.NewDefaultUserInitStrategy())
	oauthEtcd := oauthetcd.New(c.EtcdHelper)
	authorizationEtcd := authorizationetcd.New(c.EtcdHelper)

	// TODO: with sharding, this needs to be changed
	deployConfigGenerator := &deployconfiggenerator.DeploymentConfigGenerator{
		Client: deployconfiggenerator.Client{
			DCFn:   deployEtcd.GetDeploymentConfig,
			IRFn:   imageEtcd.GetImageRepository,
			LIRFn2: imageEtcd.ListImageRepositories,
		},
		Codec: latest.Codec,
	}
	_, kclient := c.DeploymentConfigControllerClients()
	deployRollback := &deployrollback.RollbackGenerator{}
	deployRollbackClient := deployrollback.Client{
		DCFn: deployEtcd.GetDeploymentConfig,
		RCFn: clientDeploymentInterface{kclient}.GetDeployment,
		GRFn: deployRollback.GenerateRollback,
	}

	// initialize OpenShift API
	storage := map[string]apiserver.RESTStorage{
		"builds":       buildregistry.NewREST(buildEtcd),
		"buildConfigs": buildconfigregistry.NewREST(buildEtcd),
		"buildLogs":    buildlogregistry.NewREST(buildEtcd, c.BuildLogClient()),

		"images":                  image.NewREST(imageEtcd),
		"imageRepositories":       imagerepository.NewREST(imageEtcd),
		"imageRepositoryMappings": imagerepositorymapping.NewREST(imageEtcd, imageEtcd),
		"imageRepositoryTags":     imagerepositorytag.NewREST(imageEtcd, imageEtcd),

		"deployments":               deployregistry.NewREST(deployEtcd),
		"deploymentConfigs":         deployconfigregistry.NewREST(deployEtcd),
		"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, v1beta1.Codec),
		"deploymentConfigRollbacks": deployrollback.NewREST(deployRollbackClient, latest.Codec),

		"templateConfigs": templateregistry.NewREST(),

		"routes": routeregistry.NewREST(routeEtcd),

		"projects": projectregistry.NewREST(projectEtcd),

		"userIdentityMappings": useridentitymapping.NewREST(userEtcd),
		"users":                userregistry.NewREST(userEtcd),

		"authorizeTokens":      authorizetokenregistry.NewREST(oauthEtcd),
		"accessTokens":         accesstokenregistry.NewREST(oauthEtcd),
		"clients":              clientregistry.NewREST(oauthEtcd),
		"clientAuthorizations": clientauthorizationregistry.NewREST(oauthEtcd),

		"policies":       policyregistry.NewREST(authorizationEtcd),
		"policyBindings": policybindingregistry.NewREST(authorizationEtcd),
		"roles":          roleregistry.NewREST(authorizationEtcd),
		"roleBindings":   rolebindingregistry.NewREST(authorizationEtcd, authorizationEtcd, userEtcd, c.MasterAuthorizationNamespace),
	}

	whPrefix := OpenShiftAPIPrefixV1Beta1 + "/buildConfigHooks/"
	container.ServeMux.Handle(whPrefix, http.StripPrefix(whPrefix,
		webhook.NewController(ClientWebhookInterface{c.WebHookClient()}, map[string]webhook.Plugin{
			"generic": generic.New(),
			"github":  github.New(),
		})))

	admissionControl := admit.NewAlwaysAdmit()

	apiserver.NewAPIGroupVersion(storage, v1beta1.Codec, OpenShiftAPIPrefixV1Beta1, latest.SelfLinker, admissionControl).InstallREST(container, OpenShiftAPIPrefix, "v1beta1")

	var root *restful.WebService
	for _, svc := range container.RegisteredWebServices() {
		switch svc.RootPath() {
		case "/":
			root = svc
		case OpenShiftAPIPrefixV1Beta1:
			svc.Doc("OpenShift REST API, version v1beta1").ApiVersion("v1beta1")
		}
	}
	if root == nil {
		root = new(restful.WebService)
		container.Add(root)
	}
	initAPIVersionRoute(root, "v1beta1")

	return []string{
		fmt.Sprintf("Started OpenShift API at %%s%s", OpenShiftAPIPrefixV1Beta1),
	}
}

//initAPIVersionRoute initializes the osapi endpoint to behave similiar to the upstream api endpoint
func initAPIVersionRoute(root *restful.WebService, version string) {
	versionHandler := apiserver.APIVersionHandler(version)
	root.Route(root.GET(OpenShiftAPIPrefix).To(versionHandler).
		Doc("list supported server API versions").
		Produces(restful.MIME_JSON).
		Consumes(restful.MIME_JSON))
}

// Run launches the OpenShift master. It takes optional installers that may install additional endpoints into the server.
// All endpoints get configured CORS behavior
// Protected installers' endpoints are protected by API authentication and authorization.
// Unprotected installers' endpoints do not have any additional protection added.
func (c *MasterConfig) Run(protectedInstallers []APIInstaller, unprotectedInstallers []APIInstaller) {
	var extra []string

	// Build container for protected endpoints
	protectedContainer := kmaster.NewHandlerContainer(http.NewServeMux())
	for _, i := range protectedInstallers {
		extra = append(extra, i.InstallAPI(protectedContainer)...)
	}
	// Add authentication
	protectedHandler := http.Handler(protectedContainer)
	if c.Authenticator != nil {
		if c.Authorizer != nil && c.AuthorizationAttributeBuilder != nil {
			protectedHandler = wrapHandlerWithAuthorization(protectedHandler, c.Authorizer, c.AuthorizationAttributeBuilder)
			c.ensureComponentAuthorizationRules()
		}

		protectedHandler = c.wireAuthenticationHandling(protectedContainer.ServeMux, protectedHandler, c.Authenticator, c.RequestsToUsers)
	}

	// Build container for unprotected endpoints
	rootMux := http.NewServeMux()
	rootHandler := http.Handler(rootMux)
	unprotectedContainer := kmaster.NewHandlerContainer(rootMux)
	for _, i := range unprotectedInstallers {
		extra = append(extra, i.InstallAPI(unprotectedContainer)...)
	}
	// Add CORS support
	if len(c.CORSAllowedOrigins) > 0 {
		rootHandler = apiserver.CORS(rootHandler, c.CORSAllowedOrigins, nil, nil, "true")
	}
	// Delegate all unhandled endpoints to the api handler
	rootMux.Handle("/", protectedHandler)

	server := &http.Server{
		Addr:           c.MasterBindAddr,
		Handler:        rootHandler,
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	go util.Forever(func() {
		for _, s := range extra {
			glog.Infof(s, c.MasterAddr)
		}
		if c.TLS {
			server.TLSConfig = &tls.Config{
				// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
				MinVersion: tls.VersionTLS10,
				// Populate PeerCertificates in requests, but don't reject connections without certificates
				// This allows certificates to be validated by authenticators, while still allowing other auth types
				ClientAuth: tls.RequestClientCert,
			}
			glog.Fatal(server.ListenAndServeTLS(c.MasterCertFile, c.MasterKeyFile))
		} else {
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial("tcp", c.MasterBindAddr, 100*time.Millisecond, 100*time.Millisecond, 100)
}

// wireAuthenticationHandling creates and binds all the objects that we only care about if authentication is turned on.  It's pulled out
// just to make the RunAPI method easier to read.  These resources include the requestsToUsers map that allows callers to know which user
// is requesting an operation, the handler wrapper that protects the passed handler behind a handler that requires valid authentication
// information on the request, and an endpoint that only functions properly with an authenticated user.
func (c *MasterConfig) wireAuthenticationHandling(osMux *http.ServeMux, handler http.Handler, authenticator authenticator.Request, requestsToUsers *authcontext.RequestContextMap) http.Handler {
	// wrapHandlerWithAuthentication binds a handler that will correlate the users and requests
	handler = wrapHandlerWithAuthentication(handler, authenticator, requestsToUsers)

	// this requires the requests and users to be present
	userContextMap := userregistry.ContextFunc(func(req *http.Request) (userregistry.Info, bool) {
		obj, found := requestsToUsers.Get(req)
		if user, ok := obj.(userregistry.Info); found && ok {
			return user, true
		}
		return nil, false
	})
	// TODO: this is flawed, needs to be able to identify the right endpoints
	thisUserEndpoint := OpenShiftAPIPrefixV1Beta1 + "/users/~"
	userregistry.InstallThisUser(osMux, thisUserEndpoint, userContextMap, handler)

	return handler
}

func (c *MasterConfig) ensureComponentAuthorizationRules() {
	registry := authorizationetcd.New(c.EtcdHelper)
	ctx := kapi.WithNamespace(kapi.NewContext(), c.MasterAuthorizationNamespace)

	if existing, err := registry.GetPolicy(ctx, authorizationapi.PolicyName); err == nil || strings.Contains(err.Error(), " not found") {
		if existing != nil && existing.Name == authorizationapi.PolicyName {
			return
		}

		bootstrapGlobalPolicy := authorizer.GetBootstrapPolicy(c.MasterAuthorizationNamespace)
		if err = registry.CreatePolicy(ctx, bootstrapGlobalPolicy); err != nil {
			glog.Errorf("Error creating policy: %v due to %v\n", bootstrapGlobalPolicy, err)
		}

	} else {
		glog.Errorf("Error getting policy: %v due to %v\n", authorizationapi.PolicyName, err)
	}

	if existing, err := registry.GetPolicyBinding(ctx, c.MasterAuthorizationNamespace); err == nil || strings.Contains(err.Error(), " not found") {
		if existing != nil && existing.Name == c.MasterAuthorizationNamespace {
			return
		}

		bootstrapGlobalPolicyBinding := authorizer.GetBootstrapPolicyBinding(c.MasterAuthorizationNamespace)
		if err = registry.CreatePolicyBinding(ctx, bootstrapGlobalPolicyBinding); err != nil {
			glog.Errorf("Error creating policy: %v due to %v\n", bootstrapGlobalPolicyBinding, err)
		}

	} else {
		glog.Errorf("Error getting policy: %v due to %v\n", c.MasterAuthorizationNamespace, err)
	}
}

// wrapHandlerWithAuthentication takes a handler and protects it behind a handler that tests to make sure that a user is authenticated.
// if the request does have value auth information, then the request is allowed through the passed handler.  If the request does not have
// valid auth information, then the request is passed to a failure handler.  Until we get authentication for system componenets, the
// failure handler logs and passes through.
func wrapHandlerWithAuthentication(handler http.Handler, authenticator authenticator.Request, requestsToUsers *authcontext.RequestContextMap) http.Handler {
	return authfilter.NewRequestAuthenticator(
		requestsToUsers,
		authenticator,
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			return
		}),
		handler)
}

func wrapHandlerWithAuthorization(handler http.Handler, authorizer authorizer.Authorizer, authorizationAttributeBuilder authorizer.AuthorizationAttributeBuilder) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attributes, err := authorizationAttributeBuilder.GetAttributes(req)
		if err != nil {
			// fail
			forbidden(err.Error(), w, req)
		}
		if attributes == nil {
			// fail
			forbidden("No attributes", w, req)
		}

		allowed, reason, err := authorizer.Authorize(attributes)
		if err != nil {
			// fail
			forbidden(err.Error(), w, req)
		}

		if allowed {
			handler.ServeHTTP(w, req)
			return
		}

		forbidden(reason, w, req)
	})
}

// forbidden renders a simple forbidden error
func forbidden(reason string, w http.ResponseWriter, req *http.Request) {
	glog.V(1).Infof("!!!!!!!!!!!! FORBIDDING because %v!\n", reason)
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, "Forbidden: \"%#v\" because \"%v\"", req.RequestURI, reason)
}

// RunAssetServer starts the asset server for the OpenShift UI.
func (c *MasterConfig) RunAssetServer() {
	// TODO use	version.Get().GitCommit as an etag cache header
	mux := http.NewServeMux()

	masterURL, err := url.Parse(c.MasterPublicAddr)
	if err != nil {
		glog.Fatalf("Error parsing master url: %v", err)
	}

	k8sURL, err := url.Parse(c.KubernetesPublicAddr)
	if err != nil {
		glog.Fatalf("Error parsing kubernetes url: %v", err)
	}

	mux.Handle("/",
		// Gzip first so that inner handlers can react to the addition of the Vary header
		assets.GzipHandler(
			// Generated config.js can not be cached since it changes depending on startup options
			assets.GeneratedConfigHandler(
				masterURL.Host,
				OpenShiftAPIPrefix,
				k8sURL.Host,
				"/api",
				// Cache control should happen after all Vary headers are added, but before
				// any asset related routing (HTML5ModeHandler and FileServer)
				assets.CacheControlHandler(
					version.Get().GitCommit,
					assets.HTML5ModeHandler(
						http.FileServer(
							&assetfs.AssetFS{
								assets.Asset,
								assets.AssetDir,
								"",
							},
						),
					),
				),
			),
		),
	)

	server := &http.Server{
		Addr:           c.AssetBindAddr,
		Handler:        mux,
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	go util.Forever(func() {
		if c.TLS {
			server.TLSConfig = &tls.Config{
				// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
				MinVersion: tls.VersionTLS10,
				// Populate PeerCertificates in requests, but don't reject connections without certificates
				// This allows certificates to be validated by authenticators, while still allowing other auth types
				ClientAuth: tls.RequestClientCert,
			}
			glog.Infof("OpenShift UI listening at https://%s", c.AssetBindAddr)
			glog.Fatal(server.ListenAndServeTLS(c.AssetCertFile, c.AssetKeyFile))
		} else {
			glog.Infof("OpenShift UI listening at https://%s", c.AssetBindAddr)
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial("tcp", c.AssetBindAddr, 100*time.Millisecond, 100*time.Millisecond, 100)

	glog.Infof("OpenShift UI available at %s", c.AssetPublicAddr)
}

// RunBuildController starts the build sync loop for builds and buildConfig processing.
func (c *MasterConfig) RunBuildController() {
	// initialize build controller
	dockerImage := env("OPENSHIFT_DOCKER_BUILDER_IMAGE", "openshift/origin-docker-builder")
	stiImage := env("OPENSHIFT_STI_BUILDER_IMAGE", "openshift/origin-sti-builder")
	useLocalImages := env("USE_LOCAL_IMAGES", "true") == "true"

	osclient, kclient := c.BuildControllerClients()
	factory := buildcontrollerfactory.BuildControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
		DockerBuildStrategy: &buildstrategy.DockerBuildStrategy{
			Image:          dockerImage,
			UseLocalImages: useLocalImages,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: v1beta1.Codec,
		},
		STIBuildStrategy: &buildstrategy.STIBuildStrategy{
			Image:                stiImage,
			TempDirectoryCreator: buildstrategy.STITempDirectoryCreator,
			UseLocalImages:       useLocalImages,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: v1beta1.Codec,
		},
		CustomBuildStrategy: &buildstrategy.CustomBuildStrategy{
			UseLocalImages: useLocalImages,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: v1beta1.Codec,
		},
	}

	controller := factory.Create()
	controller.Run()
}

// RunDeploymentController starts the build image change trigger controller process.
func (c *MasterConfig) RunBuildImageChangeTriggerController() {
	factory := buildcontrollerfactory.ImageChangeControllerFactory{Client: c.ImageChangeControllerClient()}
	factory.Create().Run()
}

// RunDeploymentController starts the deployment controller process.
func (c *MasterConfig) RunDeploymentController() {
	osclient, kclient := c.DeploymentControllerClients()
	factory := deploycontrollerfactory.DeploymentControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
		Codec:      latest.Codec,
		Environment: []api.EnvVar{
			{Name: "KUBERNETES_MASTER", Value: c.MasterAddr},
			{Name: "OPENSHIFT_MASTER", Value: c.MasterAddr},
		},
		UseLocalImages:        env("USE_LOCAL_IMAGES", "true") == "true",
		RecreateStrategyImage: env("OPENSHIFT_DEPLOY_RECREATE_IMAGE", "openshift/origin-deployer"),
	}

	envvars := clientcmd.EnvVarsFromConfig(c.DeployerClientConfig())
	factory.Environment = append(factory.Environment, envvars...)

	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentConfigController() {
	osclient, kclient := c.DeploymentConfigControllerClients()
	factory := deploycontrollerfactory.DeploymentConfigControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
		Codec:      latest.Codec,
	}
	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentConfigChangeController() {
	osclient, kclient := c.DeploymentConfigChangeControllerClients()
	factory := deploycontrollerfactory.DeploymentConfigChangeControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
		Codec:      latest.Codec,
	}
	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentImageChangeTriggerController() {
	osclient := c.DeploymentImageChangeControllerClient()
	factory := deploycontrollerfactory.ImageChangeControllerFactory{Client: osclient}
	controller := factory.Create()
	controller.Run()
}

// NewEtcdHelper returns an EtcdHelper for the provided arguments or an error if the version
// is incorrect.
func NewEtcdHelper(version string, client *etcdclient.Client) (helper tools.EtcdHelper, err error) {
	if len(version) == 0 {
		version = latest.Version
	}
	interfaces, err := latest.InterfacesFor(version)
	if err != nil {
		return helper, err
	}
	return tools.EtcdHelper{client, interfaces.Codec, tools.RuntimeVersionAdapter{interfaces.MetadataAccessor}}, nil
}

// env returns an environment variable, or the defaultValue if it is not set.
func env(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	return val
}

// ClientWebhookInterface is a webhookBuildInterface which delegates to the OpenShift client interfaces
type ClientWebhookInterface struct {
	Client osclient.Interface
}

// CreateBuild creates build using OpenShift client.
func (c ClientWebhookInterface) CreateBuild(namespace string, build *buildapi.Build) (*buildapi.Build, error) {
	return c.Client.Builds(namespace).Create(build)
}

// GetBuildConfig returns buildConfig using OpenShift client.
func (c ClientWebhookInterface) GetBuildConfig(namespace, name string) (*buildapi.BuildConfig, error) {
	return c.Client.BuildConfigs(namespace).Get(name)
}

type clientDeploymentInterface struct {
	KubeClient kclient.Interface
}

func (c clientDeploymentInterface) GetDeployment(ctx api.Context, name string) (*api.ReplicationController, error) {
	return c.KubeClient.ReplicationControllers(api.Namespace(ctx)).Get(name)
}
