package bearertoken

import (
	"net/http"
	"strings"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"k8s.io/kubernetes/pkg/auth/user"
)

type Authenticator struct {
	// auth is the token authenticator to use to validate the token
	auth authenticator.Token
	// removeHeader indicates whether the Authorization header should be removeHeaderd on successful auth
	removeHeader bool

	// headerName is the name of the header to use
	headerName string
}

func New(auth authenticator.Token, removeHeader bool) *Authenticator {
	return &Authenticator{auth, removeHeader, "Authorization"}
}

func NewForProxy(auth authenticator.Token, removeHeader bool) *Authenticator {
	return &Authenticator{auth, removeHeader, "Proxy-Authorization"}
}

func (a *Authenticator) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	auth := strings.TrimSpace(req.Header.Get(a.headerName))
	if auth == "" {
		return nil, false, nil
	}
	parts := strings.Split(auth, " ")
	if len(parts) < 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil, false, nil
	}

	token := parts[1]

	// Empty bearer tokens aren't valid
	if len(token) == 0 {
		return nil, false, nil
	}

	user, ok, err := a.auth.AuthenticateToken(token)
	if ok && a.removeHeader {
		req.Header.Del(a.headerName)
	}
	return user, ok, err
}
