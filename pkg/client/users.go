package client

import (
	"errors"

	userapi "github.com/openshift/origin/pkg/user/api"
	_ "github.com/openshift/origin/pkg/user/api/v1beta1"
)

// UsersInterface has methods to work with User resources in a namespace
type UsersInterface interface {
	Users() UserInterface
}

// UserInterface exposes methods on user resources.
type UserInterface interface {
	Get(name string) (*userapi.User, error)
}

// users implements UserIdentityMappingsNamespacer interface
type users struct {
	r *Client
}

// newUsers returns a users
func newUsers(c *Client) *users {
	return &users{
		r: c,
	}
}

// Get returns information about a particular user or an error
func (c *users) Get(name string) (result *userapi.User, err error) {
	if len(name) == 0 {
		return nil, errors.New("name is required parameter to Get")
	}

	result = &userapi.User{}
	err = c.r.Get().Path("users").Path(name).Do().Into(result)
	return
}
