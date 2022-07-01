package operators

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/stretchr/objx"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
)

var (
	vmxPattern = regexp.MustCompile(`VMwareVSphereControllerUpgradeable.+vmx-13`)
)

var _ = g.Describe("[sig-arch][Early] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("start all core operators", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		// presence of the CVO namespace gates this test
		g.By("checking for the cluster version operator")
		skipUnlessCVO(c.CoreV1().Namespaces())

		g.By("ensuring cluster version is stable")
		cvc := dc.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Resource: "clusterversions", Version: "v1"})
		obj, err := cvc.Get(context.Background(), "version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		cv := objx.Map(obj.UnstructuredContent())
		if cond := condition(cv, "Available"); cond.Get("status").String() != "True" {
			e2e.Failf("ClusterVersion Available=%s: %s: %s", cond.Get("status").String(), cond.Get("reason").String(), cond.Get("message").String())
		}
		if cond := condition(cv, "Failing"); cond.Get("status").String() != "False" {
			e2e.Failf("ClusterVersion Failing=%s: %s: %s", cond.Get("status").String(), cond.Get("reason").String(), cond.Get("message").String())
		}
		if cond := condition(cv, "Progressing"); cond.Get("status").String() != "False" {
			e2e.Failf("ClusterVersion Progressing=%s: %s: %s", cond.Get("status").String(), cond.Get("reason").String(), cond.Get("message").String())
		}

		g.By("determining if the cluster is in a TechPreview state")
		fgc := dc.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Resource: "featuregates", Version: "v1"})
		fgObj, err := fgc.Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		fg := objx.Map(fgObj.UnstructuredContent())
		featureSet := fg.Get("spec.featureSet").String()
		isNoUpgrade := featureSet == "TechPreviewNoUpgrade" || featureSet == "CustomNoUpgrade"

		// gate on all clusteroperators being ready
		g.By("ensuring all cluster operators are stable")
		coc := dc.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Resource: "clusteroperators", Version: "v1"})
		clusterOperatorsObj, err := coc.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		clusterOperators := objx.Map(clusterOperatorsObj.UnstructuredContent())
		items := objects(clusterOperators.Get("items"))
		if len(items) == 0 {
			e2e.Failf("There must be at least one cluster operator")
		}

		var unready []string
		for _, co := range items {
			name := co.Get("metadata.name").String()
			badConditions, missingTypes := surprisingConditions(co)
			if len(badConditions) > 0 {
				worstCondition := badConditions[0]

				// kube-apiserver blocks upgrades when feature gates are present.
				// Allow testing of TechPreviewNoUpgrade clusters by ignoring this condition.
				if isNoUpgrade && name == "kube-apiserver" && isKubeAPIUpgradableNoUpgradeCondition(worstCondition) {
					continue
				}

				unready = append(unready, fmt.Sprintf("%s (%s=%s %s: %s)",
					name,
					worstCondition.Type,
					worstCondition.Status,
					worstCondition.Reason,
					worstCondition.Message,
				))
			} else if len(missingTypes) > 0 {
				missingTypeStrings := make([]string, 0, len(missingTypes))
				for _, missingType := range missingTypes {
					missingTypeStrings = append(missingTypeStrings, string(missingType))
				}
				unready = append(unready, fmt.Sprintf("%s (missing: %s)", name, strings.Join(missingTypeStrings, ", ")))
			}
		}
		if len(unready) > 0 {
			sort.Strings(unready)
			e2e.Failf("Some cluster operators are not ready: %s", strings.Join(unready, ", "))
		}
	})
})

