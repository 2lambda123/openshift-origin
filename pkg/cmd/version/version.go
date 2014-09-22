package version

import (
	"fmt"

	"github.com/openshift/origin/pkg/cmd/util/formatting"
	"github.com/openshift/origin/pkg/version"
	"github.com/spf13/cobra"
)

func NewCommandVersion(name string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Command '%s' (main)", name),
		Long:  fmt.Sprintf("Command '%s' (main)", name),
		Run: func(c *cobra.Command, args []string) {
			formatting.Printfln("OpenShift %v", formatting.Strong(version.Get().String()))
		},
	}
}
