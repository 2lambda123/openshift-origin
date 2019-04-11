package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	corev1 "k8s.io/api/core/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/kubernetes/pkg/client/conditions"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("router-metrics", exutil.KubeConfigPath())

		username, password, execPodName, ns, host string
		statsPort                                 int
		routerNamespace, serviceIP, bearerToken   string
	)

	g.BeforeEach(func() {
		var err error
		var template *corev1.PodTemplateSpec
		template, routerNamespace, err = exutil.GetRouterPodTemplate(oc)
		if kapierrs.IsNotFound(err) {
			g.Skip("no router installed on the cluster")
			return
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		env := template.Spec.Containers[0].Env

		if len(findEnvVar(env, "ROUTER_METRICS_TYPE")) == 0 {
			g.Skip("router does not have ROUTER_METRICS_TYPE set")
			return
		}

		username, password, err = findStatsUsernameAndPassword(oc, routerNamespace, env)
		o.Expect(err).NotTo(o.HaveOccurred())

		statsPort = 1936
		statsPortString := findEnvVar(env, "STATS_PORT")
		if len(statsPortString) > 0 {
			if port, err := strconv.Atoi(statsPortString); err == nil {
				statsPort = port
			}
		}

		host, err = waitForRouterMetricsIP(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		serviceIP, err = waitForRouterServiceIP(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		bearerToken, err = findMetricsBearerToken(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		ns = oc.KubeFramework().Namespace.Name
	})

	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			exutil.DumpPodLogsStartingWithInNamespace("router", "default", oc.AsAdmin())
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should expose a health check on the metrics port", func() {
			execPodName = exutil.CreateExecPodOrFail(oc.AdminKubeClient().CoreV1(), ns, "execpod")
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By("listening on the health port")
			err := expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("http://%s:%d/healthz", host, statsPort), 200)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("[Flaky] should expose prometheus metrics for a route", func() {
			g.By("when a route exists")
			configPath := exutil.FixturePath("testdata", "router-metrics.yaml")
			err := oc.Run("create").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			execPodName = exutil.CreateExecPodOrFail(oc.AdminKubeClient().CoreV1(), ns, "execpod")
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By("preventing access without a username and password")
			err = expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("http://%s:%d/metrics", host, statsPort), 401, 403)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("validate access using username and password")
			_, err = getAuthenticatedURLViaPod(ns, execPodName, fmt.Sprintf("http://%s:%d/metrics", host, statsPort), username, password)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("checking for the expected metrics")
			routeLabels := promLabels{"backend": "http", "namespace": ns, "route": "weightedroute"}
			serverLabels := promLabels{"namespace": ns, "route": "weightedroute"}
			var metrics map[string]*dto.MetricFamily
			var results string
			defer func() { e2e.Logf("initial metrics:\n%s", results) }()
			times := 10
			p := expfmt.TextParser{}

			err = wait.PollImmediate(2*time.Second, 240*time.Second, func() (bool, error) {
				results, err = getBearerTokenURLViaPod(ns, execPodName, fmt.Sprintf("http://%s:%d/metrics", host, statsPort), bearerToken)
				o.Expect(err).NotTo(o.HaveOccurred())

				metrics, err = p.TextToMetricFamilies(bytes.NewBufferString(results))
				o.Expect(err).NotTo(o.HaveOccurred())
				//e2e.Logf("Metrics:\n%s", results)

				if len(findGaugesWithLabels(metrics["haproxy_server_up"], serverLabels)) == 2 {
					if findGaugesWithLabels(metrics["haproxy_backend_connections_total"], routeLabels)[0] >= float64(times) {
						return true, nil
					}
					// send a burst of traffic to the router
					g.By("sending traffic to a weighted route")
					err = expectRouteStatusCodeRepeatedExec(ns, execPodName, fmt.Sprintf("http://%s", serviceIP), "weighted.metrics.example.com", http.StatusOK, times)
					o.Expect(err).NotTo(o.HaveOccurred())
				}
				g.By("retrying metrics until all backend servers appear")
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			allEndpoints := sets.NewString()
			services := []string{"weightedendpoints1", "weightedendpoints2"}
			for _, name := range services {
				epts, err := oc.AdminKubeClient().CoreV1().Endpoints(ns).Get(name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				for _, s := range epts.Subsets {
					for _, a := range s.Addresses {
						allEndpoints.Insert(a.IP + ":8080")
					}
				}
			}
			foundEndpoints := sets.NewString(findMetricLabels(metrics["haproxy_server_http_responses_total"], serverLabels, "server")...)
			o.Expect(allEndpoints.List()).To(o.Equal(foundEndpoints.List()))
			foundServices := sets.NewString(findMetricLabels(metrics["haproxy_server_http_responses_total"], serverLabels, "service")...)
			o.Expect(services).To(o.Equal(foundServices.List()))
			foundPods := sets.NewString(findMetricLabels(metrics["haproxy_server_http_responses_total"], serverLabels, "pod")...)
			o.Expect([]string{"endpoint-1", "endpoint-2"}).To(o.Equal(foundPods.List()))

			// route specific metrics from server and backend
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_http_responses_total"], serverLabels.With("code", "2xx"))).To(o.ConsistOf(o.BeNumerically(">", 0), o.BeNumerically(">", 0)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_http_responses_total"], serverLabels.With("code", "5xx"))).To(o.Equal([]float64{0, 0}))
			// only server returns response counts
			o.Expect(findGaugesWithLabels(metrics["haproxy_backend_http_responses_total"], routeLabels.With("code", "2xx"))).To(o.HaveLen(0))
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_connections_total"], serverLabels)).To(o.ConsistOf(o.BeNumerically(">=", 0), o.BeNumerically(">=", 0)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_backend_connections_total"], routeLabels)).To(o.ConsistOf(o.BeNumerically(">=", times)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_up"], serverLabels)).To(o.Equal([]float64{1, 1}))
			o.Expect(findGaugesWithLabels(metrics["haproxy_backend_up"], routeLabels)).To(o.Equal([]float64{1}))
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_bytes_in_total"], serverLabels)).To(o.ConsistOf(o.BeNumerically(">=", 0), o.BeNumerically(">=", 0)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_bytes_out_total"], serverLabels)).To(o.ConsistOf(o.BeNumerically(">=", 0), o.BeNumerically(">=", 0)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_server_max_sessions"], serverLabels)).To(o.ConsistOf(o.BeNumerically(">", 0), o.BeNumerically(">", 0)))

			// generic metrics
			o.Expect(findGaugesWithLabels(metrics["haproxy_up"], nil)).To(o.Equal([]float64{1}))
			o.Expect(findGaugesWithLabels(metrics["haproxy_exporter_scrape_interval"], nil)).To(o.ConsistOf(o.BeNumerically(">", 0)))
			o.Expect(findCountersWithLabels(metrics["haproxy_exporter_total_scrapes"], nil)).To(o.ConsistOf(o.BeNumerically(">", 0)))
			o.Expect(findCountersWithLabels(metrics["haproxy_exporter_csv_parse_failures"], nil)).To(o.Equal([]float64{0}))
			o.Expect(findGaugesWithLabels(metrics["haproxy_process_resident_memory_bytes"], nil)).To(o.ConsistOf(o.BeNumerically(">", 0)))
			o.Expect(findGaugesWithLabels(metrics["haproxy_process_max_fds"], nil)).To(o.ConsistOf(o.BeNumerically(">", 0)))

			// router metrics
			o.Expect(findMetricsWithLabels(metrics["template_router_reload_seconds"], nil)[0].Summary.GetSampleSum()).To(o.BeNumerically(">", 0))
			o.Expect(findMetricsWithLabels(metrics["template_router_write_config_seconds"], nil)[0].Summary.GetSampleSum()).To(o.BeNumerically(">", 0))

			// verify that across a reload metrics are preserved
			g.By("forcing a router restart after a pod deletion")

			// delete the pod
			err = oc.AdminKubeClient().CoreV1().Pods(ns).Delete("endpoint-2", nil)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the router to reload")
			time.Sleep(15 * time.Second)

			g.By("checking that some metrics are not reset to 0 after router restart")
			updatedResults, err := getBearerTokenURLViaPod(ns, execPodName, fmt.Sprintf("http://%s:%d/metrics", host, statsPort), bearerToken)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() { e2e.Logf("final metrics:\n%s", updatedResults) }()

			updatedMetrics, err := p.TextToMetricFamilies(bytes.NewBufferString(updatedResults))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(findGaugesWithLabels(updatedMetrics["haproxy_backend_connections_total"], routeLabels)[0]).To(o.BeNumerically(">=", findGaugesWithLabels(metrics["haproxy_backend_connections_total"], routeLabels)[0]))
			o.Expect(findGaugesWithLabels(updatedMetrics["haproxy_server_bytes_in_total"], serverLabels)[0]).To(o.BeNumerically(">=", findGaugesWithLabels(metrics["haproxy_server_bytes_in_total"], serverLabels)[0]))
		})

		g.It("should expose the profiling endpoints", func() {
			execPodName = exutil.CreateExecPodOrFail(oc.AdminKubeClient().CoreV1(), ns, "execpod")
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			g.By("preventing access without a username and password")
			err := expectURLStatusCodeExec(ns, execPodName, fmt.Sprintf("http://%s:%d/debug/pprof/heap", host, statsPort), 401, 403)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("at /debug/pprof")
			results, err := getAuthenticatedURLViaPod(ns, execPodName, fmt.Sprintf("http://%s:%d/debug/pprof/heap?debug=1", host, statsPort), username, password)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(results).To(o.ContainSubstring("# runtime.MemStats"))
		})

		g.It("should enable openshift-monitoring to pull metrics", func() {
			prometheusURL, token, exists := locatePrometheus(oc)
			if !exists {
				g.Skip("prometheus not found on this cluster")
			}

			execPodName := e2e.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *corev1.Pod) { pod.Spec.Containers[0].Image = "centos:7" })
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

			o.Expect(wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
				contents, err := getBearerTokenURLViaPod(ns, execPodName, fmt.Sprintf("%s/api/v1/targets", prometheusURL), token)
				o.Expect(err).NotTo(o.HaveOccurred())

				targets := &promTargets{}
				err = json.Unmarshal([]byte(contents), targets)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verifying router-internal-default job has a working target")
				err = targets.Expect(promLabels{"job": "router-internal-default"}, "up", "^https://.*/metrics$")
				if err != nil {
					e2e.Logf("missing router-internal-default target: %v", err)
					return false, nil
				}
				return true, nil
			})).NotTo(o.HaveOccurred())
		})
	})
})

