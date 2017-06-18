package start

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	restclient "k8s.io/client-go/rest"
	kctrlmgr "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/capabilities"
	kinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	"k8s.io/kubernetes/pkg/controller"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/api/validation"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/etcd/etcdserver"
	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	origincontrollers "github.com/openshift/origin/pkg/cmd/server/origin/controller"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/pluginconfig"
	override "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	"github.com/openshift/origin/pkg/version"
)

type MasterOptions struct {
	MasterArgs *MasterArgs

	CreateCertificates bool
	ExpireDays         int
	SignerExpireDays   int
	ConfigFile         string
	Output             io.Writer
	DisabledFeatures   []string
}

func (o *MasterOptions) DefaultsFromName(basename string) {}

var masterLong = templates.LongDesc(`
	Start a master server

	This command helps you launch a master server.  Running

	    %[1]s start master

	will start a master listening on all interfaces, launch an etcd server to store
	persistent data, and launch the Kubernetes system components. The server will run in the
	foreground until you terminate the process.

	Note: starting the master without passing the --master address will attempt to find the IP
	address that will be visible inside running Docker containers. This is not always successful,
	so if you have problems tell the master what public address it should use via --master=<ip>.

	You may also pass --etcd=<address> to connect to an external etcd server.

	You may also pass --kubeconfig=<path> to connect to an external Kubernetes cluster.`)

// NewCommandStartMaster provides a CLI handler for 'start master' command
func NewCommandStartMaster(basename string, out, errout io.Writer) (*cobra.Command, *MasterOptions) {
	options := &MasterOptions{
		ExpireDays:       crypto.DefaultCertificateLifetimeInDays,
		SignerExpireDays: crypto.DefaultCACertificateLifetimeInDays,
		Output:           out,
	}
	options.DefaultsFromName(basename)

	cmd := &cobra.Command{
		Use:   "master",
		Short: "Launch a master",
		Long:  fmt.Sprintf(masterLong, basename),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete())
			kcmdutil.CheckErr(options.Validate(args))

			startProfiler()

			if err := options.StartMaster(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(errout, "Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(errout, "  %s: %s\n", cause.Field, cause.Message)
						}
						os.Exit(255)
					}
				}
				glog.Fatal(err)
			}
		},
	}

	options.MasterArgs = NewDefaultMasterArgs()
	options.MasterArgs.StartAPI = true
	options.MasterArgs.StartControllers = true
	options.MasterArgs.OverrideConfig = func(config *configapi.MasterConfig) error {
		if config.KubernetesMasterConfig != nil && options.MasterArgs.MasterAddr.Provided {
			if ip := net.ParseIP(options.MasterArgs.MasterAddr.Host); ip != nil {
				glog.V(2).Infof("Using a masterIP override %q", ip)
				config.KubernetesMasterConfig.MasterIP = ip.String()
			}
		}
		return nil
	}

	flags := cmd.Flags()

	flags.Var(options.MasterArgs.ConfigDir, "write-config", "Directory to write an initial config into.  After writing, exit without starting the server.")
	flags.StringVar(&options.ConfigFile, "config", "", "Location of the master configuration file to run from. When running from a configuration file, all other command-line arguments are ignored.")
	flags.BoolVar(&options.CreateCertificates, "create-certs", true, "Indicates whether missing certs should be created")
	flags.IntVar(&options.ExpireDays, "expire-days", options.ExpireDays, "Validity of the certificates in days (defaults to 2 years). WARNING: extending this above default value is highly discouraged.")
	flags.IntVar(&options.SignerExpireDays, "signer-expire-days", options.SignerExpireDays, "Validity of the CA certificate in days (defaults to 5 years). WARNING: extending this above default value is highly discouraged.")

	BindMasterArgs(options.MasterArgs, flags, "")
	BindListenArg(options.MasterArgs.ListenArg, flags, "")
	BindImageFormatArgs(options.MasterArgs.ImageFormatArgs, flags, "")
	BindKubeConnectionArgs(options.MasterArgs.KubeConnectionArgs, flags, "")
	BindNetworkArgs(options.MasterArgs.NetworkArgs, flags, "")

	// autocompletion hints
	cmd.MarkFlagFilename("write-config")
	cmd.MarkFlagFilename("config", "yaml", "yml")

	startControllers, _ := NewCommandStartMasterControllers("controllers", basename, out, errout)
	startAPI, _ := NewCommandStartMasterAPI("api", basename, out, errout)
	cmd.AddCommand(startAPI)
	cmd.AddCommand(startControllers)

	return cmd, options
}

