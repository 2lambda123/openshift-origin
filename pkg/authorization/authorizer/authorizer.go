package authorizer

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authenticationapi "github.com/openshift/origin/pkg/auth/api"
	authcontext "github.com/openshift/origin/pkg/auth/context"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
)

type openshiftAuthorizer struct {
	masterAuthorizationNamespace string
	policyRegistry               policyregistry.Registry
	policyBindingRegistry        policybindingregistry.Registry
}
type openshiftAuthorizationAttributes struct {
	user              authenticationapi.UserInfo
	verb              string
	resourceKind      string
	namespace         string
	requestAttributes interface{}
}
type openshiftAuthorizationAttributeBuilder struct {
	requestsToUsers *authcontext.RequestContextMap
}

type Authorizer interface {
	Authorize(a AuthorizationAttributes) (allowed bool, reason string, err error)
}

type AuthorizationAttributeBuilder interface {
	GetAttributes(request *http.Request) (AuthorizationAttributes, error)
}

type AuthorizationAttributes interface {
	GetUserInfo() authenticationapi.UserInfo
	GetVerb() string
	GetNamespace() string
	// GetRequestAttributes is of type interface{} because different verbs and different Authorizer/AuthorizationAttributeBuilder pairs may have different contract requirements
	GetRequestAttributes() interface{}
}

func NewAuthorizer(masterAuthorizationNamespace string, policyRuleBindingRegistry policyregistry.Registry, policyBindingRegistry policybindingregistry.Registry) Authorizer {
	return &openshiftAuthorizer{masterAuthorizationNamespace, policyRuleBindingRegistry, policyBindingRegistry}
}
func NewAuthorizationAttributeBuilder(requestsToUsers *authcontext.RequestContextMap) AuthorizationAttributeBuilder {
	return &openshiftAuthorizationAttributeBuilder{requestsToUsers}
}

func doesApplyToUser(ruleUsers, ruleGroups []string, user authenticationapi.UserInfo) bool {
	if contains(ruleUsers, user.GetName()) {
		return true
	}

	for _, currGroup := range user.GetGroups() {
		if contains(ruleGroups, currGroup) {
			return true
		}
	}

	return false
}
func contains(list []string, token string) bool {
	for _, curr := range list {
		if curr == token {
			return true
		}
	}
	return false
}

// getPolicy provides a point for easy caching
func (a *openshiftAuthorizer) getPolicy(namespace string) (*authorizationapi.Policy, error) {
	ctx := kapi.WithNamespace(kapi.NewContext(), namespace)
	policy, err := a.policyRegistry.GetPolicy(ctx, authorizationapi.PolicyName)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, err
	}

	return policy, nil
}

// getPolicy provides a point for easy caching
func (a *openshiftAuthorizer) getPolicyBindings(namespace string) ([]authorizationapi.PolicyBinding, error) {
	ctx := kapi.WithNamespace(kapi.NewContext(), namespace)
	policyBindingList, err := a.policyBindingRegistry.ListPolicyBindings(ctx, klabels.Everything(), klabels.Everything())
	if err != nil {
		return nil, err
	}

	return policyBindingList.Items, nil
}

// getRoleBindings provides a point for easy caching
func (a *openshiftAuthorizer) getRoleBindings(namespace string) ([]authorizationapi.RoleBinding, error) {
	policyBindings, err := a.getPolicyBindings(namespace)
	if err != nil {
		return nil, err
	}

	ret := make([]authorizationapi.RoleBinding, 0, len(policyBindings))
	for _, policyBinding := range policyBindings {
		for _, value := range policyBinding.RoleBindings {
			ret = append(ret, value)
		}
	}

	return ret, nil
}

// getRole
func (a *openshiftAuthorizer) getRole(roleBinding authorizationapi.RoleBinding) (*authorizationapi.Role, error) {
	roleNamespace := roleBinding.RoleRef.Namespace
	roleName := roleBinding.RoleRef.Name

	rolePolicy, err := a.getPolicy(roleNamespace)
	if err != nil {
		return nil, err
	}

	role, exists := rolePolicy.Roles[roleName]
	if !exists {
		return nil, fmt.Errorf("role %#v not found", roleBinding.RoleRef)
	}

	return &role, nil
}

