package openshift

import (
	"io"
	"io/ioutil"

	"k8s.io/apimachinery/pkg/api/errors"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/oc/cli/cmd"
	"github.com/openshift/origin/pkg/oc/cli/config"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	projectclientinternal "github.com/openshift/origin/pkg/project/generated/internalclientset"
)

// createProject creates a project
func CreateProject(f *clientcmd.Factory, name, display, desc, basecmd string, out io.Writer) error {
	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}
	projectClient, err := projectclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	pathOptions := config.NewPathOptionsWithConfig("")
	opt := &cmd.RequestProjectOptions{
		ProjectName: name,
		DisplayName: display,
		Description: desc,

		Name: basecmd,

		Client: projectClient.Project(),

		ProjectOptions: &cmd.ProjectOptions{PathOptions: pathOptions},

		IOStreams: genericclioptions.IOStreams{Out: ioutil.Discard, ErrOut: ioutil.Discard},
	}
	err = opt.ProjectOptions.Complete(f, []string{})
	if err != nil {
		return err
	}
	err = opt.Run()
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return setCurrentProject(f, name, out)
		}
		return err
	}
	return nil
}

func setCurrentProject(f *clientcmd.Factory, name string, out io.Writer) error {
	pathOptions := config.NewPathOptionsWithConfig("")
	opt := &cmd.ProjectOptions{PathOptions: pathOptions, IOStreams: genericclioptions.IOStreams{Out: out, ErrOut: ioutil.Discard}}
	opt.Complete(f, []string{name})
	return opt.RunProject()
}

func LoggedInUserFactory() (*clientcmd.Factory, error) {
	cfg, err := kclientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, err
	}
	defaultCfg := kclientcmd.NewDefaultClientConfig(*cfg, &kclientcmd.ConfigOverrides{})
	return clientcmd.NewFactory(defaultCfg), nil
}
