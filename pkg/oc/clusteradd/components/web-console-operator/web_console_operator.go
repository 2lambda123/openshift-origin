package web_console_operator

import (
	"fmt"
	"net/url"
	"path"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	operatorversionclient "github.com/openshift/origin/pkg/cmd/openshift-operators/generated/clientset/versioned"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/kubeapiserver"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/clusterup/manifests"
)

const (
	namespace = "openshift-core-operators"
)

type WebConsoleOperatorComponentOptions struct {
	InstallContext componentinstall.Context
}

func (c *WebConsoleOperatorComponentOptions) Name() string {
	return "openshift-web-console-operator"
}

func (c *WebConsoleOperatorComponentOptions) Install(dockerClient dockerhelper.Interface) error {
	kubeAdminClient, err := kubernetes.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return err
	}

	// create the actual resources required
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = c.InstallContext.ImageFormat()
	imageTemplate.Latest = false

	params := map[string]string{
		"IMAGE":                 imageTemplate.ExpandOrDie("hypershift"),
		"LOGLEVEL":              fmt.Sprintf("%d", c.InstallContext.ComponentLogLevel()),
		"COMPONENT_IMAGE":       imageTemplate.ExpandOrDie("web-console"),
		"COMPONENT_LOGLEVEL":    fmt.Sprintf("%d", c.InstallContext.ComponentLogLevel()),
		"NAMESPACE":             namespace,
		"OPENSHIFT_PULL_POLICY": c.InstallContext.ImagePullPolicy(),
	}

	glog.V(2).Infof("instantiating webconsole-operator template with parameters %v", params)

	component := componentinstall.Template{
		Name:            "openshift-web-console-operator",
		Namespace:       namespace,
		RBACTemplate:    manifests.MustAsset("install/openshift-web-console-operator/install-rbac.yaml"),
		InstallTemplate: manifests.MustAsset("install/openshift-web-console-operator/install.yaml"),

		// wait until the webconsole to an available endpoint
		WaitCondition: func() (bool, error) {
			glog.V(2).Infof("polling for web-console availability ...")
			deployment, err := kubeAdminClient.AppsV1().Deployments("openshift-web-console").Get("webconsole", metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if deployment.Status.AvailableReplicas == 0 {
				return false, nil
			}
			return true, nil
		},
	}
	err = component.MakeReady(
		c.InstallContext.ClientImage(),
		c.InstallContext.BaseDir(),
		params).Install(dockerClient)
	if err != nil {
		return err
	}

	configPath := path.Join(c.InstallContext.BaseDir(), kubeapiserver.KubeAPIServerDirName, "master-config.yaml")
	masterConfig, err := componentinstall.ReadMasterConfig(configPath)
	if err != nil {
		return err
	}

	// we to selectively add to the config, so we'll do this post installation.
	operatorClient, err := operatorversionclient.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return err
	}
	// we can race a controller.  It's not a big deal if we're a little late, so retry on conflict. It's easier than a patch.
	backoff := retry.DefaultBackoff
	backoff.Steps = 6
	err = retry.RetryOnConflict(backoff, func() error {
		operatorConfig, err := operatorClient.WebconsoleV1alpha1().OpenShiftWebConsoleConfigs().Get("instance", metav1.GetOptions{})
		if err != nil {
			return err
		}

		masterPublicURL, err := url.Parse(masterConfig.MasterPublicURL)
		if err != nil {
			return err
		}
		masterPublicHostPort := masterPublicURL.Host
		operatorConfig.Spec.WebConsoleConfig.ClusterInfo.ConsolePublicURL = "https://" + masterPublicHostPort + "/console/"
		operatorConfig.Spec.WebConsoleConfig.ClusterInfo.MasterPublicURL = masterConfig.MasterPublicURL
		if err != nil {
			return err
		}
		_, err = operatorClient.WebconsoleV1alpha1().OpenShiftWebConsoleConfigs().Update(operatorConfig)
		return err
	})
	if err != nil {
		return err
	}

	return nil
}
