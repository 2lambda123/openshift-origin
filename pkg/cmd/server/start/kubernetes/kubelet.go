package kubernetes

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/pkg/util"
)

const kubeletLog = `Start Kubelet

This command launches a Kubelet. All options are exposed. Use 'openshift start node' for
starting from a configuration file.`

// NewKubeletCommand provides a CLI handler for the 'kubelet' command
func NewKubeletCommand(name, fullName string, out io.Writer) *cobra.Command {
	s := app.NewKubeletServer()

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch the Kubelet (kubelet)",
		Long:  kubeletLog,
		Run: func(c *cobra.Command, args []string) {
			startProfiler()

			util.InitLogs()
			defer util.FlushLogs()

			if err := s.Run(nil); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()
	flags.SetNormalizeFunc(util.WordSepNormalizeFunc)
	flags.AddGoFlagSet(flag.CommandLine)
	s.AddFlags(flags)

	return cmd
}
