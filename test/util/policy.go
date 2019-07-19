package util

import (
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

const (
	PolicyCachePollInterval = 100 * time.Millisecond
	PolicyCachePollTimeout  = 10 * time.Second
)

// WaitForPolicyUpdate checks if the given client can perform the named verb and action.
// If PolicyCachePollTimeout is reached without the expected condition matching, an error is returned
func WaitForPolicyUpdate(c authorizationv1client.SelfSubjectAccessReviewsGetter, namespace, verb string, resource schema.GroupResource, allowed bool) error {
	review := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      verb,
				Group:     resource.Group,
				Resource:  resource.Resource,
			},
		},
	}
	err := wait.Poll(PolicyCachePollInterval, PolicyCachePollTimeout, func() (bool, error) {
		response, err := c.SelfSubjectAccessReviews().Create(review)
		if err != nil {
			return false, err
		}
		return response.Status.Allowed == allowed, nil
	})
	return err
}

// WaitForClusterPolicyUpdate checks if the given client can perform the named verb and action.
// If PolicyCachePollTimeout is reached without the expected condition matching, an error is returned
func WaitForClusterPolicyUpdate(c authorizationv1client.SelfSubjectAccessReviewsGetter, verb string, resource schema.GroupResource, allowed bool) error {
	review := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:     verb,
				Group:    resource.Group,
				Resource: resource.Resource,
			},
		},
	}
	err := wait.Poll(PolicyCachePollInterval, PolicyCachePollTimeout, func() (bool, error) {
		response, err := c.SelfSubjectAccessReviews().Create(review)
		if err != nil {
			return false, err
		}
		if response.Status.Allowed != allowed {
			return false, nil
		}
		return true, nil
	})
	return err
}
