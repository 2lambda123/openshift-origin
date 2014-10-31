package client

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubecfg"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/version"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	osclient "github.com/openshift/origin/pkg/client"
	. "github.com/openshift/origin/pkg/cmd/client/api"
	"github.com/openshift/origin/pkg/cmd/client/build"
	"github.com/openshift/origin/pkg/cmd/client/image"
	"github.com/openshift/origin/pkg/cmd/client/project"
	"github.com/openshift/origin/pkg/cmd/client/route"
	"github.com/openshift/origin/pkg/config"
	configapi "github.com/openshift/origin/pkg/config/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployclient "github.com/openshift/origin/pkg/deploy/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
)

type KubeConfig struct {
	ClientConfig   kubeclient.Config
	ServerVersion  bool
	PreventSkew    bool
	Config         string
	TemplateConfig string
	Selector       string
	UpdatePeriod   time.Duration
	PortSpec       string
	ServicePort    int
	AuthConfig     string
	JSON           bool
	YAML           bool
	Verbose        bool
	Proxy          bool
	WWW            string
	TemplateFile   string
	TemplateStr    string
	ID             string

	ImageName string

	APIVersion   string
	OSAPIVersion string

	Args []string
}

func (c *KubeConfig) Arg(index int) string {
	if index >= len(c.Args) {
		return ""
	}
	return c.Args[index]
}

func usage(name string) string {
	return fmt.Sprintf(`
  Kubernetes REST API:
  %[1]s [OPTIONS] get|list|create|delete|update <%[2]s>[/<id>]

  Manage replication controllers:

  %[1]s [OPTIONS] stop|rm <controller>
  %[1]s [OPTIONS] [-u <time>] [-image <image>] rollingupdate <controller>
  %[1]s [OPTIONS] resize <controller> <replicas>

  Launch a simple ReplicationController with a single container based
  on the given image:

  %[1]s [OPTIONS] [-p <port spec>] run <image> <replicas> <controller>

  Perform bulk operations on groups of Kubernetes resources:
  %[1]s [OPTIONS] apply -c config.json

  Process template into config:
  %[1]s [OPTIONS] process -c template.json

  Retrieve build logs:
  %[1]s [OPTIONS] buildLogs --id="buildID"
`, name, prettyWireStorage())
}

var parser = kubecfg.NewParser(map[string]runtime.Object{
	"pods":                    &kubeapi.Pod{},
	"services":                &kubeapi.Service{},
	"replicationControllers":  &kubeapi.ReplicationController{},
	"minions":                 &kubeapi.Minion{},
	"builds":                  &buildapi.Build{},
	"buildConfigs":            &buildapi.BuildConfig{},
	"images":                  &imageapi.Image{},
	"imageRepositories":       &imageapi.ImageRepository{},
	"imageRepositoryMappings": &imageapi.ImageRepositoryMapping{},
	"config":                  &configapi.Config{},
	"deployments":             &deployapi.Deployment{},
	"deploymentConfigs":       &deployapi.DeploymentConfig{},
	"routes":                  &routeapi.Route{},
	"projects":                &projectapi.Project{},
})

func prettyWireStorage() string {
	types := parser.SupportedWireStorage()
	sort.Strings(types)
	return strings.Join(types, "|")
}

// readConfigData reads the bytes from the specified filesytem or network location associated with the *config flag
func (c *KubeConfig) readConfigData() []byte {
	// read from STDIN
	if c.Config == "-" {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			glog.Fatalf("Unable to read from STDIN: %v\n", err)
		}
		return data
	}

	// we look for http:// or https:// to determine if valid URL, otherwise do normal file IO
	if url, err := url.Parse(c.Config); err == nil && (url.Scheme == "http" || url.Scheme == "https") {
		resp, err := http.Get(url.String())
		if err != nil {
			glog.Fatalf("Unable to access URL %v: %v\n", c.Config, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			glog.Fatalf("Unable to read URL, server reported %d %s", resp.StatusCode, resp.Status)
		}
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			glog.Fatalf("Unable to read URL %v: %v\n", c.Config, err)
		}
		return data
	}

	data, err := ioutil.ReadFile(c.Config)
	if err != nil {
		glog.Fatalf("Unable to read %v: %v\n", c.Config, err)
	}
	return data
}

