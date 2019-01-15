package util

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/test/utils"
)

// pods whose metrics show a larger ratio of requests per
// second than maxQPSAllowed are considered "unhealthy".
const (
	maxQPSAllowed = 1.5
)

var (
	// TODO: these exceptions should not exist. Update operators to have a better request-rate per second
	perComponentNamespaceMaxQPSAllowed = map[string]float64{
		"openshift-apiserver-operator":                            3.0,
		"openshift-kube-apiserver-operator":                       6.8,
		"openshift-kube-controller-manager-operator":              2.0,
		"openshift-cluster-kube-scheduler-operator":               1.8,
		"openshift-cluster-openshift-controller-manager-operator": 1.7,
	}
)

type podInfo struct {
	name      string
	qps       float64
	status    string
	namespace string
	result    string
	failed    bool
	skipped   bool
}

func CalculatePodMetrics(adminClient kubernetes.Interface, adminConfig *restclient.Config) error {
	podUrlGetter := &PortForwardURLGetter{
		Protocol:   "https",
		Host:       "localhost",
		RemotePort: "8443",
		LocalPort:  "37587",
	}

	namespaces, err := adminClient.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	failures := []error{}
	for _, ns := range namespaces.Items {
		// skip namespaces which do not meet "operator namespace" criteria
		if !strings.HasPrefix(ns.Name, "openshift-") || !strings.HasSuffix(ns.Name, "-operator") {
			continue
		}

		infos, err := getPodInfoForNamespace(adminClient, adminConfig, podUrlGetter, ns.Name)
		if err != nil {
			return err
		}

		for _, info := range infos {
			if info.failed {
				failures = append(failures, fmt.Errorf("Failed to fetch operator pod metrics for pod %q: %s", info.name, info.result))
				continue
			}
			if info.skipped {
				continue
			}

			qpsLimit := maxQPSAllowed
			if customLimit, ok := perComponentNamespaceMaxQPSAllowed[info.namespace]; ok {
				qpsLimit = customLimit
			}

			if info.qps > qpsLimit {
				failures = append(failures, fmt.Errorf("operator pod %q in namespace %q is making %v requests per second. Maximum allowed is %v requests per second", info.name, info.namespace, info.qps, maxQPSAllowed))
				continue
			}
		}
	}

	if len(failures) > 0 {
		return errors.NewAggregate(failures)
	}
	return nil
}

func getPodInfoForNamespace(adminClient kubernetes.Interface, adminConfig *restclient.Config, podUrlGetter *PortForwardURLGetter, namespace string) ([]*podInfo, error) {
	pods, err := adminClient.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	podInfos := []*podInfo{}
	for _, pod := range pods.Items {
		info := &podInfo{
			name:      pod.Name,
			namespace: pod.Namespace,
			status:    string(pod.Status.Phase),
		}

		podReady, err := utils.PodRunningReady(&pod)
		if !podReady || err != nil {
			result := "skipped, pod is not Running"
			if err != nil {
				result = fmt.Sprintf("%s: %v", result, err)
			}

			info.result = result
			info.skipped = true
			podInfos = append(podInfos, info)
			continue
		}

		if len(pod.Spec.Containers) == 0 {
			info.result = "skipped, no containers found"
			info.skipped = true
			podInfos = append(podInfos, info)
			continue
		}

		metrics, err := getPodMetrics(adminConfig, &pod, podUrlGetter)
		if err != nil {
			// ignore errors from pods with no /metrics endpoint available
			if !strings.Contains(err.Error(), "Connection refused") && !strings.Contains(err.Error(), "EOF") {
				info.result = fmt.Sprintf("error: %s", err)
				info.failed = true
			} else {
				info.skipped = true
				info.result = fmt.Sprintf("/metrics endpoint not available")
			}

			podInfos = append(podInfos, info)
			continue
		}

		metricGroup, ok := metrics["rest_client_requests_total"]
		if !ok {
			info.result = fmt.Sprintf("error: failed to find counter: %q", "rest_client_requests_total")
			info.failed = true
			podInfos = append(podInfos, info)
			continue
		}

		procStartTime, ok := metrics["process_start_time_seconds"]
		if !ok || len(procStartTime.Metric) == 0 {
			info.result = fmt.Sprintf("error: failed to find metric: %q", "process_start_time_seconds")
			info.failed = true
			podInfos = append(podInfos, info)
			continue
		}
		procStartTimeSeconds := procStartTime.Metric[0].GetGauge().GetValue()
		totalProcTimeSeconds := time.Now().Unix() - int64(procStartTimeSeconds)

		totalRequestCount := float64(0)
		for _, metric := range metricGroup.Metric {
			totalRequestCount += metric.Counter.GetValue()
		}

		comment := "within QPS bounds"
		qps := totalRequestCount / float64(totalProcTimeSeconds)
		if qps > maxQPSAllowed {
			comment = "exceeds QPS bounds"
		}
		info.status = fmt.Sprintf("%s (%s)", info.status, comment)
		info.qps = qps
		info.result = fmt.Sprintf("%v requests over a span of %v seconds", totalRequestCount, totalProcTimeSeconds)
		podInfos = append(podInfos, info)
	}

	return podInfos, nil
}

