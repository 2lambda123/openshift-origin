package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][Conformance] build without output image", func() {
	defer g.GinkgoRecover()
	var (
		dockerImageFixture = exutil.FixturePath("testdata", "test-docker-no-outputname.json")
		s2iImageFixture    = exutil.FixturePath("testdata", "test-s2i-no-outputname.json")
		oc                 = exutil.NewCLI("build-no-outputname", exutil.KubeConfigPath())
	)

	g.Describe("building from templates", func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		g.It(fmt.Sprintf("should create an image from %q docker template without an output image reference defined", dockerImageFixture), func() {
			err := oc.Run("create").Args("-f", dockerImageFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting build to pass without an output image reference specified")
			out, err := oc.Run("start-build").Args("test-docker", "--follow", "--wait").Output()
			if err != nil {
				fmt.Fprintln(g.GinkgoWriter, out)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).Should(o.ContainSubstring(`Build complete, no image push requested`))
		})

		g.It(fmt.Sprintf("should create an image from %q S2i template without an output image reference defined", s2iImageFixture), func() {
			err := oc.Run("create").Args("-f", s2iImageFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting build to pass without an output image reference specified")
			out, err := oc.Run("start-build").Args("test-sti", "--follow", "--wait").Output()
			if err != nil {
				fmt.Fprintln(g.GinkgoWriter, out)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).Should(o.ContainSubstring(`Build complete, no image push requested`))
		})
	})
})