func (o MasterOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported for start master")
	}
	if o.IsWriteConfigOnly() {
		if o.IsRunFromConfig() {
			return errors.New("--config may not be set if --write-config is set")
		}
	}

	if len(o.MasterArgs.ConfigDir.Value()) == 0 {
		return errors.New("configDir must have a value")
	}

	// if we are not starting up using a config file, run the argument validation
	if !o.IsRunFromConfig() {
		if err := o.MasterArgs.Validate(); err != nil {
			return err
		}

	}

	if o.ExpireDays < 0 {
		return errors.New("expire-days must be valid number of days")
	}
	if o.SignerExpireDays < 0 {
		return errors.New("signer-expire-days must be valid number of days")
	}

	return nil
}

func (o *MasterOptions) Complete() error {
	if !o.MasterArgs.ConfigDir.Provided() {
		o.MasterArgs.ConfigDir.Default("openshift.local.config/master")
	}

	return nil
}

// StartMaster calls RunMaster and then waits forever
func (o MasterOptions) StartMaster() error {
	if err := o.RunMaster(); err != nil {
		return err
	}

	if o.IsWriteConfigOnly() {
		return nil
	}

	// TODO: this should be encapsulated by RunMaster, but StartAllInOne has no
	// way to communicate whether RunMaster should block.
	go daemon.SdNotify("READY=1")
	select {}
}

// RunMaster takes the options and:
// 1.  Creates certs if needed
// 2.  Reads fully specified master config OR builds a fully specified master config from the args
// 3.  Writes the fully specified master config and exits if needed
// 4.  Starts the master based on the fully specified config
func (o MasterOptions) RunMaster() error {
	startUsingConfigFile := !o.IsWriteConfigOnly() && o.IsRunFromConfig()

	if !startUsingConfigFile && o.CreateCertificates {
		glog.V(2).Infof("Generating master configuration")
		if err := o.CreateCerts(); err != nil {
			return err
		}
		if err := o.CreateBootstrapPolicy(); err != nil {
			return err
		}
	}

	var masterConfig *configapi.MasterConfig
	var err error
	if startUsingConfigFile {
		masterConfig, err = configapilatest.ReadAndResolveMasterConfig(o.ConfigFile)
	} else {
		masterConfig, err = o.MasterArgs.BuildSerializeableMasterConfig()
	}
	if err != nil {
		return err
	}

	if o.IsWriteConfigOnly() {
		// Resolve relative to CWD
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := configapi.ResolveMasterConfigPaths(masterConfig, cwd); err != nil {
			return err
		}

		// Relativize to config file dir
		base, err := cmdutil.MakeAbs(filepath.Dir(o.MasterArgs.GetConfigFileToWrite()), cwd)
		if err != nil {
			return err
		}
		if err := configapi.RelativizeMasterConfigPaths(masterConfig, base); err != nil {
			return err
		}

		content, err := configapilatest.WriteYAML(masterConfig)
		if err != nil {

			return err
		}

		if err := os.MkdirAll(path.Dir(o.MasterArgs.GetConfigFileToWrite()), os.FileMode(0755)); err != nil {
			return err
		}
		if err := ioutil.WriteFile(o.MasterArgs.GetConfigFileToWrite(), content, 0644); err != nil {
			return err
		}

		fmt.Fprintf(o.Output, "Wrote master config to: %s\n", o.MasterArgs.GetConfigFileToWrite())

		return nil
	}

	if o.MasterArgs.OverrideConfig != nil {
		if err := o.MasterArgs.OverrideConfig(masterConfig); err != nil {
			return err
		}
	}

	// Inject disabled feature flags based on distribution being used and
	// regardless of configuration. They aren't written to config file to
	// prevent upgrade path issues.
	masterConfig.DisabledFeatures.Add(o.DisabledFeatures...)
	validationResults := validation.ValidateMasterConfig(masterConfig, nil)
	if len(validationResults.Warnings) != 0 {
		for _, warning := range validationResults.Warnings {
			glog.Warningf("Warning: %v, master start will continue.", warning)
		}
	}
	if len(validationResults.Errors) != 0 {
		return kerrors.NewInvalid(configapi.Kind("MasterConfig"), o.ConfigFile, validationResults.Errors)
	}

	if !o.MasterArgs.StartControllers {
		masterConfig.Controllers = configapi.ControllersDisabled
	}

	m := &Master{
		config:      masterConfig,
		api:         o.MasterArgs.StartAPI,
		controllers: o.MasterArgs.StartControllers,
	}
	return m.Start()
}

