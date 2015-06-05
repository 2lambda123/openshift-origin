package router

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	dapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/generate/app"
)

const (
	routerLong = `Install or configure an OpenShift router

This command helps to setup an OpenShift router to take edge traffic and balance it to
your application. With no arguments, the command will check for an existing router
service called 'router' and create one if it does not exist. If you want to test whether
a router has already been created add the --dry-run flag and the command will exit with
1 if the registry does not exist.

If a router does not exist with the given name, the --create flag can be passed to
create a deployment configuration and service that will run the router. If you are
running your router in production, you should pass --replicas=2 or higher to ensure
you have failover protection.

ALPHA: This command is currently being actively developed. It is intended to simplify
  the tasks of setting up routers in a new installation. `

	routerExample = `  // Check the default router ("router")
  $ %[1]s %[2]s --dry-run

  // See what the router would look like if created
  $ %[1]s %[2]s -o json

  // Create a router if it does not exist
  $ %[1]s %[2]s router-west --create --replicas=2

  // Use a different router image and see the router configuration
  $ %[1]s %[2]s region-west -o yaml --images=myrepo/somerouter:mytag`
)

type RouterConfig struct {
	Type               string
	ImageTemplate      variable.ImageTemplate
	Ports              string
	Replicas           int
	Labels             string
	DryRun             bool
	Credentials        string
	DefaultCertificate string
	Selector           string
}

var errExit = fmt.Errorf("exit")

const defaultLabel = "router=<name>"

