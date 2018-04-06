package docker

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types/versions"
	cliconfig "github.com/docker/docker/cli/config"
	dockerclient "github.com/docker/docker/client"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/net/context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	aggregatorinstall "k8s.io/kube-aggregator/pkg/apis/apiregistration/install"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	oauthclientinternal "github.com/openshift/origin/pkg/oauth/generated/internalclientset"
	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/components/registry"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/components/router"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/components/service-catalog"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/components/web-console"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubeapiserver"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/host"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/localcmd"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/openshift"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	osclientcmd "github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	"github.com/openshift/origin/pkg/version"
)

const (
	// CmdUpRecommendedName is the recommended command name
	CmdUpRecommendedName = "up"

	initialUser     = "developer"
	initialPassword = "developer"

	initialProjectName    = "myproject"
	initialProjectDisplay = "My Project"
	initialProjectDesc    = "Initial developer project"

	defaultRedirectClient  = "openshift-web-console"
	developmentRedirectURI = "https://localhost:9000"

	dockerAPIVersion122 = "1.22"
)

var (
	cmdUpLong = templates.LongDesc(`
		Starts an OpenShift cluster using Docker containers, provisioning a registry, router,
		initial templates, and a default project.

		This command will attempt to use an existing connection to a Docker daemon. Before running
		the command, ensure that you can execute docker commands successfully (i.e. 'docker ps').

		By default, the OpenShift cluster will be setup to use a routing suffix that ends in nip.io.
		This is to allow dynamic host names to be created for routes. An alternate routing suffix
		can be specified using the --routing-suffix flag.

		A public hostname can also be specified for the server with the --public-hostname flag.`)

	cmdUpExample = templates.Examples(`
	  # Start OpenShift using a specific public host name
	  %[1]s --public-hostname=my.address.example.com

	  # Specify which set of image streams to use
	  %[1]s --image-streams=centos7`)

	imageStreams = map[string]string{
		"centos7": "examples/image-streams/image-streams-centos7.json",
		"rhel7":   "examples/image-streams/image-streams-rhel7.json",
	}

	// defaultImageStreams is the default key for the above imageStreams mapping.
	// It should be set during build via -ldflags.
	defaultImageStreams string

	templateLocations = map[string]string{
		"mongodb":                     "examples/db-templates/mongodb-persistent-template.json",
		"mariadb":                     "examples/db-templates/mariadb-persistent-template.json",
		"mysql":                       "examples/db-templates/mysql-persistent-template.json",
		"postgresql":                  "examples/db-templates/postgresql-persistent-template.json",
		"cakephp quickstart":          "examples/quickstarts/cakephp-mysql-persistent.json",
		"dancer quickstart":           "examples/quickstarts/dancer-mysql-persistent.json",
		"django quickstart":           "examples/quickstarts/django-postgresql-persistent.json",
		"nodejs quickstart":           "examples/quickstarts/nodejs-mongodb-persistent.json",
		"rails quickstart":            "examples/quickstarts/rails-postgresql-persistent.json",
		"jenkins pipeline persistent": "examples/jenkins/jenkins-persistent-template.json",
		"sample pipeline":             "examples/jenkins/pipeline/samplepipeline.yaml",
	}

	adminTemplateLocations = map[string]string{
		"prometheus":          "examples/prometheus/prometheus.yaml",
		"heapster standalone": "examples/heapster/heapster-standalone.yaml",
	}
)

// NewCmdUp creates a command that starts OpenShift on Docker with reasonable defaults
func NewCmdUp(name, fullName string, f *osclientcmd.Factory, out, errout io.Writer, clusterAdd *cobra.Command) *cobra.Command {
	config := &ClusterUpConfig{
		Out:                 out,
		UsePorts:            openshift.BasePorts,
		PortForwarding:      defaultPortForwarding(),
		DNSPort:             openshift.DefaultDNSPort,
		checkAlternatePorts: true,

		// We pass cluster add as a command to prevent anyone from ever cheating with their wiring. You either work from flags or
		// or you don't work.  You cannot add glue of any sort.
		ClusterAdd: clusterAdd,
	}
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Start OpenShift on Docker with reasonable defaults",
		Long:    cmdUpLong,
		Example: fmt.Sprintf(cmdUpExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(config.Complete(f, c, out))
			kcmdutil.CheckErr(config.Validate(errout))
			kcmdutil.CheckErr(config.Check(out))
			if err := config.Start(out); err != nil {
				PrintError(err, errout)
				os.Exit(1)
			}
		},
	}
	config.Bind(cmd.Flags())
	return cmd
}

type ClusterUpConfig struct {
	Image             string
	ImageTag          string
	ImageStreams      string
	DockerMachine     string
	SkipRegistryCheck bool
	PortForwarding    bool
	ClusterAdd        *cobra.Command

	Out io.Writer

	// BaseTempDir is the directory to use as the root for temp directories
	// This allows us to bundle all of the cluster-up directories in one spot for easier cleanup and ensures we aren't
	// doing crazy thing like dirtying /var on the host (that does weird stuff)
	BaseDir           string
	SpecifiedBaseDir  bool
	HostName          string
	UseExistingConfig bool
	Environment       []string
	ServerLogLevel    int
	AddComponents     []string

	HostVolumesDir           string
	HostConfigDir            string
	WriteConfig              bool
	HostDataDir              string
	UsePorts                 []int
	DNSPort                  int
	ServerIP                 string
	AdditionalIPs            []string
	UseNsenterMount          bool
	PublicHostname           string
	RoutingSuffix            string
	HostPersistentVolumesDir string
	HTTPProxy                string
	HTTPSProxy               string
	NoProxy                  []string
	CACert                   string
	PVCount                  int

	dockerClient    dockerhelper.Interface
	dockerHelper    *dockerhelper.Helper
	hostHelper      *host.HostHelper
	openshiftHelper *openshift.Helper
	factory         *clientcmd.Factory
	originalFactory *clientcmd.Factory
	command         *cobra.Command

	usingDefaultImages         bool
	usingDefaultOpenShiftImage bool
	checkAlternatePorts        bool

	shouldInitializeData *bool
	shouldCreateUser     *bool

	containerNetworkErr chan error
}