// readConfig reads and parses pod, replicationController, and service
// configuration files. If any errors log and exit non-zero.
func (c *KubeConfig) readConfig(storage string, serverCodec runtime.Codec) []byte {
	if len(c.Config) == 0 {
		glog.Fatal("Need config file (-c)")
	}

	data, err := parser.ToWireFormat(c.readConfigData(), storage, latest.Codec, serverCodec)
	if err != nil {
		glog.Fatalf("Error parsing %v as an object for %v: %v", c.Config, storage, err)
	}
	if c.Verbose {
		glog.Infof("Parsed config file successfully; sending:\n%v", string(data))
	}
	return data
}

func (c *KubeConfig) Run() {
	util.InitLogs()
	defer util.FlushLogs()

	clientConfig := &c.ClientConfig
	// Initialize the client
	if clientConfig.Host == "" {
		clientConfig.Host = os.Getenv("KUBERNETES_MASTER")
	}
	if clientConfig.Host == "" {
		// TODO: eventually apiserver should start on 443 and be secure by default
		clientConfig.Host = "http://localhost:8080"
	}
	hosts := strings.SplitN(clientConfig.Host, ",", 2)
	for i := range hosts {
		hosts[i] = strings.TrimRight(hosts[i], "/")
	}
	clientConfig.Host = hosts[0]

	if kubeclient.IsConfigTransportSecure(clientConfig) {
		auth, err := kubecfg.LoadAuthInfo(c.AuthConfig, os.Stdin)
		if err != nil {
			glog.Fatalf("Error loading auth: %v", err)
		}
		clientConfig.Username = auth.User
		clientConfig.Password = auth.Password
		if auth.CAFile != "" {
			clientConfig.CAFile = auth.CAFile
		}
		if auth.CertFile != "" {
			clientConfig.CertFile = auth.CertFile
		}
		if auth.KeyFile != "" {
			clientConfig.KeyFile = auth.KeyFile
		}
		if auth.Insecure != nil {
			clientConfig.Insecure = *auth.Insecure
		}
	}
	clientConfig.Version = c.APIVersion
	kubeClient, err := kubeclient.New(clientConfig)
	if err != nil {
		glog.Fatalf("Unable to set up the Kubernetes API client: %v", err)
	}

	if len(hosts) > 1 {
		clientConfig.Host = hosts[1]
	}
	clientConfig.Version = c.OSAPIVersion
	client, err := osclient.New(clientConfig)
	if err != nil {
		glog.Fatalf("Unable to set up the OpenShift API client: %v", err)
	}

	// check the kubernetes server version
	if c.ServerVersion {
		got, err := kubeClient.ServerVersion()
		if err != nil {
			fmt.Printf("Couldn't read version from server: %v", err)
			os.Exit(1)
		}
		fmt.Printf("Server Version: %#v", got)
		os.Exit(0)
	}

	if c.PreventSkew {
		got, err := kubeClient.ServerVersion()
		if err != nil {
			fmt.Printf("Couldn't read version from server: %v", err)
			os.Exit(1)
		}
		if c, s := version.Get(), *got; !reflect.DeepEqual(c, s) {
			fmt.Printf("Server version (%#v) differs from client version (%#v)!", s, c)
			os.Exit(1)
		}
	}

	if c.Proxy {
		glog.Info("Starting to serve on localhost:8001")
		server := kubecfg.NewProxyServer(c.WWW, kubeClient)
		glog.Fatal(server.Serve())
	}

	method := c.Arg(0)
	clients := ClientMappings{
		"minions":                 {"Minion", kubeClient.RESTClient, klatest.Codec},
		"pods":                    {"Pod", kubeClient.RESTClient, klatest.Codec},
		"services":                {"Service", kubeClient.RESTClient, klatest.Codec},
		"replicationControllers":  {"ReplicationController", kubeClient.RESTClient, klatest.Codec},
		"builds":                  {"Build", client.RESTClient, latest.Codec},
		"buildConfigs":            {"BuildConfig", client.RESTClient, latest.Codec},
		"images":                  {"Image", client.RESTClient, latest.Codec},
		"imageRepositories":       {"ImageRepository", client.RESTClient, latest.Codec},
		"imageRepositoryMappings": {"ImageRepositoryMapping", client.RESTClient, latest.Codec},
		"deployments":             {"Deployment", client.RESTClient, latest.Codec},
		"deploymentConfigs":       {"DeploymentConfig", client.RESTClient, latest.Codec},
		"routes":                  {"Route", client.RESTClient, latest.Codec},
		"projects":                {"Project", client.RESTClient, latest.Codec},
	}

	matchFound := c.executeConfigRequest(method, clients) || c.executeTemplateRequest(method, client) || c.executeBuildLogRequest(method, client) || c.executeControllerRequest(method, kubeClient) || c.executeAPIRequest(method, clients)
	if matchFound == false {
		glog.Fatalf("Unknown command %s", method)
	}
}

