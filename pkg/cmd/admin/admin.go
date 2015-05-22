package admin

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/admin/node"
	"github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/cmd/admin/project"
	"github.com/openshift/origin/pkg/cmd/admin/prune"
	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/experimental/buildchain"
	exipfailover "github.com/openshift/origin/pkg/cmd/experimental/ipfailover"
	exregistry "github.com/openshift/origin/pkg/cmd/experimental/registry"
	exrouter "github.com/openshift/origin/pkg/cmd/experimental/router"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/version"
)

const adminLong = `OpenShift Administrative Commands

Commands for managing an OpenShift cluster are exposed here. Many administrative
actions involve interaction with the OpenShift client as well.

Note: This is a beta release of OpenShift and may change significantly.  See
    https://github.com/openshift/origin for the latest information on OpenShift.`

func NewCommandAdmin(name, fullName string, out io.Writer) *cobra.Command {
	// Main command
	cmds := &cobra.Command{
		Use:   name,
		Short: "Tools for managing an OpenShift cluster",
		Long:  fmt.Sprintf(adminLong),
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(out)
			c.Help()
		},
	}

	f := clientcmd.New(cmds.PersistentFlags())

	cmds.AddCommand(project.NewCmdNewProject(project.NewProjectRecommendedName, fullName+" "+project.NewProjectRecommendedName, f, out))
	cmds.AddCommand(policy.NewCommandPolicy(policy.PolicyRecommendedName, fullName+" "+policy.PolicyRecommendedName, f, out))
	cmds.AddCommand(exipfailover.NewCmdIPFailoverConfig(f, fullName, "ipfailover", out))
	cmds.AddCommand(exrouter.NewCmdRouter(f, fullName, "router", out))
	cmds.AddCommand(exregistry.NewCmdRegistry(f, fullName, "registry", out))
	cmds.AddCommand(buildchain.NewCmdBuildChain(f, fullName, "build-chain"))
	cmds.AddCommand(node.NewCommandManageNode(f, node.ManageNodeCommandName, fullName+" "+node.ManageNodeCommandName, out))
	cmds.AddCommand(cmd.NewCmdConfig(fullName, "config"))
	cmds.AddCommand(prune.NewCommandPrune(prune.PruneRecommendedName, fullName+" "+prune.PruneRecommendedName, f, out))

	// TODO: these probably belong in a sub command
	cmds.AddCommand(admin.NewCommandCreateKubeConfig(admin.CreateKubeConfigCommandName, fullName+" "+admin.CreateKubeConfigCommandName, out))
	cmds.AddCommand(admin.NewCommandCreateBootstrapPolicyFile(admin.CreateBootstrapPolicyFileCommand, fullName+" "+admin.CreateBootstrapPolicyFileCommand, out))
	cmds.AddCommand(admin.NewCommandCreateBootstrapProjectTemplate(f, admin.CreateBootstrapProjectTemplateCommand, fullName+" "+admin.CreateBootstrapProjectTemplateCommand, out))
	cmds.AddCommand(admin.NewCommandOverwriteBootstrapPolicy(admin.OverwriteBootstrapPolicyCommandName, fullName+" "+admin.OverwriteBootstrapPolicyCommandName, fullName+" "+admin.CreateBootstrapPolicyFileCommand, out))
	cmds.AddCommand(admin.NewCommandNodeConfig(admin.NodeConfigCommandName, fullName+" "+admin.NodeConfigCommandName, out))
	// TODO: these should be rolled up together
	cmds.AddCommand(admin.NewCommandCreateMasterCerts(admin.CreateMasterCertsCommandName, fullName+" "+admin.CreateMasterCertsCommandName, out))
	cmds.AddCommand(admin.NewCommandCreateClient(admin.CreateClientCommandName, fullName+" "+admin.CreateClientCommandName, out))
	cmds.AddCommand(admin.NewCommandCreateKeyPair(admin.CreateKeyPairCommandName, fullName+" "+admin.CreateKeyPairCommandName, out))
	cmds.AddCommand(admin.NewCommandCreateServerCert(admin.CreateServerCertCommandName, fullName+" "+admin.CreateServerCertCommandName, out))
	cmds.AddCommand(admin.NewCommandCreateSignerCert(admin.CreateSignerCertCommandName, fullName+" "+admin.CreateSignerCertCommandName, out))

	if name == fullName {
		cmds.AddCommand(version.NewVersionCommand(fullName))
	}

	cmds.AddCommand(cmd.NewCmdOptions(out))

	return cmds
}
