package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	cliconfig "github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	projectapi "github.com/openshift/origin/pkg/project/api"
)

type NewProjectOptions struct {
	ProjectName  string
	DisplayName  string
	Description  string
	NodeSelector string

	Client client.Interface

	ProjectOptions *ProjectOptions
	Out            io.Writer
}

const (
	requestProjectLong = `Create a new project for yourself in OpenShift with you as the project admin.

Assuming your cluster admin has granted you permission, this command will 
create a new project for you and assign you as the project admin.  You must 
be logged in, so you might have to run %[1]s first.

After your project is created you can switch to it using %[2]s <project name>.`

	requestProjectExample = `  // Create a new project with minimal information
  $ %[1]s web-team-dev

  // Create a new project with a description and node selector
  $ %[1]s web-team-dev --display-name="Web Team Development" --description="Development project for the web team." --node-selector="env=dev"`
)

var CheckNodeSelector bool

func NewCmdRequestProject(name, fullName, oscLoginName, oscProjectName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &NewProjectOptions{}
	options.Out = out

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s NAME [--display-name=DISPLAYNAME] [--description=DESCRIPTION] [--node-selector=<label selector>]", name),
		Short:   "Request a new project",
		Long:    fmt.Sprintf(requestProjectLong, oscLoginName, oscProjectName),
		Example: fmt.Sprintf(requestProjectExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.complete(cmd, f); err != nil {
				kcmdutil.CheckErr(err)
			}

			var err error
			if options.Client, _, err = f.Clients(); err != nil {
				kcmdutil.CheckErr(err)
			}

			// We can't depend on len(NodeSelector) > 0 as node-selector="" is valid
			// and we want to populate node selector as annotation only if explicitly set by user
			CheckNodeSelector = cmd.Flag("node-selector").Changed

			if err := options.Run(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}
	cmd.SetOutput(out)

	cmd.Flags().StringVar(&options.DisplayName, "display-name", "", "project display name")
	cmd.Flags().StringVar(&options.Description, "description", "", "project description")
	cmd.Flags().StringVar(&options.NodeSelector, "node-selector", "", "Restrict pods onto nodes matching given label selector. Format: '<key1>=<value1>, <key2>=<value2>...'")

	return cmd
}

func (o *NewProjectOptions) complete(cmd *cobra.Command, f *clientcmd.Factory) error {
	args := cmd.Flags().Args()
	if len(args) != 1 {
		cmd.Help()
		return errors.New("must have exactly one argument")
	}

	o.ProjectName = args[0]

	o.ProjectOptions = &ProjectOptions{}
	o.ProjectOptions.PathOptions = cliconfig.NewPathOptions(cmd)
	if err := o.ProjectOptions.Complete(f, []string{""}, o.Out); err != nil {
		return err
	}

	return nil
}

func (o *NewProjectOptions) Run() error {
	// TODO eliminate this when we get better forbidden messages
	_, err := o.Client.ProjectRequests().List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}

	projectRequest := &projectapi.ProjectRequest{}
	projectRequest.Name = o.ProjectName
	projectRequest.DisplayName = o.DisplayName
	projectRequest.Annotations = make(map[string]string)
	projectRequest.Annotations["description"] = o.Description
	if CheckNodeSelector {
		projectRequest.Annotations["openshift.io/node-selector"] = o.NodeSelector
	}

	project, err := o.Client.ProjectRequests().Create(projectRequest)
	if err != nil {
		return err
	}

	if o.ProjectOptions != nil {
		o.ProjectOptions.ProjectName = project.Name
		o.ProjectOptions.ProjectOnly = true

		if err := o.ProjectOptions.RunProject(); err != nil {
			return err
		}
	}

	return nil
}
