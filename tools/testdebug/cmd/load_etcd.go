package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	coreosetcdclient "github.com/coreos/etcd/client"
	etcdclient "github.com/coreos/etcd/client"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"

	knet "k8s.io/apimachinery/pkg/util/net"
	etcdutil "k8s.io/apiserver/pkg/storage/etcd/util"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	restclient "k8s.io/client-go/rest"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/etcd/etcdserver"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/openshift/origin/pkg/cmd/server/start"
	"github.com/openshift/origin/pkg/oc/cli/admin/policy"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

const RecommendedLoadEtcdDumpName = "start-api"

type DebugAPIServerOptions struct {
	Out io.Writer

	EtcdDumpFile string
	AllowAll     bool
}

func NewDebugAPIServerCommand(stopCh <-chan struct{}) *cobra.Command {
	o := &DebugAPIServerOptions{Out: os.Stdout}

	cmd := &cobra.Command{
		Use:   RecommendedLoadEtcdDumpName + " etcd_dump.json",
		Short: "Start API server using etcddump",
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(args))

			kcmdutil.CheckErr(o.Run(stopCh))
		},
	}

	cmd.Flags().BoolVar(&o.AllowAll, "allow-all", true, "change policy to grant system:authenticated cluster-admin powers")

	flagtypes.GLog(cmd.PersistentFlags())

	return cmd
}

func (o *DebugAPIServerOptions) Complete(args []string) error {
	if len(args) != 1 {
		return errors.New("etcd_dump.json file is required")
	}

	o.EtcdDumpFile = args[0]

	return nil
}

func (o *DebugAPIServerOptions) Run(stopCh <-chan struct{}) error {
	masterConfig, err := testserver.DefaultMasterOptionsWithTweaks(true /*use default ports*/)
	if err != nil {
		return err
	}

	etcdConfig := masterConfig.EtcdConfig
	masterConfig.EtcdConfig = nil
	masterConfig.DNSConfig = nil

	etcdserver.RunEtcd(etcdConfig)

	if err := o.ImportEtcdDump(masterConfig.EtcdClientInfo); err != nil {
		return err
	}

	shutdownCh, err := o.StartAPIServer(*masterConfig, stopCh)
	if err != nil {
		return err
	}

	if o.AllowAll {
		clientConfig, err := testutil.GetClusterAdminClientConfig(testutil.GetBaseDir() + "/openshift.local.config/master/admin.kubeconfig")
		if err != nil {
			return err
		}

		addClusterAdmin := &policy.RoleModificationOptions{
			RoleName:   bootstrappolicy.ClusterAdminRoleName,
			RoleKind:   "ClusterRole",
			RbacClient: rbacv1client.NewForConfigOrDie(clientConfig),
			Groups:     []string{"system:authenticated"},
		}
		if err := addClusterAdmin.AddRole(); err != nil {
			return err
		}
	}

	select {
	case <-shutdownCh:
	}

	return nil
}

func (o *DebugAPIServerOptions) StartAPIServer(masterConfig configapi.MasterConfig, stopCh <-chan struct{}) (<-chan struct{}, error) {
	informers := origin.InformerAccess(nil)
	openshiftConfig, err := origin.BuildMasterConfig(masterConfig, informers)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Starting master on %s\n", masterConfig.ServingInfo.BindAddress)
	fmt.Printf("Public master address is %s\n", masterConfig.MasterPublicURL)
	return start.StartAPI(openshiftConfig, stopCh)
}

// getAndTestEtcdClient creates an etcd client based on the provided config. It will attempt to
// connect to the etcd server and block until the server responds at least once, or return an
// error if the server never responded.
func getAndTestEtcdClient(etcdClientInfo configapi.EtcdConnectionInfo) (etcdclient.Client, error) {
	etcdClient, err := makeEtcdClient(etcdClientInfo)
	if err != nil {
		return nil, err
	}
	if err := testEtcdClient(etcdClient); err != nil {
		return nil, err
	}
	return etcdClient, nil
}

func (o *DebugAPIServerOptions) ImportEtcdDump(etcdClientInfo configapi.EtcdConnectionInfo) error {
	infile, err := os.Open(o.EtcdDumpFile)
	if err != nil {
		return err
	}
	etcdDump := &coreosetcdclient.Response{}
	if err := json.NewDecoder(infile).Decode(etcdDump); err != nil {
		return err
	}

	// Connect and setup etcd interfaces
	etcdClient, err := getAndTestEtcdClient(etcdClientInfo)
	if err != nil {
		return err
	}
	etcdKeyClient := coreosetcdclient.NewKeysAPI(etcdClient)

	nodeList := []*coreosetcdclient.Node{}
	nodeList = append(nodeList, etcdDump.Node)
	for i := 0; i < len(nodeList); i++ {
		node := nodeList[i]
		if node == nil {
			continue
		}

		for j := range node.Nodes {
			nodeList = append(nodeList, node.Nodes[j])
		}
		if len(node.Key) == 0 {
			continue
		}

		if node.Dir {
			if _, err := etcdKeyClient.Create(context.TODO(), node.Key, ""); err != nil {
				return err
			}
			continue
		}

		if _, err := etcdKeyClient.Create(context.TODO(), node.Key, node.Value); err != nil {
			return err
		}
	}

	return nil
}

// makeEtcdClient creates an etcd client based on the provided config.
func makeEtcdClient(etcdClientInfo configapi.EtcdConnectionInfo) (etcdclient.Client, error) {
	tlsConfig, err := restclient.TLSConfigFor(&restclient.Config{
		TLSClientConfig: restclient.TLSClientConfig{
			CertFile: etcdClientInfo.ClientCert.CertFile,
			KeyFile:  etcdClientInfo.ClientCert.KeyFile,
			CAFile:   etcdClientInfo.CA,
		},
	})
	if err != nil {
		return nil, err
	}

	transport := knet.SetTransportDefaults(&http.Transport{
		TLSClientConfig: tlsConfig,
		Dial: (&net.Dialer{
			// default from http.DefaultTransport
			Timeout: 30 * time.Second,
			// Lower the keep alive for connections.
			KeepAlive: 1 * time.Second,
		}).Dial,
		// Because watches are very bursty, defends against long delays in watch reconnections.
		MaxIdleConnsPerHost: 500,
	})

	cfg := etcdclient.Config{
		Endpoints: etcdClientInfo.URLs,
		// TODO: Determine if transport needs optimization
		Transport: transport,
	}
	return etcdclient.New(cfg)
}

// testEtcdClient verifies a client is functional.  It will attempt to
// connect to the etcd server and block until the server responds at least once, or return an
// error if the server never responded.
func testEtcdClient(etcdClient etcdclient.Client) error {
	for i := 0; ; i++ {
		_, err := etcdclient.NewKeysAPI(etcdClient).Get(context.Background(), "/", nil)
		if err == nil || etcdutil.IsEtcdNotFound(err) {
			break
		}
		if i > 100 {
			return fmt.Errorf("could not reach etcd(v2): %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}
