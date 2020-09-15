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

package authorizer

import (
	"fmt"
	rbacv1 "k8s.io/api/rbac/v1"
	"time"
	"context"
	"reflect"

	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/kubernetes/openshift-kube-apiserver/authorization/browsersafe"
	"k8s.io/kubernetes/openshift-kube-apiserver/authorization/scopeauthorizer"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	"k8s.io/apiserver/pkg/authorization/union"
	"k8s.io/apiserver/plugin/pkg/authorizer/webhook"
	versionedinformers "k8s.io/client-go/informers"
	"k8s.io/kubernetes/pkg/auth/authorizer/abac"
	"k8s.io/kubernetes/pkg/auth/nodeidentifier"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/node"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy"
)

// Config contains the data on how to authorize a request to the Kube API Server
type Config struct {
	AuthorizationModes []string

	// Options for ModeABAC

	// Path to an ABAC policy file.
	PolicyFile string

	// Options for ModeWebhook

	// Kubeconfig file for Webhook authorization plugin.
	WebhookConfigFile string
	// API version of subject access reviews to send to the webhook (e.g. "v1", "v1beta1")
	WebhookVersion string
	// TTL for caching of authorized responses from the webhook server.
	WebhookCacheAuthorizedTTL time.Duration
	// TTL for caching of unauthorized responses from the webhook server.
	WebhookCacheUnauthorizedTTL time.Duration

	VersionedInformerFactory versionedinformers.SharedInformerFactory

	// Optional field, custom dial function used to connect to webhook
	CustomDial utilnet.DialFunc
}

// New returns the right sort of union of multiple authorizer.Authorizer objects
// based on the authorizationMode or an error.
func (config Config) New() (authorizer.Authorizer, authorizer.RuleResolver, error) {
	if len(config.AuthorizationModes) == 0 {
		return nil, nil, fmt.Errorf("at least one authorization mode must be passed")
	}

	var (
		authorizers   []authorizer.Authorizer
		ruleResolvers []authorizer.RuleResolver
	)

	for _, authorizationMode := range config.AuthorizationModes {
		// Keep cases in sync with constant list in k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes/modes.go.
		switch authorizationMode {
		case modes.ModeNode:
			graph := node.NewGraph()
			node.AddGraphEventHandlers(
				graph,
				config.VersionedInformerFactory.Core().V1().Nodes(),
				config.VersionedInformerFactory.Core().V1().Pods(),
				config.VersionedInformerFactory.Core().V1().PersistentVolumes(),
				config.VersionedInformerFactory.Storage().V1().VolumeAttachments(),
			)
			nodeAuthorizer := node.NewAuthorizer(graph, nodeidentifier.NewDefaultNodeIdentifier(), bootstrappolicy.NodeRules())
			authorizers = append(authorizers, nodeAuthorizer)
			ruleResolvers = append(ruleResolvers, nodeAuthorizer)

		case modes.ModeAlwaysAllow:
			alwaysAllowAuthorizer := authorizerfactory.NewAlwaysAllowAuthorizer()
			authorizers = append(authorizers, alwaysAllowAuthorizer)
			ruleResolvers = append(ruleResolvers, alwaysAllowAuthorizer)
		case modes.ModeAlwaysDeny:
			alwaysDenyAuthorizer := authorizerfactory.NewAlwaysDenyAuthorizer()
			authorizers = append(authorizers, alwaysDenyAuthorizer)
			ruleResolvers = append(ruleResolvers, alwaysDenyAuthorizer)
		case modes.ModeABAC:
			abacAuthorizer, err := abac.NewFromFile(config.PolicyFile)
			if err != nil {
				return nil, nil, err
			}
			authorizers = append(authorizers, abacAuthorizer)
			ruleResolvers = append(ruleResolvers, abacAuthorizer)
		case modes.ModeWebhook:
			webhookAuthorizer, err := webhook.New(config.WebhookConfigFile,
				config.WebhookVersion,
				config.WebhookCacheAuthorizedTTL,
				config.WebhookCacheUnauthorizedTTL,
				config.CustomDial)
			if err != nil {
				return nil, nil, err
			}
			authorizers = append(authorizers, webhookAuthorizer)
			ruleResolvers = append(ruleResolvers, webhookAuthorizer)
		case modes.ModeRBAC:
			rbacAuthorizer := rbac.New(
				&rbac.RoleGetter{Lister: config.VersionedInformerFactory.Rbac().V1().Roles().Lister()},
				&rbac.RoleBindingLister{Lister: config.VersionedInformerFactory.Rbac().V1().RoleBindings().Lister()},
				&rbac.ClusterRoleGetter{Lister: config.VersionedInformerFactory.Rbac().V1().ClusterRoles().Lister()},
				&rbac.ClusterRoleBindingLister{Lister: config.VersionedInformerFactory.Rbac().V1().ClusterRoleBindings().Lister()},
			)
			// Wrap with an authorizer that detects unsafe requests and modifies verbs/resources appropriately so policy can address them separately
			authorizers = append(authorizers, browsersafe.NewBrowserSafeAuthorizer(newRBACProtector(rbacAuthorizer, config.VersionedInformerFactory), user.AllAuthenticated))
			ruleResolvers = append(ruleResolvers, newRBACProtector(rbacAuthorizer, config.VersionedInformerFactory))
		case modes.ModeScope:
			// Wrap with an authorizer that detects unsafe requests and modifies verbs/resources appropriately so policy can address them separately
			scopeLimitedAuthorizer := scopeauthorizer.NewAuthorizer(config.VersionedInformerFactory.Rbac().V1().ClusterRoles().Lister())
			authorizers = append(authorizers, browsersafe.NewBrowserSafeAuthorizer(scopeLimitedAuthorizer, user.AllAuthenticated))
		case modes.ModeSystemMasters:
			// no browsersafeauthorizer here becase that rewrites the resources.  This authorizer matches no matter which resource matches.
			authorizers = append(authorizers, authorizerfactory.NewPrivilegedGroups(user.SystemPrivilegedGroup))
		default:
			return nil, nil, fmt.Errorf("unknown authorization mode %s specified", authorizationMode)
		}
	}

	return union.New(authorizers...), union.NewRuleResolvers(ruleResolvers...), nil
}

