package kubernetes

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet"
	kconfig "github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/config"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/dockertools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/proxy"
	pconfig "github.com/GoogleCloudPlatform/kubernetes/pkg/proxy/config"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	kexec "github.com/GoogleCloudPlatform/kubernetes/pkg/util/exec"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/iptables"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	cadvisor "github.com/google/cadvisor/client"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"
	"github.com/openshift/origin/pkg/kubelet/app"
	"github.com/openshift/origin/pkg/service"
)

// NodeScheme is the default scheme for serving information about the node.
const NodeScheme = "http"

// NodePort is the default Kubelet port for serving information about the node.
const NodePort = 10250

type commandExecutor interface {
	LookPath(executable string) (string, error)
	Run(command string, args ...string) error
}

type defaultCommandExecutor struct{}

func (ce defaultCommandExecutor) LookPath(executable string) (string, error) {
	return exec.LookPath(executable)
}

func (ce defaultCommandExecutor) Run(command string, args ...string) error {
	c := exec.Command(command, args...)
	return c.Run()
}

// EnsureDocker attempts to connect to the Docker daemon defined by the helper,
// and if it is unable to it will print a warning.
func (c *NodeConfig) EnsureDocker(docker *dockerutil.Helper) {
	dockerClient, dockerAddr := docker.GetClientOrExit()
	if err := dockerClient.Ping(); err != nil {
		if !c.AllowDisabledDocker {
			glog.Fatalf("ERROR: Docker could not be reached at %s.  Docker must be installed and running to start containers.\n%v", dockerAddr, err)
		}
		glog.Errorf("WARNING: Docker could not be reached at %s.  Docker must be installed and running to start containers.\n%v", dockerAddr, err)
		c.DockerClient = &dockertools.FakeDockerClient{VersionInfo: dockerclient.Env{"apiversion=1.15"}}
	} else {
		glog.Infof("Connecting to Docker at %s", dockerAddr)
		c.DockerClient = dockerClient
	}
}

// EnsureVolumeDir attempts to convert the provided volume directory argument to
// an absolute path and create the directory if it does not exist. Will exit if
// an error is encountered.
func (c *NodeConfig) EnsureVolumeDir() {
	if volumeDir, err := c.initializeVolumeDir(&defaultCommandExecutor{}, c.VolumeDir); err != nil {
		glog.Fatal(err)
	} else {
		c.VolumeDir = volumeDir
	}
}

func (c *NodeConfig) initializeVolumeDir(ce commandExecutor, path string) (string, error) {
	rootDirectory, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("Error converting volume directory to an absolute path: %v", err)
	}

	if _, err := os.Stat(rootDirectory); os.IsNotExist(err) {
		if err := os.MkdirAll(rootDirectory, 0750); err != nil {
			return "", fmt.Errorf("Couldn't create kubelet volume root directory '%s': %s", rootDirectory, err)
		}
		if chconPath, err := ce.LookPath("chcon"); err != nil {
			glog.V(2).Infof("Couldn't locate 'chcon' to set the kubelet volume root directory SELinux context: %s", err)
		} else {
			if err := ce.Run(chconPath, "-t", "svirt_sandbox_file_t", rootDirectory); err != nil {
				glog.Warningf("Error running 'chcon' to set the kubelet volume root directory SELinux context: %s", err)
			}
		}
	}
	return rootDirectory, nil
}

