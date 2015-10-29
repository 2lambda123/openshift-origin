package scmauth

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/origin/pkg/util/file"
)

const (
	DefaultUsername      = "builder"
	UsernamePasswordName = "password"
	UsernameSecret       = "username"
	PasswordSecret       = "password"
	TokenSecret          = "token"
	UserPassGitConfig    = `# credential git config
[credential]
   helper = store --file=%s
`
)

// UsernamePassword implements SCMAuth interface for using Username and Password credentials
type UsernamePassword struct {
	SourceURL url.URL
}

// Setup creates a gitconfig fragment that includes a substitution URL with the username/password
// included in the URL. Returns source URL stripped of username/password credentials.
func (u UsernamePassword) Setup(baseDir string) (*url.URL, error) {

	// Only apply to https and http URLs
	if scheme := strings.ToLower(u.SourceURL.Scheme); scheme != "http" && scheme != "https" {
		return nil, nil
	}

	// Determine password
	// 1. Look for a token secret
	password, err := readSecret(baseDir, TokenSecret)
	if err != nil {
		return nil, err
	}
	// 2. Look for a password secret
	if password == "" {
		password, err = readSecret(baseDir, PasswordSecret)
		if err != nil {
			return nil, err
		}
	}
	// 3. Look for a password in the URL
	if password == "" && u.SourceURL.User != nil {
		password, _ = u.SourceURL.User.Password()
	}

	// Determine username
	// 1. Look for a username secret
	username, err := readSecret(baseDir, UsernameSecret)
	if err != nil {
		return nil, err
	}
	// 2. If not provided, look at the username in the URL
	if username == "" && u.SourceURL.User != nil {
		username = u.SourceURL.User.Username()
	}

	// 3. If username and password not found return error. If password is
	// present use a default username
	if username == "" {
		if password == "" {
			return nil, fmt.Errorf("username and password credentials not found in provided secret or source URL")
		}
		username = DefaultUsername
	}

	gitcredentials, err := ioutil.TempFile("", "gitcredentials.")
	if err != nil {
		return nil, err
	}
	defer gitcredentials.Close()
	gitconfig, err := ioutil.TempFile("", "gitcredentialscfg.")
	if err != nil {
		return nil, err
	}
	defer gitconfig.Close()

	usernamePasswordURL := u.SourceURL
	usernamePasswordURL.User = url.UserPassword(username, password)
	fmt.Fprintf(gitconfig, UserPassGitConfig, gitcredentials.Name())
	fmt.Fprintf(gitcredentials, "%s", usernamePasswordURL.String())

	sourceURL := u.SourceURL
	sourceURL.User = nil

	return &sourceURL, ensureGitConfigIncludes(gitconfig.Name())
}

// Name returns the name of this auth method.
func (_ UsernamePassword) Name() string {
	return UsernamePasswordName
}

// Handles returns true if a username, password or token secret is present
func (_ UsernamePassword) Handles(name string) bool {
	switch name {
	case UsernameSecret, PasswordSecret, TokenSecret:
		return true
	}
	return false
}

// readSecret reads the contents of a secret file
func readSecret(baseDir, fileName string) (string, error) {
	path := filepath.Join(baseDir, fileName)
	lines, err := file.ReadLines(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	// If the file is empty, simply return empty string
	if len(lines) == 0 {
		return "", nil
	}
	return lines[0], nil
}
