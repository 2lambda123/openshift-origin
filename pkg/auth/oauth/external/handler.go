package external

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/RangelReale/osincli"
	"github.com/golang/glog"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/oauth/handlers"
)

// Handler exposes an external oauth provider flow (including the call back) as an oauth.handlers.AuthenticationHandler to allow our internal oauth
// server to use an external oauth provider for authentication
type Handler struct {
	provider     Provider
	state        State
	clientConfig *osincli.ClientConfig
	client       *osincli.Client
	success      handlers.AuthenticationSuccessHandler
	errorHandler handlers.AuthenticationErrorHandler
	mapper       authapi.UserIdentityMapper
}

func NewExternalOAuthRedirector(provider Provider, state State, redirectURL string, success handlers.AuthenticationSuccessHandler, errorHandler handlers.AuthenticationErrorHandler, mapper authapi.UserIdentityMapper) (*Handler, error) {
	clientConfig, err := provider.NewConfig()
	if err != nil {
		return nil, err
	}

	clientConfig.RedirectUrl = redirectURL

	client, err := osincli.NewClient(clientConfig)
	if err != nil {
		return nil, err
	}

	return &Handler{
		provider:     provider,
		state:        state,
		clientConfig: clientConfig,
		client:       client,
		success:      success,
		errorHandler: errorHandler,
		mapper:       mapper,
	}, nil
}

// AuthenticationRedirect implements oauth.handlers.RedirectAuthHandler
func (h *Handler) AuthenticationRedirect(w http.ResponseWriter, req *http.Request) error {
	glog.V(4).Infof("Authentication needed for %v", h)

	authReq := h.client.NewAuthorizeRequest(osincli.CODE)
	h.provider.AddCustomParameters(authReq)

	state, err := h.state.Generate(w, req)
	if err != nil {
		glog.V(4).Infof("Error generating state: %v", err)
		return err
	}

	oauthURL := authReq.GetAuthorizeUrlWithParams(state)
	glog.V(4).Infof("redirect to %v", oauthURL)

	http.Redirect(w, req, oauthURL.String(), http.StatusFound)
	return nil
}

// ServeHTTP handles the callback request in response to an external oauth flow
func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	// Extract auth code
	authReq := h.client.NewAuthorizeRequest(osincli.CODE)
	authData, err := authReq.HandleRequest(req)
	if err != nil {
		glog.V(4).Infof("Error handling request: %v", err)
		h.errorHandler.AuthenticationError(err, w, req)
		return
	}

	glog.V(4).Infof("Got auth data")

	// Exchange code for a token
	accessReq := h.client.NewAccessRequest(osincli.AUTHORIZATION_CODE, authData)
	accessData, err := accessReq.GetToken()
	if err != nil {
		glog.V(4).Infof("Error getting access token:", err)
		h.errorHandler.AuthenticationError(err, w, req)
		return
	}

	glog.V(4).Infof("Got access data")

	identity, ok, err := h.provider.GetUserIdentity(accessData)
	if err != nil {
		glog.V(4).Infof("Error getting userIdentityInfo info: %v", err)
		h.errorHandler.AuthenticationError(err, w, req)
		return
	}
	if !ok {
		glog.V(4).Infof("Could not get userIdentityInfo info from access token")
		h.errorHandler.AuthenticationError(errors.New("Could not get userIdentityInfo info from access token"), w, req)
		return
	}

	user, err := h.mapper.UserFor(identity)
	glog.V(4).Infof("Got userIdentityMapping: %#v", user)
	if err != nil {
		glog.V(4).Infof("Error creating or updating mapping for: %#v due to %v", identity, err)
		h.errorHandler.AuthenticationError(err, w, req)
		return
	}

	ok, err = h.state.Check(authData.State, w, req)
	if !ok {
		glog.V(4).Infof("State is invalid")
		h.errorHandler.AuthenticationError(errors.New("State is invalid"), w, req)
		return
	}
	if err != nil {
		glog.V(4).Infof("Error verifying state: %v", err)
		h.errorHandler.AuthenticationError(err, w, req)
		return
	}

	_, err = h.success.AuthenticationSucceeded(user, authData.State, w, req)
	if err != nil {
		glog.V(4).Infof("Error calling success handler: %v", err)
		h.errorHandler.AuthenticationError(err, w, req)
		return
	}
}

// defaultState provides default state-building, validation, and parsing to contain CSRF and "then" redirection
type defaultState struct{}

func DefaultState() State {
	return defaultState{}
}

func (defaultState) Generate(w http.ResponseWriter, req *http.Request) (string, error) {
	state := url.Values{
		"csrf": {"..."}, // TODO: get csrf
		"then": {req.URL.String()},
	}
	return encodeState(state)
}

func (defaultState) Check(state string, w http.ResponseWriter, req *http.Request) (bool, error) {
	values, err := decodeState(state)
	if err != nil {
		return false, err
	}
	csrf := values.Get("csrf")
	if csrf != "..." {
		return false, fmt.Errorf("State did not contain valid CSRF token (expected %s, got %s)", "...", csrf)
	}

	then := values.Get("then")
	if then == "" {
		return false, errors.New("State did not contain a redirect")
	}

	return true, nil
}

func (defaultState) AuthenticationSucceeded(user user.Info, state string, w http.ResponseWriter, req *http.Request) (bool, error) {
	values, err := decodeState(state)
	if err != nil {
		return false, err
	}

	then := values.Get("then")
	if len(then) == 0 {
		return false, errors.New("No redirect given")
	}

	http.Redirect(w, req, then, http.StatusFound)
	return true, nil
}

func encodeState(values url.Values) (string, error) {
	return base64.URLEncoding.EncodeToString([]byte(values.Encode())), nil
}

func decodeState(state string) (url.Values, error) {
	decodedState, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		return nil, err
	}
	return url.ParseQuery(string(decodedState))
}
