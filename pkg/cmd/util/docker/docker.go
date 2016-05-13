package docker

import (
	"os"

	"k8s.io/kubernetes/pkg/kubelet/dockertools"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/spf13/pflag"
)

// Helper contains all the valid config options for connecting to Docker from
// a command line.
type Helper struct {
}

// NewHelper creates a Flags object with the default values set.  Use this
// to use consistent Docker client loading behavior from different contexts.
func NewHelper() *Helper {
	return &Helper{}
}

// InstallFlags installs the Docker flag helper into a FlagSet with the default
// options and default values from the Helper object.
func (_ *Helper) InstallFlags(flags *pflag.FlagSet) {
}

// GetClient returns a valid Docker client, the address of the client, or an error
// if the client couldn't be created.
func (_ *Helper) GetClient() (client *docker.Client, endpoint string, err error) {
	client, err = docker.NewClientFromEnv()
	if len(os.Getenv("DOCKER_HOST")) > 0 {
		endpoint = os.Getenv("DOCKER_HOST")
	} else {
		endpoint = "unix:///var/run/docker.sock"
	}
	return
}

type KubeDocker struct {
	dockertools.DockerInterface
}

func (c *KubeDocker) Ping() error {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		return err
	}
	return client.Ping()
}

func (_ *Helper) GetKubeClient() (*KubeDocker, string, error) {
	var endpoint string
	if len(os.Getenv("DOCKER_HOST")) > 0 {
		endpoint = os.Getenv("DOCKER_HOST")
	} else {
		endpoint = "unix:///var/run/docker.sock"
	}
	client := dockertools.ConnectToDockerOrDie(endpoint)
	originClient := &KubeDocker{client}
	return originClient, endpoint, nil
}

// GetClientOrExit returns a valid Docker client and the address of the client,
// or prints an error and exits.
func (h *Helper) GetClientOrExit() (*docker.Client, string) {
	client, addr, err := h.GetClient()
	if err != nil {
		glog.Fatalf("ERROR: Couldn't connect to Docker at %s.\n%v\n.", addr, err)
	}
	return client, addr
}
