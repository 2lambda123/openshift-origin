// Package clusterversionoperator contains utilities for exercising the cluster-version operator.
package clusterversionoperator

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

// AdminAckTest contains artifacts used during test
type AdminAckTest struct {
	Oc     *exutil.CLI
	Config *restclient.Config
	Poll   time.Duration
}

const (
	adminAckGateFmt     string = "^ack-[4-5][.]([0-9]{1,})-[^-]"
	upgradeProgression  string = `\(([0-9]{1,2})% complete\)`
	snoUpgradeThreshold int    = 80
)

var (
	adminAckGateRegexp       = regexp.MustCompile(adminAckGateFmt)
	upgradeProgressionRegexp = regexp.MustCompile(upgradeProgression)
)

// Test simply returns successfully if admin ack functionality is not part of the baseline being tested. Otherwise,
// for each configured admin ack gate, test verifies the gate name format and that it contains a description. If
// valid and the gate is applicable to the OCP version under test, test checks the value of the admin ack gate.
// If the gate has been ack'ed the test verifies that the Upgradeable condition does not complain about the ack. Test
// then clears the ack and verifies that the Upgradeable condition complains about the ack. Test then sets the ack
// and verifies that the Upgradeable condition no longer complains about the ack.
func (t *AdminAckTest) Test(ctx context.Context) {
	if t.Poll == 0 {
		t.test(ctx, nil)
		return
	}

	infra, err := t.Oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		framework.Fail(err.Error())
	}
	sno := infra.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode

	exercisedGates := map[string]struct{}{}
	if err := wait.PollImmediateUntilWithContext(ctx, t.Poll, func(ctx context.Context) (bool, error) {
		// When running against SNO the test should not run when API Server is down which happens on reboot.
		// Reboot happens around 84% of upgrade completion.
		if sno && !shouldTestRun(ctx, t.Oc) {
			return false, nil
		}
		t.test(ctx, exercisedGates)
		return false, nil
	}); err == nil || err == wait.ErrWaitTimeout {
		return
	} else {
		framework.Fail(err.Error())
	}
}

func (t *AdminAckTest) test(ctx context.Context, exercisedGates map[string]struct{}) {
	exists := struct{}{}

	gateCm, err := getAdminGatesConfigMap(ctx, t.Oc)
	if err != nil {
		framework.Fail(err.Error())
	}
	// Check if this release has admin ack functionality.
	if gateCm == nil || (gateCm != nil && len(gateCm.Data) == 0) {
		framework.Logf("Skipping admin ack test. Admin ack is not in this baseline or contains no gates.")
		return
	}
	ackCm, err := getAdminAcksConfigMap(ctx, t.Oc)
	if err != nil {
		framework.Fail(err.Error())
	}
	currentVersion := getCurrentVersion(ctx, t.Config)
	var msg string
	for k, v := range gateCm.Data {
		if exercisedGates != nil {
			if _, ok := exercisedGates[k]; ok {
				continue
			}
		}
		ackVersion := adminAckGateRegexp.FindString(k)
		if ackVersion == "" {
			framework.Failf("Configmap openshift-config-managed/admin-gates gate %s has invalid format; must comply with %q.", k, adminAckGateFmt)
		}
		if v == "" {
			framework.Failf("Configmap openshift-config-managed/admin-gates gate %s does not contain description.", k)
		}
		if !gateApplicableToCurrentVersion(ackVersion, currentVersion) {
			continue
		}
		if ackCm.Data[k] == "true" {
			if upgradeableExplicitlyFalse(ctx, t.Config) {
				if adminAckRequiredWithMessage(ctx, t.Config, v) {
					framework.Failf("Gate %s has been ack'ed but Upgradeable is "+
						"false with reason AdminAckRequired and message %q.", k, v)
				}
				framework.Logf("Gate %s has been ack'ed. Upgradeable is "+
					"false but not due to this gate which would set reason AdminAckRequired with message %s. %s", k, v, getUpgradeable(ctx, t.Config))
			}
			// Clear admin ack configmap gate ack
			if err := setAdminGate(ctx, k, "", t.Oc); err != nil {
				framework.Fail(err.Error())
			}
		}
		if err := waitForAdminAckRequired(ctx, t.Config, msg); err != nil {
			framework.Fail(err.Error())
		}
		// Update admin ack configmap with ack
		if err := setAdminGate(ctx, k, "true", t.Oc); err != nil {
			framework.Fail(err.Error())
		}
		if err = waitForAdminAckNotRequired(ctx, t.Config, msg); err != nil {
			framework.Fail(err.Error())
		}
		if exercisedGates != nil {
			exercisedGates[k] = exists
		}
	}
	framework.Logf("Admin Ack verified")
}

