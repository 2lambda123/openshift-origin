package client

import (
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// ResourceAccessReviewsNamespacer has methods to work with ResourceAccessReview resources in a namespace
type ResourceAccessReviewsNamespacer interface {
	ResourceAccessReviews(namespace string) ResourceAccessReviewInterface
}

// ResourceAccessReviewInterface exposes methods on ResourceAccessReview resources.
type ResourceAccessReviewInterface interface {
	Create(policy *authorizationapi.ResourceAccessReview) (*authorizationapi.ResourceAccessReview, error)
}

// resourceAccessReviews implements ResourceAccessReviewsNamespacer interface
type resourceAccessReviews struct {
	r  *Client
	ns string
}

// newResourceAccessReviews returns a resourceAccessReviews
func newResourceAccessReviews(c *Client, namespace string) *resourceAccessReviews {
	return &resourceAccessReviews{
		r:  c,
		ns: namespace,
	}
}

// Create creates new policy. Returns the server's representation of the policy and error if one occurs.
func (c *resourceAccessReviews) Create(policy *authorizationapi.ResourceAccessReview) (result *authorizationapi.ResourceAccessReview, err error) {
	result = &authorizationapi.ResourceAccessReview{}
	err = c.r.Post().Namespace(c.ns).Resource("resourceAccessReviews").Body(policy).Do().Into(result)
	return
}
