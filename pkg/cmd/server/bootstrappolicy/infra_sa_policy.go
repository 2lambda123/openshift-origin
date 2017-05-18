package bootstrappolicy

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/certificates"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/apis/storage"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationapiv1 "github.com/openshift/origin/pkg/authorization/api/v1"

	// we need the conversions registered for our init block
	_ "github.com/openshift/origin/pkg/authorization/api/install"
)

const (
	InfraBuildControllerServiceAccountName = "build-controller"

	InfraDeploymentConfigControllerServiceAccountName = "deploymentconfig-controller"
	DeploymentConfigControllerRoleName                = "system:deploymentconfig-controller"

	InfraDeploymentControllerServiceAccountName = "deployment-controller"
	DeploymentControllerRoleName                = "system:deployment-controller"

	InfraPersistentVolumeBinderControllerServiceAccountName = "pv-binder-controller"
	PersistentVolumeBinderControllerRoleName                = "system:pv-binder-controller"

	InfraPersistentVolumeAttachDetachControllerServiceAccountName = "pv-attach-detach-controller"
	PersistentVolumeAttachDetachControllerRoleName                = "system:pv-attach-detach-controller"

	InfraPersistentVolumeRecyclerControllerServiceAccountName = "pv-recycler-controller"
	PersistentVolumeRecyclerControllerRoleName                = "system:pv-recycler-controller"

	InfraPersistentVolumeProvisionerControllerServiceAccountName = "pv-provisioner-controller"
	PersistentVolumeProvisionerControllerRoleName                = "system:pv-provisioner-controller"

	InfraServiceLoadBalancerControllerServiceAccountName = "service-load-balancer-controller"
	ServiceLoadBalancerControllerRoleName                = "system:service-load-balancer-controller"

	InfraUnidlingControllerServiceAccountName = "unidling-controller"
	UnidlingControllerRoleName                = "system:unidling-controller"

	ServiceServingCertServiceAccountName = "service-serving-cert-controller"
	ServiceServingCertControllerRoleName = "system:service-serving-cert-controller"

	InfraServiceIngressIPControllerServiceAccountName = "service-ingress-ip-controller"
	ServiceIngressIPControllerRoleName                = "system:service-ingress-ip-controller"

	InfraNodeBootstrapServiceAccountName = "node-bootstrapper"
	NodeBootstrapRoleName                = "system:node-bootstrapper"
)

type InfraServiceAccounts struct {
	serviceAccounts sets.String
	saToRole        map[string]authorizationapi.ClusterRole
}

var InfraSAs = &InfraServiceAccounts{}

func (r *InfraServiceAccounts) addServiceAccount(saName string, role authorizationapi.ClusterRole) error {
	if _, exists := r.serviceAccounts[saName]; exists {
		return fmt.Errorf("%s already registered", saName)
	}

	for existingSAName, existingRole := range r.saToRole {
		if existingRole.Name == role.Name {
			return fmt.Errorf("clusterrole/%s is already registered for %s", existingRole.Name, existingSAName)
		}
	}

	if role.Annotations == nil {
		role.Annotations = map[string]string{}
	}
	role.Annotations[roleSystemOnly] = roleIsSystemOnly

	// TODO make this unnecessary
	// we don't want to expose the resourcegroups externally because it makes it very difficult for customers to learn from
	// our default roles and hard for them to reason about what power they are granting their users
	for j := range role.Rules {
		role.Rules[j].Resources = authorizationapi.NormalizeResources(role.Rules[j].Resources)
	}

	// TODO roundtrip roles to pick up defaulting for API groups.  Without this, the covers check in reconcile-cluster-roles will fail.
	// we can remove this again once everything gets group qualified and we have unit tests enforcing that.  other pulls are in
	// progress to do that.
	// we only want to roundtrip the sa roles now.  We'll remove this once we convert the SA roles
	versionedRole := &authorizationapiv1.ClusterRole{}
	if err := kapi.Scheme.Convert(&role, versionedRole, nil); err != nil {
		return err
	}
	defaultedInternalRole := &authorizationapi.ClusterRole{}
	if err := kapi.Scheme.Convert(versionedRole, defaultedInternalRole, nil); err != nil {
		return err
	}

	r.saToRole[saName] = *defaultedInternalRole
	r.serviceAccounts.Insert(saName)
	return nil
}

func (r *InfraServiceAccounts) GetServiceAccounts() []string {
	return r.serviceAccounts.List()
}

