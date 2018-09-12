package clusterup

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker/api/types/versions"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	aggregatorinstall "k8s.io/kube-aggregator/pkg/apis/apiregistration/install"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	oauthclientinternal "github.com/openshift/origin/pkg/oauth/generated/internalclientset"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/errors"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/host"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/openshift"
	"github.com/openshift/origin/pkg/version"
)

const (
	// CmdUpRecommendedName is the recommended command name
	CmdUpRecommendedName = "up"

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
		This is to allow dynamic host names to be created for routes.

		A public hostname can also be specified for the server with the --public-hostname flag.`)

	cmdUpExample = templates.Examples(`
	  # Start OpenShift using a specific public host name
	  %[1]s --public-hostname=my.address.example.com`)

	// defaultImageStreams is the default key for the above imageStreams mapping.
	// It should be set during build via -ldflags.
	defaultImageStreams string
)

type ClusterUpConfig struct {
	ImageTemplate variable.ImageTemplate
	ImageTag      string

	DockerMachine  string
	PortForwarding bool
	KubeOnly       bool

	// BaseTempDir is the directory to use as the root for temp directories
	// This allows us to bundle all of the cluster-up directories in one spot for easier cleanup and ensures we aren't
	// doing crazy thing like dirtying /var on the host (that does weird stuff)
	BaseDir           string
	SpecifiedBaseDir  bool
	HostName          string
	UseExistingConfig bool
	ServerLogLevel    int

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
	HostPersistentVolumesDir string
	HTTPProxy                string
	HTTPSProxy               string
	NoProxy                  []string

	dockerClient        dockerhelper.Interface
	dockerHelper        *dockerhelper.Helper
	hostHelper          *host.HostHelper
	openshiftHelper     *openshift.Helper
	command             *cobra.Command
	defaultClientConfig clientcmdapi.Config
	isRemoteDocker      bool

	usingDefaultImages         bool
	usingDefaultOpenShiftImage bool

	pullPolicy string

	createdUser bool

	genericclioptions.IOStreams
}

func NewClusterUpConfig(streams genericclioptions.IOStreams) *ClusterUpConfig {
	return &ClusterUpConfig{
		UsePorts:       openshift.BasePorts,
		PortForwarding: defaultPortForwarding(),
		DNSPort:        openshift.DefaultDNSPort,

		ImageTemplate: variable.NewDefaultImageTemplate(),

		IOStreams: streams,
	}

}

// NewCmdUp creates a command that starts OpenShift on Docker with reasonable defaults
func NewCmdUp(name, fullName string, f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	config := NewClusterUpConfig(streams)
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Start OpenShift on Docker with reasonable defaults",
		Long:    cmdUpLong,
		Example: fmt.Sprintf(cmdUpExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(config.Complete(f, c))
			kcmdutil.CheckErr(config.Validate())
			kcmdutil.CheckErr(config.Check())
			if err := config.Start(); err != nil {
				PrintError(err, streams.ErrOut)
				os.Exit(1)
			}
		},
	}
	config.Bind(cmd.Flags())
	return cmd
}

func (c *ClusterUpConfig) Bind(flags *pflag.FlagSet) {
	flags.StringVar(&c.ImageTag, "tag", "", "Specify an explicit version for OpenShift images")
	flags.MarkHidden("tag")
	flags.StringVar(&c.ImageTemplate.Format, "image", c.ImageTemplate.Format, "Specify the images to use for OpenShift")
	flags.StringVar(&c.PublicHostname, "public-hostname", "", "Public hostname for OpenShift cluster")
	flags.StringVar(&c.BaseDir, "base-dir", c.BaseDir, "Directory on Docker host for cluster up configuration")
	flags.BoolVar(&c.WriteConfig, "write-config", false, "Write the configuration files into host config dir")
	flags.BoolVar(&c.PortForwarding, "forward-ports", c.PortForwarding, "Use Docker port-forwarding to communicate with origin container. Requires 'socat' locally.")
	flags.IntVar(&c.ServerLogLevel, "server-loglevel", 0, "Log level for OpenShift server")
	flags.BoolVar(&c.KubeOnly, "kube-only", c.KubeOnly, "Only install Kubernetes, no OpenShift apiserver or controllers.  Alpha, for development only.  Can result in an unstable cluster.")
	flags.MarkHidden("kube-only")
	flags.StringVar(&c.HTTPProxy, "http-proxy", "", "HTTP proxy to use for master and builds")
	flags.StringVar(&c.HTTPSProxy, "https-proxy", "", "HTTPS proxy to use for master and builds")
	flags.StringArrayVar(&c.NoProxy, "no-proxy", c.NoProxy, "List of hosts or subnets for which a proxy should not be used")
}

func (c *ClusterUpConfig) Complete(f genericclioptions.RESTClientGetter, cmd *cobra.Command) error {
	// TODO: remove this when we move to container/apply based component installation
	aggregatorinstall.Install(legacyscheme.Scheme)

	// Set the ImagePullPolicy field in static pods and components based in whether users specified
	// the --tag flag or not.
	c.pullPolicy = "Always"
	if len(c.ImageTag) > 0 {
		c.pullPolicy = "IfNotPresent"
	}
	glog.V(5).Infof("Using %q as default image pull policy", c.pullPolicy)

	// Get the default client config for login
	var err error
	c.defaultClientConfig, err = f.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		c.defaultClientConfig = *clientcmdapi.NewConfig()
	}

	c.command = cmd

	c.isRemoteDocker = len(os.Getenv("DOCKER_HOST")) > 0

	c.ImageTemplate.Format = variable.Expand(c.ImageTemplate.Format, func(s string) (string, bool) {
		if s == "version" {
			if len(c.ImageTag) == 0 {
				return strings.TrimRight("v"+version.Get().Major+"."+version.Get().Minor, "+"), true
			}
			return c.ImageTag, true
		}
		return "", false
	}, variable.Identity)

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

	if _, err := os.Stat(c.BaseDir); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(c.BaseDir, os.ModePerm); err != nil {
			return fmt.Errorf("unable to create base directory %q: %v", c.BaseDir, err)
		}
	}

	// Get a Docker client.
	// If a Docker machine was specified, make sure that the machine is running.
	// Otherwise, use environment variables.
	c.printProgress("Getting a Docker client")
	client, err := dockerhelper.GetDockerClient()
	if err != nil {
		return err
	}
	c.dockerClient = client

	// Check whether the Docker host has the right binaries to use Kubernetes' nsenter mounter
	// If not, use a shared volume to mount volumes on OpenShift
	if isRedHatDocker, err := c.DockerHelper().IsRedHat(); err == nil && isRedHatDocker {
		c.printProgress("Checking type of volume mount")
		c.UseNsenterMount, err = c.HostHelper().CanUseNsenterMounter()
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(c.BaseDir, 0755); err != nil {
		return err
	}

	if c.UseNsenterMount {
		// This is default path when you run cluster up locally, with local docker daemon
		c.HostVolumesDir = path.Join(c.BaseDir, "openshift.local.volumes")
		if err := os.MkdirAll(c.HostVolumesDir, 0755); err != nil {
			return err
		}
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
	if !c.UseNsenterMount {
		c.printProgress("Creating shared mount directory on the remote host")
		if err := c.HostHelper().EnsureVolumeUseShareMount(c.HostVolumesDir); err != nil {
			return err
		}
	}

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
	c.printProgress("Determining server IP")
	c.ServerIP, c.AdditionalIPs, err = c.determineServerIP()
	if err != nil {
		return err
	}
	glog.V(3).Infof("Using %q as primary server IP and %q as additional IPs", c.ServerIP, strings.Join(c.AdditionalIPs, ","))

	// this used to be done in the openshift start method, but its mutating state.
	if len(c.HTTPProxy) > 0 || len(c.HTTPSProxy) > 0 {
		c.updateNoProxy()
	}

	return nil
}

// Validate validates that required fields in StartConfig have been populated
func (c *ClusterUpConfig) Validate() error {
	if c.dockerClient == nil {
		return fmt.Errorf("missing dockerClient")
	}
	return nil
}

func (c *ClusterUpConfig) printProgress(msg string) {
	fmt.Fprintf(c.Out, msg+" ...\n")
}

// Check is a spot to do NON-MUTATING, preflight checks. Over time, we should try to move our non-mutating checks out of
// Complete and into Check.
func (c *ClusterUpConfig) Check() error {
	// Check for an OpenShift container. If one exists and is running, exit.
	// If one exists but not running, delete it.
	c.printProgress("Checking if OpenShift is already running")
	if err := checkExistingOpenShiftContainer(c.DockerHelper()); err != nil {
		return err
	}

	// Docker checks
	c.printProgress(fmt.Sprintf("Checking for supported Docker version (=>%s)", dockerAPIVersion122))
	ver, err := c.DockerHelper().APIVersion()
	if err != nil {
		return err
	}
	if versions.LessThan(ver.APIVersion, dockerAPIVersion122) {
		return fmt.Errorf("unsupported Docker version %s, need at least %s", ver.APIVersion, dockerAPIVersion122)
	}

	// Networking checks
	if c.PortForwarding {
		c.printProgress("Checking prerequisites for port forwarding")
		if err := checkPortForwardingPrerequisites(); err != nil {
			return err
		}
		if err := openshift.CheckSocat(); err != nil {
			return err
		}
	}

	c.printProgress("Checking if required ports are available")
	if err := c.checkAvailablePorts(); err != nil {
		return err
	}

	// OpenShift checks
	c.printProgress("Checking if OpenShift client is configured properly")
	if err := c.checkOpenShiftClient(); err != nil {
		return err
	}

	c.printProgress(fmt.Sprintf("Checking if image %s is available", c.openshiftImage()))
	if err := c.checkRequiredImagesAvailable(); err != nil {
		return err
	}

	return nil
}

// Start runs the start tasks ensuring that they are executed in sequence
func (c *ClusterUpConfig) Start() error {
	fmt.Fprintf(c.Out, "Starting OpenShift using %s ...\n", c.openshiftImage())

	if c.PortForwarding {
		if err := c.OpenShiftHelper().StartSocatTunnel(c.ServerIP); err != nil {
			return err
		}
	}

	if err := c.StartSelfHosted(c.Out); err != nil {
		return err
	}
	if c.WriteConfig {
		return nil
	}
	if err := c.PostClusterStartupMutations(c.Out); err != nil {
		return err
	}

	// if we're only supposed to install kube, only install kube.  Maybe later we'll add back components.
	if c.KubeOnly {
		c.printProgress("Server Information")
		c.serverInfo(c.Out)
		return nil
	}

	// Add default redirect URIs to an OAuthClient to enable local web-console development.
	c.printProgress("Adding default OAuthClient redirect URIs")
	if err := c.ensureDefaultRedirectURIs(c.Out); err != nil {
		return err
	}

	c.printProgress("Server Information")
	c.serverInfo(c.Out)

	return nil
}

func defaultPortForwarding() bool {
	// Defaults to true if running on Mac, with no DOCKER_HOST defined
	return runtime.GOOS == "darwin" && len(os.Getenv("DOCKER_HOST")) == 0
}

// checkOpenShiftClient ensures that the client can be configured
// for the new server
func (c *ClusterUpConfig) checkOpenShiftClient() error {
	kubeConfig := os.Getenv("KUBECONFIG")
	if len(kubeConfig) == 0 {
		return nil
	}

	// if you're trying to use the kubeconfig into a subdirectory of the basedir, you're probably using a KUBECONFIG
	// location that is going to overwrite a "real" kubeconfig, usually admin.kubeconfig which will break every other component
	// relying on it being a full power kubeconfig
	kubeConfigDir := filepath.Dir(kubeConfig)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	absKubeConfigDir, err := cmdutil.MakeAbs(kubeConfigDir, cwd)
	if err != nil {
		return err
	}
	if strings.HasPrefix(absKubeConfigDir, c.BaseDir+"/") {
		return fmt.Errorf("cannot choose kubeconfig in subdirectory of the --base-dir: %q", kubeConfig)
	}

	var (
		kubeConfigError error
		f               *os.File
	)
	_, err = os.Stat(kubeConfig)
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

// checkExistingOpenShiftContainer checks the state of an OpenShift container.
// If one is already running, it throws an error.
// If one exists, it removes it so a new one can be created.
func checkExistingOpenShiftContainer(dockerHelper *dockerhelper.Helper) error {
	container, running, err := dockerHelper.GetContainerState(openshift.OriginContainerName)
	if err != nil {
		return errors.NewError("unexpected error while checking OpenShift container state").WithCause(err)
	}
	if running {
		return errors.NewError("OpenShift is already running").WithSolution("To start OpenShift again, stop the current cluster:\n$ %s\n", "oc cluster down")
	}
	if container != nil {
		err = dockerHelper.RemoveContainer(openshift.OriginContainerName)
		if err != nil {
			return errors.NewError("cannot delete existing OpenShift container").WithCause(err)
		}
		glog.V(2).Info("Deleted existing OpenShift container")
	}
	return nil
}

// checkRequiredImagesAvailable checks whether the OpenShift image exists.
// If not it tells the Docker daemon to pull it.
func (c *ClusterUpConfig) checkRequiredImagesAvailable() error {
	if err := c.DockerHelper().CheckAndPull(c.openshiftImage(), c.Out); err != nil {
		return err
	}
	if err := c.DockerHelper().CheckAndPull(c.cliImage(), c.Out); err != nil {
		return err
	}
	if err := c.DockerHelper().CheckAndPull(c.etcdImage(), c.Out); err != nil {
		return err
	}
	if err := c.DockerHelper().CheckAndPull(c.bootkubeImage(), c.Out); err != nil {
		return err
	}
	return nil
}

// checkPortForwardingPrerequisites checks that socat is installed when port forwarding is enabled
// Socat needs to be installed manually on MacOS
func checkPortForwardingPrerequisites() error {
	commandOut, err := exec.Command("socat", "-V").CombinedOutput()
	if err != nil {
		glog.V(2).Infof("Error from socat command execution: %v\n%s", err, string(commandOut))
		glog.Warning("Port forwarding requires socat command line utility." +
			"Cluster public ip may not be reachable. Please make sure socat installed in your operating system.")
	}
	return nil
}

// ensureDefaultRedirectURIs merges a default URL to an auth client's RedirectURIs array
func (c *ClusterUpConfig) ensureDefaultRedirectURIs(out io.Writer) error {
	restConfig, err := c.KubeControlPlaneRESTConfig()
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
		fmt.Fprintf(out, fmt.Sprintf("Unable to add development redirect URI to the %q OAuthClient.\nTo manually add it, run %q\n", defaultRedirectClient, suggestedCmd))
		return nil
	}

	return nil
}

// checkAvailablePorts ensures that ports used by OpenShift are available on the Docker host
func (c *ClusterUpConfig) checkAvailablePorts() error {
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
			glog.Warningf("Port %d is already in use and may cause routing issues for applications.\n", port)
		}
	}
	return nil
}

// determineServerIP gets an appropriate IP address to communicate with the OpenShift server
func (c *ClusterUpConfig) determineServerIP() (string, []string, error) {
	ip, err := c.determineIP()
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
	values := []string{"127.0.0.1", c.ServerIP, "localhost"}
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
	restConfig, err := c.KubeControlPlaneRESTConfig()
	if err != nil {
		return err
	}
	kClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	// Remove any duplicate nodes
	if err := c.OpenShiftHelper().CheckNodes(kClient); err != nil {
		return err
	}

	return nil
}

func (c *ClusterUpConfig) imageFormat() string {
	return c.ImageTemplate.Format
}

// serverInfo displays server information after a successful start
func (c *ClusterUpConfig) serverInfo(out io.Writer) {
	masterURL := fmt.Sprintf("https://%s:8443", c.GetPublicHostName())

	msg := fmt.Sprintf("OpenShift server started.\n\n"+
		"The server is accessible via web console at:\n"+
		"    %s\n\n", masterURL)

	msg += c.checkProxySettings()

	fmt.Fprintf(out, msg)
}

// checkProxySettings compares proxy settings specified for cluster up
// and those on the Docker daemon and generates appropriate warnings.
func (c *ClusterUpConfig) checkProxySettings() string {
	var warnings []string
	dockerHTTPProxy, dockerHTTPSProxy, _, err := c.DockerHelper().GetDockerProxySettings()
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
		c.openshiftHelper = openshift.NewHelper(c.DockerHelper(), c.openshiftImage(), openshift.OriginContainerName)
	}
	return c.openshiftHelper
}

// HostHelper returns a helper object to check Host configuration
func (c *ClusterUpConfig) HostHelper() *host.HostHelper {
	if c.hostHelper == nil {
		c.hostHelper = host.NewHostHelper(c.DockerHelper(), c.openshiftImage())
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

func (c *ClusterUpConfig) etcdImage() string {
	return "quay.io/coreos/etcd:v3.2.24"
}

func (c *ClusterUpConfig) bootkubeImage() string {
	return "quay.io/coreos/bootkube:v0.13.0"
}

func (c *ClusterUpConfig) openshiftImage() string {
	return c.ImageTemplate.ExpandOrDie("control-plane")
}

func (c *ClusterUpConfig) hypershiftImage() string {
	return c.ImageTemplate.ExpandOrDie("hypershift")
}

func (c *ClusterUpConfig) hyperkubeImage() string {
	return c.ImageTemplate.ExpandOrDie("hyperkube")
}

func (c *ClusterUpConfig) nodeImage() string {
	return c.ImageTemplate.ExpandOrDie("node")
}

func (c *ClusterUpConfig) podImage() string {
	return c.ImageTemplate.ExpandOrDie("pod")
}

func (c *ClusterUpConfig) cliImage() string {
	return c.ImageTemplate.ExpandOrDie("cli")
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
	var ips []string
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

func (c *ClusterUpConfig) determineIP() (string, error) {
	if ip := net.ParseIP(c.PublicHostname); ip != nil && !ip.IsUnspecified() {
		fmt.Fprintf(c.Out, "Using public hostname IP %s as the host IP\n", ip)
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

func (c *ClusterUpConfig) KubeControlPlaneAuthDir() string {
	return path.Join(c.BaseDir, "bootkube", "auth")
}

func (c *ClusterUpConfig) KubeControlPlaneRESTConfig() (*rest.Config, error) {
	clusterAdminKubeConfigBytes, err := c.KubeControlPlaneKubeConfigBytes()
	if err != nil {
		return nil, err
	}
	clusterAdminKubeConfig, err := kclientcmd.RESTConfigFromKubeConfig(clusterAdminKubeConfigBytes)
	if err != nil {
		return nil, err
	}

	return clusterAdminKubeConfig, nil
}

func (c *ClusterUpConfig) KubeControlPlaneKubeConfigBytes() ([]byte, error) {
	return ioutil.ReadFile(path.Join(c.KubeControlPlaneAuthDir(), "kubeconfig"))
}

func (c *ClusterUpConfig) GetPublicHostName() string {
	if len(c.PublicHostname) > 0 {
		return c.PublicHostname
	}
	return c.ServerIP
}