type promLabels map[string]string

func (l promLabels) With(name, value string) promLabels {
	n := make(promLabels)
	for k, v := range l {
		n[k] = v
	}
	n[name] = value
	return n
}

type promTargets struct {
	Data struct {
		ActiveTargets []struct {
			Labels    map[string]string
			Health    string
			ScrapeUrl string
		}
	}
	Status string
}

func (t *promTargets) Expect(l promLabels, health, scrapeURLPattern string) error {
	for _, target := range t.Data.ActiveTargets {
		match := true
		for k, v := range l {
			if target.Labels[k] != v {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		if health != target.Health {
			continue
		}
		if !regexp.MustCompile(scrapeURLPattern).MatchString(target.ScrapeUrl) {
			continue
		}
		return nil
	}
	return fmt.Errorf("no match for %v with health %s and scrape URL %s", l, health, scrapeURLPattern)
}

func waitForServiceAccountInNamespace(c clientset.Interface, ns, serviceAccountName string, timeout time.Duration) error {
	w, err := c.CoreV1().ServiceAccounts(ns).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: serviceAccountName}))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err = watchtools.UntilWithoutRetry(ctx, w, conditions.ServiceAccountHasSecrets)
	return err
}

func locatePrometheus(oc *exutil.CLI) (url, bearerToken string, ok bool) {
	_, err := oc.AdminKubeClient().CoreV1().Services("openshift-monitoring").Get("prometheus-k8s", metav1.GetOptions{})
	if kapierrs.IsNotFound(err) {
		return "", "", false
	}

	waitForServiceAccountInNamespace(oc.AdminKubeClient(), "openshift-monitoring", "prometheus-k8s", 2*time.Minute)
	for i := 0; i < 30; i++ {
		secrets, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-monitoring").List(metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, secret := range secrets.Items {
			if secret.Type != corev1.SecretTypeServiceAccountToken {
				continue
			}
			if !strings.HasPrefix(secret.Name, "prometheus-") {
				continue
			}
			bearerToken = string(secret.Data[corev1.ServiceAccountTokenKey])
			break
		}
		if len(bearerToken) == 0 {
			e2e.Logf("Waiting for prometheus service account secret to show up")
			time.Sleep(time.Second)
			continue
		}
	}
	o.Expect(bearerToken).ToNot(o.BeEmpty())

	return "https://prometheus-k8s.openshift-monitoring.svc:9091", bearerToken, true
}

