package admission

import (
	"reflect"
	"strings"
	"testing"

	kadmission "k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/util"

	allocator "github.com/openshift/origin/pkg/security"
	pspapi "github.com/openshift/origin/pkg/security/policy/api"
	pspprovider "github.com/openshift/origin/pkg/security/policy/provider"
)

func NewTestAdmission(store cache.Store, kclient client.Interface) kadmission.Interface {
	return &constraint{
		Handler: kadmission.NewHandler(kadmission.Create),
		client:  kclient,
		store:   store,
	}
}

func TestAdmit(t *testing.T) {
	// create the annotated namespace and add it to the fake client
	namespace := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation: "1/3",
				allocator.MCSAnnotation:      "s0:c1,c0",
			},
		},
	}
	serviceAccount := &kapi.ServiceAccount{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
		},
	}

	tc := testclient.NewSimpleFake(namespace, serviceAccount)

	// create scc that requires allocation retrieval
	saSCC := &pspapi.PodSecurityPolicy{
		ObjectMeta: kapi.ObjectMeta{
			Name: "scc-sa",
		},
		Spec: pspapi.PodSecurityPolicySpec{
			RunAsUser: pspapi.RunAsUserStrategyOptions{
				Type: pspapi.RunAsUserStrategyMustRunAsRange,
			},
			SELinuxContext: pspapi.SELinuxContextStrategyOptions{
				Type: pspapi.SELinuxStrategyMustRunAs,
			},
			Groups: []string{"system:serviceaccounts"},
		},
	}
	// create scc that has specific requirements that shouldn't match but is permissioned to
	// service accounts to test exact matches
	var exactUID int64 = 999
	saExactSCC := &pspapi.PodSecurityPolicy{
		ObjectMeta: kapi.ObjectMeta{
			Name: "scc-sa-exact",
		},
		Spec: pspapi.PodSecurityPolicySpec{
			RunAsUser: pspapi.RunAsUserStrategyOptions{
				Type: pspapi.RunAsUserStrategyMustRunAs,
				UID:  &exactUID,
			},
			SELinuxContext: pspapi.SELinuxContextStrategyOptions{
				Type: pspapi.SELinuxStrategyMustRunAs,
				SELinuxOptions: &kapi.SELinuxOptions{
					Level: "s9:z0,z1",
				},
			},
			Groups: []string{"system:serviceaccounts"},
		},
	}
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	store.Add(saExactSCC)
	store.Add(saSCC)

	// create the admission plugin
	p := NewTestAdmission(store, tc)

	// setup test data
	// goodPod is empty and should not be used directly for testing since we're providing
	// two different SCCs.  Since no values are specified it would be allowed to match either
	// SCC when defaults are filled in.
	goodPod := func() *kapi.Pod {
		return &kapi.Pod{
			Spec: kapi.PodSpec{
				ServiceAccountName: "default",
				Containers: []kapi.Container{
					{
						SecurityContext: &kapi.SecurityContext{},
					},
				},
			},
		}
	}

	uidNotInRange := goodPod()
	var uid int64 = 1001
	uidNotInRange.Spec.Containers[0].SecurityContext.RunAsUser = &uid

	invalidMCSLabels := goodPod()
	invalidMCSLabels.Spec.Containers[0].SecurityContext.SELinuxOptions = &kapi.SELinuxOptions{
		Level: "s1:q0,q1",
	}

	disallowedPriv := goodPod()
	var priv bool = true
	disallowedPriv.Spec.Containers[0].SecurityContext.Privileged = &priv

	// specifies a UID in the range of the preallocated UID annotation
	specifyUIDInRange := goodPod()
	var goodUID int64 = 3
	specifyUIDInRange.Spec.Containers[0].SecurityContext.RunAsUser = &goodUID

	// specifies an mcs label that matches the preallocated mcs annotation
	specifyLabels := goodPod()
	specifyLabels.Spec.Containers[0].SecurityContext.SELinuxOptions = &kapi.SELinuxOptions{
		Level: "s0:c1,c0",
	}

	requestsHostNetwork := goodPod()
	requestsHostNetwork.Spec.HostNetwork = true

	requestsHostPorts := goodPod()
	requestsHostPorts.Spec.Containers[0].Ports = []kapi.ContainerPort{{HostPort: 1}}

	testCases := map[string]struct {
		pod           *kapi.Pod
		shouldAdmit   bool
		expectedUID   int64
		expectedLevel string
		expectedPriv  bool
	}{
		"uidNotInRange": {
			pod:         uidNotInRange,
			shouldAdmit: false,
		},
		"invalidMCSLabels": {
			pod:         invalidMCSLabels,
			shouldAdmit: false,
		},
		"disallowedPriv": {
			pod:         disallowedPriv,
			shouldAdmit: false,
		},
		"specifyUIDInRange": {
			pod:           specifyUIDInRange,
			shouldAdmit:   true,
			expectedUID:   *specifyUIDInRange.Spec.Containers[0].SecurityContext.RunAsUser,
			expectedLevel: "s0:c1,c0",
		},
		"specifyLabels": {
			pod:           specifyLabels,
			shouldAdmit:   true,
			expectedUID:   1,
			expectedLevel: specifyLabels.Spec.Containers[0].SecurityContext.SELinuxOptions.Level,
		},
		"requestsHostNetwork": {
			pod:         requestsHostNetwork,
			shouldAdmit: false,
		},
		"requestsHostPorts": {
			pod:         requestsHostPorts,
			shouldAdmit: false,
		},
	}

	for k, v := range testCases {
		attrs := kadmission.NewAttributesRecord(v.pod, "Pod", "namespace", "", string(kapi.ResourcePods), "", kadmission.Create, &user.DefaultInfo{})
		err := p.Admit(attrs)

		if v.shouldAdmit && err != nil {
			t.Errorf("%s expected no errors but received %v", k, err)
		}
		if !v.shouldAdmit && err == nil {
			t.Errorf("%s expected errors but received none", k)
		}

		if v.shouldAdmit {
			validatedSCC, ok := v.pod.Annotations[allocator.ValidatedSCCAnnotation]
			if !ok {
				t.Errorf("%s expected to find the validated annotation on the pod for the scc but found none", k)
			}
			if validatedSCC != saSCC.Name {
				t.Errorf("%s should have validated against %s but found %s", k, saSCC.Name, validatedSCC)
			}
			if *v.pod.Spec.Containers[0].SecurityContext.RunAsUser != v.expectedUID {
				t.Errorf("%s expected UID %d but found %d", k, v.expectedUID, *v.pod.Spec.Containers[0].SecurityContext.RunAsUser)
			}
			if v.pod.Spec.Containers[0].SecurityContext.SELinuxOptions.Level != v.expectedLevel {
				t.Errorf("%s expected Level %s but found %s", k, v.expectedLevel, v.pod.Spec.Containers[0].SecurityContext.SELinuxOptions.Level)
			}
		}
	}

	// now add an escalated scc to the group and re-run the cases that expected failure, they should
	// now pass by validating against the escalated scc.
	adminSCC := &pspapi.PodSecurityPolicy{
		ObjectMeta: kapi.ObjectMeta{
			Name: "scc-admin",
		},
		Spec: pspapi.PodSecurityPolicySpec{
			Privileged:  true,
			HostNetwork: true,
			HostPorts: []pspapi.HostPortRange{
				{
					Start: 1,
					End:   65535,
				},
			},
			RunAsUser: pspapi.RunAsUserStrategyOptions{
				Type: pspapi.RunAsUserStrategyRunAsAny,
			},
			SELinuxContext: pspapi.SELinuxContextStrategyOptions{
				Type: pspapi.SELinuxStrategyRunAsAny,
			},
			Groups: []string{"system:serviceaccounts"},
		},
	}
	store.Add(adminSCC)

	for k, v := range testCases {
		if !v.shouldAdmit {
			attrs := kadmission.NewAttributesRecord(v.pod, "Pod", "namespace", "", string(kapi.ResourcePods), "", kadmission.Create, &user.DefaultInfo{})
			err := p.Admit(attrs)
			if err != nil {
				t.Errorf("Expected %s to pass with escalated scc but got error %v", k, err)
			}
			validatedSCC, ok := v.pod.Annotations[allocator.ValidatedSCCAnnotation]
			if !ok {
				t.Errorf("%s expected to find the validated annotation on the pod for the scc but found none", k)
			}
			if validatedSCC != adminSCC.Name {
				t.Errorf("%s should have validated against %s but found %s", k, adminSCC.Name, validatedSCC)
			}
		}
	}
}