func (o MasterOptions) CreateBootstrapPolicy() error {
	writeBootstrapPolicy := admin.CreateBootstrapPolicyFileOptions{
		File: o.MasterArgs.GetPolicyFile(),
		OpenShiftSharedResourcesNamespace: bootstrappolicy.DefaultOpenShiftSharedResourcesNamespace,
	}

	return writeBootstrapPolicy.CreateBootstrapPolicyFile()
}

func (o MasterOptions) CreateCerts() error {
	masterAddr, err := o.MasterArgs.GetMasterAddress()
	if err != nil {
		return err
	}
	publicMasterAddr, err := o.MasterArgs.GetMasterPublicAddress()
	if err != nil {
		return err
	}

	signerName := admin.DefaultSignerName()
	hostnames, err := o.MasterArgs.GetServerCertHostnames()
	if err != nil {
		return err
	}
	mintAllCertsOptions := admin.CreateMasterCertsOptions{
		CertDir:            o.MasterArgs.ConfigDir.Value(),
		SignerName:         signerName,
		ExpireDays:         o.ExpireDays,
		SignerExpireDays:   o.SignerExpireDays,
		Hostnames:          hostnames.List(),
		APIServerURL:       masterAddr.String(),
		APIServerCAFiles:   o.MasterArgs.APIServerCAFiles,
		CABundleFile:       admin.DefaultCABundleFile(o.MasterArgs.ConfigDir.Value()),
		PublicAPIServerURL: publicMasterAddr.String(),
		Output:             cmdutil.NewGLogWriterV(3),
	}
	if err := mintAllCertsOptions.Validate(nil); err != nil {
		return err
	}
	if err := mintAllCertsOptions.CreateMasterCerts(); err != nil {
		return err
	}

	return nil
}

func BuildKubernetesMasterConfig(openshiftConfig *origin.MasterConfig) (*kubernetes.MasterConfig, error) {
	if openshiftConfig.Options.KubernetesMasterConfig == nil {
		return nil, fmt.Errorf("KubernetesMasterConfig is required to start this server - use of external Kubernetes is no longer supported.")
	}
	return kubernetes.BuildKubernetesMasterConfig(
		openshiftConfig.Options,
		openshiftConfig.RequestContextMapper,
		openshiftConfig.KubeClientsetExternal(),
		openshiftConfig.KubeClientsetInternal(),
		openshiftConfig.ExternalKubeInformers,
		openshiftConfig.KubeAdmissionControl,
		openshiftConfig.Authenticator,
		openshiftConfig.Authorizer,
	)
}