func findEnvVar(vars []corev1.EnvVar, key string) string {
	for _, v := range vars {
		if v.Name == key {
			return v.Value
		}
	}
	return ""
}

func findMetricsWithLabels(f *dto.MetricFamily, promLabels map[string]string) []*dto.Metric {
	var result []*dto.Metric
	if f == nil {
		return result
	}
	for _, m := range f.Metric {
		matched := map[string]struct{}{}
		for _, l := range m.Label {
			if expect, ok := promLabels[l.GetName()]; ok {
				if expect != l.GetValue() {
					break
				}
				matched[l.GetName()] = struct{}{}
			}
		}
		if len(matched) != len(promLabels) {
			continue
		}
		result = append(result, m)
	}
	return result
}

func findCountersWithLabels(f *dto.MetricFamily, promLabels map[string]string) []float64 {
	var result []float64
	for _, m := range findMetricsWithLabels(f, promLabels) {
		result = append(result, m.Counter.GetValue())
	}
	return result
}

func findGaugesWithLabels(f *dto.MetricFamily, promLabels map[string]string) []float64 {
	var result []float64
	for _, m := range findMetricsWithLabels(f, promLabels) {
		result = append(result, m.Gauge.GetValue())
	}
	return result
}

func findMetricLabels(f *dto.MetricFamily, promLabels map[string]string, match string) []string {
	var result []string
	for _, m := range findMetricsWithLabels(f, promLabels) {
		for _, l := range m.Label {
			if l.GetName() == match {
				result = append(result, l.GetValue())
				break
			}
		}
	}
	return result
}

