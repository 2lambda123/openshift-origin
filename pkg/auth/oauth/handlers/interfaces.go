package handlers

import (
	"net/http"

	"github.com/openshift/origin/pkg/auth/api"
)

// AuthenticationHandler reacts to unauthenticated requests
type AuthenticationHandler interface {
	// AuthenticationNeeded reacts to unauthenticated requests, and returns true if the response was written,
	AuthenticationNeeded(client api.Client, w http.ResponseWriter, req *http.Request) (handled bool, err error)
}

// AuthenticationChallenger reacts to unauthenticated requests with challenges
type AuthenticationChallenger interface {
	// AuthenticationChallenge take a request and return whatever challenge headers are appropriate.  If none are appropriate, it should return an empty map, not nil.
	AuthenticationChallenge(req *http.Request) (header http.Header, err error)
}

// AuthenticationRedirector reacts to unauthenticated requests with redirects
type AuthenticationRedirector interface {
	// AuthenticationRedirect is expected to write a redirect to the ResponseWriter or to return an error.
	AuthenticationRedirect(w http.ResponseWriter, req *http.Request) (err error)
}

// AuthenticationErrorHandler reacts to authentication errors
type AuthenticationErrorHandler interface {
	// AuthenticationNeeded reacts to authentication errors, returns true if the response was written,
	// and returns any unhandled error (which could be the original error)
	AuthenticationError(error, http.ResponseWriter, *http.Request) (handled bool, err error)
}

// AuthenticationSuccessHandler reacts to a user authenticating
type AuthenticationSuccessHandler interface {
	// AuthenticationSucceeded reacts to a user authenticating, returns true if the response was written,
	// and returns false if the response was not written.
	AuthenticationSucceeded(user api.UserInfo, state string, w http.ResponseWriter, req *http.Request) (bool, error)
}

// GrantChecker is responsible for determining if a user has authorized a client for a requested grant
type GrantChecker interface {
	// HasAuthorizedClient returns true if the user has authorized the client for the requested grant
	HasAuthorizedClient(user api.UserInfo, grant *api.Grant) (bool, error)
}

// GrantHandler handles errors during the grant process, or the client requests an unauthorized grant
type GrantHandler interface {
	// GrantNeeded reacts when a client requests an unauthorized grant, and returns true if the response was written
	GrantNeeded(user api.UserInfo, grant *api.Grant, w http.ResponseWriter, req *http.Request) (handled bool, err error)
}

// GrantErrorHandler reacts to grant errors
type GrantErrorHandler interface {
	// AuthenticationNeeded reacts to grant errors, returns true if the response was written,
	// and returns any unhandled error (which could be the original error)
	GrantError(error, http.ResponseWriter, *http.Request) (handled bool, err error)
}

// AuthenticationSuccessHandlers combines multiple AuthenticationSuccessHandler objects into a chain.
// On success, each handler is called. If any handler writes the response or returns an error,
// the chain is aborted.
type AuthenticationSuccessHandlers []AuthenticationSuccessHandler

func (all AuthenticationSuccessHandlers) AuthenticationSucceeded(user api.UserInfo, state string, w http.ResponseWriter, req *http.Request) (bool, error) {
	for _, h := range all {
		if handled, err := h.AuthenticationSucceeded(user, state, w, req); handled || err != nil {
			return handled, err
		}
	}
	return false, nil
}