func TestAssignSecurityContext(t *testing.T) {
	// set up test data
	// scc that will deny privileged container requests and has a default value for a field (uid)
	var uid int64 = 9999
	scc := &pspapi.PodSecurityPolicy{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test scc",
		},
		Spec: pspapi.PodSecurityPolicySpec{
			SELinuxContext: pspapi.SELinuxContextStrategyOptions{
				Type: pspapi.SELinuxStrategyRunAsAny,
			},
			RunAsUser: pspapi.RunAsUserStrategyOptions{
				Type: pspapi.RunAsUserStrategyMustRunAs,
				UID:  &uid,
			},
		},
	}
	provider, err := pspprovider.NewSimpleProvider(scc)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	createContainer := func(priv bool) kapi.Container {
		return kapi.Container{
			SecurityContext: &kapi.SecurityContext{
				Privileged: &priv,
			},
		}
	}

	// these are set up such that the containers always have a nil uid.  If the case should not
	// validate then the uids should not have been updated by the strategy.  If the case should
	// validate then uids should be set.  This is ensuring that we're hanging on to the old SC
	// as we generate/validate and only updating the original container if the entire pod validates
	testCases := map[string]struct {
		pod            *kapi.Pod
		shouldValidate bool
		expectedUID    *int64
	}{
		"container SC is not changed when invalid": {
			pod: &kapi.Pod{
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{createContainer(true)},
				},
			},
			shouldValidate: false,
		},
		"must validate all containers": {
			pod: &kapi.Pod{
				Spec: kapi.PodSpec{
					// good pod and bad pod
					Containers: []kapi.Container{createContainer(false), createContainer(true)},
				},
			},
			shouldValidate: false,
		},
		"pod validates": {
			pod: &kapi.Pod{
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{createContainer(false)},
				},
			},
			shouldValidate: true,
		},
	}

	for k, v := range testCases {
		errs := assignSecurityContext(provider, v.pod)
		if v.shouldValidate && len(errs) > 0 {
			t.Errorf("%s expected to validate but received errors %v", k, errs)
			continue
		}
		if !v.shouldValidate && len(errs) == 0 {
			t.Errorf("%s expected validation errors but received none", k)
			continue
		}

		// if we shouldn't have validated ensure that uid is not set on the containers
		if !v.shouldValidate {
			for _, c := range v.pod.Spec.Containers {
				if c.SecurityContext.RunAsUser != nil {
					t.Errorf("%s had non-nil UID %d.  UID should not be set on test cases that dont' validate", k, *c.SecurityContext.RunAsUser)
				}
			}
		}

		// if we validated then the pod sc should be updated now with the defaults from the SCC
		if v.shouldValidate {
			for _, c := range v.pod.Spec.Containers {
				if *c.SecurityContext.RunAsUser != uid {
					t.Errorf("%s expected uid to be defaulted to %d but found %v", k, uid, c.SecurityContext.RunAsUser)
				}
			}
		}
	}
}

