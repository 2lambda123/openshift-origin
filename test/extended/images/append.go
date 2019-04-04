package images

import (
	"strings"

	"github.com/MakeNowJust/heredoc"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/image/dockerlayer"
	exutil "github.com/openshift/origin/test/extended/util"
)

func cliPodWithPullSecret(cli *exutil.CLI, shell string) *kapiv1.Pod {
	sa, err := cli.KubeClient().CoreV1().ServiceAccounts(cli.Namespace()).Get("builder", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(sa.ImagePullSecrets).NotTo(o.BeEmpty())
	pullSecretName := sa.ImagePullSecrets[0].Name

	cliImage, _ := exutil.FindCLIImage(cli)

	return &kapiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "append-test",
		},
		Spec: kapiv1.PodSpec{
			// so we have permission to push and pull to the registry
			ServiceAccountName: "builder",
			RestartPolicy:      kapiv1.RestartPolicyNever,
			Containers: []kapiv1.Container{
				{
					Name:    "test",
					Image:   cliImage,
					Command: []string{"/bin/bash", "-c", "set -euo pipefail; " + shell},
					Env: []kapiv1.EnvVar{
						{
							Name:  "HOME",
							Value: "/secret",
						},
					},
					VolumeMounts: []kapiv1.VolumeMount{
						{
							Name:      "pull-secret",
							MountPath: "/secret/.dockercfg",
							SubPath:   kapiv1.DockerConfigKey,
						},
					},
				},
			},
			Volumes: []kapiv1.Volume{
				{
					Name: "pull-secret",
					VolumeSource: kapiv1.VolumeSource{
						Secret: &kapiv1.SecretVolumeSource{
							SecretName: pullSecretName,
						},
					},
				},
			},
		},
	}
}

var _ = g.Describe("[Feature:ImageAppend] Image append", func() {
	defer g.GinkgoRecover()

	var oc *exutil.CLI
	var ns string

	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed && len(ns) > 0 {
			exutil.DumpPodLogsStartingWithInNamespace("", ns, oc)
		}
	})

	oc = exutil.NewCLI("image-append", exutil.KubeConfigPath())

	g.It("should create images by appending them", func() {
		is, err := oc.ImageClient().Image().ImageStreams("openshift").Get("php", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(is.Status.DockerImageRepository).NotTo(o.BeEmpty(), "registry not yet configured?")
		registry := strings.Split(is.Status.DockerImageRepository, "/")[0]

		ns = oc.Namespace()
		cli := oc.KubeFramework().PodClient()
		pod := cli.Create(cliPodWithPullSecret(oc, heredoc.Docf(`
			set -x

			# create a scratch image with fixed date
			oc image append --insecure --to %[2]s/%[1]s/test:scratch1 --image='{"Cmd":["/bin/sleep"]}' --created-at=0

			# create a second scratch image with fixed date
			oc image append --insecure --to %[2]s/%[1]s/test:scratch2 --image='{"Cmd":["/bin/sleep"]}' --created-at=0

			# modify a busybox image
			oc image append --insecure --from docker.io/library/busybox:latest --to %[2]s/%[1]s/test:busybox1 --image '{"Cmd":["/bin/sleep"]}'

			# verify mounting works
			oc create is test2
			oc image append --insecure --from %[2]s/%[1]s/test:scratch2 --to %[2]s/%[1]s/test2:scratch2 --force

			# add a simple layer to the image
			mkdir -p /tmp/test/dir
			touch /tmp/test/1
			touch /tmp/test/dir/2
			tar cvzf /tmp/layer.tar.gz -C /tmp/test/ .
			oc image append --insecure --from=%[2]s/%[1]s/test:busybox1 --to %[2]s/%[1]s/test:busybox2 /tmp/layer.tar.gz
		`, ns, registry)))
		cli.WaitForSuccess(pod.Name, podStartupTimeout)

		istag, err := oc.ImageClient().Image().ImageStreamTags(ns).Get("test:scratch1", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(istag.Image).NotTo(o.BeNil())
		o.Expect(istag.Image.DockerImageLayers).To(o.HaveLen(1))
		o.Expect(istag.Image.DockerImageLayers[0].Name).To(o.Equal(dockerlayer.GzippedEmptyLayerDigest))
		o.Expect(istag.Image.DockerImageMetadata.Config.Cmd).To(o.Equal([]string{"/bin/sleep"}))

		istag2, err := oc.ImageClient().Image().ImageStreamTags(ns).Get("test:scratch2", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(istag2.Image).NotTo(o.BeNil())
		o.Expect(istag2.Image.Name).To(o.Equal(istag.Image.Name))

		istag, err = oc.ImageClient().Image().ImageStreamTags(ns).Get("test:busybox1", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(istag.Image).NotTo(o.BeNil())
		o.Expect(istag.Image.DockerImageLayers).To(o.HaveLen(1))
		o.Expect(istag.Image.DockerImageLayers[0].Name).NotTo(o.Equal(dockerlayer.GzippedEmptyLayerDigest))
		o.Expect(istag.Image.DockerImageMetadata.Config.Cmd).To(o.Equal([]string{"/bin/sleep"}))
		busyboxLayer := istag.Image.DockerImageLayers[0].Name

		istag, err = oc.ImageClient().Image().ImageStreamTags(ns).Get("test:busybox2", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(istag.Image).NotTo(o.BeNil())
		o.Expect(istag.Image.DockerImageLayers).To(o.HaveLen(2))
		o.Expect(istag.Image.DockerImageLayers[0].Name).To(o.Equal(busyboxLayer))
		o.Expect(istag.Image.DockerImageLayers[1].LayerSize).NotTo(o.Equal(0))
		o.Expect(istag.Image.DockerImageMetadata.Config.Cmd).To(o.Equal([]string{"/bin/sleep"}))
	})
})
