package groups

import (
	"io"

	"github.com/spf13/cobra"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const GroupsRecommendedName = "groups"

const (
	groupLong = `
Manage groups in your cluster

Groups are sets of users that can be used when describing policy.`
)

func NewCmdGroups(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Manage groups",
		Long:  groupLong,
		Run:   cmdutil.DefaultSubCommandRun(out),
	}

	cmds.AddCommand(NewCmdNewGroup(NewGroupRecommendedName, fullName+" "+NewGroupRecommendedName, f, out))
	cmds.AddCommand(NewCmdAddUsers(AddRecommendedName, fullName+" "+AddRecommendedName, f, out))
	cmds.AddCommand(NewCmdRemoveUsers(RemoveRecommendedName, fullName+" "+RemoveRecommendedName, f, out))

	return cmds
}
