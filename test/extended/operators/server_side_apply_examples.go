package operators

import (
	"context"
	_ "embed"
	"encoding/json"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"

	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"

	corev1 "k8s.io/api/core/v1"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	imageutils "k8s.io/kubernetes/test/utils/image"

	applyconfigv1 "github.com/openshift/client-go/config/applyconfigurations/config/v1"

	configv1 "github.com/openshift/api/config/v1"

	"github.com/davecgh/go-spew/spew"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var (
	//go:embed manifests/ssa-with-set/crd-with-ssa-set.yaml
	ssaWithSet []byte
	//go:embed manifests/ssa-with-set/new-instance.yaml
	ssaWithSetNewInstance []byte
	//go:embed manifests/ssa-with-set/take-ownership-instance.yaml
	ssaWithSetTakeOwnership []byte
	//go:embed manifests/ssa-with-set/updated-list.yaml
	ssaWithSetUpdatedList []byte
	//go:embed manifests/ssa-with-set/expected-final.yaml
	ssaWithSetExpectedFinal []byte

	ssaWithSetCRD              *apiextensionsv1.CustomResourceDefinition
	ssaWithSetNewInstanceObj   *unstructured.Unstructured
	ssaWithSetTakeOwnershipObj *unstructured.Unstructured
	ssaWithSetUpdatedListObj   *unstructured.Unstructured
	ssaWithSetExpectedFinalObj *unstructured.Unstructured
)

func init() {
	ssaWithSetCRD = resourceread.ReadCustomResourceDefinitionV1OrDie(ssaWithSet)
	ssaWithSetNewInstanceObj = resourceread.ReadUnstructuredOrDie(ssaWithSetNewInstance)
	ssaWithSetTakeOwnershipObj = resourceread.ReadUnstructuredOrDie(ssaWithSetTakeOwnership)
	ssaWithSetUpdatedListObj = resourceread.ReadUnstructuredOrDie(ssaWithSetUpdatedList)
	ssaWithSetExpectedFinalObj = resourceread.ReadUnstructuredOrDie(ssaWithSetExpectedFinal)
}

var _ = g.Describe("[sig-apimachinery]", func() {

	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("server-side-apply-examples", admissionapi.LevelPrivileged)
	fieldManager := metav1.ApplyOptions{
		FieldManager: "e2e=test",
	}

	g.Describe("server-side-apply should function properly", func() {
		g.It("should take ownership of a list set", func() {
			ctx := context.Background()

			crdClient, err := apiextensionsclientset.NewForConfig(oc.AdminConfig())
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = crdClient.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, ssaWithSetCRD, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() {
				err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.TODO(), ssaWithSetCRD.Name, metav1.DeleteOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
			}()

			dynamicClient := oc.AdminDynamicClient()
			ssaClient := dynamicClient.Resource(schema.GroupVersionResource{
				Group:    "testing.openshift.io",
				Version:  "v1",
				Resource: "ssawithsets",
			})

			err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 30*time.Second, true,
				func(ctx context.Context) (bool, error) {
					_, err := ssaClient.Apply(ctx, ssaWithSetNewInstanceObj.GetName(), ssaWithSetNewInstanceObj, metav1.ApplyOptions{
						FieldManager: "creator",
					})
					if err == nil {
						return true, nil
					}
					if err != nil {
						framework.Logf("failed to create: %v", err)
					}

					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())

			postApply, err := ssaClient.Apply(ctx, ssaWithSetTakeOwnershipObj.GetName(), ssaWithSetTakeOwnershipObj, metav1.ApplyOptions{
				FieldManager: "new-owner",
				Force:        true,
			})
			postBytes, _ := json.Marshal(postApply.Object)
			framework.Logf("after sharing the field\n%v", string(postBytes))
			o.Expect(err).NotTo(o.HaveOccurred())

			postApply, err = ssaClient.Apply(ctx, ssaWithSetUpdatedListObj.GetName(), ssaWithSetUpdatedListObj, metav1.ApplyOptions{
				FieldManager: "new-owner",
				Force:        true,
			})
			postBytes, _ = json.Marshal(postApply.Object)
			framework.Logf("after trying to replace the field\n%v", string(postBytes))
			o.Expect(err).NotTo(o.HaveOccurred())

			actualFinal, err := ssaClient.Get(ctx, ssaWithSetUpdatedListObj.GetName(), metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			actualSpec, _, err := unstructured.NestedMap(actualFinal.Object, "spec")
			o.Expect(err).NotTo(o.HaveOccurred())
			expectedSpec, _, err := unstructured.NestedMap(ssaWithSetExpectedFinalObj.Object, "spec")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(actualSpec).To(o.Equal(expectedSpec))
		})

		g.It("should clear fields when they are no longer being applied on CRDs", func() {
			ctx := context.Background()
			isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isMicroShift {
				g.Skip("microshift lacks the API")
			}

			_, err = oc.AdminConfigClient().ConfigV1().ClusterOperators().Create(ctx, &configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-instance",
				},
			}, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			defer oc.AdminConfigClient().ConfigV1().ClusterOperators().Delete(ctx, "test-instance", metav1.DeleteOptions{})

			addFirstCondition := applyconfigv1.ClusterOperator("test-instance").
				WithStatus(applyconfigv1.ClusterOperatorStatus().
					WithConditions(applyconfigv1.ClusterOperatorStatusCondition().
						WithType("FirstType").
						WithStatus(configv1.ConditionTrue).
						WithReason("Dummy").
						WithMessage("No Value").
						WithLastTransitionTime(metav1.Now()),
					),
				)
			_, err = oc.AdminConfigClient().ConfigV1().ClusterOperators().ApplyStatus(ctx, addFirstCondition, fieldManager)
			o.Expect(err).NotTo(o.HaveOccurred())

			currInstance, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, "test-instance", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if !containsCondition(currInstance.Status.Conditions, "FirstType") {
				framework.Logf("got conditions: %v", spew.Sdump(currInstance.Status.Conditions))
				g.Fail("missing FirstType condition")
			}

			addJustSecondCondition := applyconfigv1.ClusterOperator("test-instance").
				WithStatus(applyconfigv1.ClusterOperatorStatus().
					WithConditions(applyconfigv1.ClusterOperatorStatusCondition().
						WithType("SecondType").
						WithStatus(configv1.ConditionTrue).
						WithReason("Dummy").
						WithMessage("No Value").
						WithLastTransitionTime(metav1.Now()),
					),
				)
			_, err = oc.AdminConfigClient().ConfigV1().ClusterOperators().ApplyStatus(ctx, addJustSecondCondition, fieldManager)
			o.Expect(err).NotTo(o.HaveOccurred())

			currInstance, err = oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, "test-instance", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if !containsCondition(currInstance.Status.Conditions, "SecondType") {
				g.Fail("missing SecondType condition")
			}
			if containsCondition(currInstance.Status.Conditions, "FirstType") {
				g.Fail("has FirstType condition unexpectedly")
			}
		})

		g.It("should clear fields when they are no longer being applied in FeatureGates", func() {
			ctx := context.Background()
			isSelfManagedHA, err := exutil.IsSelfManagedHA(ctx, oc.AdminConfigClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			isSingleNode, err := exutil.IsSelfManagedHA(ctx, oc.AdminConfigClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if !isSelfManagedHA && !isSingleNode {
				g.Skip("only SelfManagedHA and SingleNode have mutable FeatureGates")
			}

			addFirstCondition := applyconfigv1.FeatureGate("cluster").
				WithStatus(applyconfigv1.FeatureGateStatus().
					WithConditions(
						metav1.Condition{
							Type:               "FirstType",
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             "Dummy",
							Message:            "No Value",
						},
					),
				)
			_, err = oc.AdminConfigClient().ConfigV1().FeatureGates().ApplyStatus(ctx, addFirstCondition, fieldManager)
			o.Expect(err).NotTo(o.HaveOccurred())

			currInstance, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if !containsMetaCondition(currInstance.Status.Conditions, "FirstType") {
				framework.Logf("got conditions: %v", spew.Sdump(currInstance.Status.Conditions))
				g.Fail("missing FirstType condition")
			}

			addJustSecondCondition := applyconfigv1.FeatureGate("cluster").
				WithStatus(applyconfigv1.FeatureGateStatus().
					WithConditions(
						metav1.Condition{
							Type:               "SecondType",
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             "Dummy",
							Message:            "No Value",
						},
					),
				)
			_, err = oc.AdminConfigClient().ConfigV1().FeatureGates().ApplyStatus(ctx, addJustSecondCondition, fieldManager)
			o.Expect(err).NotTo(o.HaveOccurred())

			currInstance, err = oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if !containsMetaCondition(currInstance.Status.Conditions, "SecondType") {
				g.Fail("missing SecondType condition")
			}
			if containsMetaCondition(currInstance.Status.Conditions, "FirstType") {
				g.Fail("has FirstType condition unexpectedly")
			}
		})

		g.It("should clear fields when they are no longer being applied in built-in APIs", func() {
			ctx := context.Background()

			_, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx, pausePod("test-instance"), metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			defer oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, "test-instance", metav1.DeleteOptions{})

			addFirstCondition := applycorev1.Pod("test-instance", oc.Namespace()).
				WithStatus(applycorev1.PodStatus().
					WithConditions(applycorev1.PodCondition().
						WithType("FirstType").
						WithStatus(corev1.ConditionTrue).
						WithReason("Dummy").
						WithMessage("No Value").
						WithLastTransitionTime(metav1.Now()),
					),
				)
			_, err = oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).ApplyStatus(ctx, addFirstCondition, fieldManager)
			o.Expect(err).NotTo(o.HaveOccurred())

			currInstance, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Get(ctx, "test-instance", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if !containsPodCondition(currInstance.Status.Conditions, "FirstType") {
				framework.Logf("got conditions: %v", spew.Sdump(currInstance.Status.Conditions))
				g.Fail("missing FirstType condition")
			}

			addJustSecondCondition := applycorev1.Pod("test-instance", oc.Namespace()).
				WithStatus(applycorev1.PodStatus().
					WithConditions(applycorev1.PodCondition().
						WithType("SecondType").
						WithStatus(corev1.ConditionTrue).
						WithReason("Dummy").
						WithMessage("No Value").
						WithLastTransitionTime(metav1.Now()),
					),
				)
			_, err = oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).ApplyStatus(ctx, addJustSecondCondition, fieldManager)
			o.Expect(err).NotTo(o.HaveOccurred())

			currInstance, err = oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Get(ctx, "test-instance", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if !containsPodCondition(currInstance.Status.Conditions, "SecondType") {
				g.Fail("missing SecondType condition")
			}
			if containsPodCondition(currInstance.Status.Conditions, "FirstType") {
				g.Fail("has FirstType condition unexpectedly")
			}
		})
	})
})

func containsCondition(podConditions []configv1.ClusterOperatorStatusCondition, name string) bool {
	for _, curr := range podConditions {
		if string(curr.Type) == name {
			return true
		}
	}
	return false
}

func containsMetaCondition(podConditions []metav1.Condition, name string) bool {
	for _, curr := range podConditions {
		if string(curr.Type) == name {
			return true
		}
	}
	return false
}

func containsPodCondition(podConditions []corev1.PodCondition, name string) bool {
	for _, curr := range podConditions {
		if string(curr.Type) == name {
			return true
		}
	}
	return false
}

func pausePod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			SecurityContext: e2epod.GetRestrictedPodSecurityContext(),
			Containers: []corev1.Container{
				{
					Name:            "pause-container",
					Image:           imageutils.GetPauseImageName(),
					SecurityContext: e2epod.GetRestrictedContainerSecurityContext(),
				},
			},
		},
	}

}
