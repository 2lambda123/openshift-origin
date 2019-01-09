package openshift_sdn

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/clientcmd"
	kubeproxyconfig "k8s.io/kubernetes/pkg/proxy/apis/config"
	"k8s.io/kubernetes/pkg/util/interrupt"

	"github.com/openshift/library-go/pkg/serviceability"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	_ "github.com/openshift/origin/pkg/cmd/server/apis/config/install"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	networkvalidation "github.com/openshift/origin/pkg/cmd/server/apis/config/validation/network"
	sdnnode "github.com/openshift/origin/pkg/network/node"
	sdnproxy "github.com/openshift/origin/pkg/network/proxy"
	"github.com/openshift/origin/pkg/version"
)

// OpenShiftSDN stores the variables needed to initialize the real networking
// processess from the command line.
type OpenShiftSDN struct {
	ConfigFilePath            string
	KubeConfigFilePath        string
	URLOnlyKubeConfigFilePath string

	NodeConfig  *configapi.NodeConfig
	ProxyConfig *kubeproxyconfig.KubeProxyConfiguration

	informers *informers
	OsdnNode  *sdnnode.OsdnNode
	OsdnProxy *sdnproxy.OsdnProxy
}

var networkLong = `
Start OpenShift SDN node components. This includes the service proxy.

This will also read the node name from the environment variable K8S_NODE_NAME.`