// storagePathFromArg normalizes a path and breaks out the first segment if available
func storagePathFromArg(arg string) (storage, path string, hasSuffix bool) {
	path = strings.Trim(arg, "/")
	segments := strings.SplitN(path, "/", 2)
	storage = segments[0]
	if len(segments) > 1 && segments[1] != "" {
		hasSuffix = true
	}
	return storage, path, hasSuffix
}

//checkStorage returns true if the provided storage is valid
func checkStorage(storage string) bool {
	for _, allowed := range parser.SupportedWireStorage() {
		if allowed == storage {
			return true
		}
	}
	return false
}

func (c *KubeConfig) executeAPIRequest(method string, clients ClientMappings) bool {
	storage, path, hasSuffix := storagePathFromArg(c.Arg(1))
	validStorage := checkStorage(storage)
	client, ok := clients[storage]
	if !ok {
		glog.Fatalf("Unsupported storage type %s", storage)
	}

	verb := ""
	setBody := false
	var version string
	switch method {
	case "get":
		verb = "GET"
		if !validStorage || !hasSuffix {
			glog.Fatalf("usage: kubecfg [OPTIONS] %s <%s>[/<id>]", method, prettyWireStorage())
		}
	case "list":
		verb = "GET"
		if !validStorage || hasSuffix {
			glog.Fatalf("usage: kubecfg [OPTIONS] %s <%s>", method, prettyWireStorage())
		}
	case "delete":
		verb = "DELETE"
		if !validStorage || !hasSuffix {
			glog.Fatalf("usage: kubecfg [OPTIONS] %s <%s>/<id>", method, prettyWireStorage())
		}
	case "create":
		verb = "POST"
		setBody = true
		if !validStorage || hasSuffix {
			glog.Fatalf("usage: kubecfg [OPTIONS] %s <%s>", method, prettyWireStorage())
		}
	case "update":
		obj, err := client.Client.Verb("GET").Path(path).Do().Get()
		if err != nil {
			glog.Fatalf("error obtaining resource version for update: %v", err)
		}
		typeMeta, err := runtime.FindTypeMeta(obj)
		if err != nil {
			glog.Fatalf("error finding json base for update: %v", err)
		}
		version = typeMeta.ResourceVersion()
		verb = "PUT"
		setBody = true
		if !validStorage || !hasSuffix {
			glog.Fatalf("usage: kubecfg [OPTIONS] %s <%s>/<id>", method, prettyWireStorage())
		}
	default:
		return false
	}

	r := client.Client.Verb(verb).
		Path(path).
		ParseSelectorParam("labels", c.Selector)
	if setBody {
		if len(version) != 0 {
			data := c.readConfig(storage, client.Codec)
			obj, err := latest.Codec.Decode(data)
			if err != nil {
				glog.Fatalf("error setting resource version: %v", err)
			}
			typeMeta, err := runtime.FindTypeMeta(obj)
			if err != nil {
				glog.Fatalf("error setting resource version: %v", err)
			}
			typeMeta.SetResourceVersion(version)
			data, err = client.Codec.Encode(obj)
			if err != nil {
				glog.Fatalf("error setting resource version: %v", err)
			}
			r.Body(data)
		} else {
			r.Body(c.readConfig(storage, client.Codec))
		}
	}

	obj, err := r.Do().Get()
	if err != nil {
		glog.Fatalf("Got request error: %v", err)
		return false
	}

	return c.print(obj)
}

