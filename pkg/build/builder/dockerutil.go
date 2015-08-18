package builder

import (
	"fmt"
	"os"
	"time"

	"k8s.io/kubernetes/pkg/util"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/tar"
)

var (
	// DefaultPushRetryCount is the number of retries of pushing the built Docker image
	// into a configured repository
	DefaultPushRetryCount = 2
	// DefaultPushRetryDelay is the time to wait before triggering a push retry
	DefaultPushRetryDelay = 10 * time.Second
)

// DockerClient is an interface to the Docker client that contains
// the methods used by the common builder
type DockerClient interface {
	BuildImage(opts docker.BuildImageOptions) error
	PushImage(opts docker.PushImageOptions, auth docker.AuthConfiguration) error
	RemoveImage(name string) error
}

// pushImage pushes a docker image to the registry specified in its tag
func pushImage(client DockerClient, name string, authConfig docker.AuthConfiguration) error {
	repository, tag := docker.ParseRepositoryTag(name)
	opts := docker.PushImageOptions{
		Name: repository,
		Tag:  tag,
	}
	if glog.V(5) {
		opts.OutputStream = os.Stderr
	}
	var err error
	for retries := 0; retries <= DefaultPushRetryCount; retries++ {
		err = client.PushImage(opts, authConfig)
		if err == nil {
			return nil
		}
		if retries == DefaultPushRetryCount {
			return err
		}
		util.HandleError(fmt.Errorf("push for image %s failed with error %v, will retry in %s ...", name, err, DefaultPushRetryDelay))
		glog.Flush()
		time.Sleep(DefaultPushRetryDelay)
	}
	return err
}

func removeImage(client DockerClient, name string) error {
	return client.RemoveImage(name)
}

// buildImage invokes a docker build on a particular directory
func buildImage(client DockerClient, dir string, noCache bool, tag string, tar tar.Tar, pullAuth *docker.AuthConfigurations, forcePull bool) error {
	tarFile, err := tar.CreateTarFile("", dir)
	if err != nil {
		return err
	}
	tarStream, err := os.Open(tarFile)
	if err != nil {
		return err
	}
	defer tarStream.Close()
	opts := docker.BuildImageOptions{
		Name:           tag,
		RmTmpContainer: true,
		OutputStream:   os.Stdout,
		InputStream:    tarStream,
		NoCache:        noCache,
		Pull:           forcePull,
	}
	if pullAuth != nil {
		opts.AuthConfigs = *pullAuth
	}
	return client.BuildImage(opts)
}
