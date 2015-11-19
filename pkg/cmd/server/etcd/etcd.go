package etcd

import (
	"fmt"
	"net"
	"net/http"
	"time"

	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"

	client "k8s.io/kubernetes/pkg/client/unversioned"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

// RunEtcd starts an etcd server and runs it forever
func RunEtcd(etcdServerConfig *configapi.EtcdConfig) {
	cfg := &config{
		name: defaultName,
		dir:  etcdServerConfig.StorageDir,

		TickMs:       100,
		ElectionMs:   1000,
		maxSnapFiles: 5,
		maxWalFiles:  5,

		initialClusterToken: "etcd-cluster",
	}
	var err error
	if configapi.UseTLS(etcdServerConfig.ServingInfo) {
		cfg.clientTLSInfo.CAFile = etcdServerConfig.ServingInfo.ClientCA
		cfg.clientTLSInfo.CertFile = etcdServerConfig.ServingInfo.ServerCert.CertFile
		cfg.clientTLSInfo.KeyFile = etcdServerConfig.ServingInfo.ServerCert.KeyFile
	}
	if cfg.lcurls, err = urlsFromStrings(etcdServerConfig.ServingInfo.BindAddress, cfg.clientTLSInfo); err != nil {
		glog.Fatalf("Unable to build etcd client URLs: %v", err)
	}

	if configapi.UseTLS(etcdServerConfig.PeerServingInfo) {
		cfg.peerTLSInfo.CAFile = etcdServerConfig.PeerServingInfo.ClientCA
		cfg.peerTLSInfo.CertFile = etcdServerConfig.PeerServingInfo.ServerCert.CertFile
		cfg.peerTLSInfo.KeyFile = etcdServerConfig.PeerServingInfo.ServerCert.KeyFile
	}
	if cfg.lpurls, err = urlsFromStrings(etcdServerConfig.PeerServingInfo.BindAddress, cfg.peerTLSInfo); err != nil {
		glog.Fatalf("Unable to build etcd peer URLs: %v", err)
	}

	if cfg.acurls, err = urlsFromStrings(etcdServerConfig.Address, cfg.clientTLSInfo); err != nil {
		glog.Fatalf("Unable to build etcd announce client URLs: %v", err)
	}
	if cfg.apurls, err = urlsFromStrings(etcdServerConfig.PeerAddress, cfg.peerTLSInfo); err != nil {
		glog.Fatalf("Unable to build etcd announce peer URLs: %v", err)
	}

	if err := cfg.resolveUrls(); err != nil {
		glog.Fatalf("Unable to resolve etcd URLs: %v", err)
	}

	cfg.initialCluster = fmt.Sprintf("%s=%s", cfg.name, cfg.apurls[0].String())

	stopped, err := startEtcd(cfg)
	if err != nil {
		glog.Fatalf("Unable to start etcd: %v", err)
	}
	go func() {
		glog.Infof("Started etcd at %s", etcdServerConfig.Address)
		<-stopped
	}()
}

// GetAndTestEtcdClient creates an etcd client based on the provided config. It will attempt to
// connect to the etcd server and block until the server responds at least once, or return an
// error if the server never responded.
func GetAndTestEtcdClient(etcdClientInfo configapi.EtcdConnectionInfo) (*etcdclient.Client, error) {
	etcdClient, err := EtcdClient(etcdClientInfo)
	if err != nil {
		return nil, err
	}
	if err := TestEtcdClient(etcdClient); err != nil {
		return nil, err
	}
	return etcdClient, nil
}

// EtcdClient creates an etcd client based on the provided config.
func EtcdClient(etcdClientInfo configapi.EtcdConnectionInfo) (*etcdclient.Client, error) {
	tlsConfig, err := client.TLSConfigFor(&client.Config{
		TLSClientConfig: client.TLSClientConfig{
			CertFile: etcdClientInfo.ClientCert.CertFile,
			KeyFile:  etcdClientInfo.ClientCert.KeyFile,
			CAFile:   etcdClientInfo.CA,
		},
	})
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		Dial: (&net.Dialer{
			// default from http.DefaultTransport
			Timeout: 30 * time.Second,
			// Lower the keep alive for connections.
			KeepAlive: 1 * time.Second,
		}).Dial,
		// Because watches are very bursty, defends against long delays in watch reconnections.
		MaxIdleConnsPerHost: 500,
		// defaults from http.DefaultTransport
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	etcdClient := etcdclient.NewClient(etcdClientInfo.URLs)
	etcdClient.SetTransport(transport)
	return etcdClient, nil
}

// TestEtcdClient verifies a client is functional.  It will attempt to
// connect to the etcd server and block until the server responds at least once, or return an
// error if the server never responded.
func TestEtcdClient(etcdClient *etcdclient.Client) error {
	for i := 0; ; i++ {
		_, err := etcdClient.Get("/", false, false)
		if err == nil || etcdstorage.IsEtcdNotFound(err) {
			break
		}
		if i > 100 {
			return fmt.Errorf("could not reach etcd: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}