func getPodMetrics(adminConfig *restclient.Config, pod *v1.Pod, podUrlGetter *PortForwardURLGetter) (map[string]*dto.MetricFamily, error) {
	result, err := podUrlGetter.Get("/metrics", pod, adminConfig)
	if err != nil {
		return nil, err
	}

	return parseRawMetrics(result)
}

func parseRawMetrics(rawMetrics string) (map[string]*dto.MetricFamily, error) {
	p := expfmt.TextParser{}
	return p.TextToMetricFamilies(bytes.NewBufferString(rawMetrics))
}

type defaultPortForwarder struct {
	restConfig *rest.Config

	StopChannel  chan struct{}
	ReadyChannel chan struct{}
}

func NewDefaultPortForwarder(adminConfig *rest.Config) *defaultPortForwarder {
	return &defaultPortForwarder{
		restConfig:   adminConfig,
		StopChannel:  make(chan struct{}, 1),
		ReadyChannel: make(chan struct{}, 1),
	}
}

func (f *defaultPortForwarder) ForwardPortsAndExecute(pod *v1.Pod, ports []string, toExecute func()) error {
	if len(ports) < 1 {
		return fmt.Errorf("at least 1 PORT is required for port-forward")
	}

	restClient, err := rest.RESTClientFor(setRESTConfigDefaults(*f.restConfig))
	if err != nil {
		return err
	}

	if pod.Status.Phase != v1.PodRunning {
		return fmt.Errorf("unable to forward port because pod is not running. Current status=%v", pod.Status.Phase)
	}

	stdout := bytes.NewBuffer(nil)
	req := restClient.Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(f.restConfig)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	fw, err := portforward.New(dialer, ports, f.StopChannel, f.ReadyChannel, stdout, stdout)
	if err != nil {
		return err
	}

	go func() {
		if f.StopChannel != nil {
			defer close(f.StopChannel)
		}

		<-f.ReadyChannel
		toExecute()
	}()

	return fw.ForwardPorts()
}

func setRESTConfigDefaults(config rest.Config) *rest.Config {
	if config.GroupVersion == nil {
		config.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}
	}
	if config.NegotiatedSerializer == nil {
		config.NegotiatedSerializer = scheme.Codecs
	}
	if len(config.UserAgent) == 0 {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	config.APIPath = "/api"
	return &config
}

func newInsecureRESTClientForHost(host string) (rest.Interface, error) {
	insecure := true

	configFlags := &genericclioptions.ConfigFlags{}
	configFlags.Insecure = &insecure
	configFlags.APIServer = &host

	newConfig, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	return rest.RESTClientFor(setRESTConfigDefaults(*newConfig))
}

type PortForwardURLGetter struct {
	Protocol   string
	Host       string
	RemotePort string
	LocalPort  string
}

func (c *PortForwardURLGetter) Get(urlPath string, pod *v1.Pod, config *rest.Config) (string, error) {
	var result string
	var lastErr error
	forwarder := NewDefaultPortForwarder(config)

	if err := forwarder.ForwardPortsAndExecute(pod, []string{c.LocalPort + ":" + c.RemotePort}, func() {
		restClient, err := newInsecureRESTClientForHost(fmt.Sprintf("https://localhost:%s/", c.LocalPort))
		if err != nil {
			lastErr = err
			return
		}

		ioCloser, err := restClient.Get().RequestURI(urlPath).Stream()
		if err != nil {
			lastErr = err
			return
		}
		defer ioCloser.Close()

		data := bytes.NewBuffer(nil)
		_, lastErr = io.Copy(data, ioCloser)
		result = data.String()
	}); err != nil {
		return "", err
	}
	return result, lastErr
}
