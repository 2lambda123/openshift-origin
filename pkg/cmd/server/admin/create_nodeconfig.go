package admin

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strconv"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master/ports"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	latestconfigapi "github.com/openshift/origin/pkg/cmd/server/api/latest"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

const NodeConfigCommandName = "create-node-config"

type CreateNodeConfigOptions struct {
	GetSignerCertOptions *GetSignerCertOptions

	NodeConfigDir string

	NodeName              string
	Hostnames             util.StringList
	VolumeDir             string
	NetworkContainerImage string
	AllowDisabledDocker   bool
	DNSDomain             string
	DNSIP                 string
	ListenAddr            flagtypes.Addr

	ClientCertFile  string
	ClientKeyFile   string
	ServerCertFile  string
	ServerKeyFile   string
	APIServerCAFile string
	APIServerURL    string
}

func NewCommandNodeConfig(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := NewDefaultCreateNodeConfigOptions()

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a portable client folder containing a client certificate, a client key, a server certificate authority, and a .kubeconfig file.",
		Run: func(c *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				fmt.Fprintln(c.Out(), err.Error())
				c.Help()
				return
			}

			if err := options.CreateNodeFolder(); err != nil {
				glog.Fatal(err)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()

	BindGetSignerCertOptions(options.GetSignerCertOptions, flags, "")

	flags.StringVar(&options.NodeConfigDir, "node-dir", "", "The client data directory.")

	flags.StringVar(&options.NodeName, "node", "", "The name of the node as it appears in etcd.")
	flags.Var(&options.Hostnames, "hostnames", "Every hostname or IP you want server certs to be valid for. Comma delimited list")
	flags.StringVar(&options.VolumeDir, "volume-dir", options.VolumeDir, "The volume storage directory.  This path is not relativized.")
	flags.StringVar(&options.NetworkContainerImage, "network-container-image", options.NetworkContainerImage, "The exact name of the image.  No processing is done on this argument.")
	flags.BoolVar(&options.AllowDisabledDocker, "allow-disabled-docker", options.AllowDisabledDocker, "Allow the node to start without docker being available.")
	flags.StringVar(&options.DNSDomain, "dns-domain", options.DNSDomain, "DNS domain for the cluster.")
	flags.StringVar(&options.DNSIP, "dns-ip", options.DNSIP, "DNS server IP for the cluster.")
	flags.Var(&options.ListenAddr, "listen", "The address to listen for connections on (scheme://host:port).")

	flags.StringVar(&options.ClientCertFile, "client-certificate", "", "The client cert file.")
	flags.StringVar(&options.ClientKeyFile, "client-key", "", "The client key file.")
	flags.StringVar(&options.ServerCertFile, "server-certificate", "", "The server cert file for serving secure traffic.")
	flags.StringVar(&options.ServerKeyFile, "server-key", "", "The server key file for serving secure traffic.")
	flags.StringVar(&options.APIServerURL, "master", options.APIServerURL, "The API server's URL.")
	flags.StringVar(&options.APIServerCAFile, "certificate-authority", options.APIServerCAFile, "Path to the API server's CA file.")

	return cmd
}

func NewDefaultCreateNodeConfigOptions() *CreateNodeConfigOptions {
	options := &CreateNodeConfigOptions{GetSignerCertOptions: &GetSignerCertOptions{}}
	options.VolumeDir = "openshift.local.volumes"
	options.DNSDomain = "local"
	options.APIServerURL = "https://localhost:8443"
	options.APIServerCAFile = "openshift.local.certificates/ca/cert.crt"

	imageTemplate := variable.NewDefaultImageTemplate()
	options.NetworkContainerImage = imageTemplate.ExpandOrDie("pod")

	options.ListenAddr = flagtypes.Addr{Value: "0.0.0.0:10250", DefaultScheme: "http", DefaultPort: 10250, AllowPrefix: true}.Default()

	return options
}

func (o CreateNodeConfigOptions) IsCreateClientCertificate() bool {
	return len(o.ClientCertFile) == 0 && len(o.ClientKeyFile) == 0
}

func (o CreateNodeConfigOptions) IsCreateServerCertificate() bool {
	return len(o.ServerCertFile) == 0 && len(o.ServerKeyFile) == 0 && o.UseTLS()
}

func (o CreateNodeConfigOptions) UseTLS() bool {
	return o.ListenAddr.URL.Scheme == "https"
}

func (o CreateNodeConfigOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.NodeConfigDir) == 0 {
		return errors.New("node-dir must be provided")
	}
	if len(o.NodeName) == 0 {
		return errors.New("node must be provided")
	}
	if len(o.APIServerURL) == 0 {
		return errors.New("master must be provided")
	}
	if len(o.APIServerCAFile) == 0 {
		return errors.New("certificate-authority must be provided")
	}
	if len(o.Hostnames) == 0 {
		return errors.New("at least one hostname must be provided")
	}

	if len(o.ClientCertFile) != 0 {
		if len(o.ClientKeyFile) == 0 {
			return errors.New("client-key must be provided if client-certificate is provided")
		}
	} else if len(o.ClientKeyFile) != 0 {
		return errors.New("client-certificate must be provided if client-key is provided")
	}

	if len(o.ServerCertFile) != 0 {
		if len(o.ServerKeyFile) == 0 {
			return errors.New("server-key must be provided if server-certificate is provided")
		}
	} else if len(o.ServerKeyFile) != 0 {
		return errors.New("server-certificate must be provided if server-key is provided")
	}

	if o.IsCreateClientCertificate() || o.IsCreateServerCertificate() {
		if len(o.GetSignerCertOptions.KeyFile) == 0 {
			return errors.New("signer-key must be provided to create certificates")
		}
		if len(o.GetSignerCertOptions.CertFile) == 0 {
			return errors.New("signer-cert must be provided to create certificates")
		}
		if len(o.GetSignerCertOptions.SerialFile) == 0 {
			return errors.New("signer-serial must be provided to create certificates")
		}
	}

	return nil
}

