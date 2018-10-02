package util

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/credentialprovider"

	"github.com/openshift/imagebuilder/imageprogress"
	starterrors "github.com/openshift/origin/pkg/oc/clusterup/docker/errors"
)

// Helper provides utility functions to help with Docker
type Helper struct {
	client Interface
	info   *types.Info
}

// NewHelper creates a new Helper
func NewHelper(client Interface) *Helper {
	return &Helper{
		client: client,
	}
}

func (h *Helper) Client() Interface {
	return h.client
}

func (h *Helper) dockerInfo() (*types.Info, error) {
	if h.info != nil {
		return h.info, nil
	}
	if h.client == nil {
		return nil, fmt.Errorf("the Docker engine API client is not initialized")
	}
	glog.V(5).Infof("Retrieving Docker daemon info")
	info, err := h.client.Info()
	if err != nil {
		glog.V(2).Infof("Could not retrieve Docker info: %v", err)
		return nil, err
	}
	glog.V(5).Infof("Docker daemon info: %#v", info)
	h.info = info
	return h.info, nil
}

func (h *Helper) CgroupDriver() (string, error) {
	info, err := h.dockerInfo()
	if err != nil {
		return "", err
	}
	return info.CgroupDriver, nil
}

var (
	fedoraPackage = regexp.MustCompile("\\.fc[0-9_]*\\.")
	rhelPackage   = regexp.MustCompile("\\.el[0-9_]*\\.")
)

// DockerRoot returns the root directory for Docker
func (h *Helper) DockerRoot() (string, error) {
	info, err := h.dockerInfo()
	if err != nil {
		return "", err
	}
	return info.DockerRootDir, nil
}

// Version returns the Docker API version and whether it is a Red Hat distro version
func (h *Helper) APIVersion() (*types.Version, error) {
	glog.V(5).Infof("Retrieving Docker version")
	version, err := h.client.ServerVersion()
	if err != nil {
		glog.V(2).Infof("Error retrieving version: %v", err)
		return nil, err
	}
	glog.V(5).Infof("Docker version results: %#v", version)
	if len(version.APIVersion) == 0 {
		return nil, errors.New("did not get an API version")
	}
	return version, nil
}

func (h *Helper) IsRedHat() (bool, error) {
	version, err := h.APIVersion()
	if err != nil {
		return false, err
	}
	if len(version.APIVersion) == 0 {
		return false, errors.New("did not get an API version")
	}
	kernelVersion := version.KernelVersion
	if len(kernelVersion) == 0 {
		return false, nil
	}
	return fedoraPackage.MatchString(kernelVersion) || rhelPackage.MatchString(kernelVersion), nil
}

func (h *Helper) GetDockerProxySettings() (httpProxy, httpsProxy, noProxy string, err error) {
	info, err := h.dockerInfo()
	if err != nil {
		return "", "", "", err
	}
	return info.HTTPProxy, info.HTTPSProxy, info.NoProxy, nil
}

// CheckAndPullImage checks existence of local container image and if the image is not available locally it pulls it.
// The forcePull option skip checking if the image exists and forces the pull.
func (h *Helper) CheckAndPullImage(image string, forcePull bool, out io.Writer) error {
	// If the forcePull option is set, skip checking if image exists locally and pull always
	if !forcePull {
		_, _, err := h.client.ImageInspectWithRaw(image, false)
		if err == nil {
			return nil
		}
		if !client.IsErrImageNotFound(err) {
			return starterrors.NewError("unexpected error inspecting image %s", image).WithCause(err)
		}
	}

	fmt.Fprintf(out, "Pulling container image %s ...\n", image)
	logProgress := func(s string) {
		fmt.Fprintf(out, "%s\n", s)
	}

	pw := imageprogress.NewPullWriter(logProgress)
	defer pw.Close()
	outputStream := pw.(io.Writer)

	normalized, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return err
	}

	auth := ""
	var cfgPath string
	if os.Getenv("DOCKERCFG_PATH") != "" {
		cfgPath = os.Getenv("DOCKERCFG_PATH")
	} else if currentUser, err := user.Current(); err == nil {
		cfgPath = filepath.Join(currentUser.HomeDir, ".docker", "config.json")
	}

	keyring := credentialprovider.BasicDockerKeyring{}
	if _, err := os.Stat(cfgPath); os.IsExist(err) {
		cfg, err := credentialprovider.ReadSpecificDockerConfigJsonFile(cfgPath)
		if err != nil {
			glog.Errorf("Reading docker config from %v failed: %v, will attempt to pull image %s anonymously", cfgPath, err, normalized.String())
		}
		keyring.Add(cfg)
	} else {
		glog.V(3).Infof("Docker config file not found in %q, will pull without credentials", cfgPath)
	}

	authConfs, found := keyring.Lookup(normalized.String())
	if found && len(authConfs) > 0 {
		glog.V(3).Infof("Using %s user for Docker authentication for image %s", authConfs[0].Username, normalized.String())
		authConfig := types.AuthConfig{
			Username:      authConfs[0].Username,
			Password:      authConfs[0].Password,
			ServerAddress: authConfs[0].ServerAddress,
			IdentityToken: authConfs[0].IdentityToken,
			RegistryToken: authConfs[0].RegistryToken,
		}

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(authConfig); err != nil {
			return starterrors.NewError("error encoding docker credentials").WithCause(err)
		}
		auth = base64.URLEncoding.EncodeToString(buf.Bytes())
	}

	if err := h.client.ImagePull(normalized.String(), types.ImagePullOptions{RegistryAuth: auth}, outputStream); err != nil {
		return starterrors.NewError("error pulling image %s", image).WithCause(err)
	}

	// This is to work around issue https://github.com/docker/docker/api/issues/138
	// where engine-api/client/ImagePull does not return an error when it should.
	// which also still seems to exist in https://github.com/moby/moby/blob/master/client/image_pull.go
	_, _, err = h.client.ImageInspectWithRaw(image, false)
	if err != nil {
		return starterrors.NewError("error pulling image %s", image).WithCause(err)
	}

	fmt.Fprintln(out, fmt.Sprintf("Image %s successfully pulled", image))
	return nil
}

