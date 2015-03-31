package cmd

import (
	"fmt"
	"io"
	"os"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	newcmd "github.com/openshift/origin/pkg/generate/app/cmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type usage interface {
	UsageError(commandName string) string
}

const newAppLongDesc = `
Create a new application in OpenShift by specifying source code, templates, and/or images.

This command will try to build up the components of an application using images or code
located on your system. It will lookup the images on the local Docker installation (if
available), a Docker registry, or an OpenShift image repository. If you specify a source
code URL, it will set up a build that takes your source code and converts it into an
image that can run inside of a pod. The images will be deployed via a deployment
configuration, and a service will be hooked up to the first public port of the app.

Examples:

	# Try to create an application based on the source code in the current directory
	$ %[1]s new-app .

	$ Use the public Docker Hub MySQL image to create an app
	$ %[1]s new-app mysql

	# Use a MySQL image in a private registry to create an app
	$ %[1]s new-app myregistry.com/mycompany/mysql

If you specify source code, you may need to run a build with 'start-build' after the
application is created.

ALPHA: This command is under active development - feedback is appreciated.
`

func NewCmdNewApplication(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	config := newcmd.NewAppConfig()

	helper := dockerutil.NewHelper()
	cmd := &cobra.Command{
		Use:   "new-app <components> [--code=<path|url>]",
		Short: "Create a new application",
		Long:  fmt.Sprintf(newAppLongDesc, fullName),

		Run: func(c *cobra.Command, args []string) {
			namespace, err := f.DefaultNamespace()
			checkErr(err)

			if dockerClient, _, err := helper.GetClient(); err == nil {
				if err := dockerClient.Ping(); err == nil {
					config.SetDockerClient(dockerClient)
				} else {
					glog.V(2).Infof("No local Docker daemon detected: %v", err)
				}
			}

			osclient, _, err := f.Clients()
			if err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}
			config.SetOpenShiftClient(osclient, namespace)

			unknown := config.AddArguments(args)
			if len(unknown) != 0 {
				glog.Fatalf("Did not recognize the following arguments: %v", unknown)
			}

			result, err := config.Run(out)
			if err != nil {
				if errs, ok := err.(errors.Aggregate); ok {
					if len(errs.Errors()) == 1 {
						err = errs.Errors()[0]
					}
				}
				if err == newcmd.ErrNoInputs {
					// TODO: suggest things to the user
					glog.Fatal("You must specify one or more images, image repositories, or source code locations to create an application.")
				}
				if u, ok := err.(usage); ok {
					glog.Fatal(u.UsageError(c.CommandPath()))
				}
				glog.Fatalf("Error: %v", err)
			}

			if len(cmdutil.GetFlagString(c, "output")) != 0 {
				if err := f.Factory.PrintObject(c, result.List, out); err != nil {
					glog.Fatalf("Error: %v", err)
				}
				return
			}

			bulk := configcmd.Bulk{
				Factory: f.Factory,
				After:   configcmd.NewPrintNameOrErrorAfter(out, os.Stderr),
			}
			if errs := bulk.Create(result.List, namespace); len(errs) != 0 {
				os.Exit(1)
			}

			hasMissingRepo := false
			for _, item := range result.List.Items {
				switch t := item.(type) {
				case *kapi.Service:
					fmt.Fprintf(os.Stderr, "Service %q created at %s:%d to talk to pods over port %d.\n", t.Name, t.Spec.PortalIP, t.Spec.Port, t.Spec.TargetPort.IntVal)
				case *buildapi.BuildConfig:
					fmt.Fprintf(os.Stderr, "A build was created - you can run `osc start-build %s` to start it.\n", t.Name)
				case *imageapi.ImageRepository:
					if len(t.Status.DockerImageRepository) == 0 {
						if hasMissingRepo {
							continue
						}
						hasMissingRepo = true
						fmt.Fprintf(os.Stderr, "WARNING: We created an image repository %q, but it does not look like a Docker registry has been integrated with the OpenShift server. Automatic builds and deployments depend on that integration to detect new images and will not function properly.\n", t.Name)
					}
				}
			}
		},
	}

	cmd.Flags().Var(&config.SourceRepositories, "code", "Source code to use to build this application.")
	cmd.Flags().VarP(&config.ImageStreams, "image", "i", "Name of an OpenShift image repository to use in the app.")
	cmd.Flags().Var(&config.DockerImages, "docker-image", "Name of a Docker image to include in the app.")
	cmd.Flags().Var(&config.Groups, "group", "Indicate components that should be grouped together as <comp1>+<comp2>.")
	cmd.Flags().VarP(&config.Environment, "env", "e", "Specify key value pairs of environment variables to set into each container.")
	cmd.Flags().StringVar(&config.TypeOfBuild, "build", "", "Specify the type of build to use if you don't want to detect (docker|source)")

	cmdutil.AddPrinterFlags(cmd)

	return cmd
}
