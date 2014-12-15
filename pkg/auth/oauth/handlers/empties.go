package handlers

import (
	"net/http"

	"github.com/golang/glog"

	authapi "github.com/openshift/origin/pkg/auth/api"
)

type EmptyAuth struct{}

func (EmptyAuth) AuthenticationNeeded(client authapi.Client, w http.ResponseWriter, req *http.Request) (bool, error) {
	return false, nil
}

type EmptySuccess struct{}

func (EmptySuccess) AuthenticationSucceeded(user authapi.UserInfo, state string, w http.ResponseWriter, req *http.Request) (bool, error) {
	glog.V(4).Infof("AuthenticationSucceeded: %v (state=%s)", user, state)
	return false, nil
}

type EmptyError struct{}

func (EmptyError) AuthenticationError(err error, w http.ResponseWriter, req *http.Request) (bool, error) {
	glog.V(4).Infof("AuthenticationError: %v", err)
	return false, err
}

func (EmptyError) GrantError(err error, w http.ResponseWriter, req *http.Request) (bool, error) {
	glog.V(4).Infof("GrantError: %v", err)
	return false, err
}
