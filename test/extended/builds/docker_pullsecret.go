package builds

import (
	"fmt"
	"os"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][pullsecret][Conformance] docker build using a pull secret", func() {
	defer g.GinkgoRecover()
	const (
		buildTestPod     = "build-test-pod"
		buildTestService = "build-test-svc"
	)

	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-docker-build-pullsecret.json")
		oc           = exutil.NewCLI("docker-build-pullsecret", exutil.KubeConfigPath())
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.AdminKubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("Building from a template", func() {
			g.It("should create a docker build that pulls using a secret run it", func() {
				oc.SetOutputDir(exutil.TestContext.OutputDir)

				g.By(fmt.Sprintf("calling oc create -f %q", buildFixture))
				err := oc.Run("create").Args("-f", buildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting a build")
				br, err := exutil.StartBuildAndWait(oc, "docker-build")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()

				g.By("starting a second build that pulls the image from the first build")
				br, err = exutil.StartBuildAndWait(oc, "docker-build-pull")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()

				ist, err := oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("image1:latest", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				fmt.Fprintf(os.Stderr, "ist.Name: %s\nist.Image.DockerImageReference: %s\n",
					ist.Name, ist.Image.DockerImageReference)

				is, err := oc.ImageClient().Image().ImageStreams(oc.Namespace()).Get("image1", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				fmt.Fprintf(os.Stderr, "is.DockerImageRepository: %s\n", is.Spec.DockerImageRepository)
			})
		})
	})
})