func NewOpenShiftSDNCommand(basename string, errout io.Writer) *cobra.Command {
	sdn := &OpenShiftSDN{}

	cmd := &cobra.Command{
		Use:   basename,
		Short: "Start OpenShiftSDN",
		Long:  networkLong,
		Run: func(c *cobra.Command, _ []string) {
			ch := make(chan struct{})
			interrupt.New(func(s os.Signal) {
				fmt.Fprintf(errout, "interrupt: Gracefully shutting down ...\n")
				close(ch)
			}).Run(func() error {
				sdn.Run(c, errout, ch)
				return nil
			})
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&sdn.ConfigFilePath, "config", "", "Location of the node configuration file to run from (required)")
	flags.StringVar(&sdn.KubeConfigFilePath, "kubeconfig", "", "Path to the kubeconfig file to use for requests to the Kubernetes API. Optional. When omitted, will use the in-cluster config")
	flags.StringVar(&sdn.URLOnlyKubeConfigFilePath, "url-only-kubeconfig", "", "Path to a kubeconfig file to use, but only to determine the URL to the apiserver. The in-cluster credentials will be used. Cannot use with --kubeconfig.")

	return cmd
}

// Run starts the network process. Does not return.
func (sdn *OpenShiftSDN) Run(c *cobra.Command, errout io.Writer, stopCh chan struct{}) {
	err := injectKubeAPIEnv(sdn.URLOnlyKubeConfigFilePath)
	if err != nil {
		glog.Fatal(err)
	}

	// Parse config file, build config objects
	err = sdn.ValidateAndParse()
	if err != nil {
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

	// Set up a watch on our config file; if it changes, we should exit -
	// (we don't have the ability to dynamically reload config changes).
	if err := watchForChanges(sdn.ConfigFilePath, stopCh); err != nil {
		glog.Fatalf("unable to setup configuration watch: %v", err)
	}

	// Build underlying network objects
	err = sdn.Init()
	if err != nil {
		glog.Fatalf("Failed to initialize sdn: %v", err)
	}

	err = sdn.Start(stopCh)
	if err != nil {
		glog.Fatalf("Failed to start sdn: %v", err)
	}

	<-stopCh
	time.Sleep(500 * time.Millisecond) // gracefully shut down
	os.Exit(1)
}

// ValidateAndParse validates the command line options, parses the node
// configuration, and builds the upstream proxy configuration.
func (sdn *OpenShiftSDN) ValidateAndParse() error {
	if len(sdn.ConfigFilePath) == 0 {
		return errors.New("--config is required")
	}

	if len(sdn.KubeConfigFilePath) > 0 && len(sdn.URLOnlyKubeConfigFilePath) > 0 {
		return errors.New("cannot pass --kubeconfig and --url-only-kubeconfig")
	}

	glog.V(2).Infof("Reading node configuration from %s", sdn.ConfigFilePath)
	var err error
	sdn.NodeConfig, err = configapilatest.ReadAndResolveNodeConfig(sdn.ConfigFilePath)
	if err != nil {
		return err
	}

	if len(sdn.KubeConfigFilePath) > 0 {
		sdn.NodeConfig.MasterKubeConfig = sdn.KubeConfigFilePath
	}

	// Get the nodename from the environment, if available
	if len(sdn.NodeConfig.NodeName) == 0 {
		sdn.NodeConfig.NodeName = os.Getenv("K8S_NODE_NAME")
	}

	// Validate the node config
	validationResults := networkvalidation.ValidateInClusterNetworkNodeConfig(sdn.NodeConfig, nil)

	if len(validationResults.Warnings) != 0 {
		for _, warning := range validationResults.Warnings {
			glog.Warningf("Warning: %v, node start will continue.", warning)
		}
	}
	if len(validationResults.Errors) != 0 {
		glog.V(4).Infof("Configuration is invalid: %#v", sdn.NodeConfig)
		return kerrors.NewInvalid(configapi.Kind("NodeConfig"), sdn.ConfigFilePath, validationResults.Errors)
	}

	sdn.ProxyConfig, err = ProxyConfigFromNodeConfig(*sdn.NodeConfig)
	if err != nil {
		glog.V(4).Infof("Unable to build proxy config: %v", err)
		return err
	}

	return nil
}

// Init builds the underlying structs for the network processes.
func (sdn *OpenShiftSDN) Init() error {
	// Build the informers
	var err error
	err = sdn.buildInformers()
	if err != nil {
		return fmt.Errorf("failed to build informers: %v", err)
	}

	// Configure SDN
	err = sdn.initSDN()
	if err != nil {
		return fmt.Errorf("failed to initialize SDN: %v", err)
	}

	// Configure the proxy
	err = sdn.initProxy()
	if err != nil {
		return fmt.Errorf("failed to initialize proxy: %v", err)
	}

	return nil
}

// Start starts the network, proxy, and informers, then returns.
func (sdn *OpenShiftSDN) Start(stopCh <-chan struct{}) error {
	glog.Infof("Starting node networking (%s)", version.Get().String())

	serviceability.StartProfiler()
	err := sdn.runSDN()
	if err != nil {
		return err
	}
	sdn.runProxy()
	sdn.informers.start(stopCh)

	return nil
}

// injectKubeAPIEnv consumes the url-only-kubeconfig and re-injects it as
// environment variables. We need to do this because we cannot use the
// apiserver service ip (since we set it up!), but we want to use the in-cluster
// configuration. So, take the server URL from the kubelet kubeconfig.
func injectKubeAPIEnv(kcPath string) error {
	if kcPath != "" {
		kubeconfig, err := clientcmd.LoadFromFile(kcPath)
		if err != nil {
			return err
		}
		clusterName := kubeconfig.Contexts[kubeconfig.CurrentContext].Cluster
		apiURL := kubeconfig.Clusters[clusterName].Server

		url, err := url.Parse(apiURL)
		if err != nil {
			return err
		}

		// The kubernetes in-cluster functions don't let you override the apiserver
		// directly; gotta "pass" it via environment vars.
		glog.V(2).Infof("Overriding kubernetes api to %s", apiURL)
		os.Setenv("KUBERNETES_SERVICE_HOST", url.Hostname())
		os.Setenv("KUBERNETES_SERVICE_PORT", url.Port())
	}
	return nil
}

// watchForChanges closes stopCh if the configuration file changed.
func watchForChanges(configPath string, stopCh chan struct{}) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	if err := watcher.Add(configPath); err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-stopCh:
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Rename) > 0 {
					glog.V(2).Infof("Configuration file %s changed, exiting...", configPath)
					close(stopCh)
					return
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				glog.V(4).Infof("fsnotify error %v", err)
			}
		}
	}()
	return nil
}