func CopyFile(src, dest string, permissions os.FileMode) error {
	// copy the cert and key over
	if content, err := ioutil.ReadFile(src); err != nil {
		return err
	} else if err := ioutil.WriteFile(dest, content, permissions); err != nil {
		return err
	}

	return nil
}

func (o CreateNodeConfigOptions) CreateNodeFolder() error {
	clientCertFile := path.Join(o.NodeConfigDir, "client.crt")
	clientKeyFile := path.Join(o.NodeConfigDir, "client.key")
	serverCertFile := path.Join(o.NodeConfigDir, "server.crt")
	serverKeyFile := path.Join(o.NodeConfigDir, "server.key")
	clientCopyOfCAFile := path.Join(o.NodeConfigDir, "ca.crt")
	kubeConfigFile := path.Join(o.NodeConfigDir, ".kubeconfig")
	nodeConfigFile := path.Join(o.NodeConfigDir, "node-config.yaml")
	nodeJSONFile := path.Join(o.NodeConfigDir, "node-registration.json")

	if err := o.MakeClientCert(clientCertFile, clientKeyFile); err != nil {
		return err
	}
	if o.UseTLS() {
		if err := o.MakeServerCert(serverCertFile, serverKeyFile); err != nil {
			return err
		}
	}
	if err := o.MakeCA(clientCopyOfCAFile); err != nil {
		return err
	}
	if err := o.MakeKubeConfig(clientCertFile, clientKeyFile, clientCopyOfCAFile, kubeConfigFile); err != nil {
		return err
	}
	if err := o.MakeNodeConfig(serverCertFile, serverKeyFile, kubeConfigFile, nodeConfigFile); err != nil {
		return err
	}
	if err := o.MakeNodeJSON(nodeJSONFile); err != nil {
		return err
	}

	return nil
}

