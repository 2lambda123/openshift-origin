package install

import (
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/api/legacy"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationapiv1 "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
)

func init() {
	Install(kapi.GroupFactoryRegistry, kapi.Registry, kapi.Scheme)
	legacy.InstallLegacy(authorizationapi.GroupName, authorizationapi.AddToSchemeInCoreGroup, authorizationapiv1.AddToSchemeInCoreGroup,
		sets.NewString("ClusterRole", "ClusterRoleBinding", "ClusterPolicy", "ClusterPolicyBinding"),
		kapi.Registry, kapi.Scheme,
	)
}

// Install registers the API group and adds types to a scheme
func Install(groupFactoryRegistry announced.APIGroupFactoryRegistry, registry *registered.APIRegistrationManager, scheme *runtime.Scheme) {
	if err := announced.NewGroupMetaFactory(
		&announced.GroupMetaFactoryArgs{
			GroupName:                  authorizationapi.GroupName,
			VersionPreferenceOrder:     []string{authorizationapiv1.SchemeGroupVersion.Version},
			AddInternalObjectsToScheme: authorizationapi.AddToScheme,
			RootScopedKinds:            sets.NewString("ClusterRole", "ClusterRoleBinding", "ClusterPolicy", "ClusterPolicyBinding", "SubjectAccessReview", "ResourceAccessReview"),
		},
		announced.VersionToSchemeFunc{
			authorizationapiv1.SchemeGroupVersion.Version: authorizationapiv1.AddToScheme,
		},
	).Announce(groupFactoryRegistry).RegisterAndEnable(registry, scheme); err != nil {
		panic(err)
	}
}