func (c *ClusterUpConfig) Bind(flags *pflag.FlagSet) {
	flags.StringVar(&c.ImageTag, "tag", "", "Specify the tag for OpenShift images")
	flags.MarkHidden("tag")
	flags.StringVar(&c.Image, "image", variable.DefaultImagePrefix, "Specify the images to use for OpenShift")
	flags.StringVar(&c.ImageStreams, "image-streams", defaultImageStreams, "Specify which image streams to use, centos7|rhel7")
	flags.BoolVar(&c.SkipRegistryCheck, "skip-registry-check", false, "Skip Docker daemon registry check")
	flags.StringVar(&c.PublicHostname, "public-hostname", "", "Public hostname for OpenShift cluster")
	flags.StringVar(&c.RoutingSuffix, "routing-suffix", "", "Default suffix for server routes")
	flags.BoolVar(&c.UseExistingConfig, "use-existing-config", false, "Use existing configuration if present")
	flags.StringVar(&c.BaseDir, "base-dir", c.BaseDir, "Directory on Docker host for cluster up configuration")
	flags.BoolVar(&c.WriteConfig, "write-config", false, "Write the configuration files into host config dir")
	flags.BoolVar(&c.PortForwarding, "forward-ports", c.PortForwarding, "Use Docker port-forwarding to communicate with origin container. Requires 'socat' locally.")
	flags.IntVar(&c.ServerLogLevel, "server-loglevel", 0, "Log level for OpenShift server")
	flags.StringArrayVarP(&c.Environment, "env", "e", c.Environment, "Specify a key-value pair for an environment variable to set on OpenShift container")
	flags.StringArrayVar(&c.AddComponents, "enable", c.AddComponents, "Install additional components.")
	flags.StringVar(&c.HTTPProxy, "http-proxy", "", "HTTP proxy to use for master and builds")
	flags.StringVar(&c.HTTPSProxy, "https-proxy", "", "HTTPS proxy to use for master and builds")
	flags.StringArrayVar(&c.NoProxy, "no-proxy", c.NoProxy, "List of hosts or subnets for which a proxy should not be used")
}

func (c *ClusterUpConfig) Complete(f *osclientcmd.Factory, cmd *cobra.Command, out io.Writer) error {
	// TODO: remove this when we move to container/apply based component installation
	aggregatorinstall.Install(legacyscheme.GroupFactoryRegistry, legacyscheme.Registry, legacyscheme.Scheme)

	c.originalFactory = f
	c.command = cmd

	// do some defaulting
	if len(c.ImageTag) == 0 {
		c.ImageTag = strings.TrimRight("v"+version.Get().Major+"."+version.Get().Minor, "+")
	}
	if len(c.BaseDir) == 0 {
		c.SpecifiedBaseDir = false
		c.BaseDir = "openshift.local.clusterup"
	}
	if !path.IsAbs(c.BaseDir) {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		absHostDir, err := cmdutil.MakeAbs(c.BaseDir, cwd)
		if err != nil {
			return err
		}
		c.BaseDir = absHostDir
	}
	// do some struct initialization next
	// used for some pretty printing
	taskPrinter := NewTaskPrinter(GetDetailedOut(out))

	// Get a Docker client.
	// If a Docker machine was specified, make sure that the machine is running.
	// Otherwise, use environment variables.
	taskPrinter.StartTask("Getting a Docker client")
	client, err := GetDockerClient()
	if err != nil {
		return taskPrinter.ToError(err)
	}
	c.dockerClient = client
	taskPrinter.Success()

	// Ensure that the OpenShift Docker image is available.
	// If not present, pull it.
	// We do this here because the image is used in the next step if running Red Hat docker.
	taskPrinter.StartTask(fmt.Sprintf("Checking if image %s is available", c.openshiftImage()))
	if err := c.checkOpenShiftImage(out); err != nil {
		return taskPrinter.ToError(err)
	}
	taskPrinter.Success()

	// Check whether the Docker host has the right binaries to use Kubernetes' nsenter mounter
	// If not, use a shared volume to mount volumes on OpenShift
	if isRedHatDocker, err := c.DockerHelper().IsRedHat(); err == nil && isRedHatDocker {
		taskPrinter.StartTask("Checking type of volume mount")
		c.UseNsenterMount, err = c.HostHelper().CanUseNsenterMounter()
		if err != nil {
			return taskPrinter.ToError(err)
		}
		taskPrinter.Success()
	}

	if err := os.MkdirAll(c.BaseDir, 0755); err != nil {
		return err
	}

	if c.UseNsenterMount {
		c.HostVolumesDir = path.Join(c.BaseDir, "openshift.local.volumes")
		if err := os.MkdirAll(c.HostVolumesDir, 0755); err != nil {
			return err
		}
	} else {
		c.HostVolumesDir = path.Join(NonLinuxHostVolumeDirPrefix, c.BaseDir, "openshift.local.volumes")
	}
	c.HostPersistentVolumesDir = path.Join(c.BaseDir, "openshift.local.pv")
	if err := os.MkdirAll(c.HostPersistentVolumesDir, 0755); err != nil {
		return err
	}
	c.HostDataDir = path.Join(c.BaseDir, "etcd")
	if err := os.MkdirAll(c.HostDataDir, 0755); err != nil {
		return err
	}

	// Ensure that host directories exist.
	// If not using the nsenter mounter, create a volume share on the host machine to
	// mount OpenShift volumes.
	taskPrinter.StartTask("Creating host directories")
	if !c.UseNsenterMount {
		if err := c.HostHelper().EnsureVolumeUseShareMount(); err != nil {
			return taskPrinter.ToError(err)
		}
	}
	taskPrinter.Success()

	// Determine an IP to use for OpenShift.
	// The result is that c.ServerIP will be populated with
	// the IP that will be used on the client configuration file.
	// The c.ServerIP will be set to a specific IP when:
	// 1 - DOCKER_HOST is populated with a particular tcp:// type of address
	// 2 - a docker-machine has been specified
	// 3 - 127.0.0.1 is not working and an alternate IP has been found
	// Otherwise, the default c.ServerIP will be 127.0.0.1 which is what
	// will get stored in the client's config file. The reason for this is that
	// the client config will not depend on the machine's current IP address which
	// could change over time.
	//
	// c.AdditionalIPs will be populated with additional IPs that should be
	// included in the server's certificate. These include any IPs that are currently
	// assigned to the Docker host (hostname -I)
	// Each IP is tested to ensure that it can be accessed from the current client
	taskPrinter.StartTask("Determining server IP")
	c.ServerIP, c.AdditionalIPs, err = c.determineServerIP(out)
	if err != nil {
		return taskPrinter.ToError(err)
	}
	glog.V(3).Infof("Using %q as primary server IP and %q as additional IPs", c.ServerIP, strings.Join(c.AdditionalIPs, ","))
	taskPrinter.Success()
	if len(c.RoutingSuffix) == 0 {
		c.RoutingSuffix = c.ServerIP + ".nip.io"
	}

	// this used to be done in the openshift start method, but its mutating state.
	if len(c.HTTPProxy) > 0 || len(c.HTTPSProxy) > 0 {
		c.updateNoProxy()
	}

	return nil
}

