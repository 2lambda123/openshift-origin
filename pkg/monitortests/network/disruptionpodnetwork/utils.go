package disruptionpodnetwork

import (
	"bytes"
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// GetOpenshiftTestsImagePullSpec returns the pull spec or an error.
// IN ginkgo environment, oc needs to be created before BeforeEach and passed in
func GetOpenshiftTestsImagePullSpec(ctx context.Context, adminRESTConfig *rest.Config, suggestedPayloadImage string, oc *exutil.CLI) (string, error) {
	if len(suggestedPayloadImage) == 0 {
		configClient, err := configclient.NewForConfig(adminRESTConfig)
		if err != nil {
			return "", err
		}
		clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("clusterversion/version not found and no image pull spec specified")
		}
		if err != nil {
			return "", err
		}
		suggestedPayloadImage = clusterVersion.Status.History[0].Image
	}

	logrus.Infof("payload image reported by CV: %v\n", suggestedPayloadImage)
	// runImageExtract extracts src from specified image to dst
	cmd := exec.Command("oc", "adm", "release", "info", suggestedPayloadImage, "--image-for=tests")
	out := &bytes.Buffer{}
	outStr := ""
	errOut := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		logrus.WithError(err).Errorf("unable to determine openshift-tests image through exec: %v", errOut.String())
		// Now try the wrapper to see if it makes a difference
		if oc == nil {
			oc = exutil.NewCLI("openshift-tests", exutil.WithoutNamespace())
		}
		outStr, err = oc.Run("adm", "release", "info", suggestedPayloadImage).Args("--image-for=tests").Output()
		if err != nil {
			logrus.WithError(err).Errorf("unable to determine openshift-tests image through oc wrapper with default ps: %v", outStr)

			kubeClient := oc.AdminKubeClient()
			// Try to use the same pull secret as the cluster under test
			imagePullSecret, err := kubeClient.CoreV1().Secrets("openshift-config").Get(context.Background(), "pull-secret", metav1.GetOptions{})
			if err != nil {
				logrus.WithError(err).Errorf("unable to get pull secret from cluster: %v", err)
				return "", fmt.Errorf("unable to get pull secret from cluster: %v", err)
			}

			// cache file to local temp location
			imagePullFile, err := ioutil.TempFile("", "image-pull-secret")
			if err != nil {
				logrus.WithError(err).Errorf("unable to create a temporary file: %v", err)
				return "", fmt.Errorf("unable to create a temporary file: %v", err)
			}
			defer os.Remove(imagePullFile.Name())

			// write the content
			imagePullSecretBytes := imagePullSecret.Data[".dockerconfigjson"]
			if _, err = imagePullFile.Write(imagePullSecretBytes); err != nil {
				logrus.WithError(err).Errorf("unable to write pull secret to temp file: %v", err)
				return "", fmt.Errorf("unable to write pull secret to temp file: %v", err)
			}
			if err = imagePullFile.Close(); err != nil {
				logrus.WithError(err).Errorf("unable to close file: %v", err)
				return "", fmt.Errorf("unable to close file: %v", err)
			}

			outStr, err = oc.Run("adm", "release", "info", suggestedPayloadImage).Args("--image-for=tests", "--registry-config", imagePullFile.Name()).Output()
			if err != nil {
				logrus.WithError(err).Errorf("unable to determine openshift-tests image through oc wrapper with cluster ps")

				// What is the mirror mode

				return "", fmt.Errorf("unable to determine openshift-tests image oc wrapper with cluster ps: %v", err)
			} else {
				logrus.Infof("successfully getting image for test with oc wrapper with cluster ps: %s\n", outStr)
			}
		} else {
			logrus.Infof("successfully getting image for test with oc wrapper with default ps: %s\n", outStr)
		}
	} else {
		outStr = out.String()
	}

	openshiftTestsImagePullSpec := strings.TrimSpace(outStr)
	fmt.Printf("openshift-tests image pull spec is %v\n", openshiftTestsImagePullSpec)

	return openshiftTestsImagePullSpec, nil
}
