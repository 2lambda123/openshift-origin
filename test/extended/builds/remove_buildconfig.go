package builds

import (
	"time"

	"k8s.io/kubernetes/pkg/util/wait"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("builds: deleting buildconfig", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture = exutil.FixturePath("..", "extended", "fixtures", "test-build.json")
		oc           = exutil.NewCLI("cli-start-build", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.Run("create").Args("-f", buildFixture).Execute()
	})

	g.Describe("oc delete buildconfig", func() {
		g.It("should start a build and wait for the build to complete", func() {
			var (
				err    error
				builds [4]string
			)

			g.By("starting multiple builds")
			for i := range builds {
				builds[i], err = oc.Run("start-build").Args("sample-build").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("waiting for half of them")
			for i := range builds[:len(builds)/2] {
				// Note that it's not important to check for success here. We
				// only care about builds being deleted after BC is deleted and
				// we don't really care about their status prior to that.
				exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), builds[i], exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)
			}

			g.By("deleting the buildconfig")
			err = oc.Run("delete").Args("bc/sample-build").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for builds to clear")
			err = wait.Poll(3*time.Second, 3*time.Minute, func() (bool, error) {
				out, err := oc.Run("get").Args("-o", "name", "builds").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				if len(out) == 0 {
					return true, nil
				}
				return false, nil
			})
			if err == wait.ErrWaitTimeout {
				g.Fail("timed out waiting for builds to clear")
			}
		})

	})
})
