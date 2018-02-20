package kubelet

import (
	"fmt"
	"io"
	"os"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/tmpformac"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
)

const (
	NodeConfigDirName  = "oc-cluster-up-node"
	KubeDNSDirName     = "oc-cluster-up-kubedns"
	PodManifestDirName = "oc-cluster-up-pod-manifest"
)

type NodeStartConfig struct {
	// ContainerBinds is a list of local/path:image/path pairs
	ContainerBinds []string
	// NodeImage is the docker image for openshift start node
	NodeImage string

	Args []string
}

func NewNodeStartConfig() *NodeStartConfig {
	return &NodeStartConfig{
		ContainerBinds: []string{},
	}

}

func (opt NodeStartConfig) MakeKubeDNSConfig(dockerClient dockerhelper.Interface, imageRunHelper *run.Runner, out io.Writer) (string, error) {
	return opt.makeConfig(dockerClient, imageRunHelper, out, KubeDNSDirName)
}

func (opt NodeStartConfig) MakeNodeConfig(dockerClient dockerhelper.Interface, imageRunHelper *run.Runner, out io.Writer) (string, error) {
	return opt.makeConfig(dockerClient, imageRunHelper, out, NodeConfigDirName)
}

// Start starts the OpenShift master as a Docker container
// and returns a directory in the local file system where
// the OpenShift configuration has been copied
func (opt NodeStartConfig) makeConfig(dockerClient dockerhelper.Interface, imageRunHelper *run.Runner, out io.Writer, componentName string) (string, error) {
	fmt.Fprintf(out, "Creating initial OpenShift node configuration\n")
	createConfigCmd := []string{
		"adm", "create-node-config",
		fmt.Sprintf("--node-dir=%s", "/var/lib/origin/openshift.local.config"),
	}
	createConfigCmd = append(createConfigCmd, opt.Args...)

	containerId, _, err := imageRunHelper.Image(opt.NodeImage).
		Privileged().
		HostNetwork().
		HostPid().
		Bind(opt.ContainerBinds...).
		Entrypoint("oc").
		Command(createConfigCmd...).Run()
	if err != nil {
		return "", errors.NewError("could not create OpenShift configuration: %v", err).WithCause(err)
	}

	nodeConfigDir, err := tmpformac.TempDir(componentName)
	if err != nil {
		return "", err
	}
	glog.V(1).Infof("Copying OpenShift node config to local directory %s", nodeConfigDir)
	if err = dockerhelper.DownloadDirFromContainer(dockerClient, containerId, "/var/lib/origin/openshift.local.config", nodeConfigDir); err != nil {
		if removeErr := os.RemoveAll(nodeConfigDir); removeErr != nil {
			glog.V(2).Infof("Error removing temporary config dir %s: %v", nodeConfigDir, removeErr)
		}
		return "", err
	}

	return nodeConfigDir, nil
}
