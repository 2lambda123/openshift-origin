package upgrade

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"text/tabwriter"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
)

type versionMonitor struct {
	client     configv1client.Interface
	lastCV     *configv1.ClusterVersion
	oldVersion string
}

// Check returns the current ClusterVersion and a string summarizing the status.
func (m *versionMonitor) Check(initialGeneration int64, desired configv1.Update) (*configv1.ClusterVersion, string, error) {
	cv, err := m.client.ConfigV1().ClusterVersions().Get("version", metav1.GetOptions{})
	if err != nil {
		msg := fmt.Sprintf("unable to retrieve cluster version during upgrade: %v", err)
		framework.Logf(msg)
		return nil, msg, nil
	}
	m.lastCV = cv

	if cv.Status.ObservedGeneration > initialGeneration {
		if cv.Spec.DesiredUpdate == nil || desired != *cv.Spec.DesiredUpdate {
			return nil, "", fmt.Errorf("desired cluster version was changed by someone else: %v", cv.Spec.DesiredUpdate)
		}
	}

	var msg string
	for _, condition := range []configv1.ClusterStatusConditionType{
		configv1.OperatorProgressing,
		configv1.OperatorDegraded,
		configv1.ClusterStatusConditionType("Failing"),
	} {
		if c := findCondition(cv.Status.Conditions, condition); c != nil {
			if c.Status == configv1.ConditionTrue {
				msg = c.Message
				framework.Logf("cluster upgrade is %s: %v", condition, c.Message)
			}
		}
	}
	return cv, msg, nil
}

func (m *versionMonitor) Reached(cv *configv1.ClusterVersion, desired configv1.Update) (bool, error) {
	// if the operator hasn't observed our request
	if !equivalentUpdates(cv.Status.Desired, desired) {
		return false, nil
	}
	// is the latest history item equal to our desired and completed
	if target := latestHistory(cv.Status.History); target == nil || target.State != configv1.CompletedUpdate || !equivalentUpdates(desired, configv1.Update{Image: target.Image, Version: target.Version}) {
		return false, nil
	}

	if c := findCondition(cv.Status.Conditions, configv1.OperatorAvailable); c != nil {
		if c.Status != configv1.ConditionTrue {
			return false, fmt.Errorf("cluster version was Available=false after completion: %v", cv.Status.Conditions)
		}
	}
	if c := findCondition(cv.Status.Conditions, configv1.OperatorProgressing); c != nil {
		if c.Status == configv1.ConditionTrue {
			return false, fmt.Errorf("cluster version was Progressing=true after completion: %v", cv.Status.Conditions)
		}
	}
	if c := findCondition(cv.Status.Conditions, configv1.OperatorDegraded); c != nil {
		if c.Status == configv1.ConditionTrue {
			return false, fmt.Errorf("cluster version was Degraded=true after completion: %v", cv.Status.Conditions)
		}
	}
	if c := findCondition(cv.Status.Conditions, configv1.ClusterStatusConditionType("Failing")); c != nil {
		if c.Status == configv1.ConditionTrue {
			return false, fmt.Errorf("cluster version was Failing=true after completion: %v", cv.Status.Conditions)
		}
	}

	return true, nil
}

func (m *versionMonitor) ShouldReboot() []string {
	return nil
}

func (m *versionMonitor) ShouldUpgradeAbort(abortAt int) bool {
	if abortAt == 0 {
		return false
	}
	coList, err := m.client.ConfigV1().ClusterOperators().List(metav1.ListOptions{})
	if err != nil {
		framework.Logf("Unable to retrieve cluster operators, cannot check completion percentage")
		return false
	}

	changed := 0
	for _, item := range coList.Items {
		if findVersion(item.Status.Versions, "operator", m.oldVersion, m.lastCV.Status.Desired.Version) != "<old>" {
			changed++
		}
	}
	percent := float64(changed) / float64(len(coList.Items))
	if percent <= float64(abortAt)/100 {
		return false
	}

	framework.Logf("-------------------------------------------------------")
	framework.Logf("Upgraded %d/%d operators, beginning controlled rollback", changed, len(coList.Items))
	return true
}

func (m *versionMonitor) Output() {
	if m.lastCV != nil {
		data, _ := json.MarshalIndent(m.lastCV, "", "  ")
		framework.Logf("Cluster version:\n%s", data)
	}
	if coList, err := m.client.ConfigV1().ClusterOperators().List(metav1.ListOptions{}); err == nil {
		buf := &bytes.Buffer{}
		tw := tabwriter.NewWriter(buf, 0, 2, 1, ' ', 0)
		fmt.Fprintf(tw, "NAME\tA F P\tVERSION\tMESSAGE\n")
		for _, item := range coList.Items {
			fmt.Fprintf(tw,
				"%s\t%s %s %s\t%s\t%s\n",
				item.Name,
				findConditionShortStatus(item.Status.Conditions, configv1.OperatorAvailable, configv1.ConditionTrue),
				findConditionShortStatus(item.Status.Conditions, configv1.OperatorDegraded, configv1.ConditionFalse),
				findConditionShortStatus(item.Status.Conditions, configv1.OperatorProgressing, configv1.ConditionFalse),
				findVersion(item.Status.Versions, "operator", m.oldVersion, m.lastCV.Status.Desired.Version),
				findConditionMessage(item.Status.Conditions, configv1.OperatorProgressing),
			)
		}
		tw.Flush()
		framework.Logf("Cluster operators:\n%s", buf.String())
	}
}