func TestCreateProvidersFromConstraints(t *testing.T) {
	namespaceValid := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation: "1/3",
				allocator.MCSAnnotation:      "s0:c1,c0",
			},
		},
	}
	namespaceNoUID := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.MCSAnnotation: "s0:c1,c0",
			},
		},
	}
	namespaceNoMCS := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation: "1/3",
			},
		},
	}

	testCases := map[string]struct {
		// use a generating function so we can test for non-mutation
		scc         func() *pspapi.PodSecurityPolicy
		namespace   *kapi.Namespace
		expectedErr string
	}{
		"valid non-preallocated scc": {
			scc: func() *pspapi.PodSecurityPolicy {
				return &pspapi.PodSecurityPolicy{
					ObjectMeta: kapi.ObjectMeta{
						Name: "valid non-preallocated scc",
					},
					Spec: pspapi.PodSecurityPolicySpec{
						SELinuxContext: pspapi.SELinuxContextStrategyOptions{
							Type: pspapi.SELinuxStrategyRunAsAny,
						},
						RunAsUser: pspapi.RunAsUserStrategyOptions{
							Type: pspapi.RunAsUserStrategyRunAsAny,
						},
					},
				}
			},
			namespace: namespaceValid,
		},
		"valid pre-allocated scc": {
			scc: func() *pspapi.PodSecurityPolicy {
				return &pspapi.PodSecurityPolicy{
					ObjectMeta: kapi.ObjectMeta{
						Name: "valid pre-allocated scc",
					},
					Spec: pspapi.PodSecurityPolicySpec{
						SELinuxContext: pspapi.SELinuxContextStrategyOptions{
							Type:           pspapi.SELinuxStrategyMustRunAs,
							SELinuxOptions: &kapi.SELinuxOptions{User: "myuser"},
						},
						RunAsUser: pspapi.RunAsUserStrategyOptions{
							Type: pspapi.RunAsUserStrategyMustRunAsRange,
						},
					},
				}
			},
			namespace: namespaceValid,
		},
		"pre-allocated no uid annotation": {
			scc: func() *pspapi.PodSecurityPolicy {
				return &pspapi.PodSecurityPolicy{
					ObjectMeta: kapi.ObjectMeta{
						Name: "pre-allocated no uid annotation",
					},
					Spec: pspapi.PodSecurityPolicySpec{
						SELinuxContext: pspapi.SELinuxContextStrategyOptions{
							Type: pspapi.SELinuxStrategyMustRunAs,
						},
						RunAsUser: pspapi.RunAsUserStrategyOptions{
							Type: pspapi.RunAsUserStrategyMustRunAsRange,
						},
					},
				}
			},
			namespace:   namespaceNoUID,
			expectedErr: "unable to find pre-allocated uid annotation",
		},
		"pre-allocated no mcs annotation": {
			scc: func() *pspapi.PodSecurityPolicy {
				return &pspapi.PodSecurityPolicy{
					ObjectMeta: kapi.ObjectMeta{
						Name: "pre-allocated no mcs annotation",
					},
					Spec: pspapi.PodSecurityPolicySpec{
						SELinuxContext: pspapi.SELinuxContextStrategyOptions{
							Type: pspapi.SELinuxStrategyMustRunAs,
						},
						RunAsUser: pspapi.RunAsUserStrategyOptions{
							Type: pspapi.RunAsUserStrategyMustRunAsRange,
						},
					},
				}
			},
			namespace:   namespaceNoMCS,
			expectedErr: "unable to find pre-allocated mcs annotation",
		},
		"bad scc strategy options": {
			scc: func() *pspapi.PodSecurityPolicy {
				return &pspapi.PodSecurityPolicy{
					ObjectMeta: kapi.ObjectMeta{
						Name: "bad scc user options",
					},
					Spec: pspapi.PodSecurityPolicySpec{
						SELinuxContext: pspapi.SELinuxContextStrategyOptions{
							Type: pspapi.SELinuxStrategyRunAsAny,
						},
						RunAsUser: pspapi.RunAsUserStrategyOptions{
							Type: pspapi.RunAsUserStrategyMustRunAs,
						},
					},
				}
			},
			namespace:   namespaceValid,
			expectedErr: "MustRunAs requires a UID",
		},
	}

	for k, v := range testCases {
		store := cache.NewStore(cache.MetaNamespaceKeyFunc)

		// create the admission handler
		tc := testclient.NewSimpleFake(v.namespace)
		admit := &constraint{
			Handler: kadmission.NewHandler(kadmission.Create),
			client:  tc,
			store:   store,
		}

		scc := v.scc()

		// create the providers, this method only needs the namespace
		attributes := kadmission.NewAttributesRecord(nil, "", v.namespace.Name, "", "", "", kadmission.Create, nil)
		_, errs := admit.createProvidersFromConstraints(attributes.GetNamespace(), []*pspapi.PodSecurityPolicy{scc})

		if !reflect.DeepEqual(scc, v.scc()) {
			diff := util.ObjectDiff(scc, v.scc())
			t.Errorf("%s createProvidersFromConstraints mutated constraints. diff:\n%s", k, diff)
		}
		if len(v.expectedErr) > 0 && len(errs) != 1 {
			t.Errorf("%s expected a single error '%s' but received %v", k, v.expectedErr, errs)
			continue
		}
		if len(v.expectedErr) == 0 && len(errs) != 0 {
			t.Errorf("%s did not expect an error but received %v", k, errs)
			continue
		}

		// check that we got the error we expected
		if len(v.expectedErr) > 0 {
			if !strings.Contains(errs[0].Error(), v.expectedErr) {
				t.Errorf("%s expected error '%s' but received %v", k, v.expectedErr, errs[0])
			}
		}
	}
}