func expectURLStatusCodeExec(ns, execPodName, url string, statusCodes ...int) error {
	cmd := fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' %q", url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	for _, statusCode := range statusCodes {
		if output == strconv.Itoa(statusCode) {
			return nil
		}
	}
	return fmt.Errorf("last response from server was not any of %v: %s", statusCodes, output)
}

func getAuthenticatedURLViaPod(ns, execPodName, url, user, pass string) (string, error) {
	cmd := fmt.Sprintf("curl -s -u %s:%s %q", user, pass, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}

func getBearerTokenURLViaPod(ns, execPodName, url, bearer string) (string, error) {
	cmd := fmt.Sprintf("curl -s -k -H 'Authorization: Bearer %s' %q", bearer, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}

func findEnvVarSecret(vars []corev1.EnvVar, key string) (string, string) {
	for _, v := range vars {
		if v.Name == key {
			if v.ValueFrom != nil && v.ValueFrom.SecretKeyRef != nil {
				ref := v.ValueFrom.SecretKeyRef
				return ref.Name, ref.Key
			}
		}
	}
	return "", ""
}

func findStatsUsernameAndPassword(oc *exutil.CLI, ns string, env []corev1.EnvVar) (string, string, error) {
	secretName, userKey := findEnvVarSecret(env, "STATS_USERNAME")
	_, passKey := findEnvVarSecret(env, "STATS_PASSWORD")

	if len(secretName) == 0 || len(userKey) == 0 || len(passKey) == 0 {
		return "", "", fmt.Errorf("stats username and password not found, env: %v", env)
	}

	secret, err := oc.AdminKubeClient().CoreV1().Secrets(ns).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}

	username, ok1 := secret.Data[userKey]
	password, ok2 := secret.Data[passKey]
	if !ok1 || !ok2 {
		return "", "", fmt.Errorf("secret '%s/%s' does not contain key %q or %q", ns, secretName, userKey, passKey)
	}
	return string(username), string(password), nil
}

func findMetricsBearerToken(oc *exutil.CLI) (string, error) {
	sa, err := oc.AdminKubeClient().CoreV1().ServiceAccounts("openshift-monitoring").Get("prometheus-k8s", metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	var secretName string
	for _, secret := range sa.Secrets {
		if strings.Contains(secret.Name, "prometheus-k8s-token") {
			secretName = secret.Name
			break
		}
	}
	if len(secretName) == 0 {
		return "", fmt.Errorf("serviceaccount 'openshift-monitoring/prometheus-k8s' does not contain 'prometheus-k8s-token' secret")
	}

	secret, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-monitoring").Get(secretName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	token, ok := secret.Data["token"]
	if !ok {
		return "", fmt.Errorf("secret 'openshift-monitoring/%s' does not contain bearer token", secretName)
	}
	return string(token), nil
}
