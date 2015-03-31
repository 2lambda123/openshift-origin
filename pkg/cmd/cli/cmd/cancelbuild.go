package cmd

import (
	"fmt"
	"io"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const cancelBuildLongDesc = `
Cancels a pending or running build.

Examples:

	# Cancel the build with the given name
	$ %[1]s cancel-build 1da32cvq

	# Cancel the named build and print the build logs
	$ %[1]s cancel-build 1da32cvq --dump-logs

	# Cancel the named build and create a new one with the same parameters
	$ %[1]s cancel-build 1da32cvq --restart
`

// NewCmdCancelBuild manages a build cancelling event.
// To cancel a build its name has to be specified, and two options
// are available: displaying logs and restarting.
func NewCmdCancelBuild(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "cancel-build <build>",
		Short: "Cancel a pending or running build.",
		Long:  fmt.Sprintf(cancelBuildLongDesc, fullName),
		Run: func(cmd *cobra.Command, args []string) {

			if len(args) == 0 || len(args[0]) == 0 {
				usageError(cmd, "You must specify the name of a build to cancel.")
			}

			buildName := args[0]
			namespace, err := f.DefaultNamespace()
			checkErr(err)

			client, _, err := f.Clients()
			checkErr(err)
			buildClient := client.Builds(namespace)
			build, err := buildClient.Get(buildName)
			checkErr(err)

			if !isBuildCancellable(build) {
				return
			}

			// Print build logs before cancelling build.
			if cmdutil.GetFlagBool(cmd, "dump-logs") {
				// in order to dump logs, you must have a pod assigned to the build.  Since build pod creation is asynchronous, it is possible to cancel a build without a pod being assigned.
				if build.Status != buildapi.BuildStatusRunning {
					glog.V(2).Infof("Build %v has not yet generated any logs.", buildName)

				} else {
					response, err := client.BuildLogs(namespace).Redirect(buildName).Do().Raw()
					if err != nil {
						glog.Errorf("Could not fetch build logs for %s: %v", buildName, err)
					} else {
						glog.V(2).Infof("Build logs for %s:\n%v", buildName, string(response))
					}
				}
			}

			// Mark build to be cancelled.
			for {
				build.Cancelled = true
				if _, err = buildClient.Update(build); err != nil && errors.IsConflict(err) {
					build, err = buildClient.Get(buildName)
					checkErr(err)
					continue
				}
				checkErr(err)
				break
			}
			glog.V(2).Infof("Build %s was cancelled.", buildName)

			// Create a new build with the same configuration.
			if cmdutil.GetFlagBool(cmd, "restart") {
				request := &buildapi.BuildRequest{
					ObjectMeta: kapi.ObjectMeta{Name: build.Name},
				}
				newBuild, err := client.Builds(namespace).Clone(request)
				checkErr(err)
				glog.V(2).Infof("Restarted build %s.", buildName)
				fmt.Fprintf(out, "%s\n", newBuild.Name)
			} else {
				fmt.Fprintf(out, "%s\n", build.Name)
			}
		},
	}

	cmd.Flags().Bool("dump-logs", false, "Specify if the build logs for the cancelled build should be shown.")
	cmd.Flags().Bool("restart", false, "Specify if a new build should be created after the current build is cancelled.")
	return cmd
}

// isBuildCancellable checks if another cancellation event was triggered, and if the build status is correct.
func isBuildCancellable(build *buildapi.Build) bool {
	if build.Status != buildapi.BuildStatusNew &&
		build.Status != buildapi.BuildStatusPending &&
		build.Status != buildapi.BuildStatusRunning {

		glog.V(2).Infof("A build can be cancelled only if it has new/pending/running status.")
		return false
	}

	if build.Cancelled {
		glog.V(2).Infof("A cancellation event was already triggered for the build %s.", build.Name)
		return false
	}
	return true
}