func (c *KubeConfig) print(obj runtime.Object) bool {
	var printer kubecfg.ResourcePrinter

	switch {
	case c.JSON:
		printer = &kubecfg.IdentityPrinter{}
	case c.YAML:
		printer = &kubecfg.YAMLPrinter{}
	case len(c.TemplateFile) > 0 || len(c.TemplateStr) > 0:
		var data []byte
		if len(c.TemplateFile) > 0 {
			var err error
			data, err = ioutil.ReadFile(c.TemplateFile)
			if err != nil {
				glog.Fatalf("Error reading template %s, %v", c.TemplateFile, err)
				return false
			}
		} else {
			data = []byte(c.TemplateStr)
		}
		tmpl, err := template.New("output").Parse(string(data))
		if err != nil {
			glog.Fatalf("Error parsing template %s, %v", string(data), err)
			return false
		}
		printer = &kubecfg.TemplatePrinter{
			Template: tmpl,
		}
	default:
		printer = humanReadablePrinter()
	}

	if err := printer.PrintObj(obj, os.Stdout); err != nil {
		glog.Fatalf("Failed to print runtime object.")
	}
	fmt.Print("\n")

	return true
}

func (c *KubeConfig) executeControllerRequest(method string, client *kubeclient.Client) bool {
	parseController := func() string {
		if len(c.Args) != 2 {
			glog.Fatal("usage: kubecfg [OPTIONS] stop|rm|rollingupdate|run|resize <controller>")
		}
		return c.Arg(1)
	}

	ctx := kubeapi.NewContext()
	var err error
	switch method {
	case "stop":
		err = kubecfg.StopController(ctx, parseController(), client)
	case "rm":
		err = kubecfg.DeleteController(ctx, parseController(), client)
	case "rollingupdate":
		err = kubecfg.Update(ctx, parseController(), client, c.UpdatePeriod, c.ImageName)
	case "run":
		if len(c.Args) != 4 {
			glog.Fatal("usage: kubecfg [OPTIONS] run <image> <replicas> <controller>")
		}
		image := c.Arg(1)
		replicas, err := strconv.Atoi(c.Arg(2))
		name := c.Arg(3)
		if err != nil {
			glog.Fatalf("Error parsing replicas: %v", err)
		}
		err = kubecfg.RunController(ctx, image, name, replicas, client, c.PortSpec, c.ServicePort)
	case "resize":
		args := c.Args
		if len(args) < 3 {
			glog.Fatal("usage: kubecfg resize <controller> <replicas>")
		}
		name := args[1]
		replicas, err := strconv.Atoi(args[2])
		if err != nil {
			glog.Fatalf("Error parsing replicas: %v", err)
		}
		err = kubecfg.ResizeController(ctx, name, replicas, client)
	default:
		return false
	}
	if err != nil {
		glog.Fatalf("Error: %v", err)
	}
	return true
}

// executeBuildLogRequest retrieves the logs from builder container
func (c *KubeConfig) executeBuildLogRequest(method string, client *osclient.Client) bool {
	if method != "buildLogs" {
		return false
	}
	if len(c.ID) == 0 {
		glog.Fatal("Build ID required")
	}
	request := client.Verb("GET").Path("redirect").Path("buildLogs").Path(c.ID)
	readCloser, err := request.Stream()
	if err != nil {
		glog.Fatalf("Error: %v", err)
	}
	defer readCloser.Close()
	if _, err := io.Copy(os.Stdout, readCloser); err != nil {
		glog.Fatalf("Error: %v", err)
	}
	return true
}