func NewCmdRouter(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	cfg := &RouterConfig{
		ImageTemplate: variable.NewDefaultImageTemplate(),

		Labels:   defaultLabel,
		Ports:    "80:80,443:443,1936:1936",
		Replicas: 1,
	}

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [NAME]", name),
		Short:   "Install an OpenShift router",
		Long:    routerLong,
		Example: fmt.Sprintf(routerExample, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunCmdRouter(f, cmd, out, cfg, args)
			if err != errExit {
				cmdutil.CheckErr(err)
			} else {
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&cfg.Type, "type", "haproxy-router", "The type of router to use - if you specify --images this flag may be ignored.")
	cmd.Flags().StringVar(&cfg.ImageTemplate.Format, "images", cfg.ImageTemplate.Format, "The image to base this router on - ${component} will be replaced with --type")
	cmd.Flags().BoolVar(&cfg.ImageTemplate.Latest, "latest-images", cfg.ImageTemplate.Latest, "If true, attempt to use the latest images for the router instead of the latest release.")
	cmd.Flags().StringVar(&cfg.Ports, "ports", cfg.Ports, "A comma delimited list of ports or port pairs to expose on the router pod. The default is set for HAProxy.")
	cmd.Flags().IntVar(&cfg.Replicas, "replicas", cfg.Replicas, "The replication factor of the router; commonly 2 when high availability is desired.")
	cmd.Flags().StringVar(&cfg.Labels, "labels", cfg.Labels, "A set of labels to uniquely identify the router and its components.")
	cmd.Flags().BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Exit with code 1 if the specified router does not exist.")
	cmd.Flags().Bool("create", false, "deprecated; this is now the default behavior")
	cmd.Flags().StringVar(&cfg.Credentials, "credentials", "", "Path to a .kubeconfig file that will contain the credentials the router should use to contact the master.")
	cmd.Flags().StringVar(&cfg.DefaultCertificate, "default-cert", cfg.DefaultCertificate, "Optional path to a certificate file that be used as the default certificate.  The file should contain the cert, key, and any CA certs necessary for the router to serve the certificate.")
	cmd.Flags().StringVar(&cfg.Selector, "selector", cfg.Selector, "Selector used to filter nodes on deployment. Used to run routers on a specific set of nodes.")

	cmdutil.AddPrinterFlags(cmd)

	return cmd
}

func loadDefaultCert(file string) (string, error) {
	if len(file) == 0 {
		return "", nil
	}
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(bytes), err
}

func RunCmdRouter(f *clientcmd.Factory, cmd *cobra.Command, out io.Writer, cfg *RouterConfig, args []string) error {
	var name string
	switch len(args) {
	case 0:
		name = "router"
	case 1:
		name = args[0]
	default:
		return cmdutil.UsageError(cmd, "You may pass zero or one arguments to provide a name for the router")
	}

	ports, err := app.ContainerPortsFromString(cfg.Ports)
	if err != nil {
		glog.Fatal(err)
	}

	label := map[string]string{"router": name}
	if cfg.Labels != defaultLabel {
		valid, remove, err := app.LabelsFromSpec(strings.Split(cfg.Labels, ","))
		if err != nil {
			glog.Fatal(err)
		}
		if len(remove) > 0 {
			return cmdutil.UsageError(cmd, "You may not pass negative labels in %q", cfg.Labels)
		}
		label = valid
	}

	nodeSelector := map[string]string{}
	if len(cfg.Selector) > 0 {
		valid, remove, err := app.LabelsFromSpec(strings.Split(cfg.Selector, ","))
		if err != nil {
			glog.Fatal(err)
		}
		if len(remove) > 0 {
			return cmdutil.UsageError(cmd, "You may not pass negative labels in selector %q", cfg.Selector)
		}
		nodeSelector = valid
	}

	image := cfg.ImageTemplate.ExpandOrDie(cfg.Type)

	namespace, err := f.OpenShiftClientConfig.Namespace()
	if err != nil {
		return fmt.Errorf("error getting client: %v", err)
	}
	_, kClient, err := f.Clients()
	if err != nil {
		return fmt.Errorf("error getting client: %v", err)
	}

	p, output, err := cmdutil.PrinterForCommand(cmd)
	if err != nil {
		return fmt.Errorf("unable to configure printer: %v", err)
	}

	generate := output
	if !generate {
		_, err = kClient.Services(namespace).Get(name)
		if err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("can't check for existing router %q: %v", name, err)
			}
			generate = true
		}
	}

	if generate {
		if cfg.DryRun && !output {
			return fmt.Errorf("router %q does not exist (no service)", name)
		}

		// create new router
		if len(cfg.Credentials) == 0 {
			return fmt.Errorf("router could not be created; you must specify a .kubeconfig file path containing credentials for connecting the router to the master with --credentials")
		}

		clientConfigLoadingRules := &kclientcmd.ClientConfigLoadingRules{ExplicitPath: cfg.Credentials, Precedence: []string{}}
		credentials, err := clientConfigLoadingRules.Load()
		if err != nil {
			return fmt.Errorf("router could not be created; the provided credentials %q could not be loaded: %v", cfg.Credentials, err)
		}
		config, err := kclientcmd.NewDefaultClientConfig(*credentials, &kclientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return fmt.Errorf("router could not be created; the provided credentials %q could not be used: %v", cfg.Credentials, err)
		}
		if err := kclient.LoadTLSFiles(config); err != nil {
			return fmt.Errorf("router could not be created; the provided credentials %q could not load certificate info: %v", cfg.Credentials, err)
		}
		insecure := "false"
		if config.Insecure {
			insecure = "true"
		}

		defaultCert, err := loadDefaultCert(cfg.DefaultCertificate)
		if err != nil {
			return fmt.Errorf("router could not be created; error reading default certificate file", err)
		}

		env := app.Environment{
			"OPENSHIFT_MASTER":         config.Host,
			"OPENSHIFT_CA_DATA":        string(config.CAData),
			"OPENSHIFT_KEY_DATA":       string(config.KeyData),
			"OPENSHIFT_CERT_DATA":      string(config.CertData),
			"OPENSHIFT_INSECURE":       insecure,
			"DEFAULT_CERTIFICATE":      defaultCert,
			"ROUTER_SERVICE_NAME":      name,
			"ROUTER_SERVICE_NAMESPACE": namespace,
		}

		objects := []runtime.Object{
			&dapi.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{
					Name:   name,
					Labels: label,
				},
				Triggers: []dapi.DeploymentTriggerPolicy{
					{Type: dapi.DeploymentTriggerOnConfigChange},
				},
				Template: dapi.DeploymentTemplate{
					Strategy: dapi.DeploymentStrategy{
						Type: dapi.DeploymentStrategyTypeRecreate,
					},
					ControllerTemplate: kapi.ReplicationControllerSpec{
						Replicas: cfg.Replicas,
						Selector: label,
						Template: &kapi.PodTemplateSpec{
							ObjectMeta: kapi.ObjectMeta{Labels: label},
							Spec: kapi.PodSpec{
								NodeSelector: nodeSelector,
								Containers: []kapi.Container{
									{
										Name:  "router",
										Image: image,
										Ports: ports,
										Env:   env.List(),
										LivenessProbe: &kapi.Probe{
											Handler: kapi.Handler{
												TCPSocket: &kapi.TCPSocketAction{
													Port: kutil.IntOrString{
														IntVal: ports[0].ContainerPort,
													},
												},
											},
											InitialDelaySeconds: 10,
										},
										ImagePullPolicy: kapi.PullIfNotPresent,
									},
								},
							},
						},
					},
				},
			},
		}
		objects = app.AddServices(objects)
		// TODO: label all created objects with the same label - router=<name>
		list := &kapi.List{Items: objects}

		if output {
			if err := p.PrintObj(list, out); err != nil {
				return fmt.Errorf("Unable to print object: %v", err)
			}
			return nil
		}

		mapper, typer := f.Factory.Object()
		bulk := configcmd.Bulk{
			Mapper:            mapper,
			Typer:             typer,
			RESTClientFactory: f.Factory.RESTClient,

			After: configcmd.NewPrintNameOrErrorAfter(out, os.Stderr),
		}
		if errs := bulk.Create(list, namespace); len(errs) != 0 {
			return errExit
		}
		return nil
	}

	fmt.Fprintf(out, "Router %q service exists\n", name)
	return nil
}
