package cmd

import (
	"io"

	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/util"
)

// NewCmdCancelBuild manages a build cancelling event.
// To cancel a build its name has to be specified, and two options
// are available: displaying logs and restarting.
func (f *OriginFactory) NewCmdCancelBuild(out io.Writer) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "cancel-build <buildName>",
		Short: "Cancel a pending or running build.",
		Long: `Stop and delete a build pod, update build status to 'Cancelled'.
If dump-logs flag is specified then it will print the build logs.
If restart flag is specified then the build will restarted with a new pod.

Examples:
	$ kubectl cancel-build 1da32cvq
	<cancel the build with the given name>

	$ kubectl cancel-build 1da32cvq --dump-logs
	<cancel the build with the given name, and print the build logs>

	$kubectl cancel-build 1da32cvq --restart
	<cancel the build with the given name, and restart the build with a new pod>`,
		Run: func(cmd *cobra.Command, args []string) {

			if len(args) == 0 || len(args[0]) == 0 {
				usageError(cmd, "You must specify a build name.")
			}

			buildName := args[0]

			// Get build.
			mapping, namespace, _ := kubecmd.ResourceOrTypeFromArgs(cmd, []string{"build"}, f.Mapper)
			client, err := f.OriginClient(cmd, mapping)
			checkErr(err)
			resource, err := client.Get().Namespace(namespace).Path("builds").Path(buildName).Do().Get()
			checkErr(err)

			build := resource.(*buildapi.Build)
			if !isBuildCancellable(build) {
				return
			}

			// Print build logs before cancelling build.
			if kubecmd.GetFlagBool(cmd, "dump-logs") {
				// in order to dump logs, you must have a pod assigned to the build.  Since build pod creation is asynchronous, it is possible to cancel a build without a pod being assigned.
				if len(build.PodName) == 0 {
					glog.V(2).Infof("Build %v, does not have a pod assigned, so it does not have any logs.", buildName)

				} else {
					response, err := client.Get().Namespace(namespace).Path("redirect").Path("buildLogs").Path(buildName).Do().Raw()
					checkErr(err)
					glog.V(2).Infof("Build logs for %s:\n%v", buildName, string(response))
				}
			}

			// Mark build to be cancelled.
			build.Cancelled = true
			err = client.Put().Namespace(namespace).Path("builds").Path(buildName).Body(build).Do().Error()
			checkErr(err)
			glog.V(2).Infof("Build %s was cancelled.", buildName)

			// Create a new build with the same configuration.
			if kubecmd.GetFlagBool(cmd, "restart") {
				newBuild := util.GenerateBuildFromBuild(resource.(*buildapi.Build))
				err = client.Post().Namespace(namespace).Path("builds").Body(newBuild).Do().Error()
				checkErr(err)
				glog.V(2).Infof("Restarting build %s.", buildName)
			}
		},
	}

	cmd.Flags().Bool("dump-logs", false, "Specify if the build logs should be printed after it's cancelled.")
	cmd.Flags().Bool("restart", false, "Specify if the build should be restarted after it's cancelled.")
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
