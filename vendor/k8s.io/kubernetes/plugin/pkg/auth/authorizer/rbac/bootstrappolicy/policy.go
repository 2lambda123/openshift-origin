/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bootstrappolicy

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
	"k8s.io/kubernetes/pkg/features"
)

var (
	Write      = []string{"create", "update", "patch", "delete", "deletecollection"}
	ReadWrite  = []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"}
	Read       = []string{"get", "list", "watch"}
	ReadUpdate = []string{"get", "list", "watch", "update", "patch"}

	Label      = map[string]string{"kubernetes.io/bootstrapping": "rbac-defaults"}
	Annotation = map[string]string{rbacv1.AutoUpdateAnnotationKey: "true"}
)

const (
	legacyGroup         = ""
	appsGroup           = "apps"
	authenticationGroup = "authentication.k8s.io"
	authorizationGroup  = "authorization.k8s.io"
	autoscalingGroup    = "autoscaling"
	batchGroup          = "batch"
	certificatesGroup   = "certificates.k8s.io"
	extensionsGroup     = "extensions"
	policyGroup         = "policy"
	rbacGroup           = "rbac.authorization.k8s.io"
	storageGroup        = "storage.k8s.io"
	resMetricsGroup     = "metrics.k8s.io"
	customMetricsGroup  = "custom.metrics.k8s.io"
	networkingGroup     = "networking.k8s.io"
)

func addDefaultMetadata(obj runtime.Object) {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		// if this happens, then some static code is broken
		panic(err)
	}

	labels := metadata.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	for k, v := range Label {
		labels[k] = v
	}
	metadata.SetLabels(labels)

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	for k, v := range Annotation {
		annotations[k] = v
	}
	metadata.SetAnnotations(annotations)
}

func addClusterRoleLabel(roles []rbacv1.ClusterRole) {
	for i := range roles {
		addDefaultMetadata(&roles[i])
	}
	return
}

func addClusterRoleBindingLabel(rolebindings []rbacv1.ClusterRoleBinding) {
	for i := range rolebindings {
		addDefaultMetadata(&rolebindings[i])
	}
	return
}

