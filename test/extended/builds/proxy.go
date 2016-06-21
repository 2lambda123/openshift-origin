package builds

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	buildapi "github.com/openshift/origin/pkg/build/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][Slow] the s2i build should support proxies", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture = exutil.FixturePath("..", "extended", "testdata", "test-build-proxy.json")
		oc           = exutil.NewCLI("build-proxy", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.Run("create").Args("-f", buildFixture).Execute()
	})

	g.Describe("start build with broken proxy", func() {
		g.It("should start a build and wait for the build to to fail", func() {
			g.By("starting the build")
			out, err := oc.Run("start-build").Args("sample-build").Output()
			if err != nil {
				fmt.Fprintln(g.GinkgoWriter, out)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("verifying the build sample-build-1 output")
			// The git ls-remote check should exit the build when the remote
			// repository is not accessible. It should never get to the clone.
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), "sample-build-1", exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)
			o.Expect(err).To(o.HaveOccurred())
			out, err = oc.Run("logs").Args("-f", "bc/sample-build").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).NotTo(o.ContainSubstring("clone"))
			if !strings.Contains(out, `unable to access 'https://github.com/openshift/ruby-hello-world.git/': Failed connect to 127.0.0.1:3128`) {
				fmt.Fprintf(g.GinkgoWriter, "\n: build logs: %s\n", out)
			}
			o.Expect(out).To(o.ContainSubstring(`unable to access 'https://github.com/openshift/ruby-hello-world.git/': Failed connect to 127.0.0.1:3128`))
			g.By("verifying the build sample-build-1 status")
			build, err := oc.REST().Builds(oc.Namespace()).Get("sample-build-1")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(build.Status.Phase).Should(o.BeEquivalentTo(buildapi.BuildPhaseFailed))
		})

	})

})
