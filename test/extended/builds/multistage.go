package builds

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	buildv1 "github.com/openshift/api/build/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds] Multi-stage image builds", func() {
	defer g.GinkgoRecover()
	var (
		oc             = exutil.NewCLI("build-multistage", exutil.KubeConfigPath())
		testDockerfile = `
FROM scratch as test
USER 1001
FROM centos:7
COPY --from=test /usr/bin/curl /test/
`
	)

	g.Context("", func() {

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.It("should succeed [Conformance]", func() {
			g.By("creating a build directly")
			is, err := oc.ImageClient().Image().ImageStreams("openshift").Get("php", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(is.Status.DockerImageRepository).NotTo(o.BeEmpty(), "registry not yet configured?")
			registry := strings.Split(is.Status.DockerImageRepository, "/")[0]

			build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Create(&buildv1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multi-stage",
				},
				Spec: buildv1.BuildSpec{
					CommonSpec: buildv1.CommonSpec{
						Source: buildv1.BuildSource{
							Dockerfile: &testDockerfile,
							Images: []buildv1.ImageSource{
								{From: corev1.ObjectReference{Kind: "DockerImage", Name: "centos:7"}, As: []string{"scratch"}},
							},
						},
						Strategy: buildv1.BuildStrategy{
							DockerStrategy: &buildv1.DockerBuildStrategy{},
						},
						Output: buildv1.BuildOutput{
							To: &corev1.ObjectReference{
								Kind: "DockerImage",
								Name: fmt.Sprintf("%s/%s/multi-stage:v1", registry, oc.Namespace()),
							},
						},
					},
				},
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			result := exutil.NewBuildResult(oc, build)
			err = exutil.WaitForBuildResult(oc.AdminBuildClient().BuildV1().Builds(oc.Namespace()), result)
			o.Expect(err).NotTo(o.HaveOccurred())

			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(build.Name+"-build", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(result.BuildSuccess).To(o.BeTrue(), "Build did not succeed: %#v", result)

			s, err := result.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(s).ToNot(o.ContainSubstring("--> FROM scratch"))
			o.Expect(s).To(o.ContainSubstring("COPY --from"))
			o.Expect(s).To(o.ContainSubstring(fmt.Sprintf("\"OPENSHIFT_BUILD_NAMESPACE\"=\"%s\"", oc.Namespace())))
			e2e.Logf("Build logs:\n%s", result)

			c := oc.KubeFramework().PodClient()
			pod = c.Create(&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "run",
							Image:   fmt.Sprintf("%s/%s/multi-stage:v1", registry, oc.Namespace()),
							Command: []string{"/test/curl", "-k", "https://kubernetes.default.svc"},
						},
					},
				},
			})
			c.WaitForSuccess(pod.Name, e2e.PodStartTimeout)
			data, err := c.GetLogs(pod.Name, &corev1.PodLogOptions{}).DoRaw()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("Pod logs:\n%s", string(data))
		})
	})
})