func (a *openshiftAuthorizer) getEffectivePolicyRules(namespace string, user authenticationapi.UserInfo) ([]authorizationapi.PolicyRule, error) {
	roleBindings, err := a.getRoleBindings(namespace)
	if err != nil {
		return nil, err
	}

	effectiveRules := make([]authorizationapi.PolicyRule, 0, len(roleBindings))
	for _, roleBinding := range roleBindings {
		role, err := a.getRole(roleBinding)
		if err != nil {
			return nil, err
		}

		for _, curr := range role.Rules {
			if doesApplyToUser(roleBinding.UserNames, roleBinding.GroupNames, user) {
				effectiveRules = append(effectiveRules, curr)
			}
		}
	}

	return effectiveRules, nil
}

func (a *openshiftAuthorizer) Authorize(passedAttributes AuthorizationAttributes) (bool, string, error) {
	// fmt.Printf("#### checking %#v\n", passedAttributes)

	attributes, ok := passedAttributes.(openshiftAuthorizationAttributes)
	if !ok {
		return false, "", fmt.Errorf("attributes are not of expected type: %#v", attributes)
	}

	globalAllowed, globalDenied, err := a.authorizeWithNamespaceRules(a.masterAuthorizationNamespace, attributes)
	if err != nil {
		return false, "", err
	}
	if len(globalDenied) > 0 {
		return false, globalDenied, err
	}
	if len(globalAllowed) > 0 {
		return true, globalAllowed, err
	}

	if len(attributes.GetNamespace()) != 0 {
		namespaceAllowed, namespaceDenied, err := a.authorizeWithNamespaceRules(attributes.GetNamespace(), attributes)
		if err != nil {
			return false, "", err
		}
		if len(namespaceDenied) > 0 {
			return false, namespaceDenied, err
		}
		if len(namespaceAllowed) > 0 {
			return true, namespaceAllowed, err
		}
	}

	return false, "denied by default", nil
}

func (a *openshiftAuthorizer) authorizeWithNamespaceRules(namespace string, passedAttributes AuthorizationAttributes) (string, string, error) {
	attributes, ok := passedAttributes.(openshiftAuthorizationAttributes)
	if !ok {
		return "", "", fmt.Errorf("attributes are not of expected type: %#v", attributes)
	}

	allRules, err := a.getEffectivePolicyRules(namespace, attributes.GetUserInfo())
	if err != nil {
		return "", "", err
	}

	// check for denies
	for _, rule := range allRules {
		if !rule.Deny {
			continue
		}

		matches, err := attributes.ruleMatches(rule)
		if err != nil {
			return "", "", err
		}
		if matches {
			return "", fmt.Sprintf("denied by rule in %v: %#v", namespace, rule), nil
		}
	}

	// check for allows
	for _, rule := range allRules {
		if rule.Deny {
			continue
		}

		matches, err := attributes.ruleMatches(rule)
		if err != nil {
			return "", "", err
		}
		if matches {
			return fmt.Sprintf("allowed by rule in %v: %#v", namespace, rule), "", nil
		}
	}

	return "", "", nil
}

func (a openshiftAuthorizationAttributes) ruleMatches(rule authorizationapi.PolicyRule) (bool, error) {
	if a.verbMatches(rule) {
		if a.kindMatches(rule) {
			return true, nil
		}
	}

	return false, nil
}

func (a openshiftAuthorizationAttributes) verbMatches(rule authorizationapi.PolicyRule) bool {
	verbMatches := false
	verbMatches = verbMatches || contains(rule.Verbs, a.GetVerb())
	verbMatches = verbMatches || contains(rule.Verbs, "*")

	//check for negations that would force this match to false
	verbMatches = verbMatches && !contains(rule.Verbs, "-"+a.GetVerb())
	verbMatches = verbMatches && !contains(rule.Verbs, "-*")

	return verbMatches
}