func NodeRules() []rbacv1.PolicyRule {
	nodePolicyRules := []rbacv1.PolicyRule{
		// Needed to check API access.  These creates are non-mutating
		rbacv1helpers.NewRule("create").Groups(authenticationGroup).Resources("tokenreviews").RuleOrDie(),
		rbacv1helpers.NewRule("create").Groups(authorizationGroup).Resources("subjectaccessreviews", "localsubjectaccessreviews").RuleOrDie(),

		// Needed to build serviceLister, to populate env vars for services
		rbacv1helpers.NewRule(Read...).Groups(legacyGroup).Resources("services").RuleOrDie(),

		// Nodes can register Node API objects and report status.
		// Use the NodeRestriction admission plugin to limit a node to creating/updating its own API object.
		rbacv1helpers.NewRule("create", "get", "list", "watch").Groups(legacyGroup).Resources("nodes").RuleOrDie(),
		rbacv1helpers.NewRule("update", "patch").Groups(legacyGroup).Resources("nodes/status").RuleOrDie(),
		rbacv1helpers.NewRule("update", "patch").Groups(legacyGroup).Resources("nodes").RuleOrDie(),

		// TODO: restrict to the bound node as creator in the NodeRestrictions admission plugin
		rbacv1helpers.NewRule("create", "update", "patch").Groups(legacyGroup).Resources("events").RuleOrDie(),

		// TODO: restrict to pods scheduled on the bound node once field selectors are supported by list/watch authorization
		rbacv1helpers.NewRule(Read...).Groups(legacyGroup).Resources("pods").RuleOrDie(),

		// Needed for the node to create/delete mirror pods.
		// Use the NodeRestriction admission plugin to limit a node to creating/deleting mirror pods bound to itself.
		rbacv1helpers.NewRule("create", "delete").Groups(legacyGroup).Resources("pods").RuleOrDie(),
		// Needed for the node to report status of pods it is running.
		// Use the NodeRestriction admission plugin to limit a node to updating status of pods bound to itself.
		rbacv1helpers.NewRule("update", "patch").Groups(legacyGroup).Resources("pods/status").RuleOrDie(),
		// Needed for the node to create pod evictions.
		// Use the NodeRestriction admission plugin to limit a node to creating evictions for pods bound to itself.
		rbacv1helpers.NewRule("create").Groups(legacyGroup).Resources("pods/eviction").RuleOrDie(),

		// Needed for imagepullsecrets, rbd/ceph and secret volumes, and secrets in envs
		// Needed for configmap volume and envs
		// Use the Node authorization mode to limit a node to get secrets/configmaps referenced by pods bound to itself.
		rbacv1helpers.NewRule("get", "list", "watch").Groups(legacyGroup).Resources("secrets", "configmaps").RuleOrDie(),
		// Needed for persistent volumes
		// Use the Node authorization mode to limit a node to get pv/pvc objects referenced by pods bound to itself.
		rbacv1helpers.NewRule("get").Groups(legacyGroup).Resources("persistentvolumeclaims", "persistentvolumes").RuleOrDie(),

		// TODO: add to the Node authorizer and restrict to endpoints referenced by pods or PVs bound to the node
		// Needed for glusterfs volumes
		rbacv1helpers.NewRule("get").Groups(legacyGroup).Resources("endpoints").RuleOrDie(),
		// Used to create a certificatesigningrequest for a node-specific client certificate, and watch
		// for it to be signed. This allows the kubelet to rotate it's own certificate.
		rbacv1helpers.NewRule("create", "get", "list", "watch").Groups(certificatesGroup).Resources("certificatesigningrequests").RuleOrDie(),
	}

	if utilfeature.DefaultFeatureGate.Enabled(features.ExpandPersistentVolumes) {
		// Use the Node authorization mode to limit a node to update status of pvc objects referenced by pods bound to itself.
		// Use the NodeRestriction admission plugin to limit a node to just update the status stanza.
		pvcStatusPolicyRule := rbacv1helpers.NewRule("get", "update", "patch").Groups(legacyGroup).Resources("persistentvolumeclaims/status").RuleOrDie()
		nodePolicyRules = append(nodePolicyRules, pvcStatusPolicyRule)
	}

	if utilfeature.DefaultFeatureGate.Enabled(features.TokenRequest) {
		// Use the Node authorization to limit a node to create tokens for service accounts running on that node
		// Use the NodeRestriction admission plugin to limit a node to create tokens bound to pods on that node
		tokenRequestRule := rbacv1helpers.NewRule("create").Groups(legacyGroup).Resources("serviceaccounts/token").RuleOrDie()
		nodePolicyRules = append(nodePolicyRules, tokenRequestRule)
	}

	// CSI
	if utilfeature.DefaultFeatureGate.Enabled(features.CSIPersistentVolume) {
		volAttachRule := rbacv1helpers.NewRule("get").Groups(storageGroup).Resources("volumeattachments").RuleOrDie()
		nodePolicyRules = append(nodePolicyRules, volAttachRule)
		if utilfeature.DefaultFeatureGate.Enabled(features.CSIDriverRegistry) {
			csiDriverRule := rbacv1helpers.NewRule("get", "watch", "list").Groups("csi.storage.k8s.io").Resources("csidrivers").RuleOrDie()
			nodePolicyRules = append(nodePolicyRules, csiDriverRule)
		}
	}
	if utilfeature.DefaultFeatureGate.Enabled(features.KubeletPluginsWatcher) &&
		utilfeature.DefaultFeatureGate.Enabled(features.CSINodeInfo) {
		csiNodeInfoRule := rbacv1helpers.NewRule("get", "create", "update", "patch", "delete").Groups("csi.storage.k8s.io").Resources("csinodeinfos").RuleOrDie()
		nodePolicyRules = append(nodePolicyRules, csiNodeInfoRule)
	}

	// Node leases
	if utilfeature.DefaultFeatureGate.Enabled(features.NodeLease) {
		nodePolicyRules = append(nodePolicyRules, rbacv1helpers.NewRule("get", "create", "update", "patch", "delete").Groups("coordination.k8s.io").Resources("leases").RuleOrDie())
	}

	// RuntimeClass
	if utilfeature.DefaultFeatureGate.Enabled(features.RuntimeClass) {
		nodePolicyRules = append(nodePolicyRules, rbacv1helpers.NewRule("get", "list", "watch").Groups("node.k8s.io").Resources("runtimeclasses").RuleOrDie())
	}
	return nodePolicyRules
}

