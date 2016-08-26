package podsecuritypolicyreview

import (
	"fmt"
	"sort"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/runtime"
	kscc "k8s.io/kubernetes/pkg/securitycontextconstraints"
	"k8s.io/kubernetes/pkg/serviceaccount"
	kerrors "k8s.io/kubernetes/pkg/util/errors"

	oscache "github.com/openshift/origin/pkg/client/cache"
	securityapi "github.com/openshift/origin/pkg/security/api"
	securityvalidation "github.com/openshift/origin/pkg/security/api/validation"
	"github.com/openshift/origin/pkg/security/registry/podsecuritypolicysubjectreview"
	oscc "github.com/openshift/origin/pkg/security/scc"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	sccLister *oscache.IndexerToSecurityContextConstraintsLister
	client    clientset.Interface
}

// NewREST creates a new REST for policies..
func NewREST(l *oscache.IndexerToSecurityContextConstraintsLister, c clientset.Interface) *REST {
	return &REST{sccLister: l, client: c}
}

// New creates a new PodSecurityPolicyReview object
func (r *REST) New() runtime.Object {
	return &securityapi.PodSecurityPolicyReview{}
}

// Create registers a given new PodSecurityPolicyReview instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	pspr, ok := obj.(*securityapi.PodSecurityPolicyReview)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("not a PodSecurityPolicyReview: %#v", obj))
	}
	if errs := securityvalidation.ValidatePodSecurityPolicyReview(pspr); len(errs) > 0 {
		return nil, kapierrors.NewInvalid(securityapi.Kind(pspr.Kind), "", errs)
	}
	ns, ok := kapi.NamespaceFrom(ctx)
	if !ok {
		return pspr, kapierrors.NewBadRequest("namespace parameter required.")
	}

	serviceAccounts, err := getServiceAccounts(pspr.Spec, r.client, ns)
	if err != nil {
		return pspr, err
	}

	if len(serviceAccounts) == 0 {
		glog.Errorf("No service accounts for namespace %s", ns)
		return pspr, nil
	}

	errs := []error{}
	newStatus := securityapi.PodSecurityPolicyReviewStatus{}
	sccMatcher := oscc.NewDefaultSCCMatcher(r.sccLister)
	for _, sa := range serviceAccounts {
		userInfo := serviceaccount.UserInfo(ns, sa.Name, "")
		saConstraints, err := sccMatcher.FindApplicableSCCs(userInfo)
		if err != nil {
			errs = append(errs, fmt.Errorf("error finding SCC for ServiceAccount %s: %v", sa.Name, err))
			continue
		}
		oscc.DeduplicateSecurityContextConstraints(saConstraints)
		sort.Sort(oscc.ByPriority(saConstraints))
		var namespace *kapi.Namespace
		for _, constraint := range saConstraints {
			var (
				provider kscc.SecurityContextConstraintsProvider
				err      error
			)
			pspsrs := securityapi.PodSecurityPolicySubjectReviewStatus{}
			if provider, namespace, err = oscc.CreateProviderFromConstraint(ns, namespace, constraint, r.client); err != nil {
				errs = append(errs, fmt.Errorf("unable to created provider for service account %s: %v", sa.Name, err))
				continue
			}
			_, err = podsecuritypolicysubjectreview.FillPodSecurityPolicySubjectReviewStatus(&pspsrs, provider, pspr.Spec.Template.Spec, constraint)
			if err != nil {
				glog.Errorf("unable to fill PodSecurityPolicyReviewStatus from constraint %v", err)
				continue
			}
			sapsprs := securityapi.ServiceAccountPodSecurityPolicyReviewStatus{pspsrs, sa.Name}
			newStatus.AllowedServiceAccounts = append(newStatus.AllowedServiceAccounts, sapsprs)
		}

	}
	if len(errs) > 0 {
		return pspr, kerrors.NewAggregate(errs)
	}
	pspr.Status = newStatus
	return pspr, nil
}

func getServiceAccounts(psprSpec securityapi.PodSecurityPolicyReviewSpec, client clientset.Interface, namespace string) ([]*kapi.ServiceAccount, error) {
	serviceAccounts := []*kapi.ServiceAccount{}
	//  TODO: express 'all service accounts'
	//if serviceAccountList, err := client.Core().ServiceAccounts(namespace).List(kapi.ListOptions{}); err == nil {
	//	serviceAccounts = serviceAccountList.Items
	//	return serviceAccounts, fmt.Errorf("unable to retrieve service accounts: %v", err)
	//}

	if len(psprSpec.ServiceAccountNames) > 0 {
		errs := []error{}
		for _, saName := range psprSpec.ServiceAccountNames {
			sa, err := client.Core().ServiceAccounts(namespace).Get(saName)
			if err != nil {
				errs = append(errs, fmt.Errorf("unable to retrieve ServiceAccount %s: %v", saName, err))
			}
			serviceAccounts = append(serviceAccounts, sa)
		}
		return serviceAccounts, kerrors.NewAggregate(errs)
	}
	saName := "default"
	if len(psprSpec.Template.Spec.ServiceAccountName) > 0 {
		saName = psprSpec.Template.Spec.ServiceAccountName
	}
	sa, err := client.Core().ServiceAccounts(namespace).Get(saName)
	if err != nil {
		return serviceAccounts, fmt.Errorf("unable to retrieve ServiceAccount %s: %v", saName, err)
	}
	serviceAccounts = append(serviceAccounts, sa)
	return serviceAccounts, nil
}