// executeTemplateRequest transform the JSON file with Config template into a
// valid Config JSON.
func (c *KubeConfig) executeTemplateRequest(method string, client *osclient.Client) bool {
	if method != "process" {
		return false
	}
	if len(c.Config) == 0 {
		glog.Fatal("Need template file (-c)")
	}
	data, err := ioutil.ReadFile(c.Config)
	if err != nil {
		glog.Fatalf("error reading template file: %v", err)
	}
	request := client.Verb("POST").Path("/templateConfigs").Body(data)
	result := request.Do()
	body, err := result.Raw()
	if err != nil {
		glog.Fatalf("failed to process template: %v", err)
	}
	printer := JSONPrinter{}
	if err := printer.Print(body, os.Stdout); err != nil {
		glog.Fatalf("unable to pretty print config JSON: %v [%s]", err, string(body))
	}
	return true
}

func (c *KubeConfig) executeConfigRequest(method string, clients ClientMappings) bool {
	switch method {
	case "apply":
		if len(c.Config) == 0 {
			glog.Fatal("Need to pass valid configuration file (-c config.json)")
		}
		result, err := config.Apply(c.readConfig("config", latest.Codec), clients)
		if err != nil {
			glog.Fatalf("Error applying the config: %v", err)
		}
		for _, itemResult := range result {
			if itemResult.Error == nil {
				fmt.Println(itemResult.Message)
				continue
			}

			if statusErr, ok := itemResult.Error.(kubeclient.APIStatus); ok {
				fmt.Printf("Error: %v\n", statusErr.Status().Message)
			} else {
				fmt.Printf("Error: %v\n", itemResult.Error)
			}
		}
	case "list-config":
		if len(c.Config) == 0 {
			glog.Fatal("Need to pass valid configuration file (-c config.json)")
		}
		result, err := config.List(c.readConfig("config", latest.Codec), clients)
		if err != nil {
			glog.Fatalf("Unable to list config: %v", err)
		}

		var services []kubeapi.Service
		var pods []kubeapi.Pod
		var replicationControllers []kubeapi.ReplicationController
		var otherKinds []runtime.Object

		for i, itemResult := range result {
			if statusErr, ok := itemResult.Error.(kubeclient.APIStatus); ok {
				fmt.Printf("Error: %v\n", statusErr.Status().Message)
				continue
			}
			if itemResult.Error == nil {
				obj, err := itemResult.Result.Get()
				if err != nil {
					glog.Errorf("config.item[%v]: Got request error: %v", i, err)
					continue
				}

				switch obj.(type) {
				case *kubeapi.Service:
					services = append(services, *obj.(*kubeapi.Service))
				case *kubeapi.Pod:
					pods = append(pods, *obj.(*kubeapi.Pod))
				case *kubeapi.ReplicationController:
					replicationControllers = append(replicationControllers, *obj.(*kubeapi.ReplicationController))
				default:
					otherKinds = append(otherKinds, obj)
				}
			}
		}

		if len(services) > 0 {
			list := &kubeapi.ServiceList{Items: services}
			c.print(list)
		}

		if len(pods) > 0 {
			list := &kubeapi.PodList{Items: pods}
			c.print(list)
		}

		if len(replicationControllers) > 0 {
			list := &kubeapi.ReplicationControllerList{Items: replicationControllers}
			c.print(list)
		}

		for _, obj := range otherKinds {
			c.print(obj)
		}

	default:
		return false
	}
	return true
}

func humanReadablePrinter() *kubecfg.HumanReadablePrinter {
	printer := kubecfg.NewHumanReadablePrinter()

	// Add Handler calls here to support additional types
	build.RegisterPrintHandlers(printer)
	image.RegisterPrintHandlers(printer)
	deployclient.RegisterPrintHandlers(printer)
	route.RegisterPrintHandlers(printer)
	project.RegisterPrintHandlers(printer)

	return printer
}