// clusterRoles returns the cluster roles to bootstrap an API server with
func clusterRoles() []rbacv1.ClusterRole {
	roles := []rbacv1.ClusterRole{
		{
			// a "root" role which can do absolutely anything
			ObjectMeta: metav1.ObjectMeta{Name: "cluster-admin"},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("*").Groups("*").Resources("*").RuleOrDie(),
				rbacv1helpers.NewRule("*").URLs("*").RuleOrDie(),
			},
		},
		{
			// a role which provides just enough power to determine if the server is ready and discover API versions for negotiation
			ObjectMeta: metav1.ObjectMeta{Name: "system:discovery"},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("get").URLs(
					"/healthz", "/version", "/version/",
					// remove once swagger 1.2 support is removed
					"/swaggerapi", "/swaggerapi/*",
					// do not expand this pattern for openapi discovery docs
					// move to a single openapi endpoint that takes accept/accept-encoding headers
					"/swagger.json", "/swagger-2.0.0.pb-v1",
					"/openapi", "/openapi/*",
					"/api", "/api/*",
					"/apis", "/apis/*",
				).RuleOrDie(),
			},
		},
		{
			// a role which provides unauthenticated access to /readyz.
			// TODO: integrate into system:public-info-viewer (new in 1.14) when upstreaming
			ObjectMeta: metav1.ObjectMeta{Name: "system:openshift:public-info-viewer"},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("get").URLs(
					"/readyz",
				).RuleOrDie(),
			},
		},
		{
			// a role which provides minimal resource access to allow a "normal" user to learn information about themselves
			ObjectMeta: metav1.ObjectMeta{Name: "system:basic-user"},
			Rules: []rbacv1.PolicyRule{
				// TODO add future selfsubjectrulesreview, project request APIs, project listing APIs
				rbacv1helpers.NewRule("create").Groups(authorizationGroup).Resources("selfsubjectaccessreviews", "selfsubjectrulesreviews").RuleOrDie(),
			},
		},

		{
			// a role for a namespace level admin.  It is `edit` plus the power to grant permissions to other users.
			ObjectMeta: metav1.ObjectMeta{Name: "admin"},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-admin": "true"}},
				},
			},
		},
		{
			// a role for a namespace level editor.  It grants access to all user level actions in a namespace.
			// It does not grant powers for "privileged" resources which are domain of the system: `/status`
			// subresources or `quota`/`limits` which are used to control namespaces
			ObjectMeta: metav1.ObjectMeta{Name: "edit", Labels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-admin": "true"}},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-edit": "true"}},
				},
			},
		},
		{
			// a role for namespace level viewing.  It grants Read-only access to non-escalating resources in
			// a namespace.
			ObjectMeta: metav1.ObjectMeta{Name: "view", Labels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-edit": "true"}},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-view": "true"}},
				},
			},
		},
		{
			// a role for a namespace level admin.  It is `edit` plus the power to grant permissions to other users.
			ObjectMeta: metav1.ObjectMeta{Name: "system:aggregate-to-admin", Labels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-admin": "true"}},
			Rules: []rbacv1.PolicyRule{
				// additional admin powers
				rbacv1helpers.NewRule("create").Groups(authorizationGroup).Resources("localsubjectaccessreviews").RuleOrDie(),
				rbacv1helpers.NewRule(ReadWrite...).Groups(rbacGroup).Resources("roles", "rolebindings").RuleOrDie(),
			},
		},
		{
			// a role for a namespace level editor.  It grants access to all user level actions in a namespace.
			// It does not grant powers for "privileged" resources which are domain of the system: `/status`
			// subresources or `quota`/`limits` which are used to control namespaces
			ObjectMeta: metav1.ObjectMeta{Name: "system:aggregate-to-edit", Labels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-edit": "true"}},
			Rules: []rbacv1.PolicyRule{
				// Allow read on escalating resources
				rbacv1helpers.NewRule(Read...).Groups(legacyGroup).Resources("pods/attach", "pods/proxy", "pods/exec", "pods/portforward", "secrets", "services/proxy").RuleOrDie(),
				rbacv1helpers.NewRule("impersonate").Groups(legacyGroup).Resources("serviceaccounts").RuleOrDie(),

				rbacv1helpers.NewRule(Write...).Groups(legacyGroup).Resources("pods", "pods/attach", "pods/proxy", "pods/exec", "pods/portforward").RuleOrDie(),
				rbacv1helpers.NewRule(Write...).Groups(legacyGroup).Resources("replicationcontrollers", "replicationcontrollers/scale", "serviceaccounts",
					"services", "services/proxy", "endpoints", "persistentvolumeclaims", "configmaps", "secrets").RuleOrDie(),

				rbacv1helpers.NewRule(Write...).Groups(appsGroup).Resources(
					"statefulsets", "statefulsets/scale",
					"daemonsets",
					"deployments", "deployments/scale", "deployments/rollback",
					"replicasets", "replicasets/scale").RuleOrDie(),

				rbacv1helpers.NewRule(Write...).Groups(autoscalingGroup).Resources("horizontalpodautoscalers").RuleOrDie(),

				rbacv1helpers.NewRule(Write...).Groups(batchGroup).Resources("jobs", "cronjobs").RuleOrDie(),

				rbacv1helpers.NewRule(Write...).Groups(extensionsGroup).Resources("daemonsets",
					"deployments", "deployments/scale", "deployments/rollback", "ingresses",
					"replicasets", "replicasets/scale", "replicationcontrollers/scale",
					"networkpolicies").RuleOrDie(),

				rbacv1helpers.NewRule(Write...).Groups(policyGroup).Resources("poddisruptionbudgets").RuleOrDie(),

				rbacv1helpers.NewRule(Write...).Groups(networkingGroup).Resources("networkpolicies").RuleOrDie(),
			},
		},
		{
			// a role for namespace level viewing.  It grants Read-only access to non-escalating resources in
			// a namespace.
			ObjectMeta: metav1.ObjectMeta{Name: "system:aggregate-to-view", Labels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-view": "true"}},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule(Read...).Groups(legacyGroup).Resources("pods", "replicationcontrollers", "replicationcontrollers/scale", "serviceaccounts",
					"services", "endpoints", "persistentvolumeclaims", "configmaps").RuleOrDie(),
				rbacv1helpers.NewRule(Read...).Groups(legacyGroup).Resources("limitranges", "resourcequotas", "bindings", "events",
					"pods/status", "resourcequotas/status", "namespaces/status", "replicationcontrollers/status", "pods/log").RuleOrDie(),
				// read access to namespaces at the namespace scope means you can read *this* namespace.  This can be used as an
				// indicator of which namespaces you have access to.
				rbacv1helpers.NewRule(Read...).Groups(legacyGroup).Resources("namespaces").RuleOrDie(),

				rbacv1helpers.NewRule(Read...).Groups(appsGroup).Resources(
					"controllerrevisions",
					"statefulsets", "statefulsets/scale",
					"daemonsets",
					"deployments", "deployments/scale",
					"replicasets", "replicasets/scale").RuleOrDie(),

				rbacv1helpers.NewRule(Read...).Groups(autoscalingGroup).Resources("horizontalpodautoscalers").RuleOrDie(),

				rbacv1helpers.NewRule(Read...).Groups(batchGroup).Resources("jobs", "cronjobs").RuleOrDie(),

				rbacv1helpers.NewRule(Read...).Groups(extensionsGroup).Resources("daemonsets", "deployments", "deployments/scale",
					"ingresses", "replicasets", "replicasets/scale", "replicationcontrollers/scale",
					"networkpolicies").RuleOrDie(),

				rbacv1helpers.NewRule(Read...).Groups(policyGroup).Resources("poddisruptionbudgets").RuleOrDie(),

				rbacv1helpers.NewRule(Read...).Groups(networkingGroup).Resources("networkpolicies").RuleOrDie(),
			},
		},
		{
			// a role to use for heapster's connections back to the API server
			ObjectMeta: metav1.ObjectMeta{Name: "system:heapster"},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule(Read...).Groups(legacyGroup).Resources("events", "pods", "nodes", "namespaces").RuleOrDie(),
				rbacv1helpers.NewRule(Read...).Groups(extensionsGroup).Resources("deployments").RuleOrDie(),
			},
		},
		{
			// a role for nodes to use to have the access they need for running pods
			ObjectMeta: metav1.ObjectMeta{Name: "system:node"},
			Rules:      NodeRules(),
		},
		{
			// a role to use for node-problem-detector access.  It does not get bound to default location since
			// deployment locations can reasonably vary.
			ObjectMeta: metav1.ObjectMeta{Name: "system:node-problem-detector"},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("get").Groups(legacyGroup).Resources("nodes").RuleOrDie(),
				rbacv1helpers.NewRule("patch").Groups(legacyGroup).Resources("nodes/status").RuleOrDie(),
				eventsRule(),
			},
		},
		{
			// a role to use for setting up a proxy
			ObjectMeta: metav1.ObjectMeta{Name: "system:node-proxier"},
			Rules: []rbacv1.PolicyRule{
				// Used to build serviceLister
				rbacv1helpers.NewRule("list", "watch").Groups(legacyGroup).Resources("services", "endpoints").RuleOrDie(),
				rbacv1helpers.NewRule("get").Groups(legacyGroup).Resources("nodes").RuleOrDie(),

				eventsRule(),
			},
		},
		{
			// a role to use for full access to the kubelet API
			ObjectMeta: metav1.ObjectMeta{Name: "system:kubelet-api-admin"},
			Rules: []rbacv1.PolicyRule{
				// Allow read-only access to the Node API objects
				rbacv1helpers.NewRule("get", "list", "watch").Groups(legacyGroup).Resources("nodes").RuleOrDie(),
				// Allow all API calls to the nodes
				rbacv1helpers.NewRule("proxy").Groups(legacyGroup).Resources("nodes").RuleOrDie(),
				rbacv1helpers.NewRule("*").Groups(legacyGroup).Resources("nodes/proxy", "nodes/metrics", "nodes/spec", "nodes/stats", "nodes/log").RuleOrDie(),
			},
		},
		{
			// a role to use for bootstrapping a node's client certificates
			ObjectMeta: metav1.ObjectMeta{Name: "system:node-bootstrapper"},
			Rules: []rbacv1.PolicyRule{
				// used to create a certificatesigningrequest for a node-specific client certificate, and watch for it to be signed
				rbacv1helpers.NewRule("create", "get", "list", "watch").Groups(certificatesGroup).Resources("certificatesigningrequests").RuleOrDie(),
			},
		},
		{
			// a role to use for allowing authentication and authorization delegation
			ObjectMeta: metav1.ObjectMeta{Name: "system:auth-delegator"},
			Rules: []rbacv1.PolicyRule{
				// These creates are non-mutating
				rbacv1helpers.NewRule("create").Groups(authenticationGroup).Resources("tokenreviews").RuleOrDie(),
				rbacv1helpers.NewRule("create").Groups(authorizationGroup).Resources("subjectaccessreviews").RuleOrDie(),
			},
		},
		{
			// a role to use for the API registry, summarization, and proxy handling
			ObjectMeta: metav1.ObjectMeta{Name: "system:kube-aggregator"},
			Rules: []rbacv1.PolicyRule{
				// it needs to see all services so that it knows whether the ones it points to exist or not
				rbacv1helpers.NewRule(Read...).Groups(legacyGroup).Resources("services", "endpoints").RuleOrDie(),
			},
		},
		{
			// a role to use for bootstrapping the kube-controller-manager so it can create the shared informers
			// service accounts, and secrets that we need to create separate identities for other controllers
			ObjectMeta: metav1.ObjectMeta{Name: "system:kube-controller-manager"},
			Rules: []rbacv1.PolicyRule{
				eventsRule(),
				rbacv1helpers.NewRule("create").Groups(legacyGroup).Resources("endpoints", "secrets", "serviceaccounts").RuleOrDie(),
				rbacv1helpers.NewRule("delete").Groups(legacyGroup).Resources("secrets").RuleOrDie(),
				rbacv1helpers.NewRule("get").Groups(legacyGroup).Resources("endpoints", "namespaces", "secrets", "serviceaccounts", "configmaps").RuleOrDie(),
				rbacv1helpers.NewRule("update").Groups(legacyGroup).Resources("endpoints", "secrets", "serviceaccounts").RuleOrDie(),
				// Needed to check API access.  These creates are non-mutating
				rbacv1helpers.NewRule("create").Groups(authenticationGroup).Resources("tokenreviews").RuleOrDie(),
				// Needed for all shared informers
				rbacv1helpers.NewRule("list", "watch").Groups("*").Resources("*").RuleOrDie(),
			},
		},
		{
			// a role to use for the kube-scheduler
			ObjectMeta: metav1.ObjectMeta{Name: "system:kube-scheduler"},
			Rules: []rbacv1.PolicyRule{
				eventsRule(),

				// this is for leaderlease access
				// TODO: scope this to the kube-system namespace
				rbacv1helpers.NewRule("create").Groups(legacyGroup).Resources("endpoints").RuleOrDie(),
				rbacv1helpers.NewRule("get", "update", "patch", "delete").Groups(legacyGroup).Resources("endpoints").Names("kube-scheduler").RuleOrDie(),

				// fundamental resources
				rbacv1helpers.NewRule(Read...).Groups(legacyGroup).Resources("nodes").RuleOrDie(),
				rbacv1helpers.NewRule("get", "list", "watch", "delete").Groups(legacyGroup).Resources("pods").RuleOrDie(),
				rbacv1helpers.NewRule("create").Groups(legacyGroup).Resources("pods/binding", "bindings").RuleOrDie(),
				rbacv1helpers.NewRule("patch", "update").Groups(legacyGroup).Resources("pods/status").RuleOrDie(),
				// things that select pods
				rbacv1helpers.NewRule(Read...).Groups(legacyGroup).Resources("services", "replicationcontrollers").RuleOrDie(),
				rbacv1helpers.NewRule(Read...).Groups(appsGroup, extensionsGroup).Resources("replicasets").RuleOrDie(),
				rbacv1helpers.NewRule(Read...).Groups(appsGroup).Resources("statefulsets").RuleOrDie(),
				// things that pods use or applies to them
				rbacv1helpers.NewRule(Read...).Groups(policyGroup).Resources("poddisruptionbudgets").RuleOrDie(),
				rbacv1helpers.NewRule(Read...).Groups(legacyGroup).Resources("persistentvolumeclaims", "persistentvolumes").RuleOrDie(),
			},
		},
		{
			// a role to use for the kube-dns pod
			ObjectMeta: metav1.ObjectMeta{Name: "system:kube-dns"},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("list", "watch").Groups(legacyGroup).Resources("endpoints", "services").RuleOrDie(),
			},
		},
		{
			// a role for an external/out-of-tree persistent volume provisioner
			ObjectMeta: metav1.ObjectMeta{Name: "system:persistent-volume-provisioner"},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("get", "list", "watch", "create", "delete").Groups(legacyGroup).Resources("persistentvolumes").RuleOrDie(),
				// update is needed in addition to read access for setting lock annotations on PVCs
				rbacv1helpers.NewRule("get", "list", "watch", "update").Groups(legacyGroup).Resources("persistentvolumeclaims").RuleOrDie(),
				rbacv1helpers.NewRule(Read...).Groups(storageGroup).Resources("storageclasses").RuleOrDie(),

				// Needed for watching provisioning success and failure events
				rbacv1helpers.NewRule("watch").Groups(legacyGroup).Resources("events").RuleOrDie(),

				eventsRule(),
			},
		},
		{
			// a role for the csi external attacher
			ObjectMeta: metav1.ObjectMeta{Name: "system:csi-external-attacher"},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("get", "list", "watch", "update", "patch").Groups(legacyGroup).Resources("persistentvolumes").RuleOrDie(),
				rbacv1helpers.NewRule("get", "list", "watch").Groups(legacyGroup).Resources("nodes").RuleOrDie(),
				rbacv1helpers.NewRule("get", "list", "watch", "update", "patch").Groups(storageGroup).Resources("volumeattachments").RuleOrDie(),
				rbacv1helpers.NewRule("get", "list", "watch", "create", "update", "patch").Groups(legacyGroup).Resources("events").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "system:aws-cloud-provider"},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("get", "patch").Groups(legacyGroup).Resources("nodes").RuleOrDie(),
				eventsRule(),
			},
		},
		{
			// a role making the csrapprover controller approve a node client CSR
			ObjectMeta: metav1.ObjectMeta{Name: "system:certificates.k8s.io:certificatesigningrequests:nodeclient"},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("create").Groups(certificatesGroup).Resources("certificatesigningrequests/nodeclient").RuleOrDie(),
			},
		},
		{
			// a role making the csrapprover controller approve a node client CSR requested by the node itself
			ObjectMeta: metav1.ObjectMeta{Name: "system:certificates.k8s.io:certificatesigningrequests:selfnodeclient"},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("create").Groups(certificatesGroup).Resources("certificatesigningrequests/selfnodeclient").RuleOrDie(),
			},
		},
	}

	if utilfeature.DefaultFeatureGate.Enabled(features.VolumeScheduling) {
		roles = append(roles, rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "system:volume-scheduler"},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule(ReadUpdate...).Groups(legacyGroup).Resources("persistentvolumes").RuleOrDie(),
				rbacv1helpers.NewRule(Read...).Groups(storageGroup).Resources("storageclasses").RuleOrDie(),
				rbacv1helpers.NewRule(ReadUpdate...).Groups(legacyGroup).Resources("persistentvolumeclaims").RuleOrDie(),
			},
		})
	}

	externalProvisionerRules := []rbacv1.PolicyRule{
		rbacv1helpers.NewRule("create", "delete", "get", "list", "watch").Groups(legacyGroup).Resources("persistentvolumes").RuleOrDie(),
		rbacv1helpers.NewRule("get", "list", "watch", "update", "patch").Groups(legacyGroup).Resources("persistentvolumeclaims").RuleOrDie(),
		rbacv1helpers.NewRule("list", "watch").Groups(storageGroup).Resources("storageclasses").RuleOrDie(),
		rbacv1helpers.NewRule("get", "list", "watch", "create", "update", "patch").Groups(legacyGroup).Resources("events").RuleOrDie(),
		rbacv1helpers.NewRule("get", "list", "watch").Groups(legacyGroup).Resources("nodes").RuleOrDie(),
	}
	if utilfeature.DefaultFeatureGate.Enabled(features.CSINodeInfo) {
		externalProvisionerRules = append(externalProvisionerRules, rbacv1helpers.NewRule("get", "watch", "list").Groups("csi.storage.k8s.io").Resources("csinodeinfos").RuleOrDie())
	}
	roles = append(roles, rbacv1.ClusterRole{
		// a role for the csi external provisioner
		ObjectMeta: metav1.ObjectMeta{Name: "system:csi-external-provisioner"},
		Rules:      externalProvisionerRules,
	})

	addClusterRoleLabel(roles)
	return roles
}

