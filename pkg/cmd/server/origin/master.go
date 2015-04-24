package origin

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	kui "github.com/GoogleCloudPlatform/kubernetes/pkg/ui"
	assetfs "github.com/elazarl/go-bindata-assetfs"

	etcdclient "github.com/coreos/go-etcd/etcd"
	restful "github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/swagger"
	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierror "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kmaster "github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	utilerrs "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
	"github.com/openshift/origin/pkg/api/v1beta3"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildcontrollerfactory "github.com/openshift/origin/pkg/build/controller/factory"
	buildstrategy "github.com/openshift/origin/pkg/build/controller/strategy"
	buildgenerator "github.com/openshift/origin/pkg/build/generator"
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
	configchangecontroller "github.com/openshift/origin/pkg/deploy/controller/configchange"
	deployerpodcontroller "github.com/openshift/origin/pkg/deploy/controller/deployerpod"
	deploycontroller "github.com/openshift/origin/pkg/deploy/controller/deployment"
	deployconfigcontroller "github.com/openshift/origin/pkg/deploy/controller/deploymentconfig"
	imagechangecontroller "github.com/openshift/origin/pkg/deploy/controller/imagechange"
	deployconfiggenerator "github.com/openshift/origin/pkg/deploy/generator"
	deployregistry "github.com/openshift/origin/pkg/deploy/registry/deploy"
	deployconfigregistry "github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	deployetcd "github.com/openshift/origin/pkg/deploy/registry/etcd"
	deployrollback "github.com/openshift/origin/pkg/deploy/rollback"
	"github.com/openshift/origin/pkg/dns"
	imagecontroller "github.com/openshift/origin/pkg/image/controller"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorymapping"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorytag"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
	"github.com/openshift/origin/pkg/image/registry/imagestreammapping"
	"github.com/openshift/origin/pkg/image/registry/imagestreamtag"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	authorizetokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken/etcd"
	clientetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclient/etcd"
	clientauthetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization/etcd"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectcontroller "github.com/openshift/origin/pkg/project/controller"
	projectproxy "github.com/openshift/origin/pkg/project/registry/project/proxy"
	projectrequeststorage "github.com/openshift/origin/pkg/project/registry/projectrequest/delegated"
	routeallocationcontroller "github.com/openshift/origin/pkg/route/controller/allocation"
	routeetcd "github.com/openshift/origin/pkg/route/registry/etcd"
	routeregistry "github.com/openshift/origin/pkg/route/registry/route"
	"github.com/openshift/origin/pkg/service"
	templateregistry "github.com/openshift/origin/pkg/template/registry"
	templateetcd "github.com/openshift/origin/pkg/template/registry/etcd"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	identityetcd "github.com/openshift/origin/pkg/user/registry/identity/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	"github.com/openshift/origin/pkg/user/registry/useridentitymapping"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policyetcd "github.com/openshift/origin/pkg/authorization/registry/policy/etcd"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	policybindingetcd "github.com/openshift/origin/pkg/authorization/registry/policybinding/etcd"
	resourceaccessreviewregistry "github.com/openshift/origin/pkg/authorization/registry/resourceaccessreview"
	roleregistry "github.com/openshift/origin/pkg/authorization/registry/role"
	rolebindingregistry "github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	routeplugin "github.com/openshift/origin/plugins/route/allocation/simple"
)

const (
	OpenShiftAPIPrefix        = "/osapi" // TODO: make configurable
	KubernetesAPIPrefix       = "/api"   // TODO: make configurable
	OpenShiftAPIV1Beta1       = "v1beta1"
	OpenShiftAPIV1Beta3       = "v1beta3"
	OpenShiftAPIPrefixV1Beta1 = OpenShiftAPIPrefix + "/" + OpenShiftAPIV1Beta1
	OpenShiftAPIPrefixV1Beta3 = OpenShiftAPIPrefix + "/" + OpenShiftAPIV1Beta3
	OpenShiftRouteSubdomain   = "router.default.local"
	swaggerAPIPrefix          = "/swaggerapi/"
	swaggerUIPrefix           = "/swagger-ui/"
)

// APIInstaller installs additional API components into this server
type APIInstaller interface {
	// Returns an array of strings describing what was installed
	InstallAPI(*restful.Container) []string
}

// APIInstallFunc is a function for installing APIs
type APIInstallFunc func(*restful.Container) []string

// InstallAPI implements APIInstaller
func (fn APIInstallFunc) InstallAPI(container *restful.Container) []string {
	return fn(container)
}