// getClusterVersion returns the ClusterVersion object.
func getClusterVersion(ctx context.Context, config *restclient.Config) *configv1.ClusterVersion {
	c, err := configv1client.NewForConfig(config)
	if err != nil {
		framework.Failf("Error getting config, err=%v", err)
	}
	cv, err := c.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		framework.Failf("Error getting custer version, err=%v", err)
	}
	return cv
}

// getCurrentVersion determines and returns the cluster's current version by iterating through the
// provided update history until it finds the first version with update State of Completed. If a
// Completed version is not found the version of the oldest history entry, which is the originally
// installed version, is returned. If history is empty the empty string is returned.
func getCurrentVersion(ctx context.Context, config *restclient.Config) string {
	cv := getClusterVersion(ctx, config)
	for _, h := range cv.Status.History {
		if h.State == configv1.CompletedUpdate {
			return h.Version
		}
	}
	// Empty history should only occur if method is called early in startup before history is populated.
	if len(cv.Status.History) != 0 {
		return cv.Status.History[len(cv.Status.History)-1].Version
	}
	return ""
}

// getEffectiveMinor attempts to do a simple parse of the version provided.  If it does not parse, the value is considered
// an empty string, which works for a comparison for equivalence.
func getEffectiveMinor(version string) string {
	splits := strings.Split(version, ".")
	if len(splits) < 2 {
		return ""
	}
	return splits[1]
}

func gateApplicableToCurrentVersion(gateAckVersion string, currentVersion string) bool {
	parts := strings.Split(gateAckVersion, "-")
	ackMinor := getEffectiveMinor(parts[1])
	cvMinor := getEffectiveMinor(currentVersion)
	if ackMinor == cvMinor {
		return true
	}
	return false
}

func getAdminGatesConfigMap(ctx context.Context, oc *exutil.CLI) (*corev1.ConfigMap, error) {
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-config-managed").Get(ctx, "admin-gates", metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("Error accessing configmap openshift-config-managed/admin-gates: %w", err)
		} else {
			return nil, nil
		}
	}
	return cm, nil
}

func getAdminAcksConfigMap(ctx context.Context, oc *exutil.CLI) (*corev1.ConfigMap, error) {
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-config").Get(ctx, "admin-acks", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Error accessing configmap openshift-config/admin-acks: %w", err)
	}
	return cm, nil
}

// adminAckRequiredWithMessage returns true if Upgradeable condition reason contains AdminAckRequired
// and message contains given message.
func adminAckRequiredWithMessage(ctx context.Context, config *restclient.Config, message string) bool {
	clusterVersion := getClusterVersion(ctx, config)
	cond := getUpgradeableStatusCondition(clusterVersion.Status.Conditions)
	if cond != nil && strings.Contains(cond.Reason, "AdminAckRequired") && strings.Contains(cond.Message, message) {
		return true
	}
	return false
}

// upgradeableExplicitlyFalse returns true if the Upgradeable condition status is set to false.
func upgradeableExplicitlyFalse(ctx context.Context, config *restclient.Config) bool {
	clusterVersion := getClusterVersion(ctx, config)
	cond := getUpgradeableStatusCondition(clusterVersion.Status.Conditions)
	if cond != nil && cond.Status == configv1.ConditionFalse {
		return true
	}
	return false
}