// Validate validates that required fields in StartConfig have been populated
func (c *ClusterUpConfig) Validate(errout io.Writer) error {
	if c.dockerClient == nil {
		return fmt.Errorf("missing dockerClient")
	}
	cmdutil.WarnAboutCommaSeparation(errout, c.Environment, "--env")
	return nil
}

// Check is a spot to do NON-MUTATING, preflight checks. Over time, we should try to move our non-mutating checks out of
// Complete and into Check.
func (c *ClusterUpConfig) Check(out io.Writer) error {
	// used for some pretty printing
	taskPrinter := NewTaskPrinter(GetDetailedOut(out))

	// Check for an OpenShift container. If one exists and is running, exit.
	// If one exists but not running, delete it.
	taskPrinter.StartTask("Checking if OpenShift is already running")
	if err := checkExistingOpenShiftContainer(c.DockerHelper(), out); err != nil {
		return taskPrinter.ToError(err)
	}
	taskPrinter.Success()

	// Docker checks
	taskPrinter.StartTask(fmt.Sprintf("Checking for supported Docker version (=>%s)", dockerAPIVersion122))
	ver, err := c.DockerHelper().APIVersion()
	if err != nil {
		return taskPrinter.ToError(err)
	}
	if versions.LessThan(ver.APIVersion, dockerAPIVersion122) {
		return taskPrinter.ToError(fmt.Errorf("unsupported Docker version %s, need at least %s", ver.APIVersion, dockerAPIVersion122))
	}

	if !c.SkipRegistryCheck {
		taskPrinter.StartTask("Checking if insecured registry is configured properly in Docker")
		if err := c.checkDockerInsecureRegistry(out); err != nil {
			return taskPrinter.ToError(err)
		}
		taskPrinter.Success()
	}

	// Networking checks
	if c.PortForwarding {
		taskPrinter.StartTask("Checking prerequisites for port forwarding")
		if err := checkPortForwardingPrerequisites(out); err != nil {
			return taskPrinter.ToError(err)
		}
		taskPrinter.Success()

		err := openshift.CheckSocat()
		if err != nil {
			return err
		}
	}

	taskPrinter.StartTask("Checking if required ports are available")
	if err := c.checkAvailablePorts(out); err != nil {
		return taskPrinter.ToError(err)
	}
	taskPrinter.Success()

	// OpenShift checks
	taskPrinter.StartTask("Checking if OpenShift client is configured properly")
	if err := checkOpenShiftClient(); err != nil {
		return taskPrinter.ToError(err)
	}
	taskPrinter.Success()

	// Ensure that the OpenShift Docker image is available.
	// If not present, pull it.
	taskPrinter.StartTask(fmt.Sprintf("Checking if image %s is available", c.openshiftImage()))
	if err := c.checkOpenShiftImage(out); err != nil {
		return taskPrinter.ToError(err)
	}
	taskPrinter.Success()

	return nil
}

func GetDetailedOut(out io.Writer) io.Writer {
	// When loglevel > 0, just use stdout to write all messages
	if glog.V(1) {
		return out
	} else {
		return &bytes.Buffer{}
	}
}

