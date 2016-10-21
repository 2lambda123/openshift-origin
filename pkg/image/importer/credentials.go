package importer

import (
	"net/url"
	"strings"
	"sync"

	"github.com/golang/glog"

	"github.com/docker/distribution/registry/client/auth"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

var (
	NoCredentials auth.CredentialStore = &noopCredentialStore{}

	emptyKeyring = &credentialprovider.BasicDockerKeyring{}
)

type noopCredentialStore struct{}

func (s *noopCredentialStore) Basic(url *url.URL) (string, string) {
	glog.Infof("asked to provide Basic credentials for %s", url)
	return "", ""
}

func (s *noopCredentialStore) RefreshToken(url *url.URL, service string) string {
	glog.Infof("asked to provide RefreshToken for %s", url)
	return ""
}

func (s *noopCredentialStore) SetRefreshToken(url *url.URL, service string, token string) {
	glog.Infof("asked to provide SetRefreshToken for %s", url)
}

func NewCredentialsForSecrets(secrets []kapi.Secret) *SecretCredentialStore {
	return &SecretCredentialStore{secrets: secrets}
}

func NewLazyCredentialsForSecrets(secretsFn func() ([]kapi.Secret, error)) *SecretCredentialStore {
	return &SecretCredentialStore{secretsFn: secretsFn}
}

type SecretCredentialStore struct {
	lock      sync.Mutex
	secrets   []kapi.Secret
	secretsFn func() ([]kapi.Secret, error)
	err       error
	keyring   credentialprovider.DockerKeyring
}

func (s *SecretCredentialStore) Basic(url *url.URL) (string, string) {
	return basicCredentialsFromKeyring(s.init(), url)
}

func (s *SecretCredentialStore) RefreshToken(url *url.URL, service string) string {
	return ""
}

func (s *SecretCredentialStore) SetRefreshToken(url *url.URL, service string, token string) {
}

func (s *SecretCredentialStore) Err() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.err
}

func (s *SecretCredentialStore) init() credentialprovider.DockerKeyring {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.keyring != nil {
		return s.keyring
	}

	// lazily load the secrets
	if s.secrets == nil {
		if s.secretsFn != nil {
			s.secrets, s.err = s.secretsFn()
		}
	}

	// TODO: need a version of this that is best effort secret - otherwise one error blocks all secrets
	keyring, err := credentialprovider.MakeDockerKeyring(s.secrets, emptyKeyring)
	if err != nil {
		glog.V(5).Infof("Loading keyring failed for credential store: %v", err)
		s.err = err
		keyring = emptyKeyring
	}
	s.keyring = keyring
	return keyring
}

func basicCredentialsFromKeyring(keyring credentialprovider.DockerKeyring, target *url.URL) (string, string) {
	// TODO: compare this logic to Docker authConfig in v2 configuration
	value := target.Host + target.Path

	// Lookup(...) expects an image (not a URL path).
	// The keyring strips /v1/ and /v2/ version prefixes,
	// so we should also when selecting a valid auth for a URL.
	pathWithSlash := target.Path + "/"
	if strings.HasPrefix(pathWithSlash, "/v1/") || strings.HasPrefix(pathWithSlash, "/v2/") {
		value = target.Host + target.Path[3:]
	}

	configs, found := keyring.Lookup(value)

	if !found || len(configs) == 0 {
		// do a special case check for docker.io to match historical lookups when we respond to a challenge
		if value == "auth.docker.io/token" {
			glog.V(5).Infof("Being asked for %s, trying %s for legacy behavior", target, "index.docker.io/v1")
			return basicCredentialsFromKeyring(keyring, &url.URL{Host: "index.docker.io", Path: "/v1"})
		}
		// docker 1.9 saves 'docker.io' in config in f23, see https://bugzilla.redhat.com/show_bug.cgi?id=1309739
		if value == "index.docker.io" {
			glog.V(5).Infof("Being asked for %s, trying %s for legacy behavior", target, "docker.io")
			return basicCredentialsFromKeyring(keyring, &url.URL{Host: "docker.io"})
		}
		glog.V(5).Infof("Unable to find a secret to match %s (%s)", target, value)
		return "", ""
	}
	glog.V(5).Infof("Found secret to match %s (%s): %s", target, value, configs[0].ServerAddress)
	return configs[0].Username, configs[0].Password
}