var _ = g.Describe("[sig-arch] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have operators on the cluster version", func() {
		if len(os.Getenv("TEST_UNSUPPORTED_ALLOW_VERSION_SKEW")) > 0 {
			e2eskipper.Skipf("Test is disabled to allow cluster components to have different versions")
		}
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c := configclient.NewForConfigOrDie(cfg)
		coreclient, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		// presence of the CVO namespace gates this test
		g.By("checking for the cluster version operator")
		skipUnlessCVO(coreclient.CoreV1().Namespaces())

		// we need to get the list of versions
		cv, err := c.ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		coList, err := c.ConfigV1().ClusterOperators().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(coList.Items).NotTo(o.BeEmpty())

		g.By("all cluster operators report an operator version in the first position equal to the cluster version")
		for _, co := range coList.Items {
			if co.Name == "baremetal" {
				continue // Metal images are being rebuilt on RHEL9.
			}
			msg := fmt.Sprintf("unexpected operator status versions %s:\n%#v", co.Name, co.Status.Versions)
			o.Expect(co.Status.Versions).NotTo(o.BeEmpty(), msg)
			operator := findOperatorVersion(co.Status.Versions, "operator")
			o.Expect(operator).NotTo(o.BeNil(), msg)
			o.Expect(operator.Name).To(o.Equal("operator"), msg)
			o.Expect(operator.Version).To(o.Equal(cv.Status.Desired.Version), msg)
		}
	})
})