// Start runs the start tasks ensuring that they are executed in sequence
func (c *ClusterUpConfig) Start(out io.Writer) error {
	fmt.Fprintf(out, "Starting OpenShift using %s ...\n", c.openshiftImage())

	if c.PortForwarding {
		if err := c.OpenShiftHelper().StartSocatTunnel(c.ServerIP); err != nil {
			return err
		}
	}

	if err := c.StartSelfHosted(out); err != nil {
		return err
	}
	if c.WriteConfig {
		return nil
	}
	if err := c.PostClusterStartupMutations(out); err != nil {
		return err
	}

	detailedOut := GetDetailedOut(out)
	taskPrinter := NewTaskPrinter(detailedOut)

	if !c.ShouldInitializeData() {
		taskPrinter.StartTask("Server Information")
		c.ServerInfo(out)
		taskPrinter.Success()
		return nil
	}

	// Add default redirect URIs to an OAuthClient to enable local web-console development.
	taskPrinter.StartTask("Adding default OAuthClient redirect URIs")
	if err := c.ensureDefaultRedirectURIs(out); err != nil {
		return taskPrinter.ToError(err)
	}
	taskPrinter.Success()

	installContext, err := componentinstall.NewComponentInstallContext(c.openshiftImage(), c.imageFormat(), c.BaseDir, c.ServerLogLevel)
	if err != nil {
		return err
	}

	// TODO, now we build up a set of things to install here.  We build the list so that we can install everything in
	// TODO parallel to avoid anyone accidentally introducing dependencies.  We'll start with migrating what we have
	// TODO and then we'll try to clean it up.
	registryInstall := &registry.RegistryComponentOptions{
		PVDir:          c.HostPersistentVolumesDir,
		InstallContext: installContext,
	}
	//	return c.OpenShiftHelper().InstallRouter(c.ServerIP, c.PortForwarding)
	routerInstall := &router.RouterComponentOptions{
		RoutingSuffix:  c.RoutingSuffix,
		PortForwarding: c.PortForwarding,
		InstallContext: installContext,
	}
	webConsoleInstall := &web_console.WebConsoleComponentOptions{
		PublicConsoleURL: fmt.Sprintf("https://%s:8443/console", c.GetPublicHostName()),
		PublicMasterURL:  fmt.Sprintf("https://%s:8443", c.GetPublicHostName()),
		InstallContext:   installContext,
	}

	componentsToInstall := []componentinstall.Component{}
	componentsToInstall = append(componentsToInstall, c.ImportInitialObjectsComponents(c.Out)...)
	componentsToInstall = append(componentsToInstall, registryInstall, webConsoleInstall, routerInstall)

	if err := componentinstall.InstallComponents(componentsToInstall, c.GetDockerClient(), c.GetLogDir()); err != nil {
		return err
	}

	if len(c.AddComponents) > 0 {
		args := append([]string{}, "--image="+c.Image)
		args = append(args, "--tag="+c.ImageTag)
		args = append(args, "--base-dir="+c.BaseDir)
		args = append(args, c.AddComponents...)

		if err := c.ClusterAdd.ParseFlags(args); err != nil {
			return err
		}
		glog.V(2).Infof("oc cluster add %v", args)
		if err := c.ClusterAdd.RunE(c.ClusterAdd, args); err != nil {
			return err
		}
	}

	if c.ShouldCreateUser() {
		// Login with an initial default user
		taskPrinter.StartTask("Login to server")
		if err := c.Login(out); err != nil {
			return taskPrinter.ToError(err)
		}
		taskPrinter.Success()

		// Create an initial project
		taskPrinter.StartTask(fmt.Sprintf("Creating initial project %q", initialProjectName))
		if err := c.CreateProject(out); err != nil {
			return taskPrinter.ToError(err)
		}
		taskPrinter.Success()
	}

	taskPrinter.StartTask("Server Information")
	c.ServerInfo(out)
	taskPrinter.Success()

	return nil
}

func defaultPortForwarding() bool {
	// Defaults to true if running on Mac, with no DOCKER_HOST defined
	return runtime.GOOS == "darwin" && len(os.Getenv("DOCKER_HOST")) == 0
}

// checkOpenShiftClient ensures that the client can be configured
// for the new server
func checkOpenShiftClient() error {
	kubeConfig := os.Getenv("KUBECONFIG")
	if len(kubeConfig) == 0 {
		return nil
	}
	var (
		kubeConfigError error
		f               *os.File
	)
	_, err := os.Stat(kubeConfig)
	switch {
	case os.IsNotExist(err):
		err = os.MkdirAll(filepath.Dir(kubeConfig), 0755)
		if err != nil {
			kubeConfigError = fmt.Errorf("cannot make directory: %v", err)
			break
		}
		f, err = os.Create(kubeConfig)
		if err != nil {
			kubeConfigError = fmt.Errorf("cannot create file: %v", err)
			break
		}
		f.Close()
	case err == nil:
		f, err = os.OpenFile(kubeConfig, os.O_RDWR, 0644)
		if err != nil {
			kubeConfigError = fmt.Errorf("cannot open %s for write: %v", kubeConfig, err)
			break
		}
		f.Close()
	default:
		kubeConfigError = fmt.Errorf("cannot access %s: %v", kubeConfig, err)
	}
	if kubeConfigError != nil {
		return errors.ErrKubeConfigNotWriteable(kubeConfig, kubeConfigError)
	}
	return nil
}

// GetDockerClient obtains a new Docker client from the environment or
// from a Docker machine, starting it if necessary
func (c *ClusterUpConfig) GetDockerClient() dockerhelper.Interface {
	return c.dockerClient
}

