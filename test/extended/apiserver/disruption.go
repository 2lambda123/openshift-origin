package apiserver

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	authv1 "github.com/openshift/api/authorization/v1"
	projv1 "github.com/openshift/api/project/v1"
	"github.com/openshift/origin/pkg/cmd/monitor_command"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	namespace            = "openshift-e2e-disruption-monitor"
	serviceAccountName   = "disruption-monitor-sa"
	varLogPath           = "/var/log"
	disruptionDataFolder = "disruption-data"
	disruptionTypeEnvVar = "DISRUPTION_TYPE_LABEL"
	inClusterEventsFile  = "junit/AdditionalEvents__in_cluster_disruption.json"
)

var disruptionDataPath = fmt.Sprintf("%s/%s", varLogPath, disruptionDataFolder)

var _ = g.Describe("[sig-api-machinery][Feature:APIServer]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("apiserver")

	g.It("start in-cluster disruption monitors [Early]", func() {
		ctx := context.Background()
		createTestBed(ctx, oc)
	})

	g.It("tear down in-cluster disruption monitors [Late]", func() {
		ctx := context.Background()
		deleteTestBed(ctx, oc)

		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		var events monitorapi.Intervals
		var errs []error
		for _, node := range nodes.Items {
			nodeEvents, err := fetchNodeInClusterEvents(oc, &node)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			framework.Logf("in-cluster monitors: found %d events for node %s", len(events), node.Name)
			events = append(events, nodeEvents...)
		}
		if len(errs) > 0 {
			framework.Logf("found errors fetching in-cluster data: %v", errs)
		}
		err = monitorserialization.EventsToFile(exutil.ArtifactPath(inClusterEventsFile), events)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

func createTestBed(ctx context.Context, oc *exutil.CLI) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	internalAPI, err := url.Parse(infra.Status.APIServerURL)
	o.Expect(err).NotTo(o.HaveOccurred())
	apiIntHost := strings.Replace(internalAPI.Hostname(), "api.", "api-int.", 1)

	err = callProject(ctx, oc, true)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = callServiceAccount(ctx, oc, true)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = callRBACClusterAdmin(ctx, oc, true)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = callRBACHostaccess(ctx, oc, false)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = exutil.WaitForServiceAccountWithSecret(
		oc.AdminKubeClient().CoreV1().ServiceAccounts(namespace),
		serviceAccountName)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = callMasterDaemonset(ctx, oc, true, apiIntHost)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = callWorkerDaemonset(ctx, oc, true)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func deleteTestBed(ctx context.Context, oc *exutil.CLI) {
	// Stop daemonsets first so that test stop before serviceaccount is removed
	// and permission issues from apiserver are not recorded as disruption
	err := callMasterDaemonset(ctx, oc, false, "")
	o.Expect(err).NotTo(o.HaveOccurred())
	err = callWorkerDaemonset(ctx, oc, false)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = callRBACClusterAdmin(ctx, oc, false)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = callRBACHostaccess(ctx, oc, false)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = callServiceAccount(ctx, oc, false)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = callProject(ctx, oc, false)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func callMasterDaemonset(ctx context.Context, oc *exutil.CLI, create bool, apiIntHost string) error {
	labels := map[string]string{
		"app": "pod-monitor-master",
	}
	truePointer := true
	obj := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-monitor-masters",
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "artifacts",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: disruptionDataPath,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "internal-lb",
							Image: "image-registry.openshift-image-registry.svc:5000/openshift/tests:latest",
							Env: []corev1.EnvVar{
								{
									Name:  "KUBERNETES_SERVICE_HOST",
									Value: apiIntHost,
								},
								{
									Name:  "KUBERNETES_SERVICE_PORT",
									Value: "6443",
								},
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
							},
							Command: []string{
								"openshift-tests",
								"run-monitor",
								"--api-disruption-only",
								"--extra-locator",
								"target=api-int",
								"--extra-message",
								"node=$(NODE_NAME)",
								"--artifact-dir",
								disruptionDataPath,
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &truePointer,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "artifacts",
									MountPath: disruptionDataPath,
								},
							},
						},
					},
					ServiceAccountName: serviceAccountName,
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/master": "",
					},
					Tolerations: []corev1.Toleration{
						{
							Key:    "node-role.kubernetes.io/master",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
		},
	}

	client := oc.AdminKubeClient().AppsV1().DaemonSets(namespace)
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func callWorkerDaemonset(ctx context.Context, oc *exutil.CLI, create bool) error {
	labels := map[string]string{
		"app": "pod-monitor-worker",
	}
	truePointer := true
	obj := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-monitor-worker",
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "artifacts",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: disruptionDataPath,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "service-network",
							Image: "image-registry.openshift-image-registry.svc:5000/openshift/tests:latest",
							Command: []string{
								"openshift-tests",
								"run-monitor",
								"--api-disruption-only",
								"--extra-locator",
								"target=service-network",
								"--extra-message",
								"node=$(NODE_NAME)",
								"--artifact-dir",
								disruptionDataPath,
							},
							Env: []corev1.EnvVar{
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &truePointer,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "artifacts",
									MountPath: disruptionDataPath,
								},
							},
						},
					},
					ServiceAccountName: serviceAccountName,
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/worker": "",
					},
				},
			},
		},
	}

	client := oc.AdminKubeClient().AppsV1().DaemonSets(namespace)
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func callRBACClusterAdmin(ctx context.Context, oc *exutil.CLI, create bool) error {
	obj := &authv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-admin", serviceAccountName),
			Namespace: namespace,
		},
		RoleRef: corev1.ObjectReference{
			Kind: "ClusterRole",
			Name: "cluster-admin",
		},
		Subjects: []corev1.ObjectReference{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
	}

	client := oc.AdminAuthorizationClient().AuthorizationV1().ClusterRoleBindings()
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func callRBACHostaccess(ctx context.Context, oc *exutil.CLI, create bool) error {
	obj := &authv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-privileged", serviceAccountName),
			Namespace: namespace,
		},
		RoleRef: corev1.ObjectReference{
			Kind: "ClusterRole",
			Name: "system:openshift:scc:privileged",
		},
		Subjects: []corev1.ObjectReference{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
	}

	client := oc.AdminAuthorizationClient().AuthorizationV1().ClusterRoleBindings()
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func callServiceAccount(ctx context.Context, oc *exutil.CLI, create bool) error {
	obj := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}

	client := oc.AdminKubeClient().CoreV1().ServiceAccounts(namespace)
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func callProject(ctx context.Context, oc *exutil.CLI, create bool) error {
	obj := &projv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"pod-security.kubernetes.io/enforce": "privileged",
				"pod-security.kubernetes.io/audit":   "privileged",
				"pod-security.kubernetes.io/warn":    "privileged",
			},
		},
	}

	client := oc.AsAdmin().ProjectClient().ProjectV1().Projects()
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func fetchFileViaOC(oc *exutil.CLI, nodeName string, filePath string) (string, error) {
	return oc.Run("adm", "node-logs").Args(nodeName, fmt.Sprintf("--path=%s", filePath)).Output()
}

