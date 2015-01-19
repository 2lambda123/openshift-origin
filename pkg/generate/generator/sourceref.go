package generator

import (
	"fmt"
	"net/url"

	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/generate/git"
)

// Generators for SourceRef
// - Git URL        -> SourceRef
// - Directory      -> SourceRef

// SourceRefGenerator generates new SourceRefs either from a URL or a Directory
type SourceRefGenerator struct {
	repository git.Repository
}

// NewSourceRefGenerator creates a new SourceRefGenerator
func NewSourceRefGenerator() *SourceRefGenerator {
	return &SourceRefGenerator{
		repository: git.NewRepository(),
	}
}

// SourceRefForGitURL creates a SourceRef from a Git URL.
// If the URL includes a hash, it is used for the SourceRef's branch
// reference. Otherwise, 'master' is assumed
func (g *SourceRefGenerator) FromGitURL(location string) (*app.SourceRef, error) {
	url, err := url.Parse(location)
	if err != nil {
		return nil, err
	}

	ref := url.Fragment
	url.Fragment = ""
	if len(ref) == 0 {
		ref = "master"
	}
	return &app.SourceRef{URL: url, Ref: ref}, nil
}

// SourceRefForDirectory creates a SourceRef from a directory that contains
// a git repository. The URL is obtained from the origin remote branch, and
// the reference is taken from the currently checked out branch.
func (g *SourceRefGenerator) FromDirectory(directory string) (*app.SourceRef, error) {
	// Make sure that this is a git directory
	gitRoot, err := g.repository.GetRootDir(directory)
	if err != nil {
		return nil, err
	}

	// Get URL
	location, ok, err := g.repository.GetOriginURL(gitRoot)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("no origin remote defined for the provided Git repository")
	}

	// Get Branch Ref
	ref := g.repository.GetRef(gitRoot)

	srcRef, err := g.FromGitURL(fmt.Sprintf("%s#%s", location, ref))
	if err != nil {
		return nil, err
	}
	srcRef.Dir = gitRoot
	return srcRef, nil
}