// GetDockerClient obtains a new Docker client from the environment or
// from a Docker machine, starting it if necessary and permitted
func GetDockerClient() (dockerhelper.Interface, error) {
	dockerTLSVerify := os.Getenv("DOCKER_TLS_VERIFY")
	dockerCertPath := os.Getenv("DOCKER_CERT_PATH")
	if len(dockerTLSVerify) > 0 && len(dockerCertPath) == 0 {
		dockerCertPath = cliconfig.Dir()
		os.Setenv("DOCKER_CERT_PATH", dockerCertPath)
	}

	if glog.V(4) {
		dockerHost := os.Getenv("DOCKER_HOST")
		if len(dockerHost) == 0 && len(dockerTLSVerify) == 0 && len(dockerCertPath) == 0 {
			glog.Infof("No Docker environment variables found. Will attempt default socket.")
		}
		if len(dockerHost) > 0 {
			glog.Infof("Will try Docker connection with host (DOCKER_HOST) %q", dockerHost)
		} else {
			glog.Infof("No Docker host (DOCKER_HOST) configured. Will attempt default socket.")
		}
		if len(dockerTLSVerify) > 0 {
			glog.Infof("DOCKER_TLS_VERIFY=%s", dockerTLSVerify)
		}
		if len(dockerCertPath) > 0 {
			glog.Infof("DOCKER_CERT_PATH=%s", dockerCertPath)
		}
	}
	// FIXME: Workaround for docker engine API client on OS X - sets the default to
	// the wrong DOCKER_HOST string
	if runtime.GOOS == "darwin" {
		dockerHost := os.Getenv("DOCKER_HOST")
		if len(dockerHost) == 0 {
			os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
		}
	}
	dockerHost := os.Getenv("DOCKER_HOST")
	if len(dockerHost) == 0 {
		dockerHost = dockerclient.DefaultDockerHost
	}
	engineAPIClient, err := dockerclient.NewEnvClient()
	if err != nil {
		return nil, errors.ErrNoDockerClient(err)
	}
	// negotiate the correct API version with the server
	ctx, fn := context.WithTimeout(context.Background(), 10*time.Second)
	defer fn()
	engineAPIClient.NegotiateAPIVersion(ctx)
	return dockerhelper.NewClient(dockerHost, engineAPIClient), nil
}

// checkExistingOpenShiftContainer checks the state of an OpenShift container.
// If one is already running, it throws an error.
// If one exists, it removes it so a new one can be created.
func checkExistingOpenShiftContainer(dockerHelper *dockerhelper.Helper, out io.Writer) error {
	container, running, err := dockerHelper.GetContainerState(openshift.ContainerName)
	if err != nil {
		return errors.NewError("unexpected error while checking OpenShift container state").WithCause(err)
	}
	if running {
		return errors.NewError("OpenShift is already running").WithSolution("To start OpenShift again, stop the current cluster:\n$ %s\n", "oc cluster down")
	}
	if container != nil {
		err = dockerHelper.RemoveContainer(openshift.ContainerName)
		if err != nil {
			return errors.NewError("cannot delete existing OpenShift container").WithCause(err)
		}
		fmt.Fprintln(out, "Deleted existing OpenShift container")
	}
	return nil
}

// checkOpenShiftImage checks whether the OpenShift image exists.
// If not it tells the Docker daemon to pull it.
func (c *ClusterUpConfig) checkOpenShiftImage(out io.Writer) error {
	return c.DockerHelper().CheckAndPull(c.openshiftImage(), out)
}

// checkDockerInsecureRegistry checks to see if the Docker daemon has an appropriate insecure registry argument set so that our services can access the registry
func (c *ClusterUpConfig) checkDockerInsecureRegistry(out io.Writer) error {
	configured, hasEntries, err := c.DockerHelper().InsecureRegistryIsConfigured(openshift.DefaultSvcCIDR)
	if err != nil {
		return err
	}
	if !configured {
		if hasEntries {
			return errors.ErrInvalidInsecureRegistryArgument()
		}
		return errors.ErrNoInsecureRegistryArgument()
	}
	return nil
}

// checkPortForwardingPrerequisites checks that socat is installed when port forwarding is enabled
// Socat needs to be installed manually on MacOS
func checkPortForwardingPrerequisites(out io.Writer) error {
	err := localcmd.New("socat").Args("-V").Run()
	if err != nil {
		glog.V(2).Infof("Error from socat command execution: %v", err)
		fmt.Fprintln(out, "WARNING: Port forwarding requires socat command line utility."+
			"Cluster public ip may not be reachable. Please make sure socat installed in your operating system.")
	}
	return nil
}

// ensureDefaultRedirectURIs merges a default URL to an auth client's RedirectURIs array
func (c *ClusterUpConfig) ensureDefaultRedirectURIs(out io.Writer) error {
	restConfig, err := c.RESTConfig()
	if err != nil {
		return err
	}
	oauthClient, err := oauthclientinternal.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	webConsoleOAuth, err := oauthClient.Oauth().OAuthClients().Get(defaultRedirectClient, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			fmt.Fprintf(out, "Unable to find OAuthClient %q\n", defaultRedirectClient)
			return nil
		}

		// announce fetch error without interrupting remaining tasks
		suggestedCmd := fmt.Sprintf("oc patch %s/%s -p '{%q:[%q]}'", "oauthclient", defaultRedirectClient, "redirectURIs", developmentRedirectURI)
		errMsg := fmt.Sprintf("Unable to fetch OAuthClient %q.\nTo manually add a development redirect URI, run %q\n", defaultRedirectClient, suggestedCmd)
		fmt.Fprintf(out, "%s\n", errMsg)
		return nil
	}

	// ensure the default redirect URI is not already present
	redirects := sets.NewString(webConsoleOAuth.RedirectURIs...)
	if redirects.Has(developmentRedirectURI) {
		return nil
	}

	webConsoleOAuth.RedirectURIs = append(webConsoleOAuth.RedirectURIs, developmentRedirectURI)

	_, err = oauthClient.Oauth().OAuthClients().Update(webConsoleOAuth)
	if err != nil {
		// announce error without interrupting remaining tasks
		suggestedCmd := fmt.Sprintf("oc patch %s/%s -p '{%q:[%q]}'", "oauthclient", defaultRedirectClient, "redirectURIs", developmentRedirectURI)
		errMsg := fmt.Sprintf("Unable to add development redirect URI to the %q OAuthClient.\nTo manually add it, run %q\n", defaultRedirectClient, suggestedCmd)
		fmt.Fprintf(out, "%s\n", errMsg)
		return nil
	}

	return nil
}

