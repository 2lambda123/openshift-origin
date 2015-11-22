package builder

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/glog"

	s2iapi "github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/scm/git"

	"github.com/openshift/origin/pkg/build/api"
)

const (
	// urlCheckTimeout is the timeout used to check the source URL
	// If fetching the URL exceeds the timeout, then the build will
	// not proceed further and stop
	urlCheckTimeout = 16 * time.Second
)

type gitAuthError string
type gitNotFoundError string

func (e gitAuthError) Error() string {
	return fmt.Sprintf("failed to fetch requested repository %q with provided credentials", string(e))
}

func (e gitNotFoundError) Error() string {
	return fmt.Sprintf("requested repository %q not found", string(e))
}

// fetchSource retrieves the inputs defined by the build source into the
// provided directory, or returns an error if retrieval is not possible.
func fetchSource(dir string, build *api.Build, urlTimeout time.Duration, in io.Reader, git git.Git) error {
	hasGitSource := false

	// expect to receive input from STDIN
	if err := extractInputBinary(in, build.Spec.Source.Binary, dir); err != nil {
		return err
	}

	// may retrieve source from Git
	hasGitSource, err := extractGitSource(git, build.Spec.Source.Git, build.Spec.Revision, dir, urlTimeout)
	if err != nil {
		return err
	}

	// a Dockerfile has been specified, create or overwrite into the destination
	if dockerfileSource := build.Spec.Source.Dockerfile; dockerfileSource != nil {
		baseDir := dir
		// if a context dir has been defined and we cloned source, overwrite the destination
		if hasGitSource && len(build.Spec.Source.ContextDir) != 0 {
			baseDir = filepath.Join(baseDir, build.Spec.Source.ContextDir)
		}
		return ioutil.WriteFile(filepath.Join(baseDir, "Dockerfile"), []byte(*dockerfileSource), 0660)
	}

	return nil
}

// checkRemoteGit validates the specified Git URL. It returns GitNotFoundError
// when the remote repository not found and GitAuthenticationError when the
// remote repository failed to authenticate.
// Since this is calling the 'git' binary, the proxy settings should be
// available for this command.
func checkRemoteGit(url string, timeout time.Duration) error {
	glog.V(4).Infof("git ls-remote --heads %q", url)
	cmd := exec.Command("git", "ls-remote", "--heads", url)
	cmd.Env = []string{"GIT_ASKPASS=/bin/true"}

	var (
		out []byte
		err error
	)

	finish := make(chan struct{}, 1)
	go func() {
		out, err = cmd.CombinedOutput()
		close(finish)
	}()
	select {
	case <-finish:
	case <-time.After(timeout):
		return fmt.Errorf("timeout while waiting for remote repository %q", url)
	}

	glog.V(4).Infof(string(out))

	switch {
	case strings.Contains(string(out), "Authentication failed"):
		return gitAuthError(url)
	case strings.Contains(string(out), "not found"):
		return gitNotFoundError(url)
	}

	return err
}

// checkSourceURI performs a check on the URI associated with the build
// to make sure that it is valid.  It also optionally tests the connection
// to the source uri.
func checkSourceURI(git git.Git, rawurl string, timeout time.Duration) error {
	if !git.ValidCloneSpec(rawurl) {
		return fmt.Errorf("Invalid git source url: %s", rawurl)
	}
	return checkRemoteGit(rawurl, timeout)
}

// extractInputBinary processes the provided input stream as directed by BinaryBuildSource
// into dir.
func extractInputBinary(in io.Reader, source *api.BinaryBuildSource, dir string) error {
	if source == nil {
		return nil
	}

	var path string
	if len(source.AsFile) > 0 {
		glog.V(2).Infof("Receiving source from STDIN as file %s", source.AsFile)
		path = filepath.Join(dir, source.AsFile)

		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0664)
		if err != nil {
			return err
		}
		defer f.Close()
		n, err := io.Copy(f, os.Stdin)
		if err != nil {
			return err
		}
		glog.V(4).Infof("Received %d bytes into %s", n, path)
		return nil
	}

	glog.V(2).Infof("Receiving source from STDIN as archive")

	cmd := exec.Command("bsdtar", "-x", "-o", "-m", "-f", "-", "-C", dir)
	cmd.Stdin = in
	out, err := cmd.CombinedOutput()
	if err != nil {
		glog.V(2).Infof("Extracting...\n%s", string(out))
		return fmt.Errorf("unable to extract binary build input, must be a zip, tar, or gzipped tar, or specified as a file: %v", err)
	}
	return nil
}

func extractGitSource(git git.Git, gitSource *api.GitBuildSource, revision *api.SourceRevision, dir string, timeout time.Duration) (bool, error) {
	if gitSource == nil {
		return false, nil
	}

	// Set the HTTP and HTTPS proxies to be used by git clone.
	originalProxies := setHTTPProxy(gitSource.HTTPProxy, gitSource.HTTPSProxy)
	defer resetHTTPProxy(originalProxies)

	// Check source URI, trying to connect to the server only if not using a proxy.
	if err := checkSourceURI(git, gitSource.URI, timeout); err != nil {
		return true, err
	}

	glog.V(2).Infof("Cloning source from %s", gitSource.URI)
	if err := git.Clone(gitSource.URI, dir, s2iapi.CloneConfig{Recursive: true, Quiet: true}); err != nil {
		return true, err
	}

	// if we specify a commit, ref, or branch to checkout, do so
	if len(gitSource.Ref) != 0 || (revision != nil && revision.Git != nil && len(revision.Git.Commit) != 0) {
		commit := gitSource.Ref
		if revision != nil && revision.Git != nil && revision.Git.Commit != "" {
			commit = revision.Git.Commit
		}
		if err := git.Checkout(dir, commit); err != nil {
			return true, err
		}
	}
	return true, nil
}