func BuildKubernetesControllersConfig(openshiftConfig *origin.MasterConfig) (*kubernetes.MasterConfig, error) {
	if openshiftConfig.Options.KubernetesMasterConfig == nil {
		return nil, fmt.Errorf("KubernetesMasterConfig is required to start this server - use of external Kubernetes is no longer supported.")
	}
	return kubernetes.BuildKubernetesControllersConfig(
		openshiftConfig.Options,
		openshiftConfig.KubeClientsetExternal(),
		openshiftConfig.KubeClientsetInternal(),
		openshiftConfig.ExternalKubeInformers,
	)
}

// Master encapsulates starting the components of the master
type Master struct {
	config      *configapi.MasterConfig
	controllers bool
	api         bool
}

// NewMaster create a master launcher
func NewMaster(config *configapi.MasterConfig, controllers, api bool) *Master {
	return &Master{
		config:      config,
		controllers: controllers,
		api:         api,
	}
}

// Start launches a master. It will error if possible, but some background processes may still
// be running and the process should exit after it finishes.
func (m *Master) Start() error {
	// Allow privileged containers
	// TODO: make this configurable and not the default https://github.com/openshift/origin/issues/662
	capabilities.Initialize(capabilities.Capabilities{
		AllowPrivileged: true,
		PrivilegedSources: capabilities.PrivilegedSources{
			HostNetworkSources: []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
			HostPIDSources:     []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
			HostIPCSources:     []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
		},
	})

	var (
		openshiftConfig  *origin.MasterConfig
		kubeMasterConfig *kubernetes.MasterConfig
		err              error
	)
	switch {
	case m.controllers && !m.api:
		openshiftConfig, err = origin.BuildControllersConfig(*m.config)
		if err != nil {
			return err
		}
		kubeMasterConfig, err = BuildKubernetesControllersConfig(openshiftConfig)
		if err != nil {
			return err
		}
	default:
		openshiftConfig, err = origin.BuildMasterConfig(*m.config)
		if err != nil {
			return err
		}
		kubeMasterConfig, err = BuildKubernetesMasterConfig(openshiftConfig)
		if err != nil {
			return err
		}
	}

	// initialize the election module if the controllers will start
	if m.controllers {
		openshiftConfig.ControllerPlug, openshiftConfig.ControllerPlugStart, err = origin.NewLeaderElection(
			*m.config,
			kubeMasterConfig.ControllerManager.KubeControllerManagerConfiguration.LeaderElection,
			openshiftConfig.PrivilegedLoopbackKubernetesClientsetExternal,
		)
		if err != nil {
			return err
		}
	}

	// any controller that uses a core informer must be initialized *before* the API server starts core informers
	// the API server adds its controllers at the correct time, but if the controllers are running, they need to be
	// kicked separately

	switch {
	case m.api:
		glog.Infof("Starting master on %s (%s)", m.config.ServingInfo.BindAddress, version.Get().String())
		glog.Infof("Public master address is %s", m.config.MasterPublicURL)
		if len(m.config.DisabledFeatures) > 0 {
			glog.V(4).Infof("Disabled features: %s", strings.Join(m.config.DisabledFeatures, ", "))
		}
		glog.Infof("Using images from %q", openshiftConfig.ImageFor("<component>"))

		if err := StartAPI(openshiftConfig, kubeMasterConfig); err != nil {
			return err
		}

	case m.controllers:
		glog.Infof("Starting controllers on %s (%s)", m.config.ServingInfo.BindAddress, version.Get().String())
		if len(m.config.DisabledFeatures) > 0 {
			glog.V(4).Infof("Disabled features: %s", strings.Join(m.config.DisabledFeatures, ", "))
		}
		glog.Infof("Using images from %q", openshiftConfig.ImageFor("<component>"))

		if err := startHealth(openshiftConfig); err != nil {
			return err
		}
	}

	if m.controllers {
		// run controllers asynchronously (not required to be "ready")
		go func() {
			if err := startControllers(openshiftConfig, kubeMasterConfig); err != nil {
				glog.Fatal(err)
			}

			openshiftConfig.InternalKubeInformers.Start(utilwait.NeverStop)
			openshiftConfig.ExternalKubeInformers.Start(utilwait.NeverStop)
			openshiftConfig.AppInformers.Start(utilwait.NeverStop)
			openshiftConfig.AuthorizationInformers.Start(utilwait.NeverStop)
			openshiftConfig.BuildInformers.Start(utilwait.NeverStop)
			openshiftConfig.ImageInformers.Start(utilwait.NeverStop)
			openshiftConfig.QuotaInformers.Start(utilwait.NeverStop)
			openshiftConfig.TemplateInformers.Start(utilwait.NeverStop)
		}()
	} else {
		openshiftConfig.InternalKubeInformers.Start(utilwait.NeverStop)
		openshiftConfig.ExternalKubeInformers.Start(utilwait.NeverStop)
		openshiftConfig.AppInformers.Start(utilwait.NeverStop)
		openshiftConfig.AuthorizationInformers.Start(utilwait.NeverStop)
		openshiftConfig.BuildInformers.Start(utilwait.NeverStop)
		openshiftConfig.ImageInformers.Start(utilwait.NeverStop)
		openshiftConfig.QuotaInformers.Start(utilwait.NeverStop)
		openshiftConfig.TemplateInformers.Start(utilwait.NeverStop)
	}

	return nil
}

