package podsecuritypolicyselfsubjectreview

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	admissionttesting "github.com/openshift/origin/pkg/security/apiserver/admission/testing"
	oscc "github.com/openshift/origin/pkg/security/apiserver/securitycontextconstraints"
	securitylisters "github.com/openshift/origin/pkg/security/generated/listers/security/internalversion"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestPodSecurityPolicySelfSubjectReview(t *testing.T) {
	testcases := map[string]struct {
		sccs  []*securityapi.SecurityContextConstraints
		check func(p *securityapi.PodSecurityPolicySelfSubjectReview) (bool, string)
	}{
		"user foo": {
			sccs: []*securityapi.SecurityContextConstraints{
				admissionttesting.UserScc("bar"),
				admissionttesting.UserScc("foo"),
			},
			check: func(p *securityapi.PodSecurityPolicySelfSubjectReview) (bool, string) {
				fmt.Printf("-> Is %q", p.Status.AllowedBy.Name)
				return p.Status.AllowedBy.Name == "foo", "SCC should be foo"
			},
		},
		"user bar ": {
			sccs: []*securityapi.SecurityContextConstraints{
				admissionttesting.UserScc("bar"),
			},
			check: func(p *securityapi.PodSecurityPolicySelfSubjectReview) (bool, string) {
				return p.Status.AllowedBy == nil, "Allowed by should be nil"
			},
		},
	}
	for testName, testcase := range testcases {
		namespace := admissionttesting.CreateNamespaceForTest()
		serviceAccount := admissionttesting.CreateSAForTest()
		reviewRequest := &securityapi.PodSecurityPolicySelfSubjectReview{
			Spec: securityapi.PodSecurityPolicySelfSubjectReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name:                     "ctr",
								Image:                    "image",
								ImagePullPolicy:          "IfNotPresent",
								TerminationMessagePolicy: kapi.TerminationMessageReadFile,
							},
						},
						RestartPolicy:      kapi.RestartPolicyAlways,
						SecurityContext:    &kapi.PodSecurityContext{},
						DNSPolicy:          kapi.DNSClusterFirst,
						ServiceAccountName: "default",
						SchedulerName:      kapi.DefaultSchedulerName,
					},
				},
			},
		}

		sccIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		sccCache := securitylisters.NewSecurityContextConstraintsLister(sccIndexer)

		for _, scc := range testcase.sccs {
			if err := sccIndexer.Add(scc); err != nil {
				t.Fatalf("error adding sccs to store: %v", err)
			}
		}

		csf := clientsetfake.NewSimpleClientset(namespace, serviceAccount)
		storage := REST{oscc.NewDefaultSCCMatcher(sccCache, &noopTestAuthorizer{}), csf}
		ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), metav1.NamespaceAll), &user.DefaultInfo{Name: "foo", Groups: []string{"bar", "baz"}})
		obj, err := storage.Create(ctx, reviewRequest, rest.ValidateAllObjectFunc, false)
		if err != nil {
			t.Errorf("%s - Unexpected error", testName)
		}
		pspssr, ok := obj.(*securityapi.PodSecurityPolicySelfSubjectReview)
		if !ok {
			t.Errorf("%s - Unable to convert created runtime.Object to PodSecurityPolicySelfSubjectReview", testName)
			continue
		}
		if ok, message := testcase.check(pspssr); !ok {
			t.Errorf("%s - %s", testName, message)
		}
	}
}

type noopTestAuthorizer struct{}

func (s *noopTestAuthorizer) Authorize(a authorizer.Attributes) (authorizer.Decision, string, error) {
	return authorizer.DecisionNoOpinion, "", nil
}
