package clusteradd

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/components/service-catalog"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/components/template-service-broker"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/version"
)

const (
	CmdAddRecommendedName = "add"
)

var (
	cmdAddLong = templates.LongDesc(`
		Adds a component to an 'oc cluster up' cluster.
`)

	cmdAddExample = templates.Examples(`
	  # Add service catalog
	  %[1]s service-catalog

	  # Add template service broker to a different basedir
	  %[1]s --base-dir=other/path template-service-broker`)
)

// availableComponents lists the components that are available for installation.
var availableComponents = map[string]func(ctx componentinstall.Context) componentinstall.Component{
	"service-catalog": func(ctx componentinstall.Context) componentinstall.Component {
		return &service_catalog.ServiceCatalogComponentOptions{InstallContext: ctx}
	},
	"template-service-broker": func(ctx componentinstall.Context) componentinstall.Component {
		return &template_service_broker.TemplateServiceBrokerComponentOptions{InstallContext: ctx}
	},
}

func NewCmdAdd(name, fullName string, out, errout io.Writer) *cobra.Command {
	config := &ClusterAddConfig{
		Out:    out,
		ErrOut: errout,
	}
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Add components to an 'oc cluster up' cluster",
		Long:    cmdAddLong,
		Example: fmt.Sprintf(cmdAddExample, fullName),
		RunE: func(c *cobra.Command, args []string) error {
			if err := config.Complete(c); err != nil {
				return err
			}
			if err := config.Validate(); err != nil {
				return err
			}
			if err := config.Check(); err != nil {
				return err
			}
			if err := config.Run(); err != nil {
				return err
			}

			return nil
		},
	}
	config.Bind(cmd.Flags())
	return cmd
}

// Start runs the start tasks ensuring that they are executed in sequence
func (c *ClusterAddConfig) Run() error {
	componentsToInstall := []componentinstall.Component{}
	installContext, err := componentinstall.NewComponentInstallContext(c.openshiftImage(), c.imageFormat(), c.BaseDir, c.ServerLogLevel)
	if err != nil {
		return err
	}
	for _, componentName := range c.ComponentsToInstall {
		fmt.Fprintf(c.Out, "Adding %s ...\n", componentName)
		component := availableComponents[componentName](installContext)
		componentsToInstall = append(componentsToInstall, component)
	}
	return componentinstall.InstallComponents(componentsToInstall, c.dockerClient, c.GetLogDir())
}

type ClusterAddConfig struct {
	BaseDir             string
	ImageTag            string
	Image               string
	ServerLogLevel      int
	ComponentsToInstall []string

	Out    io.Writer
	ErrOut io.Writer

	dockerClient dockerhelper.Interface
}

func (c *ClusterAddConfig) Complete(cmd *cobra.Command) error {
	c.ComponentsToInstall = cmd.Flags().Args()
	validComponentNames := sets.StringKeySet(availableComponents)
	for _, name := range c.ComponentsToInstall {
		if !validComponentNames.Has(name) {
			return fmt.Errorf("unknown component %q, valid component names are: %q", name, strings.Join(validComponentNames.List(), ","))
		}
	}

	// do some defaulting
	if len(c.ImageTag) == 0 {
		c.ImageTag = strings.TrimRight("v"+version.Get().Major+"."+version.Get().Minor, "+")
	}
	if len(c.BaseDir) == 0 {
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

	client, err := docker.GetDockerClient()
	if err != nil {
		return err
	}
	c.dockerClient = client

	return nil
}

// Validate validates that required fields in StartConfig have been populated
func (c *ClusterAddConfig) Validate() error {
	if c.dockerClient == nil {
		return fmt.Errorf("missing dockerClient")
	}
	return nil
}

// Check is a spot to do NON-MUTATING, preflight checks. Over time, we should try to move our non-mutating checks out of
// Complete and into Check.
// TODO check for basedir correctness
func (c *ClusterAddConfig) Check() error {
	return nil
}

func (c *ClusterAddConfig) Bind(flags *pflag.FlagSet) {
	flags.StringVar(&c.ImageTag, "tag", "", "Specify the tag for OpenShift images")
	flags.MarkHidden("tag")
	flags.StringVar(&c.Image, "image", variable.DefaultImagePrefix, "Specify the images to use for OpenShift")
	flags.StringVar(&c.BaseDir, "base-dir", c.BaseDir, "Directory on Docker host for cluster up configuration")
	flags.IntVar(&c.ServerLogLevel, "server-loglevel", 0, "Log level for OpenShift server")
}

func (c *ClusterAddConfig) openshiftImage() string {
	return fmt.Sprintf("%s:%s", c.Image, c.ImageTag)
}

func (c *ClusterAddConfig) GetLogDir() string {
	return path.Join(c.BaseDir, "logs")
}

func (c *ClusterAddConfig) imageFormat() string {
	return fmt.Sprintf("%s-${component}:%s", c.Image, c.ImageTag)
}