func startHealth(openshiftConfig *origin.MasterConfig) error {
	return openshiftConfig.RunHealth()
}

// StartAPI starts the components of the master that are considered part of the API - the Kubernetes
// API and core controllers, the Origin API, the group, policy, project, and authorization caches,
// etcd, the asset server (for the UI), the OAuth server endpoints, and the DNS server.
// TODO: allow to be more granularly targeted
func StartAPI(oc *origin.MasterConfig, kc *kubernetes.MasterConfig) error {
	// start etcd
	if oc.Options.EtcdConfig != nil {
		etcdserver.RunEtcd(oc.Options.EtcdConfig)
	}

	// verify we can connect to etcd with the provided config
	if len(kc.Options.APIServerArguments) > 0 && len(kc.Options.APIServerArguments["storage-backend"]) > 0 && kc.Options.APIServerArguments["storage-backend"][0] == "etcd3" {
		if _, err := etcd.GetAndTestEtcdClientV3(oc.Options.EtcdClientInfo); err != nil {
			return err
		}
	} else {
		if _, err := etcd.GetAndTestEtcdClient(oc.Options.EtcdClientInfo); err != nil {
			return err
		}
	}

	// Must start policy and quota caching immediately
	oc.QuotaInformers.Start(utilwait.NeverStop)
	oc.AuthorizationInformers.Start(utilwait.NeverStop)
	oc.RunClusterQuotaMappingController()
	oc.RunGroupCache()
	oc.RunProjectCache()

	var standaloneAssetConfig, embeddedAssetConfig *origin.AssetConfig
	if oc.WebConsoleEnabled() {
		overrideConfig, err := getResourceOverrideConfig(oc)
		if err != nil {
			return err
		}
		config, err := origin.NewAssetConfig(*oc.Options.AssetConfig, overrideConfig)
		if err != nil {
			return err
		}

		if oc.Options.AssetConfig.ServingInfo.BindAddress == oc.Options.ServingInfo.BindAddress {
			embeddedAssetConfig = config
		} else {
			standaloneAssetConfig = config
		}
	}

	oc.Run(kc.Master, embeddedAssetConfig, utilwait.NeverStop)

	// start DNS before the informers are started because it adds a ClusterIP index.
	if oc.Options.DNSConfig != nil {
		oc.RunDNSServer()
	}

	// start up the informers that we're trying to use in the API server
	oc.InternalKubeInformers.Start(utilwait.NeverStop)
	oc.ExternalKubeInformers.Start(utilwait.NeverStop)
	oc.InitializeObjects()

	if standaloneAssetConfig != nil {
		standaloneAssetConfig.Run()
	}

	oc.RunProjectAuthorizationCache()
	return nil
}