// checkAvailablePorts ensures that ports used by OpenShift are available on the Docker host
func (c *ClusterUpConfig) checkAvailablePorts(out io.Writer) error {
	err := c.OpenShiftHelper().TestPorts(openshift.AllPorts)
	if err == nil {
		return nil
	}
	if !openshift.IsPortsNotAvailableErr(err) {
		return err
	}
	unavailable := sets.NewInt(openshift.UnavailablePorts(err)...)
	if unavailable.HasAny(openshift.BasePorts...) {
		return errors.NewError("a port needed by OpenShift is not available").WithCause(err)
	}
	if unavailable.Has(openshift.DefaultDNSPort) {
		return errors.NewError(fmt.Sprintf("DNS port %d is not available", openshift.DefaultDNSPort))
	}

	for _, port := range openshift.RouterPorts {
		if unavailable.Has(port) {
			fmt.Fprintf(out, "WARNING: Port %d is already in use and may cause routing issues for applications.\n", port)
		}
	}
	return nil
}

// determineServerIP gets an appropriate IP address to communicate with the OpenShift server
func (c *ClusterUpConfig) determineServerIP(out io.Writer) (string, []string, error) {
	ip, err := c.determineIP(out)
	if err != nil {
		return "", nil, errors.NewError("cannot determine a server IP to use").WithCause(err)
	}
	serverIP := ip
	additionalIPs, err := c.determineAdditionalIPs(c.ServerIP)
	if err != nil {
		return "", nil, errors.NewError("cannot determine additional IPs").WithCause(err)
	}
	return serverIP, additionalIPs, nil
}

// updateNoProxy will add some default values to the NO_PROXY setting if they are not present
func (c *ClusterUpConfig) updateNoProxy() {
	values := []string{"127.0.0.1", c.ServerIP, "localhost", service_catalog.ServiceCatalogServiceIP, registry.RegistryServiceClusterIP}
	ipFromServer, err := c.OpenShiftHelper().ServerIP()
	if err == nil {
		values = append(values, ipFromServer)
	}
	noProxySet := sets.NewString(c.NoProxy...)
	for _, v := range values {
		if !noProxySet.Has(v) {
			noProxySet.Insert(v)
			c.NoProxy = append(c.NoProxy, v)
		}
	}
}

func (c *ClusterUpConfig) PostClusterStartupMutations(out io.Writer) error {
	restConfig, err := c.RESTConfig()
	if err != nil {
		return err
	}
	kClient, err := kclientset.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	// Remove any duplicate nodes
	if err := c.OpenShiftHelper().CheckNodes(kClient); err != nil {
		return err
	}

	err = c.OpenShiftHelper().SetupPersistentStorage(restConfig, c.HostPersistentVolumesDir)
	if err != nil {
		return err
	}
	return nil
}

func (c *ClusterUpConfig) imageFormat() string {
	return fmt.Sprintf("%s-${component}:%s", c.Image, c.ImageTag)
}

// TODO this should become a separate thing we can install, like registry
func (c *ClusterUpConfig) ImportInitialObjectsComponents(out io.Writer) []componentinstall.Component {
	componentsToInstall := []componentinstall.Component{}
	componentsToInstall = append(componentsToInstall,
		c.makeObjectImportInstallationComponentsOrDie(out, openshift.Namespace, map[string]string{
			c.ImageStreams: imageStreams[c.ImageStreams],
		})...)
	componentsToInstall = append(componentsToInstall,
		c.makeObjectImportInstallationComponentsOrDie(out, openshift.Namespace, templateLocations)...)
	componentsToInstall = append(componentsToInstall,
		c.makeObjectImportInstallationComponentsOrDie(out, "kube-system", adminTemplateLocations)...)

	return componentsToInstall
}

// RegisterTemplateServiceBroker will register the tsb with the service catalog
func (c *ClusterUpConfig) RegisterTemplateServiceBroker(out io.Writer) error {
	clusterAdminKubeConfig, err := c.ClusterAdminKubeConfigBytes()
	if err != nil {
		return err
	}
	return c.OpenShiftHelper().RegisterTemplateServiceBroker(clusterAdminKubeConfig, c.GetKubeAPIServerConfigDir(), c.GetLogDir())
}

// Login logs into the new server and sets up a default user and project
func (c *ClusterUpConfig) Login(out io.Writer) error {
	server := c.OpenShiftHelper().Master(c.ServerIP)
	return openshift.Login(initialUser, initialPassword, server, c.GetKubeAPIServerConfigDir(), c.originalFactory, c.command, out, out)
}

// CreateProject creates a new project for the current user
func (c *ClusterUpConfig) CreateProject(out io.Writer) error {
	f, err := openshift.LoggedInUserFactory()
	if err != nil {
		return errors.NewError("cannot get logged in user client").WithCause(err)
	}
	return openshift.CreateProject(f, initialProjectName, initialProjectDisplay, initialProjectDesc, "oc", out)
}

// ServerInfo displays server information after a successful start
func (c *ClusterUpConfig) ServerInfo(out io.Writer) {
	masterURL := fmt.Sprintf("https://%s:8443", c.GetPublicHostName())

	msg := fmt.Sprintf("OpenShift server started.\n\n"+
		"The server is accessible via web console at:\n"+
		"    %s\n\n", masterURL)

	if c.ShouldCreateUser() {
		msg += fmt.Sprintf("You are logged in as:\n"+
			"    User:     %s\n"+
			"    Password: <any value>\n\n", initialUser)
		msg += "To login as administrator:\n" +
			"    oc login -u system:admin\n\n"
	}

	msg += c.checkProxySettings()

	fmt.Fprintf(out, msg)
}

