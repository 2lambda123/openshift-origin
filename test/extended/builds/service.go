package builds

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kdeployutil "k8s.io/kubernetes/test/e2e/framework/deployment"
	k8simage "k8s.io/kubernetes/test/utils/image"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-builds][Feature:Builds] build can reference a cluster service", func() {
	defer g.GinkgoRecover()
	var (
		oc             = exutil.NewCLI("build-service")
		testDockerfile = fmt.Sprintf(`
FROM %s
RUN cat /etc/resolv.conf
RUN curl -vvv hello-openshift:6379
`, image.ShellImage())
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("with a build being created from new-build", func() {
			g.It("should be able to run a build that references a cluster service", func() {
				g.By("standing up a new hello world service")
				err := oc.Run("new-app").Args("--name", "hello-openshift", k8simage.GetE2EImage(k8simage.Redis)).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				deploy, derr := oc.KubeClient().AppsV1().Deployments(oc.Namespace()).Get(context.Background(), "hello-openshift", metav1.GetOptions{})
				if kapierrs.IsNotFound(derr) {
					// if deployment is not there we're working with old new-app producing deployment configs
					err := exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), "hello-openshift", 1, true, oc)
					o.Expect(err).NotTo(o.HaveOccurred())
				} else {
					// if present - wait for deployment
					o.Expect(derr).NotTo(o.HaveOccurred())
					err := kdeployutil.WaitForDeploymentComplete(oc.KubeClient(), deploy)
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				err = exutil.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), "hello-openshift")
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("calling oc new-build with a Dockerfile")
				err = oc.Run("new-build").Args("-D", "-", "--to", "test:latest").InputString(testDockerfile).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the build is in Complete phase")
				err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "test-1", nil, nil, nil)
				//debug for failures
				if err != nil {
					exutil.DumpBuildLogs("test", oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})
	})
})
