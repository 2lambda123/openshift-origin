package componentinstall

import (
	"io/ioutil"
	"path"

	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubeapiserver"
	restclient "k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
)

const adminKubeConfigFileName = "admin.kubeconfig"

type Context interface {
	ClusterAdminClientConfig() *restclient.Config
	BaseDir() string
	ClientImage() string
	ImageFormat() string
	ComponentLogLevel() int
}

type installContext struct {
	restConfig  *restclient.Config
	clientImage string
	imageFormat string
	baseDir     string
	logLevel    int
}

// ImageFormat returns the format of the images to use when running commands like 'oc adm'
func (c *installContext) ImageFormat() string {
	return c.imageFormat
}

// ComponentLogLevel tells what log level user desire for the component
func (c *installContext) ComponentLogLevel() int {
	return c.logLevel
}

// ClusterAdminClientConfig provides a cluster admin REST client config
func (c *installContext) ClusterAdminClientConfig() *restclient.Config {
	return c.restConfig
}

// BaseDir provides the base directory with all configuration files
func (c *installContext) BaseDir() string {
	return c.baseDir
}

// ClientImage returns the name of the Docker image that provide the 'oc' binary
func (c *installContext) ClientImage() string {
	return c.clientImage
}

func NewComponentInstallContext(clientImageName, imageFormat, baseDir string, logLevel int) (Context, error) {
	clusterAdminConfigBytes, err := ioutil.ReadFile(path.Join(baseDir, kubeapiserver.KubeAPIServerDirName, adminKubeConfigFileName))
	if err != nil {
		return nil, err
	}
	restConfig, err := kclientcmd.RESTConfigFromKubeConfig(clusterAdminConfigBytes)
	if err != nil {
		return nil, err
	}
	return &installContext{
		restConfig:  restConfig,
		clientImage: clientImageName,
		baseDir:     baseDir,
		logLevel:    logLevel,
		imageFormat: imageFormat,
	}, nil
}