func (a openshiftAuthorizationAttributes) kindMatches(rule authorizationapi.PolicyRule) bool {
	kindMatches := false
	kindMatches = kindMatches || contains(rule.ResourceKinds, a.GetResourceKind())
	kindMatches = kindMatches || contains(rule.ResourceKinds, "*")

	//check for negations that would force this match to false
	kindMatches = kindMatches && !contains(rule.ResourceKinds, "-"+a.GetResourceKind())
	kindMatches = kindMatches && !contains(rule.ResourceKinds, "-*")

	return kindMatches
}

func (a openshiftAuthorizationAttributes) GetUserInfo() authenticationapi.UserInfo {
	return a.user
}
func (a openshiftAuthorizationAttributes) GetVerb() string {
	return a.verb
}
func (a openshiftAuthorizationAttributes) GetResourceKind() string {
	return a.resourceKind
}
func (a openshiftAuthorizationAttributes) GetNamespace() string {
	return a.namespace
}
func (a openshiftAuthorizationAttributes) GetRequestAttributes() interface{} {
	return a.requestAttributes
}

func (a *openshiftAuthorizationAttributeBuilder) GetAttributes(req *http.Request) (AuthorizationAttributes, error) {
	verb, kind, namespace, _, err := VerbAndKindAndNamespace(req)
	if err != nil {
		return nil, err
	}

	userInterface, ok := a.requestsToUsers.Get(req)
	if !ok {
		return nil, errors.New("could not get user")
	}
	userInfo, ok := userInterface.(authenticationapi.UserInfo)
	if !ok {
		return nil, errors.New("wrong type returned for user")
	}

	return openshiftAuthorizationAttributes{
		user:              userInfo,
		verb:              verb,
		resourceKind:      kind,
		namespace:         namespace,
		requestAttributes: nil,
	}, nil
}

// TODO waiting on kube rebase
// this section is copied from kube.  Need to modify kube to make this pluggable
var specialVerbs = map[string]bool{
	"proxy":    true,
	"redirect": true,
	"watch":    true,
}

func VerbAndKindAndNamespace(req *http.Request) (string, string, string, []string, error) {
	parts := splitPath(req.URL.Path)

	verb := ""
	switch req.Method {
	case "POST":
		verb = "create"
	case "GET":
		verb = "get"
	case "PUT":
		verb = "update"
	case "DELETE":
		verb = "delete"
	}

	if parts[0] == "osapi" {
		if len(parts) > 2 {
			parts = parts[2:]
		} else {
			return "", "", "", nil, fmt.Errorf("Unable to determine kind and namespace from url, %v", req.URL)
		}
	}

	// TODO tweak upstream to eliminate this copy
	// handle input of form /api/{version}/* by adjusting special paths
	if parts[0] == "api" {
		if len(parts) > 2 {
			parts = parts[2:]
		} else {
			return "", "", "", parts, fmt.Errorf("Unable to determine kind and namespace from url, %v", req.URL)
		}
	}

	// handle input of form /{specialVerb}/*
	if _, ok := specialVerbs[parts[0]]; ok {
		verb = parts[0]
		if len(parts) > 1 {
			parts = parts[1:]
		} else {
			return "", "", "", parts, fmt.Errorf("Unable to determine kind and namespace from url, %v", req.URL)
		}
	}

	// URL forms: /ns/{namespace}/{kind}/*, where parts are adjusted to be relative to kind
	if parts[0] == "ns" {
		if len(parts) < 3 {
			return "", "", "", parts, fmt.Errorf("ResourceTypeAndNamespace expects a path of form /ns/{namespace}/*")
		}
		return verb, parts[1], parts[2], parts[2:], fmt.Errorf("Unable to determine kind and namespace from url, %v", req.URL)
	}

	// URL forms: /{kind}/*
	// URL forms: POST /{kind} is a legacy API convention to create in "default" namespace
	// URL forms: /{kind}/{resourceName} use the "default" namespace if omitted from query param
	// URL forms: /{kind} assume cross-namespace operation if omitted from query param
	kind := parts[0]
	namespace := req.URL.Query().Get("namespace")
	if len(namespace) == 0 {
		if len(parts) > 1 || req.Method == "POST" {
			namespace = kapi.NamespaceDefault
		} else {
			namespace = kapi.NamespaceAll
		}
	}
	return verb, kind, namespace, parts, nil
}