func (r *InfraServiceAccounts) RoleFor(saName string) (authorizationapi.ClusterRole, bool) {
	ret, exists := r.saToRole[saName]
	return ret, exists
}

func (r *InfraServiceAccounts) AllRoles() []authorizationapi.ClusterRole {
	saRoles := []authorizationapi.ClusterRole{}
	for _, saName := range r.serviceAccounts.List() {
		saRoles = append(saRoles, r.saToRole[saName])
	}

	return saRoles
}

func init() {
	var err error

	InfraSAs.serviceAccounts = sets.String{}
	InfraSAs.saToRole = map[string]authorizationapi.ClusterRole{}

	err = InfraSAs.addServiceAccount(
		InfraDeploymentConfigControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: DeploymentConfigControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// DeploymentControllerFactory.deploymentLW
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("replicationcontrollers"),
				},
				// DeploymentControllerFactory.deploymentClient
				{
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("replicationcontrollers"),
				},
				// DeploymentController.podClient
				{
					Verbs:     sets.NewString("get", "list", "create", "watch", "delete", "update"),
					Resources: sets.NewString("pods"),
				},
				// DeploymentController.recorder (EventBroadcaster)
				{
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraDeploymentControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: DeploymentControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{extensions.GroupName},
					Verbs:     sets.NewString("get", "list", "watch", "update"),
					Resources: sets.NewString("deployments"),
				},
				{
					APIGroups: []string{extensions.GroupName},
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("deployments/status"),
				},
				{
					APIGroups: []string{extensions.GroupName},
					Verbs:     sets.NewString("list", "watch", "get", "create", "patch", "update", "delete"),
					Resources: sets.NewString("replicasets"),
				},
				{
					APIGroups: []string{""},
					// TODO: remove "update" once
					// https://github.com/kubernetes/kubernetes/issues/36897 is resolved.
					Verbs:     sets.NewString("get", "list", "watch", "update"),
					Resources: sets.NewString("pods"),
				},
				{
					APIGroups: []string{""},
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraPersistentVolumeRecyclerControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: PersistentVolumeRecyclerControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// PersistentVolumeRecycler.volumeController.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("persistentvolumes"),
				},
				// PersistentVolumeRecycler.syncVolume()
				{
					Verbs:     sets.NewString("get", "update", "create", "delete"),
					Resources: sets.NewString("persistentvolumes"),
				},
				// PersistentVolumeRecycler.syncVolume()
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("persistentvolumes/status"),
				},
				// PersistentVolumeRecycler.claimController.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("persistentvolumeclaims"),
				},
				// PersistentVolumeRecycler.syncClaim()
				{
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("persistentvolumeclaims"),
				},
				// PersistentVolumeRecycler.syncClaim()
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("persistentvolumeclaims/status"),
				},
				// PersistentVolumeRecycler.reclaimVolume() -> handleRecycle()
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("pods"),
				},
				// PersistentVolumeRecycler.reclaimVolume() -> handleRecycle()
				{
					Verbs:     sets.NewString("get", "create", "delete"),
					Resources: sets.NewString("pods"),
				},
				// PersistentVolumeRecycler.reclaimVolume() -> handleRecycle()
				{
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraPersistentVolumeAttachDetachControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: PersistentVolumeAttachDetachControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// shared informer on PVs
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("persistentvolumes"),
				},
				// shared informer on PVCs
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("persistentvolumeclaims"),
				},
				// shared informer on nodes
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("nodes"),
				},
				// operationexecutor uses get with nodes
				{
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("nodes"),
				},
				// strategic patch on nodes/status
				{
					Verbs:     sets.NewString("patch", "update"),
					Resources: sets.NewString("nodes/status"),
				},
				// shared informer on pods
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("pods"),
				},
				// normal event usage
				{
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraPersistentVolumeBinderControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: PersistentVolumeBinderControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// PersistentVolumeBinder.volumeController.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("persistentvolumes"),
				},
				// PersistentVolumeBinder.syncVolume()
				{
					Verbs:     sets.NewString("get", "update", "create", "delete"),
					Resources: sets.NewString("persistentvolumes"),
				},
				// PersistentVolumeBinder.syncVolume()
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("persistentvolumes/status"),
				},
				// PersistentVolumeBinder.claimController.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("persistentvolumeclaims"),
				},
				// PersistentVolumeBinder.syncClaim()
				{
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("persistentvolumeclaims"),
				},
				// PersistentVolumeBinder.syncClaim()
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("persistentvolumeclaims/status"),
				},
				// PersistentVolumeRecycler.reclaimVolume() -> handleRecycle()
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("pods"),
				},
				// PersistentVolumeRecycler.reclaimVolume() -> handleRecycle()
				{
					Verbs:     sets.NewString("get", "create", "delete"),
					Resources: sets.NewString("pods"),
				},
				// RecycleVolumeByWatchingPodUntilCompletion
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("events"),
				},
				// PersistentVolumeRecycler.reclaimVolume() -> handleRecycle()
				{
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
				},
				// PersistentVolumeBinder.findProvisionablePlugin()
				// Glusterfs provisioner
				{
					APIGroups: []string{storage.GroupName},
					Verbs:     sets.NewString("list", "watch", "get"),
					Resources: sets.NewString("storageclasses"),
				},
				// Glusterfs provisioner
				{
					Verbs:     sets.NewString("get", "create", "delete"),
					Resources: sets.NewString("services", "endpoints"),
				},
				// Glusterfs & Ceph provisioner
				{
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("secrets"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraPersistentVolumeProvisionerControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: PersistentVolumeProvisionerControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// PersistentVolumeProvisioner.volumeController.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("persistentvolumes"),
				},
				// PersistentVolumeProvisioner.syncVolume()
				{
					Verbs:     sets.NewString("get", "update", "create", "delete"),
					Resources: sets.NewString("persistentvolumes"),
				},
				// PersistentVolumeProvisioner.syncVolume()
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("persistentvolumes/status"),
				},
				// PersistentVolumeProvisioner.claimController.ListWatch
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("persistentvolumeclaims"),
				},
				// PersistentVolumeProvisioner.syncClaim()
				{
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("persistentvolumeclaims"),
				},
				// PersistentVolumeProvisioner.syncClaim()
				{
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("persistentvolumeclaims/status"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraServiceLoadBalancerControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: ServiceLoadBalancerControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// ServiceController.cache.ListWatch
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("services"),
				},
				// ServiceController.processDelta needs to fetch the latest service
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("get"),
					Resources: sets.NewString("services"),
				},
				// ServiceController.persistUpdate changes the status of the service
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("services/status"),
				},
				// ServiceController.nodeLister.ListWatch
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("nodes"),
				},
				// ServiceController.eventRecorder
				{
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraUnidlingControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: UnidlingControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{kapi.GroupName, extensions.GroupName},
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("replicationcontrollers/scale"),
				},
				{
					APIGroups: []string{extensions.GroupName},
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("replicasets/scale", "deployments/scale"),
				},
				{
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("deploymentconfigs/scale"),
				},
				{
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("events"),
				},
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("get", "update"),
					Resources: sets.NewString("endpoints"),
				},
				// these are used to "manually" scale and annotate known objects, and should be
				// removed once we can set the last-scale-reason field via the scale subresource
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("get", "update", "patch"),
					Resources: sets.NewString("replicationcontrollers"),
				},
				{
					APIGroups: []string{},
					Verbs:     sets.NewString("get", "update", "patch"),
					Resources: sets.NewString("deploymentconfigs"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		ServiceServingCertServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: ServiceServingCertControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("list", "watch", "update"),
					Resources: sets.NewString("services"),
				},
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("get", "list", "watch", "create", "update"),
					Resources: sets.NewString("secrets"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraServiceIngressIPControllerServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: ServiceIngressIPControllerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// Listing and watching services
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("list", "watch"),
					Resources: sets.NewString("services"),
				},
				// IngressIPController.persistSpec changes the spec of the service
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("services"),
				},
				// IngressIPController.persistStatus changes the status of the service
				{
					APIGroups: []string{kapi.GroupName},
					Verbs:     sets.NewString("update"),
					Resources: sets.NewString("services/status"),
				},
				// IngressIPController.recorder
				{
					Verbs:     sets.NewString("create", "update", "patch"),
					Resources: sets.NewString("events"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

	err = InfraSAs.addServiceAccount(
		InfraNodeBootstrapServiceAccountName,
		authorizationapi.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: NodeBootstrapRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				{
					APIGroups: []string{certificates.GroupName},
					// match the upstream role for now
					// TODO sort out how to deconflict this with upstream
					Verbs:     sets.NewString("create", "get", "list", "watch"),
					Resources: sets.NewString("certificatesigningrequests"),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}

}