func skipUnlessCVO(c coreclient.NamespaceInterface) {
	err := wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		_, err := c.Get(context.Background(), "openshift-cluster-version", metav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		if errors.IsNotFound(err) {
			e2eskipper.Skipf("The cluster is not managed by a cluster-version operator")
		}
		e2e.Logf("Unable to check for cluster version operator: %v", err)
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func findOperatorVersion(versions []configv1.OperandVersion, name string) *configv1.OperandVersion {
	for i := range versions {
		if versions[i].Name == name {
			return &versions[i]
		}
	}
	return nil
}

func contains(names []string, name string) bool {
	for _, s := range names {
		if s == name {
			return true
		}
	}
	return false
}

func jsonString(from objx.Map) string {
	s, _ := from.JSON()
	return s
}

func objects(from *objx.Value) []objx.Map {
	var values []objx.Map
	switch {
	case from.IsObjxMapSlice():
		return from.ObjxMapSlice()
	case from.IsInterSlice():
		for _, i := range from.InterSlice() {
			if msi, ok := i.(map[string]interface{}); ok {
				values = append(values, objx.Map(msi))
			}
		}
	}
	return values
}

func condition(cv objx.Map, condition string) objx.Map {
	for _, obj := range objects(cv.Get("status.conditions")) {
		if obj.Get("type").String() == condition {
			return obj
		}
	}
	return objx.Map(nil)
}

// surprisingConditions returns conditions with surprising statuses
// (Available=False, Degraded=True, etc.) in order of descending
// severity (e.g. Available=False is more severe than Degraded=True).
// It also returns a slice of types for which a condition entry was
// expected but not supplied on the ClusterOperator.
func surprisingConditions(co objx.Map) ([]configv1.ClusterOperatorStatusCondition, []configv1.ClusterStatusConditionType) {
	name := co.Get("metadata.name").String()
	var badConditions []configv1.ClusterOperatorStatusCondition
	var missingTypes []configv1.ClusterStatusConditionType
	for _, conditionType := range []configv1.ClusterStatusConditionType{
		configv1.OperatorAvailable,
		configv1.OperatorDegraded,
		configv1.OperatorUpgradeable,
	} {
		cond := condition(co, string(conditionType))
		if len(cond) == 0 {
			if conditionType != configv1.OperatorUpgradeable {
				missingTypes = append(missingTypes, conditionType)
			}
		} else {
			expected := configv1.ConditionFalse
			if conditionType == configv1.OperatorAvailable || conditionType == configv1.OperatorUpgradeable {
				expected = configv1.ConditionTrue
			}
			if cond.Get("status").String() != string(expected) {
				reason := cond.Get("reason").String()
				message := cond.Get("message").String()
				status := cond.Get("status").String()
				if conditionType == configv1.OperatorUpgradeable && (name == "kube-storage-version-migrator" || // https://bugzilla.redhat.com/show_bug.cgi?id=1928141 , currently fixed for 4.10, but no backports at the moment.  We currently have ...-upgrade-4.y-to-4.(y+1)-to-4.(y+2)-to-4.(y+3)-ci jobs, so as long as we don't extend that +3 skew for those jobs, we should be able to drop this code once 4.13 forks off of the development branch.
					name == "openshift-controller-manager" || // https://bugzilla.redhat.com/show_bug.cgi?id=1948011 , currently fixed for 4.8, but no backports at the moment.  We currently have ...-upgrade-4.y-to-4.(y+1)-to-4.(y+2)-to-4.(y+3)-ci jobs, so as long as we don't extend that +3 skew for those jobs, we should be able to drop this code once 4.10 forks off the development branch.
					name == "service-ca" || // https://bugzilla.redhat.com/show_bug.cgi?id=1948012 , currently fixed for 4.8, but no backports at the moment.  We currently have ...-upgrade-4.y-to-4.(y+1)-to-4.(y+2)-to-4.(y+3)-ci jobs, so as long as we don't extend that +3 skew for those jobs, we should be able to drop this code once 4.10 forks off the development branch.

					// Allow some node skew for post-update unpause monitoring.  Without this,
					// 4.8->4.9->4.10 jobs are updating successfully to 4.10, unpausing
					// compute, running the post-unpause compute-settling monitor, and
					// failing on [1,2]:
					//
					//   : [sig-arch][Early] Managed cluster should start all core operators [Skipped:Disconnected] [Suite:openshift/conformance/parallel]     0s
					//   fail [github.com/onsi/ginkgo@v4.7.0-origin.0+incompatible/internal/leafnodes/runner.go:113]: Oct 17 23:28:57.284: Some cluster operators are not ready: kube-apiserver (Upgradeable=False KubeletMinorVersion_KubeletMinorVersionUnsupportedNextUpgrade: KubeletMinorVersionUpgradeable: Kubelet minor versions on nodes ip-10-0-135-91.ec2.internal, ip-10-0-168-151.ec2.internal, and ip-10-0-192-244.ec2.internal will not be supported in the next OpenShift minor version upgrade.)
					//
					// With this change, that skew guard from [3] is allowed to trip early in
					// the suite.  But if it trips for long enough to set off alerts, we'd
					// still fail on that.
					//
					// [1]: https://testgrid.k8s.io/redhat-openshift-ocp-release-4.10-informing#periodic-ci-openshift-release-master-nightly-4.10-upgrade-from-stable-4.8-e2e-aws-upgrade-paused
					// [2]: https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/periodic-ci-openshift-release-master-nightly-4.10-upgrade-from-stable-4.8-e2e-aws-upgrade-paused/1449821870344900608
					// [3]: https://github.com/openshift/cluster-kube-apiserver-operator/pull/1199
					(name == "kube-apiserver" && reason == "KubeletMinorVersion_KubeletMinorVersionUnsupportedNextUpgrade") ||
					// https://bugzilla.redhat.com/show_bug.cgi?id=2015187
					(name == "storage" && vmxPattern.MatchString(message)) ||
					// storage attempts to contact vsphere to determine if it can be ugpraded.  If the storage operator cannot reach vsphere to determine
					// whether the upgrade is safe or not, it is appropriate to be upgradeable=Unknown
					(name == "storage" && status == "Unknown" && strings.Contains(message, "Failed to connect to vSphere"))) {
					continue
				}
				badConditions = append(badConditions, configv1.ClusterOperatorStatusCondition{
					Type:    conditionType,
					Status:  configv1.ConditionStatus(cond.Get("status").String()),
					Reason:  reason,
					Message: cond.Get("message").String(),
				})
			}
		}
	}
	return badConditions, missingTypes
}

// When a TechPreviewNoUpgrade or CustomNoUpgrades feature set are in force in the cluster, the following condition
// is set on the kube-apiserver cluster operator
// Ref: https://github.com/openshift/cluster-kube-apiserver-operator/blob/39a98d67c3b825b9215454a7817ffadb0577609b/pkg/operator/featureupgradablecontroller/feature_upgradeable_controller_test.go#L41-L46
func isKubeAPIUpgradableNoUpgradeCondition(cond configv1.ClusterOperatorStatusCondition) bool {
	return (cond.Reason == "FeatureGates_RestrictedFeatureGates_TechPreviewNoUpgrade" ||
		cond.Reason == "FeatureGates_RestrictedFeatureGates_CustomNoUpgrade") &&
		cond.Status == "False" &&
		cond.Type == "Upgradeable"
}