func TestMatchingSecurityContextConstraints(t *testing.T) {
	sccs := []*pspapi.PodSecurityPolicy{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: "match group",
			},
			Spec: pspapi.PodSecurityPolicySpec{
				Groups: []string{"group"},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: "match user",
			},
			Spec: pspapi.PodSecurityPolicySpec{
				Users: []string{"user"},
			},
		},
	}
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	for _, v := range sccs {
		store.Add(v)
	}

	// single match cases
	testCases := map[string]struct {
		userInfo    user.Info
		expectedSCC string
	}{
		"find none": {
			userInfo: &user.DefaultInfo{
				Name:   "foo",
				Groups: []string{"bar"},
			},
		},
		"find user": {
			userInfo: &user.DefaultInfo{
				Name:   "user",
				Groups: []string{"bar"},
			},
			expectedSCC: "match user",
		},
		"find group": {
			userInfo: &user.DefaultInfo{
				Name:   "foo",
				Groups: []string{"group"},
			},
			expectedSCC: "match group",
		},
	}

	for k, v := range testCases {
		sccs, err := getMatchingSecurityContextConstraints(store, v.userInfo)
		if err != nil {
			t.Errorf("%s received error %v", k, err)
			continue
		}
		if v.expectedSCC == "" {
			if len(sccs) > 0 {
				t.Errorf("%s expected to match 0 sccs but found %d: %#v", k, len(sccs), sccs)
			}
		}
		if v.expectedSCC != "" {
			if len(sccs) != 1 {
				t.Errorf("%s returned more than one scc, use case can not validate: %#v", k, sccs)
				continue
			}
			if v.expectedSCC != sccs[0].Name {
				t.Errorf("%s expected to match %s but found %s", k, v.expectedSCC, sccs[0].Name)
			}
		}
	}

	// check that we can match many at once
	userInfo := &user.DefaultInfo{
		Name:   "user",
		Groups: []string{"group"},
	}
	sccs, err := getMatchingSecurityContextConstraints(store, userInfo)
	if err != nil {
		t.Fatalf("matching many sccs returned error %v", err)
	}
	if len(sccs) != 2 {
		t.Errorf("matching many sccs expected to match 2 sccs but found %d: %#v", len(sccs), sccs)
	}
}

