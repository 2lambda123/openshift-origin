package images

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[networking][router] weighted openshift router", func() {
	defer g.GinkgoRecover()
	var (
		configPath = exutil.FixturePath("testdata", "weighted-router.yaml")
		oc         = exutil.NewCLI("weighted-router", exutil.KubeConfigPath())
	)

	g.BeforeEach(func() {
		// defer oc.Run("delete").Args("-f", configPath).Execute()
		err := oc.AsAdmin().Run("adm").Args("policy", "add-cluster-role-to-user", "system:router", oc.Username()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("create").Args("-f", configPath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router", func() {
		g.It("should appropriately serve a route that points to two services", func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)
			ns := oc.KubeFramework().Namespace.Name
			execPodName := exutil.CreateExecPodOrFail(oc.AdminKubeClient().Core(), ns, "execpod")
			defer func() { oc.AdminKubeClient().Core().Pods(ns).Delete(execPodName, kapi.NewDeleteOptions(1)) }()

			g.By(fmt.Sprintf("creating a weighted router from a config file %q", configPath))

			var routerIP string
			err := wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				pod, err := oc.KubeFramework().ClientSet.Core().Pods(oc.KubeFramework().Namespace.Name).Get("weighted-router")
				if err != nil {
					return false, err
				}
				if len(pod.Status.PodIP) == 0 {
					return false, nil
				}

				routerIP = pod.Status.PodIP
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			// router expected to listen on port 80
			routerURL := fmt.Sprintf("http://%s", routerIP)

			g.By("waiting for the healthz endpoint to respond")
			healthzURI := fmt.Sprintf("http://%s:1936/healthz", routerIP)
			err = waitForRouterOKResponseExec(ns, execPodName, healthzURI, routerIP, changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			host := "weighted.example.com"
			times := 100
			g.By(fmt.Sprintf("checking that %d requests go through successfully", times))
			// wait for the request to stabilize
			err = waitForRouterOKResponseExec(ns, execPodName, routerURL, "weighted.example.com", changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())
			// all requests should now succeed
			err = expectRouteStatusCodeRepeatedExec(ns, execPodName, routerURL, "weighted.example.com", http.StatusOK, times)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("checking that there are two weighted backends in the router stats"))
			var trafficValues []string
			err = wait.PollImmediate(100*time.Millisecond, changeTimeoutSeconds*time.Second, func() (bool, error) {
				statsURL := fmt.Sprintf("http://%s:1936/;csv", routerIP)
				stats, err := getAuthenticatedRouteURLViaPod(ns, execPodName, statsURL, host, "admin", "password")
				o.Expect(err).NotTo(o.HaveOccurred())
				trafficValues, err = parseStats(stats, "weightedroute", 7)
				o.Expect(err).NotTo(o.HaveOccurred())
				return len(trafficValues) == 2, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			trafficEP1, err := strconv.Atoi(trafficValues[0])
			o.Expect(err).NotTo(o.HaveOccurred())
			trafficEP2, err := strconv.Atoi(trafficValues[1])
			o.Expect(err).NotTo(o.HaveOccurred())

			weightedRatio := float32(trafficEP1) / float32(trafficEP2)
			if weightedRatio < 5 && weightedRatio > 0.2 {
				e2e.Failf("Unexpected weighted ratio for incoming traffic: %v (%d/%d)", weightedRatio, trafficEP1, trafficEP2)
			}

			g.By(fmt.Sprintf("checking that zero weights are also respected by the router"))
			host = "zeroweight.example.com"
			err = expectRouteStatusCodeExec(ns, execPodName, routerURL, host, http.StatusServiceUnavailable)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

func parseStats(stats string, backendSubstr string, statsField int) ([]string, error) {
	r := csv.NewReader(strings.NewReader(stats))
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	fieldValues := make([]string, 0)
	for _, rec := range records {
		if strings.Contains(rec[0], backendSubstr) && !strings.Contains(rec[1], "BACKEND") {
			fieldValues = append(fieldValues, rec[statsField])
		}
	}
	return fieldValues, nil
}