func (c *MasterConfig) InstallProtectedAPI(container *restful.Container) []string {
	defaultRegistry := env("OPENSHIFT_DEFAULT_REGISTRY", "${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT}")
	svcCache := service.NewServiceResolverCache(c.KubeClient().Services(api.NamespaceDefault).Get)
	defaultRegistryFunc, err := svcCache.Defer(defaultRegistry)
	if err != nil {
		glog.Fatalf("OPENSHIFT_DEFAULT_REGISTRY variable is invalid %q: %v", defaultRegistry, err)
	}

	kubeletClient, err := kclient.NewKubeletClient(c.KubeletClientConfig)
	if err != nil {
		glog.Fatalf("Unable to configure Kubelet client: %v", err)
	}

	buildEtcd := buildetcd.New(c.EtcdHelper)
	deployEtcd := deployetcd.New(c.EtcdHelper)
	routeEtcd := routeetcd.New(c.EtcdHelper)

	userStorage := useretcd.NewREST(c.EtcdHelper)
	userRegistry := userregistry.NewRegistry(userStorage)
	identityStorage := identityetcd.NewREST(c.EtcdHelper)
	identityRegistry := identityregistry.NewRegistry(identityStorage)
	userIdentityMappingStorage := useridentitymapping.NewREST(userRegistry, identityRegistry)

	policyStorage := policyetcd.NewStorage(c.EtcdHelper)
	policyRegistry := policyregistry.NewRegistry(policyStorage)
	policyBindingStorage := policybindingetcd.NewStorage(c.EtcdHelper)
	policyBindingRegistry := policybindingregistry.NewRegistry(policyBindingStorage)
	roleBindingRegistry := rolebindingregistry.NewVirtualRegistry(policyBindingRegistry, policyRegistry, c.Options.PolicyConfig.MasterAuthorizationNamespace)
	subjectAccessReviewStorage := subjectaccessreview.NewREST(c.Authorizer)
	subjectAccessReviewRegistry := subjectaccessreview.NewRegistry(subjectAccessReviewStorage)

	imageStorage := imageetcd.NewREST(c.EtcdHelper)
	imageRegistry := image.NewRegistry(imageStorage)
	imageStreamStorage, imageStreamStatusStorage := imagestreametcd.NewREST(c.EtcdHelper, imagestream.DefaultRegistryFunc(defaultRegistryFunc), subjectAccessReviewRegistry)
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatusStorage)
	imageStreamMappingStorage := imagestreammapping.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamMappingRegistry := imagestreammapping.NewRegistry(imageStreamMappingStorage)
	imageStreamTagStorage := imagestreamtag.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamTagRegistry := imagestreamtag.NewRegistry(imageStreamTagStorage)
	imageStreamImageStorage := imagestreamimage.NewREST(imageRegistry, imageStreamRegistry)

	imageRepositoryStorage, imageRepositoryStatusStorage := imagerepository.NewREST(imageStreamRegistry)
	imageRepositoryMappingStorage := imagerepositorymapping.NewREST(imageStreamMappingRegistry)
	imageRepositoryTagStorage := imagerepositorytag.NewREST(imageStreamTagRegistry)

	routeAllocator := c.RouteAllocator()

	buildGenerator := &buildgenerator.BuildGenerator{
		Client: buildgenerator.Client{
			GetBuildConfigFunc:    buildEtcd.GetBuildConfig,
			UpdateBuildConfigFunc: buildEtcd.UpdateBuildConfig,
			GetBuildFunc:          buildEtcd.GetBuild,
			CreateBuildFunc:       buildEtcd.CreateBuild,
			GetImageStreamFunc:    imageStreamRegistry.GetImageStream,
		},
	}
	buildClone, buildConfigInstantiate := buildgenerator.NewREST(buildGenerator)

	// TODO: with sharding, this needs to be changed
	deployConfigGenerator := &deployconfiggenerator.DeploymentConfigGenerator{
		Client: deployconfiggenerator.Client{
			DCFn:   deployEtcd.GetDeploymentConfig,
			ISFn:   imageStreamRegistry.GetImageStream,
			LISFn2: imageStreamRegistry.ListImageStreams,
		},
	}
	_, kclient := c.DeploymentConfigControllerClients()
	deployRollback := &deployrollback.RollbackGenerator{}
	deployRollbackClient := deployrollback.Client{
		DCFn: deployEtcd.GetDeploymentConfig,
		RCFn: clientDeploymentInterface{kclient}.GetDeployment,
		GRFn: deployRollback.GenerateRollback,
	}

	projectStorage := projectproxy.NewREST(kclient.Namespaces(), c.ProjectAuthorizationCache)

	// initialize OpenShift API
	storage := map[string]rest.Storage{
		"builds":                   buildregistry.NewREST(buildEtcd),
		"builds/clone":             buildClone,
		"buildConfigs":             buildconfigregistry.NewREST(buildEtcd),
		"buildConfigs/instantiate": buildConfigInstantiate,
		"buildLogs":                buildlogregistry.NewREST(buildEtcd, c.BuildLogClient(), kubeletClient),

		"images":                   imageStorage,
		"imageStreams":             imageStreamStorage,
		"imageStreams/status":      imageStreamStatusStorage,
		"imageStreamImages":        imageStreamImageStorage,
		"imageStreamMappings":      imageStreamMappingStorage,
		"imageStreamTags":          imageStreamTagStorage,
		"imageRepositories":        imageRepositoryStorage,
		"imageRepositories/status": imageRepositoryStatusStorage,
		"imageRepositoryMappings":  imageRepositoryMappingStorage,
		"imageRepositoryTags":      imageRepositoryTagStorage,

		"deployments":               deployregistry.NewREST(deployEtcd),
		"deploymentConfigs":         deployconfigregistry.NewREST(deployEtcd),
		"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, latest.Codec),
		"deploymentConfigRollbacks": deployrollback.NewREST(deployRollbackClient, latest.Codec),

		"processedTemplates": templateregistry.NewREST(false),
		"templates":          templateetcd.NewREST(c.EtcdHelper),
		// DEPRECATED: remove with v1beta1
		"templateConfigs": templateregistry.NewREST(true),

		"routes": routeregistry.NewREST(routeEtcd, routeAllocator),

		"projects":        projectStorage,
		"projectRequests": projectrequeststorage.NewREST(c.Options.PolicyConfig.MasterAuthorizationNamespace, roleBindingRegistry, *projectStorage),

		"users":                userStorage,
		"identities":           identityStorage,
		"userIdentityMappings": userIdentityMappingStorage,

		"oAuthAuthorizeTokens":      authorizetokenetcd.NewREST(c.EtcdHelper),
		"oAuthAccessTokens":         accesstokenetcd.NewREST(c.EtcdHelper),
		"oAuthClients":              clientetcd.NewREST(c.EtcdHelper),
		"oAuthClientAuthorizations": clientauthetcd.NewREST(c.EtcdHelper),

		"policies":              policyStorage,
		"policyBindings":        policyBindingStorage,
		"roles":                 roleregistry.NewREST(roleregistry.NewVirtualRegistry(policyRegistry)),
		"roleBindings":          rolebindingregistry.NewREST(roleBindingRegistry),
		"resourceAccessReviews": resourceaccessreviewregistry.NewREST(c.Authorizer),
		"subjectAccessReviews":  subjectAccessReviewStorage,
	}

	// for v1beta1, we dual register camelCase and camelcase names
	v1beta1Storage := map[string]rest.Storage{}
	for k, v := range storage {
		v1beta1Storage[k] = v
		v1beta1Storage[strings.ToLower(k)] = v
	}
	v1beta3Storage := map[string]rest.Storage{}
	for k, v := range storage {
		if k == "templateConfigs" {
			continue
		}
		v1beta3Storage[strings.ToLower(k)] = v
	}

	version := &apiserver.APIGroupVersion{
		Root:    OpenShiftAPIPrefix,
		Version: OpenShiftAPIV1Beta1,

		Storage: v1beta1Storage,
		Codec:   v1beta1.Codec,

		Mapper: latest.RESTMapper,

		Creater:   kapi.Scheme,
		Typer:     kapi.Scheme,
		Convertor: kapi.Scheme,
		Linker:    latest.SelfLinker,

		Admit:   c.AdmissionControl,
		Context: c.getRequestContextMapper(),
	}

	if err := version.InstallREST(container); err != nil {
		glog.Fatalf("Unable to initialize v1beta1 API: %v", err)
	}

	version2 := &apiserver.APIGroupVersion{
		Root:    OpenShiftAPIPrefix,
		Version: OpenShiftAPIV1Beta3,

		Storage: v1beta3Storage,
		Codec:   v1beta3.Codec,

		Mapper: latest.RESTMapper,

		Creater: kapi.Scheme,
		Typer:   kapi.Scheme,
		Linker:  latest.SelfLinker,

		Admit:   c.AdmissionControl,
		Context: c.getRequestContextMapper(),
	}

	if err := version2.InstallREST(container); err != nil {
		// TODO: remove this check once v1beta3 is complete
		if utilerrs.FilterOut(err, func(err error) bool {
			return strings.Contains(err.Error(), "is registered for version")
		}) != nil {
			glog.Fatalf("Unable to initialize v1beta3 API: %v", err)
		}
	}

	var root *restful.WebService
	for _, svc := range container.RegisteredWebServices() {
		switch svc.RootPath() {
		case "/":
			root = svc
		case OpenShiftAPIPrefixV1Beta1:
			svc.Doc("OpenShift REST API, version v1beta1").ApiVersion("v1beta1")
		case OpenShiftAPIPrefixV1Beta3:
			svc.Doc("OpenShift REST API, version v1beta3").ApiVersion("v1beta3")
		}
	}
	if root == nil {
		root = new(restful.WebService)
		container.Add(root)
	}
	initAPIVersionRoute(root, "v1beta1", "v1beta3")

	return []string{
		fmt.Sprintf("Started OpenShift API at %%s%s", OpenShiftAPIPrefixV1Beta1),
		fmt.Sprintf("Started OpenShift API at %%s%s (experimental)", OpenShiftAPIPrefixV1Beta3),
	}
}

