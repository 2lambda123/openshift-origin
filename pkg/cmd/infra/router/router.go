package router

import (
	"flag"
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/router"
	controllerfactory "github.com/openshift/origin/pkg/router/controller/factory"
	templateplugin "github.com/openshift/origin/plugins/router/template"
)

const longCommandDesc = `
Start an OpenShift router

This command launches a router connected to your OpenShift master. The router listens for routes and endpoints
created by users and keeps a local router configuration up to date with those changes.
`

type templateRouterConfig struct {
	Config       *clientcmd.Config
	TemplateFile string
	ReloadScript string
}

// NewCommndTemplateRouter provides CLI handler for the template router backend
func NewCommandTemplateRouter(name string) *cobra.Command {
	flag.Set("v", "4")
	cfg := &templateRouterConfig{
		Config: clientcmd.NewConfig(),
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, clientcmd.ConfigSyntax),
		Short: "Start an OpenShift router",
		Long:  longCommandDesc,
		Run: func(c *cobra.Command, args []string) {
			plugin, err := makeTemplatePlugin(cfg)
			if err != nil {
				glog.Fatal(err)
			}

			if err = start(cfg.Config, plugin); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flag := cmd.Flags()
	cfg.Config.Bind(flag)
	flag.StringVar(&cfg.TemplateFile, "template", util.Env("TEMPLATE_FILE", ""), "The path to the template file to use")
	flag.StringVar(&cfg.ReloadScript, "reload", util.Env("RELOAD_SCRIPT", ""), "The path to the reload script to use")

	return cmd
}

func makeTemplatePlugin(cfg *templateRouterConfig) (*templateplugin.TemplatePlugin, error) {
	return templateplugin.NewTemplatePlugin(cfg.TemplateFile, cfg.ReloadScript)
}

// start launches the load balancer.
func start(cfg *clientcmd.Config, plugin router.Plugin) error {
	kubeClient, osClient, err := cfg.Clients()
	if err != nil {
		return err
	}

	factory := controllerfactory.RouterControllerFactory{kubeClient, osClient}
	controller := factory.Create(plugin)
	controller.Run()

	select {}

	return nil
}
