package operators

import (
	"context"
	"fmt"
	"sort"
	"strings"

	g "github.com/onsi/ginkgo"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-arch] Managed cluster", func() {
	oc := exutil.NewCLIWithoutNamespace("operator-daemonsets")

	// Daemonsets shipped with the platform must be able to upgrade without disruption to workloads.
	// Daemonsets that are in the data path must gracefully shutdown and redirect workload traffic, or
	// mitigate outage by holding requests briefly (very briefly!). Daemonsets that are part of control
	// plane (anything that changes state) must already be able to be upgraded without disruption by
	// having any components that call them retry when unavailable up to a specific duration. Therefore
	// all workloads must individually be graceful, and so a core daemonset can upgrade up to 10% or 33%
	// of pods at a time. In the future, we may allow this percentage to be tuned via a global config,
	// and this test would enforce that.
	//
	// Use 33% maxUnavailable if you are a workload that has no impact on other workload. This ensures
	// that if there is a bug in the newly rolled out workload, 2/3 of instances remain working.
	// Workloads in this category include the spot instance termination signal observer which listens
	// for when the cloud signals a node that it will be shutdown in 30s. At worst, only 1/3 of machines
	// would be impacted by a bug and at best the new code would roll out that much faster in very large
	// spot instance machine sets.
	//
	// Use 10% maxUnavailable in all other cases, most especially if you have ANY impact on user
	// workloads. This limits the additional load placed on the cluster to a more reasonable degree
	// during an upgrade as new pods start and then establish connections.
	//
	// Currently only applies to daemonsets that don't explicitly target the control plane.
	g.It("should only include cluster daemonsets that have maxUnavailable update of 10 or 33 percent", func() {
		// iterate over the references to find valid images
		daemonSets, err := oc.KubeFramework().ClientSet.AppsV1().DaemonSets("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list daemonsets: %v", err)
		}

		// daemonsets that have an explicit exception, every entry here must have a bug associated
		// e.g. "openshift-x/foo": "link to bug"
		bugsAgainstBrokenDaemonSets := map[string]string{
			"openshift-multus/multus":                 "https://bugzilla.redhat.com/show_bug.cgi?id=1933159",
			"openshift-multus/network-metrics-daemon": "https://bugzilla.redhat.com/show_bug.cgi?id=1933159",
		}

		var debug []string
		var knownBroken []string
		var invalidDaemonSets []string
		for _, ds := range daemonSets.Items {
			if !strings.HasPrefix(ds.Namespace, "openshift-") {
				continue
			}
			if ds.Spec.Selector != nil {
				selector := labels.SelectorFromSet(ds.Spec.Template.Spec.NodeSelector)
				if !selector.Empty() && selector.Matches(labels.Set(map[string]string{"node-role.kubernetes.io/master": ""})) {
					continue
				}
			}
			key := fmt.Sprintf("%s/%s", ds.Namespace, ds.Name)
			switch {
			case ds.Spec.UpdateStrategy.RollingUpdate == nil,
				ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable == nil:
				violation := fmt.Sprintf("expected daemonset %s to have a maxUnavailable strategy of 10%% or 33%%", key)
				if bug, ok := bugsAgainstBrokenDaemonSets[key]; ok {
					knownBroken = append(knownBroken, fmt.Sprintf("%s (bug %s)", violation, bug))
				} else {
					invalidDaemonSets = append(invalidDaemonSets, violation)
				}
				debug = append(debug, violation)
			case ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.StrVal != "10%" &&
				ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.StrVal != "33%":
				violation := fmt.Sprintf("expected daemonset %s to have maxUnavailable 10%% or 33%% (see comment) instead of %s", key, ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.String())
				if bug, ok := bugsAgainstBrokenDaemonSets[key]; ok {
					knownBroken = append(knownBroken, fmt.Sprintf("%s (bug %s)", violation, bug))
				} else {
					invalidDaemonSets = append(invalidDaemonSets, violation)
				}
				debug = append(debug, violation)
			default:
				debug = append(debug, fmt.Sprintf("daemonset %s has %s", key, ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.String()))
			}
		}

		sort.Strings(debug)
		e2e.Logf("Daemonset configuration in payload:\n%s", strings.Join(debug, "\n"))

		// All known bugs are listed as flakes so we can see them as dashboards
		if len(knownBroken) > 0 {
			sort.Strings(knownBroken)
			result.Flakef("Daemonsets with outstanding bugs in payload:\n%s", strings.Join(knownBroken, "\n"))
		}

		// Users are not allowed to add new violations
		if len(invalidDaemonSets) > 0 {
			e2e.Failf("Daemonsets found that do not meet platform requirements for update strategy:\n  %s", strings.Join(invalidDaemonSets, "\n  "))
		}
	})
})