func (c *MasterConfig) InstallUnprotectedAPI(container *restful.Container) []string {
	bcClient, _ := c.BuildControllerClients()
	bcGetterUpdater := buildclient.NewOSClientBuildConfigClient(bcClient)
	handler := webhook.NewController(
		bcGetterUpdater,
		buildclient.NewOSClientBuildConfigInstantiatorClient(bcClient),
		bcClient.ImageStreams(kapi.NamespaceAll).(osclient.ImageStreamNamespaceGetter),
		map[string]webhook.Plugin{
			"generic": generic.New(),
			"github":  github.New(),
		})

	// TODO: go-restfulize this
	prefix := OpenShiftAPIPrefixV1Beta1 + "/buildConfigHooks/"
	handler = http.StripPrefix(prefix, handler)
	container.Handle(prefix, handler)
	return []string{}
}

//initAPIVersionRoute initializes the osapi endpoint to behave similar to the upstream api endpoint
func initAPIVersionRoute(root *restful.WebService, versions ...string) {
	versionHandler := apiserver.APIVersionHandler(versions...)
	root.Route(root.GET(OpenShiftAPIPrefix).To(versionHandler).
		Doc("list supported server API versions").
		Produces(restful.MIME_JSON).
		Consumes(restful.MIME_JSON))
}

