package apiserver

import (
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kubeinformers "k8s.io/client-go/informers"
	restclient "k8s.io/client-go/rest"
	rbacclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/rbac/internalversion"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"

	authorizationapiv1 "github.com/openshift/api/authorization/v1"
	authorizationapiv1alpha1 "github.com/openshift/api/authorization/v1alpha1"
	accessrestrictionetcd "github.com/openshift/origin/pkg/authorization/registry/accessrestriction/etcd"
	"github.com/openshift/origin/pkg/authorization/registry/clusterrole"
	"github.com/openshift/origin/pkg/authorization/registry/clusterrolebinding"
	"github.com/openshift/origin/pkg/authorization/registry/localresourceaccessreview"
	"github.com/openshift/origin/pkg/authorization/registry/localsubjectaccessreview"
	"github.com/openshift/origin/pkg/authorization/registry/resourceaccessreview"
	"github.com/openshift/origin/pkg/authorization/registry/role"
	"github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	rolebindingrestrictionetcd "github.com/openshift/origin/pkg/authorization/registry/rolebindingrestriction/etcd"
	"github.com/openshift/origin/pkg/authorization/registry/selfsubjectrulesreview"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/authorization/registry/subjectrulesreview"
)

type ExtraConfig struct {
	KubeAPIServerClientConfig *restclient.Config
	KubeInformers             kubeinformers.SharedInformerFactory
	RuleResolver              rbacregistryvalidation.AuthorizationRuleResolver
	SubjectLocator            rbac.SubjectLocator

	// TODO these should all become local eventually
	Scheme *runtime.Scheme
	Codecs serializer.CodecFactory

	makeV1Storage sync.Once
	v1Storage     map[string]rest.Storage
	v1StorageErr  error
}

type AuthorizationAPIServerConfig struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

type AuthorizationAPIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

type CompletedConfig struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *AuthorizationAPIServerConfig) Complete() completedConfig {
	cfg := completedConfig{
		c.GenericConfig.Complete(),
		&c.ExtraConfig,
	}

	return cfg
}

// New returns a new instance of AuthorizationAPIServer from the given config.
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*AuthorizationAPIServer, error) {
	genericServer, err := c.GenericConfig.New("authorization.openshift.io-apiserver", delegationTarget)
	if err != nil {
		return nil, err
	}

	s := &AuthorizationAPIServer{
		GenericAPIServer: genericServer,
	}

	v1Storage, err := c.V1RESTStorage()
	if err != nil {
		return nil, err
	}

	v1alpha1Storage, err := c.v1alpha1RESTStorage()
	if err != nil {
		return nil, err
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(authorizationapiv1.GroupName, c.ExtraConfig.Scheme, metav1.ParameterCodec, c.ExtraConfig.Codecs)
	apiGroupInfo.PrioritizedVersions = []schema.GroupVersion{authorizationapiv1.GroupVersion, authorizationapiv1alpha1.GroupVersion}
	apiGroupInfo.VersionedResourcesStorageMap[authorizationapiv1.GroupVersion.Version] = v1Storage
	apiGroupInfo.VersionedResourcesStorageMap[authorizationapiv1alpha1.GroupVersion.Version] = v1alpha1Storage
	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

func (c *completedConfig) V1RESTStorage() (map[string]rest.Storage, error) {
	c.ExtraConfig.makeV1Storage.Do(func() {
		c.ExtraConfig.v1Storage, c.ExtraConfig.v1StorageErr = c.newV1RESTStorage()
	})

	return c.ExtraConfig.v1Storage, c.ExtraConfig.v1StorageErr
}

func (c *completedConfig) newV1RESTStorage() (map[string]rest.Storage, error) {
	rbacClient, err := rbacclient.NewForConfig(c.ExtraConfig.KubeAPIServerClientConfig)
	if err != nil {
		return nil, err
	}

	selfSubjectRulesReviewStorage := selfsubjectrulesreview.NewREST(c.ExtraConfig.RuleResolver, c.ExtraConfig.KubeInformers.Rbac().V1().ClusterRoles().Lister())
	subjectRulesReviewStorage := subjectrulesreview.NewREST(c.ExtraConfig.RuleResolver, c.ExtraConfig.KubeInformers.Rbac().V1().ClusterRoles().Lister())
	subjectAccessReviewStorage := subjectaccessreview.NewREST(c.GenericConfig.Authorization.Authorizer)
	subjectAccessReviewRegistry := subjectaccessreview.NewRegistry(subjectAccessReviewStorage)
	localSubjectAccessReviewStorage := localsubjectaccessreview.NewREST(subjectAccessReviewRegistry)
	resourceAccessReviewStorage := resourceaccessreview.NewREST(c.GenericConfig.Authorization.Authorizer, c.ExtraConfig.SubjectLocator)
	resourceAccessReviewRegistry := resourceaccessreview.NewRegistry(resourceAccessReviewStorage)
	localResourceAccessReviewStorage := localresourceaccessreview.NewREST(resourceAccessReviewRegistry)
	roleBindingRestrictionStorage, err := rolebindingrestrictionetcd.NewREST(c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, fmt.Errorf("error building REST storage: %v", err)
	}

	v1Storage := map[string]rest.Storage{}
	v1Storage["resourceAccessReviews"] = resourceAccessReviewStorage
	v1Storage["subjectAccessReviews"] = subjectAccessReviewStorage
	v1Storage["localSubjectAccessReviews"] = localSubjectAccessReviewStorage
	v1Storage["localResourceAccessReviews"] = localResourceAccessReviewStorage
	v1Storage["selfSubjectRulesReviews"] = selfSubjectRulesReviewStorage
	v1Storage["subjectRulesReviews"] = subjectRulesReviewStorage
	v1Storage["roles"] = role.NewREST(rbacClient.RESTClient())
	v1Storage["roleBindings"] = rolebinding.NewREST(rbacClient.RESTClient())
	v1Storage["clusterRoles"] = clusterrole.NewREST(rbacClient.RESTClient())
	v1Storage["clusterRoleBindings"] = clusterrolebinding.NewREST(rbacClient.RESTClient())
	v1Storage["roleBindingRestrictions"] = roleBindingRestrictionStorage
	return v1Storage, nil
}

func (c *completedConfig) v1alpha1RESTStorage() (map[string]rest.Storage, error) {
	accessRestrictionStorage, err := accessrestrictionetcd.NewREST(c.GenericConfig.RESTOptionsGetter, c.ExtraConfig.RuleResolver)
	if err != nil {
		return nil, fmt.Errorf("error building accessRestrictions REST storage: %v", err)
	}

	v1alpha1Storage := map[string]rest.Storage{}
	v1alpha1Storage["accessRestrictions"] = accessRestrictionStorage
	return v1alpha1Storage, nil
}