// RunKubelet starts the Kubelet.
func (c *NodeConfig) RunKubelet() {
	// TODO: clean this up and make it more formal (service named 'dns'?). Use multiple ports.
	clusterDNS := c.ClusterDNS
	if clusterDNS == nil {
		if service, err := c.Client.Endpoints(kapi.NamespaceDefault).Get("kubernetes"); err == nil && len(service.Endpoints) > 0 {
			firstIP := service.Endpoints[0].IP
			if err := cmdutil.WaitForSuccessfulDial(false, "tcp", fmt.Sprintf("%s:%d", firstIP, 53), 50*time.Millisecond, 0, 2); err == nil {
				clusterDNS = net.ParseIP(firstIP)
			}
		}
	}

	// initialize Kubelet
	// Allow privileged containers
	// TODO: make this configurable and not the default https://github.com/openshift/origin/issues/662
	kubelet.SetupCapabilities(true)
	cfg := kconfig.NewPodConfig(kconfig.PodConfigNotificationSnapshotAndUpdates)
	kconfig.NewSourceApiserver(c.Client, c.NodeHost, cfg.Channel("api"))
	k, err := kubelet.NewMainKubelet(
		c.NodeHost,
		c.DockerClient,
		nil,
		c.Client,
		c.VolumeDir,
		c.NetworkContainerImage,
		30*time.Second,
		0.0,
		10,
		1*time.Second,
		5,
		cfg.IsSourceSeen,
		c.ClusterDomain,
		clusterDNS,
		kapi.NamespaceDefault,
		app.ProbeVolumePlugins(),
		5*time.Minute)
	if err != nil {
		glog.Fatalf("Couldn't run kubelet: %s", err)
	}
	go util.Forever(func() { k.Run(cfg.Updates()) }, 0)

	handler := kubelet.NewServer(k, true)

	server := &http.Server{
		Addr:           c.BindAddress,
		Handler:        &handler,
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	go util.Forever(func() {
		glog.Infof("Started Kubelet for node %s, server at %s", c.NodeHost, c.BindAddress)
		if clusterDNS != nil {
			glog.Infof("  Kubelet is setting %s as a DNS nameserver for domain %q", clusterDNS, c.ClusterDomain)
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
			glog.Fatal(server.ListenAndServeTLS(c.KubeletCertFile, c.KubeletKeyFile))
		} else {
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)

	go func() {
		defer util.HandleCrash()
		// TODO: Monitor this connection, reconnect if needed?
		glog.V(1).Infof("Trying to create cadvisor client.")
		// cAdvisor should be running on the local machine
		cadvisorClient, err := cadvisor.NewClient("http://" + c.NodeHost + ":4194")
		if err != nil {
			glog.Errorf("Error on creating cadvisor client: %v", err)
			return
		}
		glog.V(1).Infof("Successfully created cadvisor client.")
		// this binds the cadvisor to the kubelet for later reference
		k.SetCadvisorClient(cadvisorClient)
	}()

}

// RunProxy starts the proxy
func (c *NodeConfig) RunProxy() {
	// initialize kube proxy
	serviceConfig := pconfig.NewServiceConfig()
	endpointsConfig := pconfig.NewEndpointsConfig()
	pconfig.NewSourceAPI(
		c.Client.Services(kapi.NamespaceAll),
		c.Client.Endpoints(kapi.NamespaceAll),
		30*time.Second,
		serviceConfig.Channel("api"),
		endpointsConfig.Channel("api"))
	loadBalancer := proxy.NewLoadBalancerRR()
	endpointsConfig.RegisterHandler(loadBalancer)

	host, _, err := net.SplitHostPort(c.BindAddress)
	if err != nil {
		glog.Fatalf("The provided value to bind to must be an ip:port %q", c.BindAddress)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		glog.Fatalf("The provided value to bind to must be an ip:port: %q", c.BindAddress)
	}

	protocol := iptables.ProtocolIpv4
	if ip.To4() == nil {
		protocol = iptables.ProtocolIpv6
	}

	var proxier pconfig.ServiceConfigHandler
	proxier = proxy.NewProxier(loadBalancer, ip, iptables.New(kexec.New(), protocol))
	if proxier == nil || reflect.ValueOf(proxier).IsNil() { // explicitly declared interfaces aren't plain nil, you must reflect inside to see if it's really nil or not
		glog.Errorf("WARNING: Could not modify iptables.  iptables must be mutable by this process to use services.  Do you have root permissions?")
		proxier = &service.FailingServiceConfigProxy{}
	}
	serviceConfig.RegisterHandler(proxier)

	glog.Infof("Started Kubernetes Proxy on %s", host)
}