// checkProxySettings compares proxy settings specified for cluster up
// and those on the Docker daemon and generates appropriate warnings.
func (c *ClusterUpConfig) checkProxySettings() string {
	warnings := []string{}
	dockerHTTPProxy, dockerHTTPSProxy, dockerNoProxy, err := c.DockerHelper().GetDockerProxySettings()
	if err != nil {
		return "Unexpected error: " + err.Error()
	}
	// Check HTTP proxy
	if len(c.HTTPProxy) > 0 && len(dockerHTTPProxy) == 0 {
		warnings = append(warnings, "You specified an HTTP proxy for cluster up, but one is not configured for the Docker daemon")
	} else if len(c.HTTPProxy) == 0 && len(dockerHTTPProxy) > 0 {
		warnings = append(warnings, fmt.Sprintf("An HTTP proxy (%s) is configured for the Docker daemon, but you did not specify one for cluster up", dockerHTTPProxy))
	} else if c.HTTPProxy != dockerHTTPProxy {
		warnings = append(warnings, fmt.Sprintf("The HTTP proxy configured for the Docker daemon (%s) is not the same one you specified for cluster up", dockerHTTPProxy))
	}

	// Check HTTPS proxy
	if len(c.HTTPSProxy) > 0 && len(dockerHTTPSProxy) == 0 {
		warnings = append(warnings, "You specified an HTTPS proxy for cluster up, but one is not configured for the Docker daemon")
	} else if len(c.HTTPSProxy) == 0 && len(dockerHTTPSProxy) > 0 {
		warnings = append(warnings, fmt.Sprintf("An HTTPS proxy (%s) is configured for the Docker daemon, but you did not specify one for cluster up", dockerHTTPSProxy))
	} else if c.HTTPSProxy != dockerHTTPSProxy {
		warnings = append(warnings, fmt.Sprintf("The HTTPS proxy configured for the Docker daemon (%s) is not the same one you specified for cluster up", dockerHTTPSProxy))
	}

	if len(dockerHTTPProxy) > 0 || len(dockerHTTPSProxy) > 0 {
		dockerNoProxyList := strings.Split(dockerNoProxy, ",")
		dockerNoProxySet := sets.NewString(dockerNoProxyList...)
		if !dockerNoProxySet.Has(registry.RegistryServiceClusterIP) {
			warnings = append(warnings, fmt.Sprintf("A proxy is configured for Docker, however %[1]s is not included in its NO_PROXY list.\n"+
				"   %[1]s needs to be included in the Docker daemon's NO_PROXY environment variable so pushes to the local OpenShift registry can succeed.", registry.RegistryServiceClusterIP))
		}
	}

	if len(warnings) > 0 {
		buf := &bytes.Buffer{}
		for _, w := range warnings {
			fmt.Fprintf(buf, "WARNING: %s\n", w)
		}
		return buf.String()
	}
	return ""
}

// OpenShiftHelper returns a helper object to work with OpenShift on the server
func (c *ClusterUpConfig) OpenShiftHelper() *openshift.Helper {
	if c.openshiftHelper == nil {
		c.openshiftHelper = openshift.NewHelper(c.DockerHelper(), c.openshiftImage(), openshift.ContainerName)
	}
	return c.openshiftHelper
}

// HostHelper returns a helper object to check Host configuration
func (c *ClusterUpConfig) HostHelper() *host.HostHelper {
	if c.hostHelper == nil {
		c.hostHelper = host.NewHostHelper(c.DockerHelper(), c.openshiftImage(), c.HostVolumesDir)
	}
	return c.hostHelper
}

// DockerHelper returns a helper object to work with the Docker client
func (c *ClusterUpConfig) DockerHelper() *dockerhelper.Helper {
	if c.dockerHelper == nil {
		c.dockerHelper = dockerhelper.NewHelper(c.dockerClient)
	}
	return c.dockerHelper
}

func (c *ClusterUpConfig) makeObjectImportInstallationComponents(out io.Writer, namespace string, locations map[string]string) ([]componentinstall.Component, error) {
	clusterAdminKubeConfig, err := c.ClusterAdminKubeConfigBytes()
	if err != nil {
		return nil, err
	}

	componentsToInstall := []componentinstall.Component{}
	for name, location := range locations {
		componentsToInstall = append(componentsToInstall, componentinstall.List{
			ComponentName: namespace + "/" + name,
			Image:         c.openshiftImage(),
			Namespace:     namespace,
			KubeConfig:    clusterAdminKubeConfig,
			List:          bootstrap.MustAsset(location),
		})
	}

	return componentsToInstall, nil
}

func (c *ClusterUpConfig) makeObjectImportInstallationComponentsOrDie(out io.Writer, namespace string, locations map[string]string) []componentinstall.Component {
	componentsToInstall, err := c.makeObjectImportInstallationComponents(out, namespace, locations)
	if err != nil {
		panic(err)
	}
	return componentsToInstall
}

func (c *ClusterUpConfig) openshiftImage() string {
	return fmt.Sprintf("%s:%s", c.Image, c.ImageTag)
}

func (c *ClusterUpConfig) determineAdditionalIPs(ip string) ([]string, error) {
	additionalIPs := sets.NewString()
	serverIPs, err := c.OpenShiftHelper().OtherIPs(ip)
	if err != nil {
		return nil, errors.NewError("could not determine additional IPs").WithCause(err)
	}
	additionalIPs.Insert(serverIPs...)
	if c.PortForwarding {
		localIPs, err := c.localIPs()
		if err != nil {
			return nil, errors.NewError("could not determine additional local IPs").WithCause(err)
		}
		additionalIPs.Insert(localIPs...)
	}
	return additionalIPs.List(), nil
}