type rbacProtector struct {
	delegate *rbac.RBACAuthorizer
	versionedInformerFactory versionedinformers.SharedInformerFactory
}

func newRBACProtector(delegate *rbac.RBACAuthorizer, versionedInformerFactory versionedinformers.SharedInformerFactory) *rbacProtector {
	return &rbacProtector{
		delegate:            delegate,
		versionedInformerFactory: versionedInformerFactory,
	}
}

func (a *rbacProtector) assertCachesSynced() error {
	stopCh := make(chan struct{})
	close(stopCh) // close stopCh to force checking if informers are synced now.
	informersByStarted := map[bool][]string{}

	for informerType, started := range a.versionedInformerFactory.WaitForCacheSync(stopCh) {
		switch informerType {
		// we are interested in knowing if the RBAC related informers sync and avoid the risk that comes with waiting on syncing everything.
		case reflect.TypeOf(&rbacv1.ClusterRole{}), reflect.TypeOf(&rbacv1.ClusterRoleBinding{}), reflect.TypeOf(&rbacv1.Role{}), reflect.TypeOf(&rbacv1.RoleBinding{}):
			informersByStarted[started] = append(informersByStarted[started], informerType.String())
		}
	}
	if notStarted := informersByStarted[false]; len(notStarted) > 0 {
		return fmt.Errorf("%d informers not started yet: %v", len(notStarted), notStarted)
	}
	return nil
}

func (a *rbacProtector) Authorize(ctx context.Context, attributes authorizer.Attributes) (authorizer.Decision, string, error) {
	if err := a.assertCachesSynced(); err != nil {
		return authorizer.DecisionDeny, err.Error(), err
	}
	return a.delegate.Authorize(ctx, attributes)
}

func (a *rbacProtector) RulesFor(user user.Info, namespace string) ([]authorizer.ResourceRuleInfo, []authorizer.NonResourceRuleInfo, bool, error) {
	if err := a.assertCachesSynced(); err != nil {
		return nil, nil, false, err
	}
	return a.delegate.RulesFor(user, namespace)
}