// setAdminGate gets the admin ack configmap and then updates it with given gate name and given value.
func setAdminGate(ctx context.Context, gateName string, gateValue string, oc *exutil.CLI) error {
	ackCm, err := getAdminAcksConfigMap(ctx, oc)
	if err != nil {
		return err
	}
	if ackCm.Data == nil {
		ackCm.Data = make(map[string]string)
	}
	ackCm.Data[gateName] = gateValue
	if _, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-config").Update(ctx, ackCm, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("Unable to update configmap openshift-config/admin-acks: %w", err)
	}
	return nil
}

func waitForAdminAckRequired(ctx context.Context, config *restclient.Config, message string) error {
	framework.Logf("Waiting for Upgradeable to be AdminAckRequired for %q ...", message)
	if err := wait.PollImmediate(10*time.Second, 3*time.Minute, func() (bool, error) {
		if adminAckRequiredWithMessage(ctx, config, message) {
			return true, nil
		}
		return false, nil
	}); err != nil {
		return fmt.Errorf("Error while waiting for Upgradeable to go AdminAckRequired with message %q: %w\n%s", message, err, getUpgradeable(ctx, config))
	}
	return nil
}

func waitForAdminAckNotRequired(ctx context.Context, config *restclient.Config, message string) error {
	framework.Logf("Waiting for Upgradeable to not be AdminAckRequired for %q ...", message)
	if err := wait.PollImmediate(10*time.Second, 3*time.Minute, func() (bool, error) {
		if !adminAckRequiredWithMessage(ctx, config, message) {
			return true, nil
		}
		return false, nil
	}); err != nil {
		return fmt.Errorf("Error while waiting for Upgradeable to not be AdminAckRequired with message %q: %w\n%s", message, err, getUpgradeable(ctx, config))
	}
	return nil
}

func getUpgradeableStatusCondition(conditions []configv1.ClusterOperatorStatusCondition) *configv1.ClusterOperatorStatusCondition {
	return findOperatorStatusCondition(conditions, configv1.OperatorUpgradeable)
}

func getUpgradeable(ctx context.Context, config *restclient.Config) string {
	clusterVersion := getClusterVersion(ctx, config)
	cond := getUpgradeableStatusCondition(clusterVersion.Status.Conditions)
	if cond != nil {
		return fmt.Sprintf("Upgradeable: Status=%s, Reason=%s, Message=%q.", cond.Status, cond.Reason, cond.Message)
	}
	return "Upgradeable nil"
}

func findOperatorStatusCondition(conditions []configv1.ClusterOperatorStatusCondition, conditionType configv1.ClusterStatusConditionType) *configv1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// shouldTestRun decides if a test should be executed when running against SNO.
// Admin ack test should not run against SNO when:
// - API Server is not responding (SNO lacks HA), or
// - Reboot will happen shortly (upgrade progression above the threshold)
func shouldTestRun(ctx context.Context, oc *exutil.CLI) bool {
	ver, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if _, ok := err.(net.Error); ok {
		framework.Logf("Skipping admin ack test. Failed to contact the API Server.")
		return false
	}

	progCond := findOperatorStatusCondition(ver.Status.Conditions, configv1.OperatorProgressing)

	// If there's not enough data to make a decision - just run the test
	if progCond == nil || progCond.Status != configv1.ConditionTrue || !strings.Contains(progCond.Message, "% complete") {
		return true
	}

	matches := upgradeProgressionRegexp.FindStringSubmatch(progCond.Message)
	if len(matches) != 2 {
		framework.Failf("Admin ack test: unexpected amount regexp match: %v", matches)
	}

	percent, err := strconv.Atoi(matches[1])
	if err != nil {
		framework.Failf("Admin ack test: failed to convert upgrade percentage to int: %v", err.Error())
	}

	return percent < snoUpgradeThreshold
}
