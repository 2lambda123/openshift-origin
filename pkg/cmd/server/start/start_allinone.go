package start

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"

	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/limitranger"
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/namespace/exists"
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/namespace/lifecycle"
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/resourcequota"
)

type AllInOneOptions struct {
	MasterArgs *MasterArgs
	NodeArgs   *NodeArgs

	WriteConfigOnly  bool
	MasterConfigFile string
	NodeConfigFile   string
}

const longAllInOneCommandDesc = `
Start an OpenShift all-in-one server

This command helps you launch an OpenShift all-in-one server, which allows
you to run all of the components of an OpenShift system on a server with Docker. Running

    $ openshift start

will start OpenShift listening on all interfaces, launch an etcd server to store persistent
data, and launch the Kubernetes system components. The server will run in the foreground until
you terminate the process.  This command delegates to "openshift start master" and 
"openshift start node".


Note: starting OpenShift without passing the --master address will attempt to find the IP
address that will be visible inside running Docker containers. This is not always successful,
so if you have problems tell OpenShift what public address it will be via --master=<ip>.

You may also pass --etcd=<address> to connect to an external etcd server.

You may also pass --kubeconfig=<path> to connect to an external Kubernetes cluster.
`

// NewCommandStartMaster provides a CLI handler for 'start' command
func NewCommandStartAllInOne() (*cobra.Command, *AllInOneOptions) {
	options := &AllInOneOptions{}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Launch OpenShift All-In-One",
		Long:  longAllInOneCommandDesc,
		Run: func(c *cobra.Command, args []string) {
			if err := options.Complete(); err != nil {
				fmt.Println(err.Error())
				c.Help()
				return
			}
			if err := options.Validate(args); err != nil {
				fmt.Println(err.Error())
				c.Help()
				return
			}

			startProfiler()

			if err := options.StartAllInOne(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(c.Out(), "Invalid %s %s\n", details.Kind, details.ID)
						for _, cause := range details.Causes {
							fmt.Fprintln(c.Out(), cause.Message)
						}
						os.Exit(255)
					}
				}
				glog.Fatal(err)
			}
		},
	}

	flags := cmd.Flags()

	flags.BoolVar(&options.WriteConfigOnly, "write-config", false, "Indicates that the command should build the configuration from command-line arguments, write it to the locations specified by --master-config and --node-config, and exit.")
	flags.StringVar(&options.MasterConfigFile, "master-config", "", "Location of the master configuration file to run from, or write to (when used with --write-config). When running from configuration files, all other command-line arguments are ignored.")
	flags.StringVar(&options.NodeConfigFile, "node-config", "", "Location of the node configuration file to run from, or write to (when used with --write-config). When running from configuration files, all other command-line arguments are ignored.")

	masterArgs, nodeArgs, listenArg, imageFormatArgs, _, certArgs := GetAllInOneArgs()
	options.MasterArgs, options.NodeArgs = masterArgs, nodeArgs
	// by default, all-in-ones all disabled docker.  Set it here so that if we allow it to be bound later, bindings take precendence
	options.NodeArgs.AllowDisabledDocker = true

	BindMasterArgs(masterArgs, flags, "")
	BindNodeArgs(nodeArgs, flags, "")
	BindListenArg(listenArg, flags, "")
	BindPolicyArgs(options.MasterArgs.PolicyArgs, flags, "")
	BindImageFormatArgs(imageFormatArgs, flags, "")
	BindCertArgs(certArgs, flags, "")

	startMaster, _ := NewCommandStartMaster()
	startNode, _ := NewCommandStartNode()
	cmd.AddCommand(startMaster)
	cmd.AddCommand(startNode)

	return cmd, options
}

// GetAllInOneArgs makes sure that the node and master args that should be shared, are shared
func GetAllInOneArgs() (*MasterArgs, *NodeArgs, *ListenArg, *ImageFormatArgs, *KubeConnectionArgs, *CertArgs) {
	masterArgs := NewDefaultMasterArgs()
	nodeArgs := NewDefaultNodeArgs()

	listenArg := NewDefaultListenArg()
	masterArgs.ListenArg = listenArg
	nodeArgs.ListenArg = listenArg

	imageFormatArgs := NewDefaultImageFormatArgs()
	masterArgs.ImageFormatArgs = imageFormatArgs
	nodeArgs.ImageFormatArgs = imageFormatArgs

	kubeConnectionArgs := NewDefaultKubeConnectionArgs()
	masterArgs.KubeConnectionArgs = kubeConnectionArgs
	nodeArgs.KubeConnectionArgs = kubeConnectionArgs

	certArgs := NewDefaultCertArgs()
	masterArgs.CertArgs = certArgs
	nodeArgs.CertArgs = certArgs
	kubeConnectionArgs.CertArgs = certArgs

	return masterArgs, nodeArgs, listenArg, imageFormatArgs, kubeConnectionArgs, certArgs
}