const systemNodeRoleName = "system:node"

// ClusterRoleBindings return default rolebindings to the default roles
func clusterRoleBindings() []rbacv1.ClusterRoleBinding {
	rolebindings := []rbacv1.ClusterRoleBinding{
		rbacv1helpers.NewClusterBinding("cluster-admin").Groups(user.SystemPrivilegedGroup).BindingOrDie(),
		rbacv1helpers.NewClusterBinding("system:discovery").Groups(user.AllAuthenticated, user.AllUnauthenticated).BindingOrDie(),
		// TODO: integrate into system:public-info-viewer (new in 1.14) when upstreaming
		rbacv1helpers.NewClusterBinding("system:openshift:public-info-viewer").Groups(user.AllAuthenticated, user.AllUnauthenticated).BindingOrDie(), rbacv1helpers.NewClusterBinding("system:basic-user").Groups(user.AllAuthenticated, user.AllUnauthenticated).BindingOrDie(),
		rbacv1helpers.NewClusterBinding("system:node-proxier").Users(user.KubeProxy).BindingOrDie(),
		rbacv1helpers.NewClusterBinding("system:kube-controller-manager").Users(user.KubeControllerManager).BindingOrDie(),
		rbacv1helpers.NewClusterBinding("system:kube-dns").SAs("kube-system", "kube-dns").BindingOrDie(),
		rbacv1helpers.NewClusterBinding("system:kube-scheduler").Users(user.KubeScheduler).BindingOrDie(),
		rbacv1helpers.NewClusterBinding("system:aws-cloud-provider").SAs("kube-system", "aws-cloud-provider").BindingOrDie(),

		// This default binding of the system:node role to the system:nodes group is deprecated in 1.7 with the availability of the Node authorizer.
		// This leaves the binding, but with an empty set of subjects, so that tightening reconciliation can remove the subject.
		{
			ObjectMeta: metav1.ObjectMeta{Name: systemNodeRoleName},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: systemNodeRoleName},
		},
	}

	if utilfeature.DefaultFeatureGate.Enabled(features.VolumeScheduling) {
		rolebindings = append(rolebindings, rbacv1helpers.NewClusterBinding("system:volume-scheduler").Users(user.KubeScheduler).BindingOrDie())
	}

	addClusterRoleBindingLabel(rolebindings)

	return rolebindings
}

func ClusterRolesToAggregate() map[string]string {
	return map[string]string{
		"admin": "system:aggregate-to-admin",
		"edit":  "system:aggregate-to-edit",
		"view":  "system:aggregate-to-view",
	}
}
