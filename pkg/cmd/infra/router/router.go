package router

import (
	"flag"
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	controllerfactory "github.com/openshift/origin/pkg/router/controller/factory"
	"github.com/openshift/origin/plugins/router/haproxy"
)

const longCommandDesc = `
Start an OpenShift router

This command launches a router connected to your OpenShift master. The router listens for routes and endpoints
created by users and keeps a local router configuration up to date with those changes.
`

// NewCommandRouter provides CLI handler for router command
func NewCommandRouter(name string) *cobra.Command {
	flag.Set("v", "4")
	cfg := clientcmd.NewConfig()

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, clientcmd.ConfigSyntax),
		Short: "Start an OpenShift router",
		Long:  longCommandDesc,
		Run: func(c *cobra.Command, args []string) {
			if err := start(cfg); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flag := cmd.Flags()
	cfg.Bind(flag)

	return cmd
}

// start launches the load balancer.
func start(cfg *clientcmd.Config) error {
	kubeClient, osClient, err := cfg.Clients()
	if err != nil {
		return err
	}

	routes := haproxy.NewRouter()
	factory := controllerfactory.RouterControllerFactory{kubeClient, osClient}
	controller := factory.Create(routes)
	controller.Run()

	select {}

	return nil
}
