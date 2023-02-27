package nodedetails

import (
	"context"
	"os"
	"path/filepath"
	"time"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"

	"k8s.io/client-go/kubernetes"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/spf13/cobra"
)

type auditLogSummaryOptions struct {
	ArtifactDir string

	ConfigFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams
}

func AuditLogSummaryCommand() *cobra.Command {
	o := &auditLogSummaryOptions{
		ConfigFlags: genericclioptions.NewConfigFlags(true),
		IOStreams: genericclioptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
	}
	cmd := &cobra.Command{
		Use:   "summarize-audit-logs",
		Short: "Download and inspect audit logs for interesting things.",

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.Run(context.Background())
		},
	}

	cmd.Flags().StringVar(&o.ArtifactDir, "artifact-dir", o.ArtifactDir, "The directory where monitor events will be stored.")
	o.ConfigFlags.AddFlags(cmd.Flags())
	return cmd
}

func (o auditLogSummaryOptions) Run(ctx context.Context) error {
	restConfig, err := o.ConfigFlags.ToRESTConfig()
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	auditLogSummary, monitorEvents, err := IntervalsFromAuditLogs(ctx, kubeClient, time.Time{}, time.Time{})
	if err != nil {
		return err
	}

	if err := WriteAuditLogSummary(o.ArtifactDir, "", auditLogSummary); err != nil {
		return err
	}
	if err := monitorserialization.EventsToFile(filepath.Join(o.ArtifactDir, "audit-events.json"), monitorEvents); err != nil {
		return err
	}

	return nil
}