func (o AllInOneOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported for start")
	}
	if o.WriteConfigOnly {
		if len(o.MasterConfigFile) == 0 {
			return errors.New("--master-config is required if --write-config is true")
		}
		if len(o.NodeConfigFile) == 0 {
			return errors.New("--node-config is required if --write-config is true")
		}
	}

	// if we are not starting up using a config file, run the argument validation
	if o.WriteConfigOnly || ((len(o.MasterConfigFile) == 0) && (len(o.NodeConfigFile) == 0)) {
		if err := o.MasterArgs.Validate(); err != nil {
			return err
		}

		if err := o.NodeArgs.Validate(); err != nil {
			return err
		}

	}

	if len(o.MasterArgs.KubeConnectionArgs.ClientConfigLoadingRules.ExplicitPath) != 0 {
		return errors.New("all-in-one cannot start against with a remote kubernetes, start just the master instead")
	}

	return nil
}

func (o AllInOneOptions) Complete() error {
	nodeList := util.NewStringSet(strings.ToLower(o.NodeArgs.NodeName))
	// take everything toLower
	for _, s := range o.MasterArgs.NodeList {
		nodeList.Insert(strings.ToLower(s))
	}
	o.MasterArgs.NodeList = nodeList.List()

	masterAddr, err := o.MasterArgs.GetMasterAddress()
	if err != nil {
		return err
	}
	// in the all-in-one, default kubernetes URL to the master's address
	o.NodeArgs.DefaultKubernetesURL = masterAddr
	o.NodeArgs.NodeName = strings.ToLower(o.NodeArgs.NodeName)

	// in the all-in-one, default ClusterDNS to the master's address
	if host, _, err := net.SplitHostPort(masterAddr.Host); err == nil {
		if ip := net.ParseIP(host); ip != nil {
			o.NodeArgs.ClusterDNS = ip
		}
	}

	return nil
}

// StartAllInOne:
// 1.  Creates the signer certificate if needed
// 2.  Calls RunMaster
// 3.  Calls RunNode
// 4.  If only writing configs, it exits
// 5.  Waits forever
func (o AllInOneOptions) StartAllInOne() error {
	if !o.WriteConfigOnly {
		glog.Infof("Starting an OpenShift all-in-one")
	}

	// if either one of these wants to mint certs, make sure the signer is present BEFORE they start up to make sure they always share
	if o.MasterArgs.CertArgs.CreateCerts || o.NodeArgs.CertArgs.CreateCerts {
		signerOptions := &admin.CreateSignerCertOptions{
			CertFile:   admin.DefaultCertFilename(o.NodeArgs.CertArgs.CertDir, "ca"),
			KeyFile:    admin.DefaultKeyFilename(o.NodeArgs.CertArgs.CertDir, "ca"),
			SerialFile: admin.DefaultSerialFilename(o.NodeArgs.CertArgs.CertDir, "ca"),
			Name:       admin.DefaultSignerName(),
		}

		if err := signerOptions.Validate(nil); err != nil {
			return err
		}
		if _, err := signerOptions.CreateSignerCert(); err != nil {
			return err
		}
	}

	masterOptions := MasterOptions{o.MasterArgs, o.WriteConfigOnly, o.MasterConfigFile}
	if err := masterOptions.RunMaster(); err != nil {
		return err
	}

	nodeOptions := NodeOptions{o.NodeArgs, o.WriteConfigOnly, o.NodeConfigFile}
	if err := nodeOptions.RunNode(); err != nil {
		return err
	}

	if o.WriteConfigOnly {
		return nil
	}

	daemon.SdNotify("READY=1")
	select {}

	return nil
}

func startProfiler() {
	if cmdutil.Env("OPENSHIFT_PROFILE", "") == "web" {
		go func() {
			glog.Infof("Starting profiling endpoint at http://127.0.0.1:6060/debug/pprof/")
			glog.Fatal(http.ListenAndServe("127.0.0.1:6060", nil))
		}()
	}
}
