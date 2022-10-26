package builds

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds] buildconfig secret injector", func() {
	defer g.GinkgoRecover()

	var (
		itemsPath = exutil.FixturePath("testdata", "builds", "test-buildconfigsecretinjector.yaml")
		oc        = exutil.NewCLI("buildconfigsecretinjector")
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			g.By("creating buildconfigs")
			err := oc.Run("create").Args("-f", itemsPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.It("should inject secrets to the appropriate buildconfigs [apigroup:build.openshift.io]", func() {
			out, err := oc.Run("get").Args("bc/test1", "-o", "template", "--template", "{{.spec.source.sourceSecret.name}}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.Equal("secret1"))

			out, err = oc.Run("get").Args("bc/test2", "-o", "template", "--template", "{{.spec.source.sourceSecret.name}}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.Equal("secret2"))

			out, err = oc.Run("get").Args("bc/test3", "-o", "template", "--template", "{{.spec.source.sourceSecret.name}}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.Equal("secret3"))

			out, err = oc.Run("get").Args("bc/test4", "-o", "template", "--template", "{{.spec.source.sourceSecret.name}}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.Equal("<no value>"))
		})
	})
})
