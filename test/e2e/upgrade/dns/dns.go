package dns

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/onsi/ginkgo"

	kappsv1 "k8s.io/api/apps/v1"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

type UpgradeTest struct{}

type upgradePhase string

const duringUpgrade upgradePhase = "duringUpgrade"
const afterUpgrade upgradePhase = "afterUpgrade"

var appName string

func (t *UpgradeTest) Name() string { return "check-for-dns-availability" }
func (UpgradeTest) DisplayName() string {
	return "[sig-network-edge] Verify DNS availability during and after upgrade success"
}

// Setup creates a DaemonSet to verify DNS availability during and after upgrade
func (t *UpgradeTest) Setup(f *framework.Framework) {
	ginkgo.By("Setting up upgrade DNS availability test")

	ginkgo.By("Getting DNS Service Cluster IP")
	dnsServiceIP := t.getServiceIP(f)

	ginkgo.By("Creating a DaemonSet to verify DNS availability")
	appName = fmt.Sprintf("dns-test-%s", string(uuid.NewUUID()))
	t.createDNSTestDaemonSet(f, dnsServiceIP)
}

// Test checks for logs from DNS availability test a minute after upgrade finishes
// to cover during and after upgrade phase, and verifies the results.
func (t *UpgradeTest) Test(f *framework.Framework, done <-chan struct{}, _ upgrades.UpgradeType) {
	ginkgo.By("Validating DNS results during upgrade")
	t.validateDNSResults(f, duringUpgrade)

	// Block until upgrade is done
	<-done

	ginkgo.By("Sleeping for a minute to give it time for verifying DNS after upgrade")
	time.Sleep(1 * time.Minute)

	ginkgo.By("Validating DNS results after upgrade")
	t.validateDNSResults(f, afterUpgrade)
}

// getServiceIP gets Cluster IP from DNS Service
func (t *UpgradeTest) getServiceIP(f *framework.Framework) string {
	dnsService, err := f.ClientSet.CoreV1().Services("openshift-dns").Get(context.Background(), "dns-default", metav1.GetOptions{})
	framework.ExpectNoError(err)
	return dnsService.Spec.ClusterIP
}

// createDNSTestDaemonSet creates a DaemonSet to test DNS availability
func (t *UpgradeTest) createDNSTestDaemonSet(f *framework.Framework, dnsServiceIP string) {
	cmd := fmt.Sprintf("while true; do dig +short @%s google.com || echo $(date) fail && sleep 1; done", dnsServiceIP)
	_, err := f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Create(context.Background(), &kappsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: appName},
		Spec: kappsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": appName},
			},
			Template: kapiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": appName},
				},
				Spec: kapiv1.PodSpec{
					Containers: []kapiv1.Container{
						{
							Name:    "querier",
							Image:   imageutils.GetE2EImage(imageutils.JessieDnsutils),
							Command: []string{"sh", "-c", cmd},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	framework.ExpectNoError(err)
}

// validateDNSResults retrieves the Pod logs and validates the results
func (t *UpgradeTest) validateDNSResults(f *framework.Framework, phase upgradePhase) {
	ginkgo.By(fmt.Sprintf("Listing Pods with label app=%s", appName))
	podClient := f.ClientSet.CoreV1().Pods(f.Namespace.Name)
	selector, _ := labels.Parse(fmt.Sprintf("app=%s", appName))
	pods, err := podClient.List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
	framework.ExpectNoError(err)

	ginkgo.By("Retrieving logs from all the Pods belonging to the DaemonSet and asserting no failure")
	for _, pod := range pods.Items {
		framework.Logf("Everything is fine until here. 1")
		p, err := podClient.Get(context.Background(), pod.Name, metav1.GetOptions{})
		if err != nil || p == nil || p.Status.Phase != kapiv1.PodRunning {
			if phase == duringUpgrade {
				framework.Logf("Failed to get pod [%s] during upgrade. Skipping validating DNS result for this pod", pod.Name)
				continue
			} else {
				if err != nil {
					framework.Failf("Failed to get pod %s after upgrade: %v", pod.Name, err)
				}
				framework.Failf("Pod %s is not in running state after upgrade: %s", pod.Name, pod.Status.Phase)
			}
		}
		framework.Logf("Everything is fine until here. 2")

		req := podClient.GetLogs(pod.Name, &kapiv1.PodLogOptions{Container: "querier"})
		if req == nil {
			framework.Failf("GetLogs request failed")
		}

		r, err := req.Stream(context.Background())
		framework.ExpectNoError(err)
		if r == nil {
			framework.Failf("Stream returned a nil ReadCloser")
		}

		framework.Logf("Everything is fine until here. 3")

		failureCount := 0.0
		successCount := 0.0
		scan := bufio.NewScanner(r)
		for scan.Scan() {
			line := scan.Text()
			if strings.Contains(line, "fail") {
				failureCount++
			} else if ip := net.ParseIP(line); ip != nil {
				successCount++
			}
		}

		framework.Logf("Everything is fine until here. 4")
		framework.Logf("successCount: [%d], failureCount: [%d]", successCount, failureCount)
		framework.Logf("Pod.Spec: [%q]", pod.Spec)

		if successRate := (successCount / (successCount + failureCount)) * 100; successRate < 99 {
			err = fmt.Errorf("success rate is less than 99%% on the node %s: [%0.2f]", pod.Spec.NodeName, successRate)
		} else {
			err = nil
		}
		framework.ExpectNoError(err)

		framework.Logf("Everything is fine until here. 5")
	}
}

// Teardown cleans up any objects that are created that
// aren't already cleaned up by the framework.
func (t *UpgradeTest) Teardown(_ *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}