func (o CreateNodeConfigOptions) MakeClientCert(clientCertFile, clientKeyFile string) error {
	if o.IsCreateClientCertificate() {
		createNodeClientCert := CreateClientCertOptions{
			GetSignerCertOptions: o.GetSignerCertOptions,

			CertFile: clientCertFile,
			KeyFile:  clientKeyFile,

			User:   "system:node-" + o.NodeName,
			Groups: util.StringList([]string{bootstrappolicy.NodesGroup}),
		}

		if err := createNodeClientCert.Validate(nil); err != nil {
			return err
		}
		if _, err := createNodeClientCert.CreateClientCert(); err != nil {
			return err
		}

	} else {
		if err := CopyFile(o.ClientCertFile, clientCertFile, 0644); err != nil {
			return err
		}
		if err := CopyFile(o.ClientKeyFile, clientKeyFile, 0600); err != nil {
			return err
		}
	}

	return nil
}

func (o CreateNodeConfigOptions) MakeServerCert(serverCertFile, serverKeyFile string) error {
	if o.IsCreateServerCertificate() {
		nodeServerCertOptions := CreateServerCertOptions{
			GetSignerCertOptions: o.GetSignerCertOptions,

			CertFile: serverCertFile,
			KeyFile:  serverKeyFile,

			Hostnames: o.Hostnames,
		}

		if err := nodeServerCertOptions.Validate(nil); err != nil {
			return err
		}
		if _, err := nodeServerCertOptions.CreateServerCert(); err != nil {
			return err
		}

	} else {
		if err := CopyFile(o.ServerCertFile, serverCertFile, 0644); err != nil {
			return err
		}
		if err := CopyFile(o.ServerKeyFile, serverKeyFile, 0600); err != nil {
			return err
		}
	}

	return nil
}

func (o CreateNodeConfigOptions) MakeCA(clientCopyOfCAFile string) error {
	if err := CopyFile(o.APIServerCAFile, clientCopyOfCAFile, 0644); err != nil {
		return err
	}

	return nil
}

func (o CreateNodeConfigOptions) MakeKubeConfig(clientCertFile, clientKeyFile, clientCopyOfCAFile, kubeConfigFile string) error {
	createKubeConfigOptions := CreateKubeConfigOptions{
		APIServerURL:    o.APIServerURL,
		APIServerCAFile: clientCopyOfCAFile,
		ServerNick:      "master",

		CertFile: clientCertFile,
		KeyFile:  clientKeyFile,
		UserNick: "node",

		KubeConfigFile: kubeConfigFile,
	}
	if err := createKubeConfigOptions.Validate(nil); err != nil {
		return err
	}
	if _, err := createKubeConfigOptions.CreateKubeConfig(); err != nil {
		return err
	}

	return nil
}

func (o CreateNodeConfigOptions) MakeNodeConfig(serverCertFile, serverKeyFile, kubeConfigFile, nodeConfigFile string) error {
	config := &configapi.NodeConfig{
		NodeName: o.NodeName,

		ServingInfo: configapi.ServingInfo{
			BindAddress: net.JoinHostPort(o.ListenAddr.Host, strconv.Itoa(ports.KubeletPort)),
		},

		VolumeDirectory:       o.VolumeDir,
		NetworkContainerImage: o.NetworkContainerImage,
		AllowDisabledDocker:   o.AllowDisabledDocker,

		DNSDomain: o.DNSDomain,
		DNSIP:     o.DNSIP,

		MasterKubeConfig: kubeConfigFile,
	}

	if o.UseTLS() {
		config.ServingInfo.ServerCert = configapi.CertInfo{
			CertFile: serverCertFile,
			KeyFile:  serverKeyFile,
		}
	}

	// Resolve relative to CWD
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := configapi.ResolveNodeConfigPaths(config, cwd); err != nil {
		return err
	}

	// Relativize to config file dir
	base, err := cmdutil.MakeAbs(o.NodeConfigDir, cwd)
	if err != nil {
		return err
	}
	if err := configapi.RelativizeNodeConfigPaths(config, base); err != nil {
		return err
	}

	content, err := latestconfigapi.WriteNode(config)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(nodeConfigFile, content, 0644); err != nil {
		return err
	}

	return nil
}

func (o CreateNodeConfigOptions) MakeNodeJSON(nodeJSONFile string) error {
	node := &kapi.Node{}
	node.Name = o.NodeName

	json, err := klatest.Codec.Encode(node)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(nodeJSONFile, json, 0644); err != nil {
		return err
	}

	return nil
}