// If we know the location of the asset server, redirect to it when / is requested
// and the Accept header supports text/html
func assetServerRedirect(handler http.Handler, assetPublicURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		accept := req.Header.Get("Accept")
		if req.URL.Path == "/" && strings.Contains(accept, "text/html") {
			http.Redirect(w, req, assetPublicURL, http.StatusFound)
		} else {
			// Dispatch to the next handler
			handler.ServeHTTP(w, req)
		}
	})
}

// TODO We would like to use the IndexHandler from k8s but we do not yet have a
// MuxHelper to track all registered paths
func indexAPIPaths(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			// TODO once we have a MuxHelper we will not need to hardcode this list of paths
			object := api.RootPaths{Paths: []string{
				"/api",
				"/api/v1beta1",
				"/api/v1beta3",
				"/api/v1beta3",
				"/healthz",
				"/healthz/ping",
				"/logs/",
				"/metrics",
				"/osapi",
				"/osapi/v1beta1",
				swaggerAPIPrefix,
				swaggerUIPrefix,
			}}
			// TODO it would be nice if apiserver.writeRawJSON was not private
			output, err := json.MarshalIndent(object, "", "  ")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(output)
		} else {
			// Dispatch to the next handler
			handler.ServeHTTP(w, req)
		}
	})
}