func (m *versionMonitor) Disrupt(ctx context.Context, kubeClient kubernetes.Interface, rebootPolicy string) {
	rebootHard := false
	switch rebootPolicy {
	case "graceful":
		framework.Logf("Periodically reboot master nodes with clean shutdown")
	case "force":
		framework.Logf("Periodically reboot master nodes without allowing for clean shutdown")
		rebootHard = true
	case "":
		return
	}
	for {
		time.Sleep(time.Duration(rand.Int31n(90)) * time.Second)
		if ctx.Err() != nil {
			return
		}
		nodes, err := kubeClient.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})
		if err != nil || len(nodes.Items) == 0 {
			framework.Logf("Unable to find nodes to reboot: %v", err)
			continue
		}
		rand.Shuffle(len(nodes.Items), func(i, j int) { nodes.Items[i], nodes.Items[j] = nodes.Items[j], nodes.Items[i] })
		name := nodes.Items[0].Name
		framework.Logf("DISRUPTION: Triggering reboot of %s", name)
		if err := triggerReboot(kubeClient, name, 0, rebootHard); err != nil {
			framework.Logf("Failed to reboot %s: %v", name, err)
			continue
		}
		time.Sleep(wait.Jitter(5*time.Minute, 2))
	}
}

func sequence(fns ...wait.ConditionFunc) wait.ConditionFunc {
	return func() (bool, error) {
		if len(fns) == 0 {
			return true, nil
		}
		ok, err := fns[0]()
		if err != nil {
			return ok, err
		}
		if !ok {
			return false, nil
		}
		fns = fns[1:]
		return len(fns) == 0, nil
	}
}

func findVersion(versions []configv1.OperandVersion, name string, oldVersion, newVersion string) string {
	for _, version := range versions {
		if version.Name == name {
			if len(oldVersion) > 0 && version.Version == oldVersion {
				return "<old>"
			}
			if len(newVersion) > 0 && version.Version == newVersion {
				return "<new>"
			}
			return version.Version
		}
	}
	return ""
}

func findConditionShortStatus(conditions []configv1.ClusterOperatorStatusCondition, name configv1.ClusterStatusConditionType, unless configv1.ConditionStatus) string {
	if c := findCondition(conditions, name); c != nil {
		switch c.Status {
		case configv1.ConditionTrue:
			if unless == c.Status {
				return " "
			}
			return "T"
		case configv1.ConditionFalse:
			if unless == c.Status {
				return " "
			}
			return "F"
		default:
			return "U"
		}
	}
	return " "
}

func findConditionMessage(conditions []configv1.ClusterOperatorStatusCondition, name configv1.ClusterStatusConditionType) string {
	if c := findCondition(conditions, name); c != nil {
		return c.Message
	}
	return ""
}

func findCondition(conditions []configv1.ClusterOperatorStatusCondition, name configv1.ClusterStatusConditionType) *configv1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if name == conditions[i].Type {
			return &conditions[i]
		}
	}
	return nil
}

func equivalentUpdates(a, b configv1.Update) bool {
	if len(a.Image) > 0 && len(b.Image) > 0 {
		return a.Image == b.Image
	}
	if len(a.Version) > 0 && len(b.Version) > 0 {
		return a.Version == b.Version
	}
	return false
}

func versionString(update configv1.Update) string {
	switch {
	case len(update.Version) > 0 && len(update.Image) > 0:
		return fmt.Sprintf("%s (%s)", update.Version, update.Image)
	case len(update.Image) > 0:
		return update.Image
	case len(update.Version) > 0:
		return update.Version
	default:
		return "<empty>"
	}
}

func triggerReboot(kubeClient kubernetes.Interface, target string, attempt int, rebootHard bool) error {
	command := "echo 'reboot in 1 minute'; exec chroot /host shutdown -r 1"
	if rebootHard {
		command = "echo 'reboot in 1 minute'; exec chroot /host sudo systemd-run sh -c 'sleep 60 && reboot --force --force'"
	}
	isTrue := true
	zero := int64(0)
	name := fmt.Sprintf("reboot-%s-%d", target, attempt)
	_, err := kubeClient.CoreV1().Pods("kube-system").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"test.openshift.io/upgrades-target": target,
			},
		},
		Spec: corev1.PodSpec{
			HostPID:       true,
			RestartPolicy: corev1.RestartPolicyNever,
			NodeName:      target,
			Volumes: []corev1.Volume{
				{
					Name: "host",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name: "reboot",
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  &zero,
						Privileged: &isTrue,
					},
					Image: "centos:7",
					Command: []string{
						"/bin/bash",
						"-c",
						command,
					},
					TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/host",
							Name:      "host",
						},
					},
				},
			},
		},
	})
	if errors.IsAlreadyExists(err) {
		return triggerReboot(kubeClient, target, attempt+1, rebootHard)
	}
	return err
}