func (c *ClusterUpConfig) localIPs() ([]string, error) {
	ips := []string{}
	devices, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, dev := range devices {
		if (dev.Flags&net.FlagUp != 0) && (dev.Flags&net.FlagLoopback == 0) {
			addrs, err := dev.Addrs()
			if err != nil {
				continue
			}
			for i := range addrs {
				if ip, ok := addrs[i].(*net.IPNet); ok {
					if ip.IP.To4() != nil {
						ips = append(ips, ip.IP.String())
					}
				}
			}
		}
	}
	return ips, nil
}

func (c *ClusterUpConfig) determineIP(out io.Writer) (string, error) {
	if ip := net.ParseIP(c.PublicHostname); ip != nil && !ip.IsUnspecified() {
		fmt.Fprintf(out, "Using public hostname IP %s as the host IP\n", ip)
		return ip.String(), nil
	}

	// If using port-forwarding, use the default loopback address
	if c.PortForwarding {
		return "127.0.0.1", nil
	}

	// Try to get the host from the DOCKER_HOST if communicating via tcp
	var err error
	ip := c.DockerHelper().HostIP()
	if ip != "" {
		glog.V(2).Infof("Testing Docker host IP (%s)", ip)
		if err = c.OpenShiftHelper().TestIP(ip); err == nil {
			return ip, nil
		}
	}
	glog.V(2).Infof("Cannot use the Docker host IP(%s): %v", ip, err)

	// If IP is not specified, try to use the loopback IP
	// This is to default to an ip-agnostic client setup
	// where the real IP of the host will not affect client operations
	if err = c.OpenShiftHelper().TestIP("127.0.0.1"); err == nil {
		return "127.0.0.1", nil
	}

	// Next, use the the --print-ip output from openshift
	ip, err = c.OpenShiftHelper().ServerIP()
	if err == nil {
		glog.V(2).Infof("Testing openshift --print-ip (%s)", ip)
		if err = c.OpenShiftHelper().TestIP(ip); err == nil {
			return ip, nil
		}
		glog.V(2).Infof("OpenShift server ip test failed: %v", err)
	}
	glog.V(2).Infof("Cannot use OpenShift IP: %v", err)

	// Next, try other IPs on Docker host
	ips, err := c.OpenShiftHelper().OtherIPs(ip)
	if err != nil {
		return "", err
	}
	for i := range ips {
		glog.V(2).Infof("Testing additional IP (%s)", ip)
		if err = c.OpenShiftHelper().TestIP(ips[i]); err == nil {
			return ip, nil
		}
		glog.V(2).Infof("OpenShift additional ip test failed: %v", err)
	}
	return "", errors.NewError("cannot determine an IP to use for your server.")
}

// ShouldInitializeData tries to determine whether we're dealing with
// an existing OpenShift data and config. It determines that data exists by checking
// for the existence of a docker-registry service.
func (c *ClusterUpConfig) ShouldInitializeData() bool {
	if c.shouldInitializeData != nil {
		return *c.shouldInitializeData
	}

	result := func() bool {
		if !c.UseExistingConfig {
			return true
		}
		// For now, we determine if using existing etcd data by looking
		// for the registry service
		restConfig, err := c.RESTConfig()
		if err != nil {
			glog.V(2).Info(err)
			return true
		}
		kclient, err := kclientset.NewForConfig(restConfig)
		if err != nil {
			glog.V(2).Info(err)
			return true
		}

		if _, err = kclient.Core().Services(openshift.DefaultNamespace).Get(registry.SvcDockerRegistry, metav1.GetOptions{}); err != nil {
			return true
		}

		// If a registry exists, then don't initialize data
		return false
	}()
	c.shouldInitializeData = &result
	return result
}

// ShouldCreateUser determines whether a user and project should
// be created. If the user provider has been modified in the config, then it should
// not attempt to create a user. Also, even if the user provider has not been
// modified, but data has been initialized, then we should also not create user.
func (c *ClusterUpConfig) ShouldCreateUser() bool {
	if c.shouldCreateUser != nil {
		return *c.shouldCreateUser
	}

	result := func() bool {
		if !c.UseExistingConfig {
			return true
		}

		cfg, _, err := c.OpenShiftHelper().GetConfigFromLocalDir(c.GetKubeAPIServerConfigDir())
		if err != nil {
			glog.V(2).Infof("error reading config: %v", err)
			return true
		}
		if cfg.OAuthConfig == nil || len(cfg.OAuthConfig.IdentityProviders) != 1 {
			return false
		}
		if _, ok := cfg.OAuthConfig.IdentityProviders[0].Provider.(*configapi.AllowAllPasswordIdentityProvider); !ok {
			return false
		}

		return c.ShouldInitializeData()
	}()

	c.shouldCreateUser = &result
	return result
}

func (c *ClusterUpConfig) GetKubeAPIServerConfigDir() string {
	return path.Join(c.BaseDir, kubeapiserver.KubeAPIServerDirName)
}

func (c *ClusterUpConfig) GetLogDir() string {
	return path.Join(c.BaseDir, "logs")
}

func (c *ClusterUpConfig) RESTConfig() (*rest.Config, error) {
	clusterAdminKubeConfigBytes, err := c.ClusterAdminKubeConfigBytes()
	if err != nil {
		return nil, err
	}
	clusterAdminKubeConfig, err := kclientcmd.RESTConfigFromKubeConfig(clusterAdminKubeConfigBytes)
	if err != nil {
		return nil, err
	}

	return clusterAdminKubeConfig, nil
}

func (c *ClusterUpConfig) ClusterAdminKubeConfigBytes() ([]byte, error) {
	return ioutil.ReadFile(path.Join(c.GetKubeAPIServerConfigDir(), "admin.kubeconfig"))
}

func (c *ClusterUpConfig) GetPublicHostName() string {
	if len(c.PublicHostname) > 0 {
		return c.PublicHostname
	}
	return c.ServerIP
}