// Run launches the OpenShift master. It takes optional installers that may install additional endpoints into the server.
// All endpoints get configured CORS behavior
// Protected installers' endpoints are protected by API authentication and authorization.
// Unprotected installers' endpoints do not have any additional protection added.
func (c *MasterConfig) Run(protected []APIInstaller, unprotected []APIInstaller) {
	var extra []string

	safe := kmaster.NewHandlerContainer(http.NewServeMux())
	open := kmaster.NewHandlerContainer(http.NewServeMux())

	// enforce authentication on protected endpoints
	protected = append(protected, APIInstallFunc(c.InstallProtectedAPI))
	for _, i := range protected {
		extra = append(extra, i.InstallAPI(safe)...)
	}
	handler := c.authorizationFilter(safe)
	handler = authenticationHandlerFilter(handler, c.Authenticator, c.getRequestContextMapper())
	handler = namespacingFilter(handler, c.getRequestContextMapper())

	// unprotected resources
	unprotected = append(unprotected, APIInstallFunc(c.InstallUnprotectedAPI))
	for _, i := range unprotected {
		extra = append(extra, i.InstallAPI(open)...)
	}

	handler = indexAPIPaths(handler)

	open.Handle("/", handler)

	// install swagger
	// Expose files in third_party/swagger-ui/ on <host>/swagger-ui
	fileServer := http.FileServer(&assetfs.AssetFS{Asset: kui.Asset, AssetDir: kui.AssetDir, Prefix: "third_party/swagger-ui"})
	open.Handle(swaggerUIPrefix, http.StripPrefix(swaggerUIPrefix, fileServer))

	swaggerConfig := swagger.Config{
		WebServicesUrl:  c.Options.MasterPublicURL,
		WebServices:     append(safe.RegisteredWebServices(), open.RegisteredWebServices()...),
		ApiPath:         swaggerAPIPrefix,
		SwaggerPath:     "/swaggerui/",
		SwaggerFilePath: swaggerUIPrefix,
	}
	// log nothing from swagger
	swagger.LogInfo = func(format string, v ...interface{}) {}
	swagger.RegisterSwaggerService(swaggerConfig, open)
	extra = append(extra, fmt.Sprintf("Started Swagger Schema API at %%s%s", swaggerAPIPrefix))
	extra = append(extra, fmt.Sprintf("Started Swagger UI at %%s%s", swaggerUIPrefix))

	handler = open

	// add CORS support
	if origins := c.ensureCORSAllowedOrigins(); len(origins) != 0 {
		handler = apiserver.CORS(handler, origins, nil, nil, "true")
	}

	if c.Options.AssetConfig != nil {
		handler = assetServerRedirect(handler, c.Options.AssetConfig.PublicURL)
	}

	// Make the outermost filter the requestContextMapper to ensure all components share the same context
	if contextHandler, err := kapi.NewRequestContextFilter(c.getRequestContextMapper(), handler); err != nil {
		glog.Fatalf("Error setting up request context filter: %v", err)
	} else {
		handler = contextHandler
	}

	server := &http.Server{
		Addr:           c.Options.ServingInfo.BindAddress,
		Handler:        handler,
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	go util.Forever(func() {
		for _, s := range extra {
			glog.Infof(s, c.Options.ServingInfo.BindAddress)
		}
		if c.TLS {
			server.TLSConfig = &tls.Config{
				// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
				MinVersion: tls.VersionTLS10,
				// Populate PeerCertificates in requests, but don't reject connections without certificates
				// This allows certificates to be validated by authenticators, while still allowing other auth types
				ClientAuth: tls.RequestClientCert,
				ClientCAs:  c.ClientCAs,
			}
			glog.Fatal(server.ListenAndServeTLS(c.Options.ServingInfo.ServerCert.CertFile, c.Options.ServingInfo.ServerCert.KeyFile))
		} else {
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial(c.TLS, "tcp", c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)

	// Attempt to create the required policy rules now, and then stick in a forever loop to make sure they are always available
	c.ensureComponentAuthorizationRules()
	c.ensureMasterAuthorizationNamespace()
	c.ensureOpenShiftSharedResourcesNamespace()
	go util.Forever(func() {
		c.ensureMasterAuthorizationNamespace()
		c.ensureOpenShiftSharedResourcesNamespace()
	}, 10*time.Second)
}

// getRequestContextMapper returns a mapper from requests to contexts, initializing it if needed
func (c *MasterConfig) getRequestContextMapper() kapi.RequestContextMapper {
	if c.RequestContextMapper == nil {
		c.RequestContextMapper = kapi.NewRequestContextMapper()
	}
	return c.RequestContextMapper
}

// ensureMasterAuthorizationNamespace is called as part of global policy initialization to ensure master namespace exists
func (c *MasterConfig) ensureMasterAuthorizationNamespace() {

	// ensure that master namespace actually exists
	namespace, err := c.KubeClient().Namespaces().Get(c.Options.PolicyConfig.MasterAuthorizationNamespace)
	if err != nil {
		namespace = &kapi.Namespace{
			ObjectMeta: kapi.ObjectMeta{Name: c.Options.PolicyConfig.MasterAuthorizationNamespace},
			Spec: kapi.NamespaceSpec{
				Finalizers: []kapi.FinalizerName{projectapi.FinalizerProject},
			},
		}
		kapi.FillObjectMetaSystemFields(api.NewContext(), &namespace.ObjectMeta)
		_, err = c.KubeClient().Namespaces().Create(namespace)
		if err != nil {
			glog.Errorf("Error creating namespace: %v due to %v\n", namespace, err)
		}
	}
}

// ensureOpenShiftSharedResourcesNamespace is called as part of global policy initialization to ensure shared namespace exists
func (c *MasterConfig) ensureOpenShiftSharedResourcesNamespace() {
	namespace, err := c.KubeClient().Namespaces().Get(c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace)
	if err != nil {
		namespace = &kapi.Namespace{
			ObjectMeta: kapi.ObjectMeta{Name: c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace},
			Spec: kapi.NamespaceSpec{
				Finalizers: []kapi.FinalizerName{projectapi.FinalizerProject},
			},
		}
		kapi.FillObjectMetaSystemFields(api.NewContext(), &namespace.ObjectMeta)
		_, err = c.KubeClient().Namespaces().Create(namespace)
		if err != nil {
			glog.Errorf("Error creating namespace: %v due to %v\n", namespace, err)
		}
	}
}

// ensureComponentAuthorizationRules initializes the global policies
func (c *MasterConfig) ensureComponentAuthorizationRules() {
	policyRegistry := policyregistry.NewRegistry(policyetcd.NewStorage(c.EtcdHelper))
	ctx := kapi.WithNamespace(kapi.NewContext(), c.Options.PolicyConfig.MasterAuthorizationNamespace)

	if _, err := policyRegistry.GetPolicy(ctx, authorizationapi.PolicyName); kapierror.IsNotFound(err) {
		glog.Infof("No master policy found.  Creating bootstrap policy based on: %v", c.Options.PolicyConfig.BootstrapPolicyFile)

		if err := admin.OverwriteBootstrapPolicy(c.EtcdHelper, c.Options.PolicyConfig.MasterAuthorizationNamespace, c.Options.PolicyConfig.BootstrapPolicyFile, admin.CreateBootstrapPolicyFileFullCommand, true, ioutil.Discard); err != nil {
			glog.Errorf("Error creating bootstrap policy: %v", err)
		}

	} else {
		glog.V(2).Infof("Ignoring bootstrap policy file because master policy found")
	}
}

func (c *MasterConfig) authorizationFilter(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attributes, err := c.AuthorizationAttributeBuilder.GetAttributes(req)
		if err != nil {
			forbidden(err.Error(), "", w, req)
			return
		}
		if attributes == nil {
			forbidden("No attributes", "", w, req)
			return
		}

		ctx, exists := c.RequestContextMapper.Get(req)
		if !exists {
			forbidden("context not found", attributes.GetAPIVersion(), w, req)
			return
		}

		allowed, reason, err := c.Authorizer.Authorize(ctx, attributes)
		if err != nil {
			forbidden(err.Error(), attributes.GetAPIVersion(), w, req)
			return
		}
		if !allowed {
			forbidden(reason, attributes.GetAPIVersion(), w, req)
			return
		}

		handler.ServeHTTP(w, req)
	})
}

// forbidden renders a simple forbidden error
func forbidden(reason, apiVersion string, w http.ResponseWriter, req *http.Request) {
	// the api version can be empty for two basic reasons:
	// 1. malformed API request
	// 2. not an API request at all
	// In these cases, just assume the latest version that will work better than nothing
	if len(apiVersion) == 0 {
		apiVersion = klatest.Version
	}

	// Reason is an opaque string that describes why access is allowed or forbidden (forbidden by the time we reach here).
	// We don't have direct access to kind or name (not that those apply either in the general case)
	// We create a NewForbidden to stay close the API, but then we override the message to get a serialization
	// that makes sense when a human reads it.
	forbiddenError, _ := kapierror.NewForbidden("", "", errors.New("")).(*kapierror.StatusError)
	forbiddenError.ErrStatus.Message = fmt.Sprintf("%q is forbidden because %s", req.RequestURI, reason)

	// Not all API versions in valid API requests will have a matching codec in kubernetes.  If we can't find one,
	// just default to the latest kube codec.
	codec := klatest.Codec
	if requestedCodec, err := klatest.InterfacesFor(apiVersion); err == nil {
		codec = requestedCodec
	}

	formatted := &bytes.Buffer{}
	output, err := codec.Encode(&forbiddenError.ErrStatus)
	if err != nil {
		fmt.Fprintf(formatted, "%s", forbiddenError.Error())
	} else {
		_ = json.Indent(formatted, output, "", "  ")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write(formatted.Bytes())
}

// RunProjectAuthorizationCache starts the project authorization cache
func (c *MasterConfig) RunProjectAuthorizationCache() {
	// TODO: look at exposing a configuration option in future to control how often we run this loop
	period := 1 * time.Second
	c.ProjectAuthorizationCache.Run(period)
}

// RunOriginNamespaceController starts the controller that takes part in namespace termination of openshift content
func (c *MasterConfig) RunOriginNamespaceController() {
	osclient, kclient := c.OriginNamespaceControllerClients()
	factory := projectcontroller.NamespaceControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
	}
	controller := factory.Create()
	controller.Run()
}

// RunPolicyCache starts the policy cache
func (c *MasterConfig) RunPolicyCache() {
	c.PolicyCache.Run()
}

// RunAssetServer starts the asset server for the OpenShift UI.
func (c *MasterConfig) RunAssetServer() {

}

func (c *MasterConfig) RunDNSServer() {
	config, err := dns.NewServerDefaults()
	if err != nil {
		glog.Fatalf("Could not start DNS: %v", err)
	}
	config.DnsAddr = c.Options.DNSConfig.BindAddress

	_, port, err := net.SplitHostPort(c.Options.DNSConfig.BindAddress)
	if err != nil {
		glog.Fatalf("Could not start DNS: %v", err)
	}
	if port != "53" {
		glog.Warningf("Binding DNS on port %v instead of 53 (you may need to run as root and update your config), using %s which will not resolve from all locations", port, c.Options.DNSConfig.BindAddress)
	}

	if ok, err := cmdutil.TryListen(c.Options.DNSConfig.BindAddress); !ok {
		glog.Warningf("Could not start DNS: %v", err)
		return
	}

	go func() {
		err := dns.ListenAndServe(config, c.DNSServerClient(), c.EtcdHelper.Client.(*etcdclient.Client))
		glog.Fatalf("Could not start DNS: %v", err)
	}()

	cmdutil.WaitForSuccessfulDial(false, "tcp", c.Options.DNSConfig.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)

	glog.Infof("OpenShift DNS listening at %s", c.Options.DNSConfig.BindAddress)
}

// RunBuildController starts the build sync loop for builds and buildConfig processing.
func (c *MasterConfig) RunBuildController() {
	// initialize build controller
	dockerImage := c.ImageFor("docker-builder")
	stiImage := c.ImageFor("sti-builder")

	osclient, kclient := c.BuildControllerClients()
	factory := buildcontrollerfactory.BuildControllerFactory{
		OSClient:     osclient,
		KubeClient:   kclient,
		BuildUpdater: buildclient.NewOSClientBuildClient(osclient),
		DockerBuildStrategy: &buildstrategy.DockerBuildStrategy{
			Image: dockerImage,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: v1beta1.Codec,
		},
		STIBuildStrategy: &buildstrategy.STIBuildStrategy{
			Image:                stiImage,
			TempDirectoryCreator: buildstrategy.STITempDirectoryCreator,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: v1beta1.Codec,
		},
		CustomBuildStrategy: &buildstrategy.CustomBuildStrategy{
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: v1beta1.Codec,
		},
	}

	controller := factory.Create()
	controller.Run()
}

// RunBuildPodController starts the build/pod status sync loop for build status
func (c *MasterConfig) RunBuildPodController() {
	osclient, kclient := c.BuildControllerClients()
	factory := buildcontrollerfactory.BuildPodControllerFactory{
		OSClient:     osclient,
		KubeClient:   kclient,
		BuildUpdater: buildclient.NewOSClientBuildClient(osclient),
	}
	controller := factory.Create()
	controller.Run()
}

// RunBuildImageChangeTriggerController starts the build image change trigger controller process.
func (c *MasterConfig) RunBuildImageChangeTriggerController() {
	bcClient, _ := c.BuildControllerClients()
	bcUpdater := buildclient.NewOSClientBuildConfigClient(bcClient)
	bcInstantiator := buildclient.NewOSClientBuildConfigInstantiatorClient(bcClient)
	factory := buildcontrollerfactory.ImageChangeControllerFactory{Client: bcClient, BuildConfigInstantiator: bcInstantiator, BuildConfigUpdater: bcUpdater}
	factory.Create().Run()
}

// RunDeploymentController starts the deployment controller process.
func (c *MasterConfig) RunDeploymentController() error {
	_, kclient := c.DeploymentControllerClients()

	_, kclientConfig, err := configapi.GetKubeClient(c.Options.MasterClients.OpenShiftLoopbackKubeConfig)
	if err != nil {
		return err
	}
	// TODO eliminate these environment variables once we figure out what they do
	env := []api.EnvVar{
		{Name: "KUBERNETES_MASTER", Value: kclientConfig.Host},
		{Name: "OPENSHIFT_MASTER", Value: kclientConfig.Host},
	}
	env = append(env, clientcmd.EnvVarsFromConfig(c.DeployerClientConfig())...)

	factory := deploycontroller.DeploymentControllerFactory{
		KubeClient:            kclient,
		Codec:                 latest.Codec,
		Environment:           env,
		RecreateStrategyImage: c.ImageFor("deployer"),
	}

	controller := factory.Create()
	controller.Run()

	return nil
}

// RunDeployerPodController starts the deployer pod controller process.
func (c *MasterConfig) RunDeployerPodController() {
	_, kclient := c.DeploymentControllerClients()
	factory := deployerpodcontroller.DeployerPodControllerFactory{
		KubeClient: kclient,
	}

	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentConfigController() {
	osclient, kclient := c.DeploymentConfigControllerClients()
	factory := deployconfigcontroller.DeploymentConfigControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
		Codec:      latest.Codec,
	}
	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentConfigChangeController() {
	osclient, kclient := c.DeploymentConfigChangeControllerClients()
	factory := configchangecontroller.DeploymentConfigChangeControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
		Codec:      latest.Codec,
	}
	controller := factory.Create()
	controller.Run()
}

func (c *MasterConfig) RunDeploymentImageChangeTriggerController() {
	osclient := c.DeploymentImageChangeControllerClient()
	factory := imagechangecontroller.ImageChangeControllerFactory{Client: osclient}
	controller := factory.Create()
	controller.Run()
}

// RouteAllocator returns a route allocation controller.
func (c *MasterConfig) RouteAllocator() *routeallocationcontroller.RouteAllocationController {
	factory := routeallocationcontroller.RouteAllocationControllerFactory{
		OSClient:   c.OSClient,
		KubeClient: c.KubeClient(),
	}

	subdomain := env("OPENSHIFT_ROUTE_SUBDOMAIN", OpenShiftRouteSubdomain)

	plugin, err := routeplugin.NewSimpleAllocationPlugin(subdomain)
	if err != nil {
		glog.Fatalf("Route plugin initialization failed: %v", err)
	}

	return factory.Create(plugin)
}

func (c *MasterConfig) RunImageImportController() {
	osclient := c.ImageImportControllerClient()
	factory := imagecontroller.ImportControllerFactory{
		Client: osclient,
	}
	controller := factory.Create()
	controller.Run()
}

// ensureCORSAllowedOrigins takes a string list of origins and attempts to covert them to CORS origin
// regexes, or exits if it cannot.
func (c *MasterConfig) ensureCORSAllowedOrigins() []*regexp.Regexp {
	if len(c.Options.CORSAllowedOrigins) == 0 {
		return []*regexp.Regexp{}
	}
	allowedOriginRegexps, err := util.CompileRegexps(util.StringList(c.Options.CORSAllowedOrigins))
	if err != nil {
		glog.Fatalf("Invalid --cors-allowed-origins: %v", err)
	}
	return allowedOriginRegexps
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
	KubeClient kclient.Interface
}

func (c clientDeploymentInterface) GetDeployment(ctx api.Context, name string) (*api.ReplicationController, error) {
	return c.KubeClient.ReplicationControllers(api.NamespaceValue(ctx)).Get(name)
}

// namespacingFilter adds a filter that adds the namespace of the request to the context.  Not all requests will have namespaces,
// but any that do will have the appropriate value added.
func namespacingFilter(handler http.Handler, contextMapper kapi.RequestContextMapper) http.Handler {
	infoResolver := &apiserver.APIRequestInfoResolver{util.NewStringSet("api", "osapi"), latest.RESTMapper}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx, ok := contextMapper.Get(req)
		if !ok {
			http.Error(w, "Unable to find request context", http.StatusInternalServerError)
			return
		}

		if _, exists := kapi.NamespaceFrom(ctx); !exists {
			if requestInfo, err := infoResolver.GetAPIRequestInfo(req); err == nil {
				// only set the namespace if the apiRequestInfo was resolved
				// keep in mind that GetAPIRequestInfo will fail on non-api requests, so don't fail the entire http request on that
				// kind of failure.

				// TODO reconsider special casing this.  Having the special case hereallow us to fully share the kube
				// APIRequestInfoResolver without any modification or customization.
				namespace := requestInfo.Namespace
				if (requestInfo.Resource == "projects") && (len(requestInfo.Name) > 0) {
					namespace = requestInfo.Name
				}

				ctx = kapi.WithNamespace(ctx, namespace)
				contextMapper.Update(req, ctx)
			}
		}

		handler.ServeHTTP(w, req)
	})
}