func fetchNodeInClusterEvents(oc *exutil.CLI, node *corev1.Node) (monitorapi.Intervals, error) {
	var events monitorapi.Intervals
	var errs []error

	// Fetch a list of e2e data files
	basePath := fmt.Sprintf("/%s/%s", disruptionDataFolder, monitor_command.EventDir)
	fileListOutput, err := fetchFileViaOC(oc, node.Name, basePath)
	if err != nil {
		return events, fmt.Errorf("failed to list files in disruption event folder on node %s: %v", node.Name, err)
	}
	fileList := strings.Split(strings.Trim(fileListOutput, "\n"), "\n")
	for _, fileName := range fileList {
		if len(fileName) == 0 {
			continue
		}
		framework.Logf("Found events file %s on node %s", fileName, node.Name)
		filePath := fmt.Sprintf("%s/%s", basePath, fileName)
		fileEvents, err := fetchEventsFromFileOnNode(oc, filePath, node.Name)
		if err != nil {
			errs = append(errs, err)
		}
		events = append(events, fileEvents...)
	}

	if len(errs) > 0 {
		return events, fmt.Errorf("failed to process files on node %s: %v", node.Name, errs)
	}

	return events, nil
}

func fetchEventsFromFileOnNode(oc *exutil.CLI, filePath string, nodeName string) (monitorapi.Intervals, error) {
	var filteredEvents monitorapi.Intervals
	var err error

	eventsJson, err := fetchFileViaOC(oc, nodeName, filePath)
	if err != nil {
		return filteredEvents, fmt.Errorf("failed to fetch file %s on node %s: %v", filePath, nodeName, err)
	}

	allEvents, err := monitorserialization.EventsFromJSON([]byte(eventsJson))
	if err != nil {
		return nil, fmt.Errorf("failed to convert file %s from node %s to intervals: %v", filePath, nodeName, err)
	}
	// Keep only disruption events
	for _, event := range allEvents {
		if !strings.HasPrefix(event.Locator, "disruption/") {
			continue
		}
		filteredEvents = append(filteredEvents, event)
	}
	framework.Logf("Fetched %d events from node %s", len(filteredEvents), nodeName)
	return filteredEvents, err
}
