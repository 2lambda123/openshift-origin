package operators

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	g "github.com/onsi/ginkgo"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-arch] Managed cluster", func() {
	oc := exutil.NewCLIWithoutNamespace("operator-resources")

	// Pods that are part of the control plane should set both cpu and memory requests, but require an exception
	// to set limits on memory (CPU limits are generally not allowed). This enforces the rules described in
	// https://github.com/openshift/enhancements/blob/master/CONVENTIONS.md#resources-and-limits.
	//
	// This test enforces all pods in the openshift-*, kube-*, and default namespace have requests set for both
	// CPU and memory, and no limits set. Known bugs will transform this to a flake. Otherwise the test will fail.
	//
	// Release architects can justify an exception with text but must ensure CONVENTIONS.md is updated to document
	// why the exception is granted.
	g.It("should set requests but not limits", func() {
		pods, err := oc.KubeFramework().ClientSet.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list pods: %v", err)
		}

		// pods that have a bug opened, every entry here must have a bug associated
		knownBrokenPods := map[string]string{
			//"<apiVersion>/<kind>/<namespace>/<name>/(initContainer|container)/<container_name>/<violation_type>": "<url to bug>",

			"apps/v1/Deployment/openshift-machine-api/cluster-autoscaler-default/container/cluster-autoscaler/request[cpu]":    "https://bugzilla.redhat.com/show_bug.cgi?id=1938467",
			"apps/v1/Deployment/openshift-machine-api/cluster-autoscaler-default/container/cluster-autoscaler/request[memory]": "https://bugzilla.redhat.com/show_bug.cgi?id=1938467",
			"apps/v1/Deployment/openshift-monitoring/thanos-querier/container/thanos-query/request[cpu]":                       "https://bugzilla.redhat.com/show_bug.cgi?id=1938465",
			"apps/v1/Deployment/openshift-operator-lifecycle-manager/packageserver/container/packageserver/request[cpu]":       "https://bugzilla.redhat.com/show_bug.cgi?id=1938466",
			"apps/v1/Deployment/openshift-operator-lifecycle-manager/packageserver/container/packageserver/request[memory]":    "https://bugzilla.redhat.com/show_bug.cgi?id=1938466",

			"batch/v1/Job/openshift-marketplace/<batch_job>/container/extract/request[cpu]":    "https://bugzilla.redhat.com/show_bug.cgi?id=1938492",
			"batch/v1/Job/openshift-marketplace/<batch_job>/container/extract/request[memory]": "https://bugzilla.redhat.com/show_bug.cgi?id=1938492",

			"apps/v1/DaemonSet/openshift-machine-api/metal3-image-cache/container/metal3-httpd/request[cpu]":          "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/DaemonSet/openshift-machine-api/metal3-image-cache/container/metal3-httpd/request[memory]":       "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/ironic-deploy-ramdisk-logs/request[cpu]":       "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/ironic-deploy-ramdisk-logs/request[memory]":    "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/ironic-inspector-ramdisk-logs/request[cpu]":    "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/ironic-inspector-ramdisk-logs/request[memory]": "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-baremetal-operator/request[cpu]":        "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-baremetal-operator/request[memory]":     "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-dnsmasq/request[cpu]":                   "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-dnsmasq/request[memory]":                "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-httpd/request[cpu]":                     "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-httpd/request[memory]":                  "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-ironic-api/request[cpu]":                "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-ironic-api/request[memory]":             "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-ironic-conductor/request[cpu]":          "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-ironic-conductor/request[memory]":       "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-ironic-inspector/request[cpu]":          "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-ironic-inspector/request[memory]":       "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-mariadb/request[cpu]":                   "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-mariadb/request[memory]":                "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-static-ip-manager/request[cpu]":         "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",
			"apps/v1/Deployment/openshift-machine-api/metal3/container/metal3-static-ip-manager/request[memory]":      "https://bugzilla.redhat.com/show_bug.cgi?id=1940518",

			"apps/v1/Deployment/openshift-cluster-csi-drivers/ovirt-csi-driver-controller/container/csi-provisioner/request[cpu]":    "https://bugzilla.redhat.com/show_bug.cgi?id=1940876",
			"apps/v1/Deployment/openshift-cluster-csi-drivers/ovirt-csi-driver-controller/container/csi-provisioner/request[memory]": "https://bugzilla.redhat.com/show_bug.cgi?id=1940876",
		}

		// pods with an exception granted, the value should be the justification and the approver (a release architect)
		exceptionGranted := map[string]string{
			//"<apiVersion>/<kind>/<namespace>/<name>/(initContainer|container)/<container_name>/<violation_type>": "<github handle of approver>: <brief description of the reason for the exception>",

			// CPU limits on these containers may be inappropriate in the future
			"v1/Pod/openshift-etcd/installer-<revision>-<node>/container/installer/limit[cpu]":                          "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-etcd/installer-<revision>-<node>/container/installer/limit[memory]":                       "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-etcd/revision-pruner-<revision>-<node>/container/pruner/limit[cpu]":                       "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-etcd/revision-pruner-<revision>-<node>/container/pruner/limit[memory]":                    "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-apiserver/installer-<revision>-<node>/container/installer/limit[cpu]":                "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-apiserver/installer-<revision>-<node>/container/installer/limit[memory]":             "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-apiserver/revision-pruner-<revision>-<node>/container/pruner/limit[cpu]":             "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-apiserver/revision-pruner-<revision>-<node>/container/pruner/limit[memory]":          "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-controller-manager/installer-<revision>-<node>/container/installer/limit[cpu]":       "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-controller-manager/installer-<revision>-<node>/container/installer/limit[memory]":    "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-controller-manager/revision-pruner-<revision>-<node>/container/pruner/limit[cpu]":    "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-controller-manager/revision-pruner-<revision>-<node>/container/pruner/limit[memory]": "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-scheduler/installer-<revision>-<node>/container/installer/limit[cpu]":                "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-scheduler/installer-<revision>-<node>/container/installer/limit[memory]":             "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-scheduler/revision-pruner-<revision>-<node>/container/pruner/limit[cpu]":             "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",
			"v1/Pod/openshift-kube-scheduler/revision-pruner-<revision>-<node>/container/pruner/limit[memory]":          "smarterclayton: run-once pod with very well-known resource usage, does not vary based on workload or cluster size",

			"apps/v1/Deployment/openshift-monitoring/thanos-querier/container/thanos-query/limit[memory]": "smarterclayton: granted a temporary exception (reasses in 4.10) until Thanos can properly control resource usage from arbitrary queries",
		}

		reNormalizeRunOnceNames := regexp.MustCompile(`^(installer-|revision-pruner-)[\d]+-`)

		waitingForFix := sets.NewString()
		notAllowed := sets.NewString()
		possibleFuture := sets.NewString()
		for _, pod := range pods.Items {
			// Only pods in the openshift-*, kube-*, and default namespaces are considered
			if !strings.HasPrefix(pod.Namespace, "openshift-") && !strings.HasPrefix(pod.Namespace, "kube-") && pod.Namespace != "default" {
				continue
			}
			// Must-gather runs are excluded from this rule
			if strings.HasPrefix(pod.Namespace, "openshift-must-gather") {
				continue
			}
			// var controlPlaneTarget bool
			// selector := labels.SelectorFromSet(pod.Spec.NodeSelector)
			// if !selector.Empty() && selector.Matches(labels.Set(map[string]string{"node-role.kubernetes.io/master": ""})) {
			// 	controlPlaneTarget = true
			// }

			// Find a unique string that identifies who creates the pod, or the pod itself
			var controller string
			for _, ref := range pod.OwnerReferences {
				if ref.Controller == nil || !*ref.Controller {
					continue
				}
				// simple hack to make the rules cluster better, if we get new hierarchies just add more checks here
				switch ref.Kind {
				case "ReplicaSet":
					if i := strings.LastIndex(ref.Name, "-"); i != -1 {
						name := ref.Name[0:i]
						if deploy, err := oc.KubeFramework().ClientSet.AppsV1().Deployments(pod.Namespace).Get(context.Background(), name, metav1.GetOptions{}); err == nil {
							ref.Name = deploy.Name
							ref.Kind = "Deployment"
							ref.APIVersion = "apps/v1"
						}
					}
				case "Job":
					if pod.Namespace == "openshift-marketplace" {
						ref.Name = "<batch_job>"
					}
				case "Node":
					continue
				}
				controller = fmt.Sprintf("%s/%s/%s/%s", ref.APIVersion, ref.Kind, pod.Namespace, ref.Name)
				break
			}
			if len(controller) == 0 {
				if len(pod.GenerateName) > 0 {
					name := strings.ReplaceAll(pod.GenerateName, pod.Spec.NodeName, "<node>")
					if pod.Spec.RestartPolicy != v1.RestartPolicyAlways {
						name = reNormalizeRunOnceNames.ReplaceAllString(name, "$1<revision>")
					}
					controller = fmt.Sprintf("v1/Pod/%s/%s", pod.Namespace, name)
				} else {
					name := strings.ReplaceAll(pod.Name, pod.Spec.NodeName, "<node>")
					if pod.Spec.RestartPolicy != v1.RestartPolicyAlways {
						name = reNormalizeRunOnceNames.ReplaceAllString(name, "$1<revision>-")
					}
					controller = fmt.Sprintf("v1/Pod/%s/%s", pod.Namespace, name)
				}
			}

			// These rules apply to both init and regular containers
			for containerType, containers := range map[string][]v1.Container{
				"initContainer": pod.Spec.InitContainers,
				"container":     pod.Spec.Containers,
			} {
				for _, c := range containers {
					key := fmt.Sprintf("%s/%s/%s", controller, containerType, c.Name)

					// Pods may not set limits
					if len(c.Resources.Limits) > 0 {
						for resource, v := range c.Resources.Limits {
							rule := fmt.Sprintf("%s/%s[%s]", key, "limit", resource)
							if len(exceptionGranted[rule]) == 0 {
								violation := fmt.Sprintf("%s defines a limit on %s of %s which is not allowed", key, resource, v.String())
								if bug, ok := knownBrokenPods[rule]; ok {
									waitingForFix.Insert(fmt.Sprintf("%s (bug %s)", violation, bug))
								} else {
									notAllowed.Insert(fmt.Sprintf("%s (rule: %q)", violation, rule))
								}
							}
						}
					}

					// Pods must have at least CPU and memory requests
					for _, resource := range []string{"cpu", "memory"} {
						v := c.Resources.Requests[v1.ResourceName(resource)]
						if !v.IsZero() {
							continue
						}
						rule := fmt.Sprintf("%s/%s[%s]", key, "request", resource)
						violation := fmt.Sprintf("%s does not have a %s request", key, resource)
						if len(exceptionGranted[rule]) == 0 {
							if bug, ok := knownBrokenPods[rule]; ok {
								waitingForFix.Insert(fmt.Sprintf("%s (bug %s)", violation, bug))
							} else {
								if containerType == "initContainer" {
									possibleFuture.Insert(fmt.Sprintf("%s (candidate rule: %q)", violation, rule))
								} else {
									notAllowed.Insert(fmt.Sprintf("%s (rule: %q)", violation, rule))
								}
							}
						}
					}
				}
			}
		}

		// Some things we may start checking in the future
		if len(possibleFuture) > 0 {
			e2e.Logf("Pods in platform namespaces had resource request/limit that we may enforce in the future:\n\n%s", strings.Join(possibleFuture.List(), "\n"))
		}

		// Users are not allowed to add new violations
		if len(notAllowed) > 0 {
			e2e.Failf("Pods in platform namespaces are not following resource request/limit rules or do not have an exception granted:\n  %s", strings.Join(notAllowed.List(), "\n  "))
		}

		// All known bugs are listed as flakes so we can see them as dashboards
		if len(waitingForFix) > 0 {
			result.Flakef("Pods in platform namespaces had known broken resource request/limit that have not been resolved:\n\n%s", strings.Join(waitingForFix.List(), "\n"))
		}
	})
})