// GetContainerState returns whether a container exists and if it does whether it's running
func (h *Helper) GetContainerState(id string) (*types.ContainerJSON, bool, error) {
	glog.V(5).Infof("Inspecting docker container %q", id)
	container, err := h.client.ContainerInspect(id)
	if err != nil {
		if client.IsErrContainerNotFound(err) {
			glog.V(5).Infof("Container %q was not found", id)
			return nil, false, nil
		}
		glog.V(5).Infof("An error occurred inspecting container %q: %v", id, err)
		return nil, false, err
	}
	glog.V(5).Infof("Container inspect result: %#v", container)

	running := container.State != nil && container.State.Running
	glog.V(5).Infof("Container running = %v", running)
	return container, running, nil
}

// RemoveContainer removes the container with the given id
func (h *Helper) RemoveContainer(id string) error {
	glog.V(5).Infof("Removing container %q", id)
	err := h.client.ContainerRemove(id, types.ContainerRemoveOptions{})
	if err != nil {
		return starterrors.NewError("cannot delete container %s", id).WithCause(err)
	}
	glog.V(5).Infof("Removed container %q", id)
	return nil
}

// HostIP returns the IP of the Docker host if connecting via TCP
func (h *Helper) HostIP() string {
	// By default, if the Docker client uses tcp, then use the Docker daemon's address
	endpoint := h.client.Endpoint()
	u, err := url.Parse(endpoint)
	if err == nil && (u.Scheme == "tcp" || u.Scheme == "http" || u.Scheme == "https") {
		glog.V(2).Infof("Using the Docker host %s for the server IP", endpoint)
		if host, _, serr := net.SplitHostPort(u.Host); serr == nil {
			return host
		}
		return u.Host
	}
	glog.V(5).Infof("Cannot use Docker endpoint (%s) because it is not using one of the following protocols: tcp, http, https", endpoint)
	return ""
}

func (h *Helper) ContainerLog(container string, numLines int) string {
	outBuf := &bytes.Buffer{}
	if err := h.client.ContainerLogs(container, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Tail: strconv.Itoa(numLines)}, outBuf, outBuf); err != nil {
		glog.V(2).Infof("Error getting container %q log: %v", container, err)
	}
	return outBuf.String()
}

func (h *Helper) StopAndRemoveContainer(container string) error {
	err := h.client.ContainerStop(container, 10)
	if err != nil {
		glog.V(2).Infof("Cannot stop container %s: %v", container, err)
	}
	return h.RemoveContainer(container)
}

func (h *Helper) ListContainerNames() ([]string, error) {
	containers, err := h.client.ContainerList(types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, c := range containers {
		names = append(names, c.Names...)
	}
	return names, nil
}

// UserNamespaceEnabled checks whether docker daemon is running in user
// namespace mode.
func (h *Helper) UserNamespaceEnabled() (bool, error) {
	info, err := h.dockerInfo()
	if err != nil {
		return false, err
	}
	for _, val := range info.SecurityOptions {
		if val == "name=userns" {
			return true, nil
		}
	}
	return false, nil
}