// getResourceOverrideConfig looks in two potential places where ClusterResourceOverrideConfig can be specified
func getResourceOverrideConfig(oc *origin.MasterConfig) (*overrideapi.ClusterResourceOverrideConfig, error) {
	overrideConfig, err := checkForOverrideConfig(oc.Options.AdmissionConfig)
	if err != nil {
		return nil, err
	}
	if overrideConfig != nil {
		return overrideConfig, nil
	}
	if oc.Options.KubernetesMasterConfig == nil { // external kube gets you a nil pointer here
		return nil, nil
	}
	overrideConfig, err = checkForOverrideConfig(oc.Options.KubernetesMasterConfig.AdmissionConfig)
	if err != nil {
		return nil, err
	}
	return overrideConfig, nil
}

// checkForOverrideConfig looks for ClusterResourceOverrideConfig plugin cfg in the admission PluginConfig
func checkForOverrideConfig(ac configapi.AdmissionConfig) (*overrideapi.ClusterResourceOverrideConfig, error) {
	overridePluginConfigFile, err := pluginconfig.GetPluginConfigFile(ac.PluginConfig, overrideapi.PluginName, "")
	if err != nil {
		return nil, err
	}
	if overridePluginConfigFile == "" {
		return nil, nil
	}
	configFile, err := os.Open(overridePluginConfigFile)
	if err != nil {
		return nil, err
	}
	overrideConfig, err := override.ReadConfig(configFile)
	if err != nil {
		return nil, err
	}
	return overrideConfig, nil
}

type GenericResourceInformer interface {
	ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error)
}

// genericInternalResourceInformerFunc will return an internal informer for any resource matching
// its group resource, instead of the external version. Only valid for use where the type is accessed
// via generic interfaces, such as the garbage collector with ObjectMeta.
type genericInternalResourceInformerFunc func(resource schema.GroupVersionResource) (kinformers.GenericInformer, error)

func (fn genericInternalResourceInformerFunc) ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
	resource.Version = runtime.APIVersionInternal
	return fn(resource)
}

type genericInformers struct {
	kinformers.SharedInformerFactory
	generic []GenericResourceInformer
}

func (i genericInformers) ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
	informer, firstErr := i.SharedInformerFactory.ForResource(resource)
	if firstErr == nil {
		return informer, nil
	}
	for _, generic := range i.generic {
		if informer, err := generic.ForResource(resource); err == nil {
			return informer, nil
		}
	}
	glog.V(4).Infof("Couldn't find informer for %v", resource)
	return nil, firstErr
}

