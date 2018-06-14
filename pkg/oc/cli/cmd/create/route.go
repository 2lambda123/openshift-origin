package create

import (
	"fmt"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/cmd/create"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

var (
	routeLong = templates.LongDesc(`
		Expose containers externally via secured routes

		Three types of secured routes are supported: edge, passthrough, and reencrypt.
		If you wish to create unsecured routes, see "%[1]s expose -h"`)
)

// NewCmdCreateRoute is a macro command to create a secured route.
func NewCmdCreateRoute(fullName string, f *clientcmd.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Expose containers externally via secured routes",
		Long:  fmt.Sprintf(routeLong, fullName),
		Run:   kcmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	cmd.AddCommand(NewCmdCreateEdgeRoute(fullName, f, streams))
	cmd.AddCommand(NewCmdCreatePassthroughRoute(fullName, f, streams))
	cmd.AddCommand(NewCmdCreateReencryptRoute(fullName, f, streams))

	return cmd
}

// CreateRouteSubcommandOptions is an options struct to support create subcommands
type CreateRouteSubcommandOptions struct {
	// PrintFlags holds options necessary for obtaining a printer
	PrintFlags *create.PrintFlags
	// Name of resource being created
	Name        string
	ServiceName string
	// DryRun is true if the command should be simulated but not run against the server
	DryRun bool

	Namespace        string
	EnforceNamespace bool

	Mapper meta.RESTMapper

	PrintObj func(obj runtime.Object) error

	genericclioptions.IOStreams

	Client     routev1client.RoutesGetter
	KubeClient kclientset.Interface
}

func NewCreateRouteSubcommandOptions(ioStreams genericclioptions.IOStreams) *CreateRouteSubcommandOptions {
	return &CreateRouteSubcommandOptions{
		PrintFlags: create.NewPrintFlags("created", legacyscheme.Scheme),
		IOStreams:  ioStreams,
	}
}

func (o *CreateRouteSubcommandOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error
	o.Name, err = resolveRouteName(args)
	if err != nil {
		return err
	}

	o.KubeClient, err = f.ClientSet()
	if err != nil {
		return err
	}
	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}
	o.Client, err = routev1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.Mapper, _ = f.Object()
	o.Namespace, o.EnforceNamespace, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.DryRun = kcmdutil.GetDryRunFlag(cmd)
	if o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}
	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}
	o.PrintObj = func(obj runtime.Object) error {
		return printer.PrintObj(obj, o.Out)
	}

	return nil
}

func resolveRouteName(args []string) (string, error) {
	switch len(args) {
	case 0:
	case 1:
		return args[0], nil
	default:
		return "", fmt.Errorf("multiple names provided. Please specify at most one")
	}
	return "", nil
}