// splitPath returns the segments for a URL path.
func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}

func GetBootstrapPolicy(masterNamespace string) *authorizationapi.Policy {
	return &authorizationapi.Policy{
		ObjectMeta: kapi.ObjectMeta{
			Name:              authorizationapi.PolicyName,
			Namespace:         masterNamespace,
			CreationTimestamp: util.Now(),
		},
		LastModified: util.Now(),
		Roles: map[string]authorizationapi.Role{
			"cluster-admin": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "cluster-admin",
					Namespace: masterNamespace,
				},
				Rules: append(make([]authorizationapi.PolicyRule, 0),
					authorizationapi.PolicyRule{
						Verbs:         append(make([]string, 0), "*"),
						ResourceKinds: append(make([]string, 0), "*"),
					}),
			},
			"admin": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "admin",
					Namespace: masterNamespace,
				},
				Rules: append(make([]authorizationapi.PolicyRule, 0),
					authorizationapi.PolicyRule{
						Verbs:         append(make([]string, 0), "*", "-create", "-update", "-delete"),
						ResourceKinds: append(make([]string, 0), "*"),
					},
					authorizationapi.PolicyRule{
						Verbs:         append(make([]string, 0), "create", "update", "delete"),
						ResourceKinds: append(make([]string, 0), "*", "-roles", "-policyBindings"),
					},
				),
			},
			"edit": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "edit",
					Namespace: masterNamespace,
				},
				Rules: append(make([]authorizationapi.PolicyRule, 0),
					authorizationapi.PolicyRule{
						Verbs:         append(make([]string, 0), "*", "-create", "-update", "-delete"),
						ResourceKinds: append(make([]string, 0), "*", "-roles", "-roleBindings", "-policyBindings", "-policies"),
					},
					authorizationapi.PolicyRule{
						Verbs:         append(make([]string, 0), "create", "update", "delete"),
						ResourceKinds: append(make([]string, 0), "*", "-roles", "-roleBindings", "-policyBindings", "-policies"),
					},
				),
			},
			"view": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "view",
					Namespace: masterNamespace,
				},
				Rules: append(make([]authorizationapi.PolicyRule, 0),
					authorizationapi.PolicyRule{
						Verbs:         append(make([]string, 0), "watch", "list", "get"),
						ResourceKinds: append(make([]string, 0), "*", "-roles", "-roleBindings", "-policyBindings", "-policies"),
					}),
			},
			"ComponentRole": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "ComponentRole",
					Namespace: masterNamespace,
				},
				Rules: append(make([]authorizationapi.PolicyRule, 0),
					authorizationapi.PolicyRule{
						Verbs:         append(make([]string, 0), "*"),
						ResourceKinds: append(make([]string, 0), "*"),
					}),
			},
		},
	}
}

func GetBootstrapPolicyBinding(masterNamespace string) *authorizationapi.PolicyBinding {
	return &authorizationapi.PolicyBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name:              masterNamespace,
			Namespace:         masterNamespace,
			CreationTimestamp: util.Now(),
		},
		LastModified: util.Now(),
		PolicyRef:    kapi.ObjectReference{Namespace: masterNamespace},
		RoleBindings: map[string]authorizationapi.RoleBinding{
			"Components": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "Components",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "ComponentRole",
					Namespace: masterNamespace,
				},
				// until we get components added to their proper groups, enumerate them here
				UserNames:  append(make([]string, 0), "openshift-client", "kube-client"),
				GroupNames: append(make([]string, 0), "SystemComponents"),
			},
			"Cluster-Admins": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "Cluster-Admins",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "cluster-admin",
					Namespace: masterNamespace,
				},
				// until we decide to enforce policy, simply allow every one access
				GroupNames: append(make([]string, 0), "system:authenticated", "system:unauthenticated"),
			},
		},
	}
}
