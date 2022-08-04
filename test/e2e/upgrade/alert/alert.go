package alert

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/synthetictests/allowedalerts"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/disruption"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

// UpgradeTest runs verifies invariants regarding what alerts are allowed to fire
// during the upgrade process.
type UpgradeTest struct {
	oc               *exutil.CLI
	prometheusClient prometheusv1.API
	configClient     configclient.Interface
}

func (UpgradeTest) Name() string { return "check-for-alerts" }
func (UpgradeTest) DisplayName() string {
	return "[sig-arch] Check if alerts are firing during or after upgrade success"
}

// Setup creates parameters to query Prometheus
func (t *UpgradeTest) Setup(f *framework.Framework) {
	g.By("Setting up upgrade alert test")

	oc := exutil.NewCLIWithFramework(f)
	t.oc = oc
	t.prometheusClient = oc.NewPrometheusClient(context.TODO())
	t.configClient = oc.AdminConfigClient()
	framework.Logf("Post-upgrade alert test setup complete")
}

// Test checks if alerts are firing at various points during upgrade.
// An alert firing during an upgrade is a high severity bug - it either points to a real issue in
// a dependency, or a failure of the component, and therefore must be fixed.
func (t *UpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	tolerateDuringSkew := exutil.TolerateVersionSkewInTests()
	firingAlertsWithBugs := helper.MetricConditions{
		{
			Selector: map[string]string{"alertname": "KubePodNotReady", "namespace": "openshift-kube-apiserver-operator"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1939580",
		},
		{
			Selector: map[string]string{"alertname": "KubePodNotReady", "namespace": "openshift-kube-apiserver-operator"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1939580",
		},
		{
			Selector: map[string]string{"alertname": "ClusterOperatorDegraded", "name": "openshift-apiserver"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1939580",
		},
		{
			Selector: map[string]string{"alertname": "ClusterOperatorDown", "name": "authentication"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1939580",
		},
		{
			Selector: map[string]string{"alertname": "ClusterOperatorDown", "name": "machine-config"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1955300",
		},
		{
			Selector: map[string]string{"alertname": "ClusterOperatorDegraded", "name": "authentication"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1939580",
		},
		{
			Selector: map[string]string{"alertname": "KubeDaemonSetRolloutStuck"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1943667",
		},
		{
			Selector: map[string]string{"alertname": "KubeAPIErrorBudgetBurn"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1953798",
			Matches: func(_ *model.Sample) bool {
				return framework.ProviderIs("gce")
			},
		},
		{
			Selector: map[string]string{"alertname": "AggregatedAPIDown", "namespace": "default", "name": "v1beta1.metrics.k8s.io"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1970624",
			Matches: func(_ *model.Sample) bool {
				return framework.ProviderIs("gce")
			},
		},
		{
			// Should be removed one release after the attached bugzilla is fixed.
			Selector: map[string]string{"alertname": "HighlyAvailableWorkloadIncorrectlySpread", "namespace": "openshift-monitoring", "workload": "prometheus-k8s"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1949262",
		},
		{
			// Should be removed one release after the attached bugzilla is fixed.
			Selector: map[string]string{"alertname": "HighlyAvailableWorkloadIncorrectlySpread", "namespace": "openshift-monitoring", "workload": "alertmanager-main"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1955489",
		},
		{
			// Should be removed one release after the attached bugzilla is fixed, or after that bug is fixed in a backport to the previous minor.
			Selector: map[string]string{"alertname": "ExtremelyHighIndividualControlPlaneCPU"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1985073",
			Matches: func(_ *model.Sample) bool {
				return framework.ProviderIs("gce")
			},
		},
		{
			Selector: map[string]string{"alertname": "KubeJobFailed", "namespace": "openshift-multus"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=2054426",
			Matches: func(sample *model.Sample) bool {
				// Only match if the job_name label starts with ip-reconciler:
				if strings.HasPrefix(string(sample.Metric[model.LabelName("job_name")]), "ip-reconciler-") {
					return true
				}
				return false
			},
		},
	}
	allowedFiringAlerts := helper.MetricConditions{
		{
			Selector: map[string]string{"alertname": "TargetDown", "namespace": "openshift-e2e-loki"},
			Text:     "Loki is nice to have, but we can allow it to be down",
		},
		{
			Selector: map[string]string{"alertname": "KubePodNotReady", "namespace": "openshift-e2e-loki"},
			Text:     "Loki is nice to have, but we can allow it to be down",
		},
		{
			Selector: map[string]string{"alertname": "KubeDeploymentReplicasMismatch", "namespace": "openshift-e2e-loki"},
			Text:     "Loki is nice to have, but we can allow it to be down",
		},
	}

	pendingAlertsWithBugs := helper.MetricConditions{
		{
			Selector: map[string]string{"alertname": "ClusterMonitoringOperatorReconciliationErrors"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1932624",
		},
		{
			Selector: map[string]string{"alertname": "KubeClientErrors"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1925698",
		},
		{
			Selector: map[string]string{"alertname": "NetworkPodsCrashLooping"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=2009078",
		},
	}
	allowedPendingAlerts := helper.MetricConditions{
		{
			Selector: map[string]string{"alertname": "etcdMemberCommunicationSlow"},
			Text:     "Excluded because it triggers during upgrade (detects ~5m of high latency immediately preceeding the end of the test), and we don't want to change the alert because it is correct",
		},
	}

	if featureGates, err := t.configClient.ConfigV1().FeatureGates().Get(context.TODO(), "cluster", metav1.GetOptions{}); err == nil {
		switch featureGates.Spec.FeatureSet {
		case configv1.TechPreviewNoUpgrade:
			allowedFiringAlerts = append(allowedFiringAlerts,
				helper.MetricCondition{
					Selector: map[string]string{"alertname": "TechPreviewNoUpgrade", "namespace": "openshift-kube-apiserver-operator"},
					Text:     "When running as TechPreviewNoUpgrade, we allow TechPreviewNoUpgrade alert to be firing.",
				},
			)
		}
	}

	// we exclude alerts that have their own separate tests.
	for _, alertTest := range allowedalerts.AllAlertTests(context.TODO(), nil, 0) {
		switch alertTest.AlertState() {
		case allowedalerts.AlertPending:
			// a pending test covers pending and everything above (firing)
			allowedPendingAlerts = append(allowedPendingAlerts,
				helper.MetricCondition{
					Selector: map[string]string{"alertname": alertTest.AlertName()},
					Text:     "has a separate e2e test",
				},
			)
			allowedFiringAlerts = append(allowedFiringAlerts,
				helper.MetricCondition{
					Selector: map[string]string{"alertname": alertTest.AlertName()},
					Text:     "has a separate e2e test",
				},
			)
		case allowedalerts.AlertInfo:
			// an info test covers all firing
			allowedFiringAlerts = append(allowedFiringAlerts,
				helper.MetricCondition{
					Selector: map[string]string{"alertname": alertTest.AlertName()},
					Text:     "has a separate e2e test",
				},
			)
		}
	}

	knownViolations := sets.NewString()
	unexpectedViolations := sets.NewString()
	unexpectedViolationsAsFlakes := sets.NewString()
	debug := sets.NewString()

	g.By("Checking for alerts")

	start := time.Now()

	// Block until upgrade is done
	g.By("Waiting for upgrade to finish before checking for alerts")
	<-done

	// Additonal delay after upgrade completion to allow pending alerts to settle
	g.By("Waiting before checking for alerts")
	time.Sleep(1 * time.Minute)

	testDuration := time.Now().Sub(start).Round(time.Second)

	// Invariant: No non-info level alerts should have fired during the upgrade
	firingAlertQuery := fmt.Sprintf(`
sort_desc(
count_over_time(ALERTS{alertstate="firing",severity!="info",alertname!~"Watchdog|AlertmanagerReceiversNotConfigured"}[%[1]s:1s])
) > 0
`, testDuration)
	result, err := helper.RunQuery(context.TODO(), t.prometheusClient, firingAlertQuery)
	o.Expect(err).NotTo(o.HaveOccurred(), "unable to check firing alerts during upgrade")
	for _, series := range result.Data.Result {
		labels := helper.StripLabels(series.Metric, "alertname", "alertstate", "prometheus")
		violation := fmt.Sprintf("alert %s fired for %s seconds with labels: %s", series.Metric["alertname"], series.Value, helper.LabelsAsSelector(labels))
		if cause := allowedFiringAlerts.Matches(series); cause != nil {
			debug.Insert(fmt.Sprintf("%s (allowed: %s)", violation, cause.Text))
			continue
		}
		if cause := firingAlertsWithBugs.Matches(series); cause != nil {
			knownViolations.Insert(fmt.Sprintf("%s (open bug: %s)", violation, cause.Text))
		} else {
			unexpectedViolations.Insert(violation)
		}
	}

	// Invariant: There should be no pending alerts 1m after the upgrade completes
	pendingAlertQuery := fmt.Sprintf(`
sort_desc(
  time() * ALERTS + 1
  -
  last_over_time((
    time() * ALERTS{alertname!~"Watchdog|AlertmanagerReceiversNotConfigured",alertstate="pending",severity!="info"}
    unless
    ALERTS offset 1s
  )[%[1]s:1s])
)
`, testDuration)
	result, err = helper.RunQuery(context.TODO(), t.prometheusClient, pendingAlertQuery)
	o.Expect(err).NotTo(o.HaveOccurred(), "unable to retrieve pending alerts after upgrade")
	for _, series := range result.Data.Result {
		labels := helper.StripLabels(series.Metric, "alertname", "alertstate", "prometheus")
		violation := fmt.Sprintf("alert %s pending for %s seconds with labels: %s", series.Metric["alertname"], series.Value, helper.LabelsAsSelector(labels))
		if cause := allowedPendingAlerts.Matches(series); cause != nil {
			debug.Insert(fmt.Sprintf("%s (allowed: %s)", violation, cause.Text))
			continue
		}
		if cause := pendingAlertsWithBugs.Matches(series); cause != nil {
			knownViolations.Insert(fmt.Sprintf("%s (open bug: %s)", violation, cause.Text))
		} else {
			// treat pending errors as a flake right now because we are still trying to determine the scope
			// TODO: move this to unexpectedViolations later
			unexpectedViolationsAsFlakes.Insert(violation)
		}
	}

	if len(debug) > 0 {
		framework.Logf("Alerts were detected during upgrade which are allowed:\n\n%s", strings.Join(debug.List(), "\n"))
	}
	if len(unexpectedViolations) > 0 {
		if !tolerateDuringSkew {
			framework.Failf("Unexpected alerts fired or pending during the upgrade:\n\n%s", strings.Join(unexpectedViolations.List(), "\n"))
		}
	}
	if flakes := sets.NewString().Union(knownViolations).Union(unexpectedViolations).Union(unexpectedViolationsAsFlakes); len(flakes) > 0 {
		disruption.FrameworkFlakef(f, "Unexpected alert behavior during upgrade:\n\n%s", strings.Join(flakes.List(), "\n"))
	}
	framework.Logf("No alerts fired during upgrade")
}

// Teardown cleans up any remaining resources.
func (t *UpgradeTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}
