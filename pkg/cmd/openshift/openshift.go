package openshift

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/admin"
	diagnostics "github.com/openshift/origin/pkg/cmd/admin/diagnostics"
	sync "github.com/openshift/origin/pkg/cmd/admin/groups/sync/cli"
	"github.com/openshift/origin/pkg/cmd/admin/validate"
	"github.com/openshift/origin/pkg/cmd/cli"
	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/experimental/buildchain"
	exipfailover "github.com/openshift/origin/pkg/cmd/experimental/ipfailover"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/infra/builder"
	"github.com/openshift/origin/pkg/cmd/infra/deployer"
	irouter "github.com/openshift/origin/pkg/cmd/infra/router"
	"github.com/openshift/origin/pkg/cmd/server/start"
	"github.com/openshift/origin/pkg/cmd/server/start/kubernetes"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/version"
)

const (
	openshiftLong = `
%[2]s

The %[3]s helps you build, deploy, and manage your applications on top of
Docker containers. To start an all-in-one server with the default configuration, run:

  $ %[1]s start &`
)

// CommandFor returns the appropriate command for this base name,
// or the global OpenShift command
func CommandFor(basename string) *cobra.Command {
	var cmd *cobra.Command

	in, out, errout := os.Stdin, os.Stdout, os.Stderr

	// Make case-insensitive and strip executable suffix if present
	if runtime.GOOS == "windows" {
		basename = strings.ToLower(basename)
		basename = strings.TrimSuffix(basename, ".exe")
	}

	switch basename {
	case "openshift-router":
		cmd = irouter.NewCommandTemplateRouter(basename)
	case "openshift-f5-router":
		cmd = irouter.NewCommandF5Router(basename)
	case "openshift-deploy":
		cmd = deployer.NewCommandDeployer(basename)
	case "openshift-sti-build":
		cmd = builder.NewCommandSTIBuilder(basename)
	case "openshift-docker-build":
		cmd = builder.NewCommandDockerBuilder(basename)
	case "oc", "osc":
		cmd = cli.NewCommandCLI(basename, basename, in, out, errout)
	case "oadm", "osadm":
		cmd = admin.NewCommandAdmin(basename, basename, out, errout)
	case "kubectl":
		cmd = cli.NewCmdKubectl(basename, out)
	case "kube-apiserver":
		cmd = kubernetes.NewAPIServerCommand(basename, basename, out)
	case "kube-controller-manager":
		cmd = kubernetes.NewControllersCommand(basename, basename, out)
	case "kubelet":
		cmd = kubernetes.NewKubeletCommand(basename, basename, out)
	case "kube-proxy":
		cmd = kubernetes.NewProxyCommand(basename, basename, out)
	case "kube-scheduler":
		cmd = kubernetes.NewSchedulerCommand(basename, basename, out)
	case "kubernetes":
		cmd = kubernetes.NewCommand(basename, basename, out)
	case "origin", "atomic-enterprise":
		cmd = NewCommandOpenShift(basename)
	default:
		cmd = NewCommandOpenShift("openshift")
	}

	if cmd.UsageFunc() == nil {
		templates.ActsAsRootCommand(cmd, []string{"options"})
	}
	flagtypes.GLog(cmd.PersistentFlags())

	return cmd
}

// NewCommandOpenShift creates the standard OpenShift command
func NewCommandOpenShift(name string) *cobra.Command {
	in, out, errout := os.Stdin, os.Stdout, os.Stderr

	root := &cobra.Command{
		Use:   name,
		Short: "Build, deploy, and manage your cloud applications",
		Long:  fmt.Sprintf(openshiftLong, name, cmdutil.GetPlatformName(name), cmdutil.GetDistributionName(name)),
		Run:   cmdutil.DefaultSubCommandRun(out),
	}

	startAllInOne, _ := start.NewCommandStartAllInOne(name, out)
	root.AddCommand(startAllInOne)
	root.AddCommand(admin.NewCommandAdmin("admin", name+" admin", out, errout))
	root.AddCommand(cli.NewCommandCLI("cli", name+" cli", in, out, errout))
	root.AddCommand(cli.NewCmdKubectl("kube", out))
	root.AddCommand(newExperimentalCommand("ex", name+" ex"))
	root.AddCommand(version.NewVersionCommand(name, true))

	// infra commands are those that are bundled with the binary but not displayed to end users
	// directly
	infra := &cobra.Command{
		Use: "infra", // Because this command exposes no description, it will not be shown in help
	}

	infra.AddCommand(
		irouter.NewCommandTemplateRouter("router"),
		irouter.NewCommandF5Router("f5-router"),
		deployer.NewCommandDeployer("deploy"),
		builder.NewCommandSTIBuilder("sti-build"),
		builder.NewCommandDockerBuilder("docker-build"),
		diagnostics.NewCommandPodDiagnostics("diagnostic-pod", out),
	)
	root.AddCommand(infra)

	root.AddCommand(cmd.NewCmdOptions(out))

	// TODO: add groups
	templates.ActsAsRootCommand(root, []string{"options"})

	return root
}

func newExperimentalCommand(name, fullName string) *cobra.Command {
	out := os.Stdout

	experimental := &cobra.Command{
		Use:   name,
		Short: "Experimental commands under active development",
		Long:  "The commands grouped here are under development and may change without notice.",
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(out)
			c.Help()
		},
		BashCompletionFunction: admin.BashCompletionFunc,
	}

	f := clientcmd.New(experimental.PersistentFlags())

	experimental.AddCommand(validate.NewCommandValidate(validate.ValidateRecommendedName, fullName+" "+validate.ValidateRecommendedName, out))
	experimental.AddCommand(exipfailover.NewCmdIPFailoverConfig(f, fullName, "ipfailover", out))
	experimental.AddCommand(buildchain.NewCmdBuildChain(name, fullName+" "+buildchain.BuildChainRecommendedCommandName, f, out))
	deprecatedDiag := diagnostics.NewCmdDiagnostics(diagnostics.DiagnosticsRecommendedName, fullName+" "+diagnostics.DiagnosticsRecommendedName, out)
	deprecatedDiag.Deprecated = fmt.Sprintf(`use "oadm %[1]s" to run diagnostics instead.`, diagnostics.DiagnosticsRecommendedName)
	experimental.AddCommand(deprecatedDiag)
	experimental.AddCommand(cmd.NewCmdOptions(out))

	// these groups also live under `oadm groups {sync,prune}` and are here only for backwards compatibility
	experimental.AddCommand(sync.NewCmdSync("sync-groups", fullName+" "+"sync-groups", f, out))
	experimental.AddCommand(sync.NewCmdPrune("prune-groups", fullName+" "+"prune-groups", f, out))
	return experimental
}