// startControllers launches the controllers
func startControllers(oc *origin.MasterConfig, kc *kubernetes.MasterConfig) error {
	if oc.Options.Controllers == configapi.ControllersDisabled {
		return nil
	}

	go func() {
		oc.ControllerPlugStart()
		// when a manual shutdown (DELETE /controllers) or lease lost occurs, the process should exit
		// this ensures no code is still running as a controller, and allows a process manager to reset
		// the controller to come back into a candidate state and compete for the lease
		if err := oc.ControllerPlug.WaitForStop(); err != nil {
			glog.Fatalf("Controller shutdown due to lease being lost: %v", err)
		}
		glog.Fatalf("Controller graceful shutdown requested")
	}()

	oc.ControllerPlug.WaitForStart()
	glog.Infof("Controllers starting (%s)", oc.Options.Controllers)

	// Get configured options (or defaults) for k8s controllers
	controllerManagerOptions := cmapp.NewCMServer()
	if kc != nil && kc.ControllerManager != nil {
		controllerManagerOptions = kc.ControllerManager
	}

	rootClientBuilder := controller.SimpleControllerClientBuilder{
		ClientConfig: &oc.PrivilegedLoopbackClientConfig,
	}

	availableResources, err := kctrlmgr.GetAvailableResources(rootClientBuilder)
	if err != nil {
		return err
	}

	openshiftControllerContext := origincontrollers.ControllerContext{
		KubeControllerContext: kctrlmgr.ControllerContext{
			ClientBuilder: controller.SAControllerClientBuilder{
				ClientConfig:         restclient.AnonymousClientConfig(&oc.PrivilegedLoopbackClientConfig),
				CoreClient:           oc.PrivilegedLoopbackKubernetesClientsetExternal.Core(),
				AuthenticationClient: oc.PrivilegedLoopbackKubernetesClientsetExternal.Authentication(),
				Namespace:            "kube-system",
			},
			InformerFactory: genericInformers{
				SharedInformerFactory: oc.ExternalKubeInformers,
				generic: []GenericResourceInformer{
					// use our existing internal informers to satisfy the generic informer requests (which don't require strong
					// types).
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
						return oc.AppInformers.ForResource(resource)
					}),
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
						return oc.AuthorizationInformers.ForResource(resource)
					}),
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
						return oc.BuildInformers.ForResource(resource)
					}),
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
						return oc.ImageInformers.ForResource(resource)
					}),
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
						return oc.TemplateInformers.ForResource(resource)
					}),
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
						return oc.QuotaInformers.ForResource(resource)
					}),
					oc.ExternalKubeInformers,
					genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
						return oc.InternalKubeInformers.ForResource(resource)
					}),
				},
			},
			Options:            *controllerManagerOptions,
			AvailableResources: availableResources,
			Stop:               utilwait.NeverStop,
		},
		ClientBuilder: origincontrollers.OpenshiftControllerClientBuilder{
			ControllerClientBuilder: controller.SAControllerClientBuilder{
				ClientConfig:         restclient.AnonymousClientConfig(&oc.PrivilegedLoopbackClientConfig),
				CoreClient:           oc.PrivilegedLoopbackKubernetesClientsetExternal.Core(),
				AuthenticationClient: oc.PrivilegedLoopbackKubernetesClientsetExternal.Authentication(),
				Namespace:            bootstrappolicy.DefaultOpenShiftInfraNamespace,
			},
		},
		InternalKubeInformers: oc.InternalKubeInformers,
		ExternalKubeInformers: oc.ExternalKubeInformers,
		AppInformers:          oc.AppInformers,
		BuildInformers:        oc.BuildInformers,
		ImageInformers:        oc.ImageInformers,
		TemplateInformers:     oc.TemplateInformers,
		Stop:                  utilwait.NeverStop,
	}
	// We need to start the serviceaccount-tokens controller first as it provides token
	// generation for other controllers.
	preStartControllers, err := oc.NewOpenShiftControllerPreStartInitializers()
	if err != nil {
		return err
	}
	if started, err := preStartControllers["serviceaccount-token"](openshiftControllerContext); err != nil {
		return fmt.Errorf("Error starting serviceaccount-token controller: %v", err)
	} else if !started {
		glog.Warningf("Skipping serviceaccount-token controller")
	}
	glog.Infof("Started serviceaccount-token controller")

	// The service account controllers require informers in order to create service account tokens
	// for other controllers, which means we need to start their informers (which use the privileged
	// loopback client) before the other controllers will run.
	oc.ExternalKubeInformers.Start(utilwait.NeverStop)

	oc.RunSecurityAllocationController()

	// These controllers are special-cased upstream.  We'll need custom init functions for them downstream.
	// As we make them less special, we should re-visit this
	kc.RunNodeController()
	kc.RunScheduler()

	_, _, _, binderClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraPersistentVolumeBinderControllerServiceAccountName)
	if err != nil {
		glog.Fatalf("Could not get client for persistent volume binder controller: %v", err)
	}

	_, _, _, attachDetachControllerClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraPersistentVolumeAttachDetachControllerServiceAccountName)
	if err != nil {
		glog.Fatalf("Could not get client for attach detach controller: %v", err)
	}

	_, _, _, serviceLoadBalancerClient, err := oc.GetServiceAccountClients(bootstrappolicy.InfraServiceLoadBalancerControllerServiceAccountName)
	if err != nil {
		glog.Fatalf("Could not get client for pod gc controller: %v", err)
	}
	kc.RunPersistentVolumeController(binderClient, oc.Options.PolicyConfig.OpenShiftInfrastructureNamespace, oc.ImageFor("recycler"), bootstrappolicy.InfraPersistentVolumeRecyclerControllerServiceAccountName)
	kc.RunPersistentVolumeAttachDetachController(attachDetachControllerClient)
	kc.RunServiceLoadBalancerController(serviceLoadBalancerClient)

	openshiftControllerInitializers, err := oc.NewOpenshiftControllerInitializers()
	if err != nil {
		return err
	}
	for name, initFn := range kctrlmgr.NewControllerInitializers() {
		if _, ok := openshiftControllerInitializers[name]; ok {
			// don't overwrite, openshift takes priority
			continue
		}
		openshiftControllerInitializers[name] = origincontrollers.FromKubeInitFunc(initFn)
	}

	allowedControllers := sets.NewString(
		// TODO I think this kube part should become a blacklist kept in sync during rebases with a unit test.
		"endpoint",
		"replicationcontroller",
		"podgc",
		"namespace",
		"garbagecollector",
		"daemonset",
		"job",
		"deployment",
		"replicaset",
		"horizontalpodautoscaling",
		"disruption",
		"statefuleset",
		"cronjob",
		"certificatesigningrequests",
		// not used in openshift.  Yet?
		// "ttl",
		// "bootstrapsigner",
		// "tokencleaner",

		// These controllers need to have their own init functions until we extend the upstream controller config
		// TODO this controller takes different evaluators.
		// "resourcequota",

		// TODO we manage this one differently, so its wired by openshift but overrides upstreams
		"serviceaccount",

		"openshift.io/serviceaccount-pull-secrets",
		"openshift.io/origin-namespace",
		"openshift.io/deployer",
		"openshift.io/deploymentconfig",
		"openshift.io/deploymenttrigger",
		"openshift.io/image-trigger",
		"openshift.io/image-import",
		"openshift.io/service-serving-cert",
	)

	if configapi.IsBuildEnabled(&oc.Options) {
		allowedControllers.Insert("openshift.io/build")
		allowedControllers.Insert("openshift.io/build-config-change")
	}
	if oc.Options.TemplateServiceBrokerConfig != nil {
		allowedControllers.Insert("openshift.io/templateinstance")
	}

	for controllerName, initFn := range openshiftControllerInitializers {
		// TODO remove this.  Only call one to start to prove the principle
		if !allowedControllers.Has(controllerName) {
			glog.Warningf("%q is skipped", controllerName)
			continue
		}
		if !openshiftControllerContext.IsControllerEnabled(controllerName) {
			glog.Warningf("%q is disabled", controllerName)
			continue
		}

		glog.V(1).Infof("Starting %q", controllerName)
		started, err := initFn(openshiftControllerContext)
		if err != nil {
			glog.Fatalf("Error starting %q", controllerName)
			return err
		}
		if !started {
			glog.Warningf("Skipping %q", controllerName)
			continue
		}
		glog.Infof("Started %q", controllerName)
	}

	oc.RunSDNController()
	oc.RunOriginToRBACSyncControllers()

	// initializes quota docs used by admission
	oc.RunResourceQuotaManager(controllerManagerOptions)
	oc.RunClusterQuotaReconciliationController()
	oc.RunClusterQuotaMappingController()

	oc.RunUnidlingController()

	_, _, ingressIPClientInternal, ingressIPClientExternal, err := oc.GetServiceAccountClients(bootstrappolicy.InfraServiceIngressIPControllerServiceAccountName)
	if err != nil {
		glog.Fatalf("Could not get client: %v", err)
	}
	oc.RunIngressIPController(ingressIPClientInternal, ingressIPClientExternal)

	glog.Infof("Started Origin Controllers")

	return nil
}

func (o MasterOptions) IsWriteConfigOnly() bool {
	return o.MasterArgs.ConfigDir.Provided()
}

func (o MasterOptions) IsRunFromConfig() bool {
	return (len(o.ConfigFile) > 0)
}