func TestRequiresPreAllocatedUIDRange(t *testing.T) {
	var uid int64 = 1

	testCases := map[string]struct {
		scc      *pspapi.PodSecurityPolicy
		requires bool
	}{
		"must run as": {
			scc: &pspapi.PodSecurityPolicy{
				Spec: pspapi.PodSecurityPolicySpec{
					RunAsUser: pspapi.RunAsUserStrategyOptions{
						Type: pspapi.RunAsUserStrategyMustRunAs,
					},
				},
			},
		},
		"run as any": {
			scc: &pspapi.PodSecurityPolicy{
				Spec: pspapi.PodSecurityPolicySpec{
					RunAsUser: pspapi.RunAsUserStrategyOptions{
						Type: pspapi.RunAsUserStrategyRunAsAny,
					},
				},
			},
		},
		"run as non-root": {
			scc: &pspapi.PodSecurityPolicy{
				Spec: pspapi.PodSecurityPolicySpec{
					RunAsUser: pspapi.RunAsUserStrategyOptions{
						Type: pspapi.RunAsUserStrategyMustRunAsNonRoot,
					},
				},
			},
		},
		"run as range": {
			scc: &pspapi.PodSecurityPolicy{
				Spec: pspapi.PodSecurityPolicySpec{
					RunAsUser: pspapi.RunAsUserStrategyOptions{
						Type: pspapi.RunAsUserStrategyMustRunAsRange,
					},
				},
			},
			requires: true,
		},
		"run as range with specified params": {
			scc: &pspapi.PodSecurityPolicy{
				Spec: pspapi.PodSecurityPolicySpec{
					RunAsUser: pspapi.RunAsUserStrategyOptions{
						Type:        pspapi.RunAsUserStrategyMustRunAsRange,
						UIDRangeMin: &uid,
						UIDRangeMax: &uid,
					},
				},
			},
		},
	}

	for k, v := range testCases {
		result := requiresPreAllocatedUIDRange(v.scc)
		if result != v.requires {
			t.Errorf("%s expected result %t but got %t", k, v.requires, result)
		}
	}
}

