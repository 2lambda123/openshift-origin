/*
Copyright 2014 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package app does all of the work necessary to create a Kubernetes
// APIServer by binding together the API, master and APIServer infrastructure.
// It can be configured and called directly or via the hyperkube framework.
package app

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/capabilities"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	forked "github.com/GoogleCloudPlatform/kubernetes/third_party/forked/coreos/go-etcd/etcd"

	"github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"
	"github.com/spf13/pflag"
)

const (
	// Maximum duration before timing out read/write requests
	// Set to a value larger than the timeouts in each watch server.
	ReadWriteTimeout = time.Minute * 60
)

// APIServer runs a kubernetes api server.
type APIServer struct {
	InsecureBindAddress        util.IP
	InsecurePort               int
	BindAddress                util.IP
	ReadOnlyPort               int
	SecurePort                 int
	ExternalHost               string
	APIRate                    float32
	APIBurst                   int
	TLSCertFile                string
	TLSPrivateKeyFile          string
	CertDirectory              string
	APIPrefix                  string
	StorageVersion             string
	CloudProvider              string
	CloudConfigFile            string
	EventTTL                   time.Duration
	BasicAuthFile              string
	ClientCAFile               string
	TokenAuthFile              string
	ServiceAccountKeyFile      string
	ServiceAccountLookup       bool
	AuthorizationMode          string
	AuthorizationPolicyFile    string
	AdmissionControl           string
	AdmissionControlConfigFile string
	EtcdServerList             util.StringList
	EtcdConfigFile             string
	EtcdPathPrefix             string
	OldEtcdPathPrefix          string
	CorsAllowedOriginList      util.StringList
	AllowPrivileged            bool
	PortalNet                  util.IPNet // TODO: make this a list
	ServiceNodePorts           util.PortRange
	EnableLogsSupport          bool
	MasterServiceNamespace     string
	RuntimeConfig              util.ConfigurationMap
	KubeletConfig              client.KubeletConfig
	ClusterName                string
	EnableProfiling            bool
	MaxRequestsInFlight        int
	LongRunningRequestRE       string
}

// NewAPIServer creates a new APIServer object with default parameters
func NewAPIServer() *APIServer {
	s := APIServer{
		InsecurePort:           8080,
		InsecureBindAddress:    util.IP(net.ParseIP("127.0.0.1")),
		BindAddress:            util.IP(net.ParseIP("0.0.0.0")),
		ReadOnlyPort:           7080,
		SecurePort:             6443,
		APIRate:                10.0,
		APIBurst:               200,
		APIPrefix:              "/api",
		EventTTL:               1 * time.Hour,
		AuthorizationMode:      "AlwaysAllow",
		AdmissionControl:       "AlwaysAdmit",
		EtcdPathPrefix:         master.DefaultEtcdPathPrefix,
		OldEtcdPathPrefix:      master.DefaultEtcdPathPrefix,
		EnableLogsSupport:      true,
		MasterServiceNamespace: api.NamespaceDefault,
		ClusterName:            "kubernetes",
		CertDirectory:          "/var/run/kubernetes",

		RuntimeConfig: make(util.ConfigurationMap),
		KubeletConfig: client.KubeletConfig{
			Port:        10250,
			EnableHttps: true,
			HTTPTimeout: time.Duration(5) * time.Second,
		},
	}

	return &s
}

// AddFlags adds flags for a specific APIServer to the specified FlagSet
func (s *APIServer) AddFlags(fs *pflag.FlagSet) {
	// Note: the weird ""+ in below lines seems to be the only way to get gofmt to
	// arrange these text blocks sensibly. Grrr.
	fs.IntVar(&s.InsecurePort, "insecure-port", s.InsecurePort, ""+
		"The port on which to serve unsecured, unauthenticated access. Default 8080. It is assumed "+
		"that firewall rules are set up such that this port is not reachable from outside of "+
		"the cluster and that port 443 on the cluster's public address is proxied to this "+
		"port. This is performed by nginx in the default setup.")
	fs.IntVar(&s.InsecurePort, "port", s.InsecurePort, "DEPRECATED: see --insecure-port instead")
	fs.Var(&s.InsecureBindAddress, "insecure-bind-address", ""+
		"The IP address on which to serve the --insecure-port (set to 0.0.0.0 for all interfaces). "+
		"Defaults to localhost.")
	fs.Var(&s.InsecureBindAddress, "address", "DEPRECATED: see --insecure-bind-address instead")
	fs.Var(&s.BindAddress, "bind-address", ""+
		"The IP address on which to serve the --read-only-port and --secure-port ports. This "+
		"address must be reachable by the rest of the cluster. If blank, all interfaces will be used.")
	fs.Var(&s.BindAddress, "public-address-override", "DEPRECATED: see --bind-address instead")
	fs.IntVar(&s.ReadOnlyPort, "read-only-port", s.ReadOnlyPort, ""+
		"The port on which to serve read-only resources. If 0, don't serve read-only "+
		"at all. It is assumed that firewall rules are set up such that this port is "+
		"not reachable from outside of the cluster.")
	fs.IntVar(&s.SecurePort, "secure-port", s.SecurePort, ""+
		"The port on which to serve HTTPS with authentication and authorization. If 0, "+
		"don't serve HTTPS at all.")
	fs.Float32Var(&s.APIRate, "api-rate", s.APIRate, "API rate limit as QPS for the read only port")
	fs.IntVar(&s.APIBurst, "api-burst", s.APIBurst, "API burst amount for the read only port")
	fs.StringVar(&s.TLSCertFile, "tls-cert-file", s.TLSCertFile, ""+
		"File containing x509 Certificate for HTTPS.  (CA cert, if any, concatenated after server cert). "+
		"If HTTPS serving is enabled, and --tls-cert-file and --tls-private-key-file are not provided, "+
		"a self-signed certificate and key are generated for the public address and saved to /var/run/kubernetes.")
	fs.StringVar(&s.TLSPrivateKeyFile, "tls-private-key-file", s.TLSPrivateKeyFile, "File containing x509 private key matching --tls-cert-file.")
	fs.StringVar(&s.CertDirectory, "cert-dir", s.CertDirectory, "The directory where the TLS certs are located (by default /var/run/kubernetes). "+
		"If --tls-cert-file and --tls-private-key-file are provided, this flag will be ignored.")
	fs.StringVar(&s.APIPrefix, "api-prefix", s.APIPrefix, "The prefix for API requests on the server. Default '/api'.")
	fs.StringVar(&s.StorageVersion, "storage-version", s.StorageVersion, "The version to store resources with. Defaults to server preferred")
	fs.StringVar(&s.CloudProvider, "cloud-provider", s.CloudProvider, "The provider for cloud services.  Empty string for no provider.")
	fs.StringVar(&s.CloudConfigFile, "cloud-config", s.CloudConfigFile, "The path to the cloud provider configuration file.  Empty string for no configuration file.")
	fs.DurationVar(&s.EventTTL, "event-ttl", s.EventTTL, "Amount of time to retain events. Default 1 hour.")
	fs.StringVar(&s.BasicAuthFile, "basic-auth-file", s.BasicAuthFile, "If set, the file that will be used to admit requests to the secure port of the API server via http basic authentication.")
	fs.StringVar(&s.ClientCAFile, "client-ca-file", s.ClientCAFile, "If set, any request presenting a client certificate signed by one of the authorities in the client-ca-file is authenticated with an identity corresponding to the CommonName of the client certificate.")
	fs.StringVar(&s.TokenAuthFile, "token-auth-file", s.TokenAuthFile, "If set, the file that will be used to secure the secure port of the API server via token authentication.")
	fs.StringVar(&s.ServiceAccountKeyFile, "service-account-key-file", s.ServiceAccountKeyFile, "File containing PEM-encoded x509 RSA private or public key, used to verify ServiceAccount tokens. If unspecified, --tls-private-key-file is used.")
	fs.BoolVar(&s.ServiceAccountLookup, "service-account-lookup", s.ServiceAccountLookup, "If true, validate ServiceAccount tokens exist in etcd as part of authentication.")
	fs.StringVar(&s.AuthorizationMode, "authorization-mode", s.AuthorizationMode, "Selects how to do authorization on the secure port.  One of: "+strings.Join(apiserver.AuthorizationModeChoices, ","))
	fs.StringVar(&s.AuthorizationPolicyFile, "authorization-policy-file", s.AuthorizationPolicyFile, "File with authorization policy in csv format, used with --authorization-mode=ABAC, on the secure port.")
	fs.StringVar(&s.AdmissionControl, "admission-control", s.AdmissionControl, "Ordered list of plug-ins to do admission control of resources into cluster. Comma-delimited list of: "+strings.Join(admission.GetPlugins(), ", "))
	fs.StringVar(&s.AdmissionControlConfigFile, "admission-control-config-file", s.AdmissionControlConfigFile, "File with admission control configuration.")
	fs.Var(&s.EtcdServerList, "etcd-servers", "List of etcd servers to watch (http://ip:port), comma separated. Mutually exclusive with -etcd-config")
	fs.StringVar(&s.EtcdConfigFile, "etcd-config", s.EtcdConfigFile, "The config file for the etcd client. Mutually exclusive with -etcd-servers.")
	fs.StringVar(&s.EtcdPathPrefix, "etcd-prefix", s.EtcdPathPrefix, "The prefix for all resource paths in etcd.")
	fs.StringVar(&s.OldEtcdPathPrefix, "old-etcd-prefix", s.OldEtcdPathPrefix, "The previous prefix for all resource paths in etcd, if any.")
	fs.Var(&s.CorsAllowedOriginList, "cors-allowed-origins", "List of allowed origins for CORS, comma separated.  An allowed origin can be a regular expression to support subdomain matching.  If this list is empty CORS will not be enabled.")
	fs.BoolVar(&s.AllowPrivileged, "allow-privileged", s.AllowPrivileged, "If true, allow privileged containers.")
	fs.Var(&s.PortalNet, "portal-net", "A CIDR notation IP range from which to assign portal IPs. This must not overlap with any IP ranges assigned to nodes for pods.")
	fs.Var(&s.ServiceNodePorts, "service-node-ports", "A port range to reserve for services with NodePort visibility.  Example: '30000-32767'.  Inclusive at both ends of the range.")
	fs.StringVar(&s.MasterServiceNamespace, "master-service-namespace", s.MasterServiceNamespace, "The namespace from which the kubernetes master services should be injected into pods")
	fs.Var(&s.RuntimeConfig, "runtime-config", "A set of key=value pairs that describe runtime configuration that may be passed to the apiserver. api/<version> key can be used to turn on/off specific api versions. api/all and api/legacy are special keys to control all and legacy api versions respectively.")
	client.BindKubeletClientConfigFlags(fs, &s.KubeletConfig)
	fs.StringVar(&s.ClusterName, "cluster-name", s.ClusterName, "The instance prefix for the cluster")
	fs.BoolVar(&s.EnableProfiling, "profiling", true, "Enable profiling via web interface host:port/debug/pprof/")
	fs.StringVar(&s.ExternalHost, "external-hostname", "", "The hostname to use when generating externalized URLs for this master (e.g. Swagger API Docs.)")
	fs.IntVar(&s.MaxRequestsInFlight, "max-requests-inflight", 400, "The maximum number of requests in flight at a given time.  When the server exceeds this, it rejects requests.  Zero for no limit.")
	fs.StringVar(&s.LongRunningRequestRE, "long-running-request-regexp", "[.*\\/watch$][^\\/proxy.*]", "A regular expression matching long running requests which should be excluded from maximum inflight request handling.")
}

// TODO: Longer term we should read this from some config store, rather than a flag.
func (s *APIServer) verifyPortalFlags() {
	if s.PortalNet.IP == nil {
		glog.Fatal("No --portal-net specified")
	}
}

func newEtcd(etcdConfigFile string, etcdServerList util.StringList, storageVersion string, pathPrefix string) (helper tools.EtcdHelper, err error) {
	var client tools.EtcdGetSet
	if etcdConfigFile != "" {
		client, err = etcd.NewClientFromFile(etcdConfigFile)
		if err != nil {
			return helper, err
		}
	} else {
		etcdClient := etcd.NewClient(etcdServerList)
		transport := &http.Transport{
			Dial: forked.Dial,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			MaxIdleConnsPerHost: 500,
		}
		etcdClient.SetTransport(transport)
		client = etcdClient
	}

	return master.NewEtcdHelper(client, storageVersion, pathPrefix)
}

// Run runs the specified APIServer.  This should never exit.
func (s *APIServer) Run(_ []string) error {
	s.verifyPortalFlags()

	if (s.EtcdConfigFile != "" && len(s.EtcdServerList) != 0) || (s.EtcdConfigFile == "" && len(s.EtcdServerList) == 0) {
		glog.Fatalf("specify either --etcd-servers or --etcd-config")
	}

	capabilities.Initialize(capabilities.Capabilities{
		AllowPrivileged: s.AllowPrivileged,
		// TODO(vmarmol): Implement support for HostNetworkSources.
		HostNetworkSources: []string{},
	})

	cloud, err := cloudprovider.InitCloudProvider(s.CloudProvider, s.CloudConfigFile)
	if err != nil {
		glog.Fatalf("Cloud provider could not be initialized: %v", err)
	}

	kubeletClient, err := client.NewKubeletClient(&s.KubeletConfig)
	if err != nil {
		glog.Fatalf("Failure to start kubelet client: %v", err)
	}

	// "api/all=false" allows users to selectively enable specific api versions.
	disableAllAPIs := false
	allAPIFlagValue, ok := s.RuntimeConfig["api/all"]
	if ok && allAPIFlagValue == "false" {
		disableAllAPIs = true
	}

	// "api/legacy=false" allows users to disable legacy api versions.
	// Right now, v1beta1 and v1beta2 are considered legacy.
	disableLegacyAPIs := false
	legacyAPIFlagValue, ok := s.RuntimeConfig["api/legacy"]
	if ok && legacyAPIFlagValue == "false" {
		disableLegacyAPIs = true
	}

	// "api/v1beta1={true|false} allows users to enable/disable v1beta1 API.
	// This takes preference over api/all and api/legacy, if specified.
	disableV1beta1 := disableAllAPIs || disableLegacyAPIs
	disableV1beta1 = !s.getRuntimeConfigValue("api/v1beta1", !disableV1beta1)

	// "api/v1beta2={true|false} allows users to enable/disable v1beta2 API.
	// This takes preference over api/all and api/legacy, if specified.
	disableV1beta2 := disableAllAPIs || disableLegacyAPIs
	disableV1beta2 = !s.getRuntimeConfigValue("api/v1beta2", !disableV1beta2)

	// "api/v1beta3={true|false} allows users to enable/disable v1beta3 API.
	// This takes preference over api/all and api/legacy, if specified.
	disableV1beta3 := disableAllAPIs
	disableV1beta3 = !s.getRuntimeConfigValue("api/v1beta3", !disableV1beta3)

	// V1 is disabled by default. Users can enable it using "api/v1={true}".
	_, enableV1 := s.RuntimeConfig["api/v1"]

	// TODO: expose same flags as client.BindClientConfigFlags but for a server
	clientConfig := &client.Config{
		Host:    net.JoinHostPort(s.InsecureBindAddress.String(), strconv.Itoa(s.InsecurePort)),
		Version: s.StorageVersion,
	}
	client, err := client.New(clientConfig)
	if err != nil {
		glog.Fatalf("Invalid server address: %v", err)
	}

	helper, err := newEtcd(s.EtcdConfigFile, s.EtcdServerList, s.StorageVersion, s.EtcdPathPrefix)
	if err != nil {
		glog.Fatalf("Invalid storage version or misconfigured etcd: %v", err)
	}

	// TODO Is this the right place for migration to happen? Must *both* old and
	// new etcd prefix params be supplied for this to be valid?
	if s.OldEtcdPathPrefix != "" {
		if err = helper.MigrateKeys(s.OldEtcdPathPrefix); err != nil {
			glog.Fatalf("Migration of old etcd keys failed: %v", err)
		}
	}

	n := net.IPNet(s.PortalNet)

	// Default to the private server key for service account token signing
	if s.ServiceAccountKeyFile == "" && s.TLSPrivateKeyFile != "" {
		s.ServiceAccountKeyFile = s.TLSPrivateKeyFile
	}
	authenticator, err := apiserver.NewAuthenticator(s.BasicAuthFile, s.ClientCAFile, s.TokenAuthFile, s.ServiceAccountKeyFile, s.ServiceAccountLookup, helper)
	if err != nil {
		glog.Fatalf("Invalid Authentication Config: %v", err)
	}

	authorizer, err := apiserver.NewAuthorizerFromAuthorizationConfig(s.AuthorizationMode, s.AuthorizationPolicyFile)
	if err != nil {
		glog.Fatalf("Invalid Authorization Config: %v", err)
	}

	admissionControlPluginNames := strings.Split(s.AdmissionControl, ",")
	admissionController := admission.NewFromPlugins(client, admissionControlPluginNames, s.AdmissionControlConfigFile)

	if len(s.ExternalHost) == 0 {
		// TODO: extend for other providers
		if s.CloudProvider == "gce" {
			instances, supported := cloud.Instances()
			if !supported {
				glog.Fatalf("gce cloud provider has no instances.  this shouldn't happen. exiting.")
			}
			name, err := os.Hostname()
			if err != nil {
				glog.Fatalf("failed to get hostname: %v", err)
			}
			addrs, err := instances.NodeAddresses(name)
			if err != nil {
				glog.Warningf("unable to obtain external host address from cloud provider: %v", err)
			} else {
				for _, addr := range addrs {
					if addr.Type == api.NodeExternalIP {
						s.ExternalHost = addr.Address
					}
				}
			}
		}
	}

	config := &master.Config{
		EtcdHelper:             helper,
		EventTTL:               s.EventTTL,
		KubeletClient:          kubeletClient,
		PortalNet:              &n,
		EnableCoreControllers:  true,
		EnableLogsSupport:      s.EnableLogsSupport,
		EnableUISupport:        true,
		EnableSwaggerSupport:   true,
		EnableProfiling:        s.EnableProfiling,
		EnableIndex:            true,
		APIPrefix:              s.APIPrefix,
		CorsAllowedOriginList:  s.CorsAllowedOriginList,
		ReadOnlyPort:           s.ReadOnlyPort,
		ReadWritePort:          s.SecurePort,
		PublicAddress:          net.IP(s.BindAddress),
		Authenticator:          authenticator,
		SupportsBasicAuth:      len(s.BasicAuthFile) > 0,
		Authorizer:             authorizer,
		AdmissionControl:       admissionController,
		DisableV1Beta1:         disableV1beta1,
		DisableV1Beta2:         disableV1beta2,
		DisableV1Beta3:         disableV1beta3,
		EnableV1:               enableV1,
		MasterServiceNamespace: s.MasterServiceNamespace,
		ClusterName:            s.ClusterName,
		ExternalHost:           s.ExternalHost,
	}
	m := master.New(config)

	// We serve on 3 ports.  See docs/accessing_the_api.md
	roLocation := ""
	if s.ReadOnlyPort != 0 {
		roLocation = net.JoinHostPort(s.BindAddress.String(), strconv.Itoa(s.ReadOnlyPort))
	}
	secureLocation := ""
	if s.SecurePort != 0 {
		secureLocation = net.JoinHostPort(s.BindAddress.String(), strconv.Itoa(s.SecurePort))
	}
	insecureLocation := net.JoinHostPort(s.InsecureBindAddress.String(), strconv.Itoa(s.InsecurePort))

	// See the flag commentary to understand our assumptions when opening the read-only and read-write ports.

	var sem chan bool
	if s.MaxRequestsInFlight > 0 {
		sem = make(chan bool, s.MaxRequestsInFlight)
	}

	longRunningRE := regexp.MustCompile(s.LongRunningRequestRE)

	if roLocation != "" {
		// Default settings allow 1 read-only request per second, allow up to 20 in a burst before enforcing.
		rl := util.NewTokenBucketRateLimiter(s.APIRate, s.APIBurst)
		readOnlyServer := &http.Server{
			Addr:           roLocation,
			Handler:        apiserver.MaxInFlightLimit(sem, longRunningRE, apiserver.RecoverPanics(apiserver.ReadOnly(apiserver.RateLimit(rl, m.InsecureHandler)))),
			ReadTimeout:    ReadWriteTimeout,
			WriteTimeout:   ReadWriteTimeout,
			MaxHeaderBytes: 1 << 20,
		}
		glog.Infof("Serving read-only insecurely on %s", roLocation)
		go func() {
			defer util.HandleCrash()
			for {
				if err := readOnlyServer.ListenAndServe(); err != nil {
					glog.Errorf("Unable to listen for read only traffic (%v); will try again.", err)
				}
				time.Sleep(15 * time.Second)
			}
		}()
	}

	if secureLocation != "" {
		secureServer := &http.Server{
			Addr:           secureLocation,
			Handler:        apiserver.MaxInFlightLimit(sem, longRunningRE, apiserver.RecoverPanics(m.Handler)),
			ReadTimeout:    ReadWriteTimeout,
			WriteTimeout:   ReadWriteTimeout,
			MaxHeaderBytes: 1 << 20,
			TLSConfig: &tls.Config{
				// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
				MinVersion: tls.VersionTLS10,
			},
		}

		if len(s.ClientCAFile) > 0 {
			clientCAs, err := util.CertPoolFromFile(s.ClientCAFile)
			if err != nil {
				glog.Fatalf("unable to load client CA file: %v", err)
			}
			// Populate PeerCertificates in requests, but don't reject connections without certificates
			// This allows certificates to be validated by authenticators, while still allowing other auth types
			secureServer.TLSConfig.ClientAuth = tls.RequestClientCert
			// Specify allowed CAs for client certificates
			secureServer.TLSConfig.ClientCAs = clientCAs
		}

		glog.Infof("Serving securely on %s", secureLocation)
		go func() {
			defer util.HandleCrash()
			for {
				if s.TLSCertFile == "" && s.TLSPrivateKeyFile == "" {
					s.TLSCertFile = path.Join(s.CertDirectory, "apiserver.crt")
					s.TLSPrivateKeyFile = path.Join(s.CertDirectory, "apiserver.key")
					if err := util.GenerateSelfSignedCert(config.PublicAddress.String(), s.TLSCertFile, s.TLSPrivateKeyFile); err != nil {
						glog.Errorf("Unable to generate self signed cert: %v", err)
					} else {
						glog.Infof("Using self-signed cert (%s, %s)", s.TLSCertFile, s.TLSPrivateKeyFile)
					}
				}
				if err := secureServer.ListenAndServeTLS(s.TLSCertFile, s.TLSPrivateKeyFile); err != nil {
					glog.Errorf("Unable to listen for secure (%v); will try again.", err)
				}
				time.Sleep(15 * time.Second)
			}
		}()
	}
	http := &http.Server{
		Addr:           insecureLocation,
		Handler:        apiserver.RecoverPanics(m.InsecureHandler),
		ReadTimeout:    ReadWriteTimeout,
		WriteTimeout:   ReadWriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}
	glog.Infof("Serving insecurely on %s", insecureLocation)
	glog.Fatal(http.ListenAndServe())
	return nil
}

func (s *APIServer) getRuntimeConfigValue(apiKey string, defaultValue bool) bool {
	flagValue, ok := s.RuntimeConfig[apiKey]
	if ok {
		if flagValue == "" {
			return true
		}
		boolValue, err := strconv.ParseBool(flagValue)
		if err != nil {
			glog.Fatalf("Invalid value of %s: %s", apiKey, flagValue)
		}
		return boolValue
	}
	return defaultValue
}