func TestRequiresPreAllocatedSELinuxLevel(t *testing.T) {
	testCases := map[string]struct {
		scc      *pspapi.PodSecurityPolicy
		requires bool
	}{
		"must run as": {
			scc: &pspapi.PodSecurityPolicy{
				Spec: pspapi.PodSecurityPolicySpec{
					SELinuxContext: pspapi.SELinuxContextStrategyOptions{
						Type: pspapi.SELinuxStrategyMustRunAs,
					},
				},
			},
			requires: true,
		},
		"must with level specified": {
			scc: &pspapi.PodSecurityPolicy{
				Spec: pspapi.PodSecurityPolicySpec{
					SELinuxContext: pspapi.SELinuxContextStrategyOptions{
						Type: pspapi.SELinuxStrategyMustRunAs,
						SELinuxOptions: &kapi.SELinuxOptions{
							Level: "foo",
						},
					},
				},
			},
		},
		"run as any": {
			scc: &pspapi.PodSecurityPolicy{
				Spec: pspapi.PodSecurityPolicySpec{
					SELinuxContext: pspapi.SELinuxContextStrategyOptions{
						Type: pspapi.SELinuxStrategyRunAsAny,
					},
				},
			},
		},
	}

	for k, v := range testCases {
		result := requiresPreAllocatedSELinuxLevel(v.scc)
		if result != v.requires {
			t.Errorf("%s expected result %t but got %t", k, v.requires, result)
		}
	}
}

func TestDeduplicateSecurityContextConstraints(t *testing.T) {
	duped := []*pspapi.PodSecurityPolicy{
		{ObjectMeta: kapi.ObjectMeta{Name: "a"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "a"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "b"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "b"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "c"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "d"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "e"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "e"}},
	}

	deduped := deduplicateSecurityContextConstraints(duped)

	if len(deduped) != 5 {
		t.Fatalf("expected to have 5 remaining sccs but found %d: %v", len(deduped), deduped)
	}

	constraintCounts := map[string]int{}

	for _, scc := range deduped {
		if _, ok := constraintCounts[scc.Name]; !ok {
			constraintCounts[scc.Name] = 0
		}
		constraintCounts[scc.Name] = constraintCounts[scc.Name] + 1
	}

	for k, v := range constraintCounts {
		if v > 1 {
			t.Errorf("%s was found %d times after de-duping", k, v)
		}
	}

}
