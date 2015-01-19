package generator

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/generate/dockerfile"
	"github.com/openshift/origin/pkg/generate/errors"
	"github.com/openshift/origin/pkg/generate/git"
	"github.com/openshift/origin/pkg/generate/source"
)

// Flows for BuildStrategyRef
// SourceRef -> BuildStrategyRef
// SourceRef + Docker Context -> BuildStrategyRef
// Docker Context + Parent Image -> BuildStrategyRef
// STI Builder Image -> BuildStrategyRef

// BuildStrategyRefGenerator generates BuildStrategyRef
type BuildStrategyRefGenerator struct {
	gitRepository     git.Repository
	dockerfileFinder  dockerfile.Finder
	dockerfileParser  dockerfile.Parser
	sourceDetectors   source.Detectors
	imageRefGenerator ImageRefGenerator
}

// NewBuildStrategyRefGenerator creates a BuildStrategyRefGenerator
func NewBuildStrategyRefGenerator(sourceDetectors source.Detectors) *BuildStrategyRefGenerator {
	return &BuildStrategyRefGenerator{
		gitRepository:     git.NewRepository(),
		dockerfileFinder:  dockerfile.NewFinder(),
		dockerfileParser:  dockerfile.NewParser(),
		sourceDetectors:   sourceDetectors,
		imageRefGenerator: NewImageRefGenerator(),
	}
}

// FromSourceRef creates a build strategy from a source reference
func (g *BuildStrategyRefGenerator) FromSourceRef(srcRef app.SourceRef) (*app.BuildStrategyRef, error) {

	// Download source locally first if not available
	if len(srcRef.Dir) == 0 {
		if err := g.getSource(&srcRef); err != nil {
			return nil, err
		}
	}

	// Detect a Dockerfile
	context, found, err := g.detectDockerFile(srcRef.Dir)
	if err != nil {
		return nil, err
	}
	if found {
		return g.FromSourceRefAndDockerContext(srcRef, context)
	}

	// Detect a STI repository
	sourceInfo, ok := g.sourceDetectors.DetectSource(srcRef.Dir)
	if !ok {
		return nil, errors.CouldNotDetect
	}
	builderImage, err := g.imageForSourceInfo(sourceInfo)
	if err != nil {
		return nil, err
	}
	return g.FromSTIBuilderImage(builderImage)
}

// FromSourceRefAndDockerContext generates a BuildStrategyRef from a source ref and context path
func (g *BuildStrategyRefGenerator) FromSourceRefAndDockerContext(srcRef app.SourceRef, context string) (*app.BuildStrategyRef, error) {
	// Download source locally first if not available
	if len(srcRef.Dir) == 0 {
		if err := g.getSource(&srcRef); err != nil {
			return nil, err
		}
	}

	// Look for Dockerfile in repository
	file, err := os.Open(filepath.Join(srcRef.Dir, context, "Dockerfile"))
	if err != nil {
		return nil, err
	}

	dockerFile, err := g.dockerfileParser.Parse(file)
	if err != nil {
		return nil, err
	}

	parentImageName, ok := dockerFile.GetDirective("FROM")
	if !ok {
		return nil, errors.InvalidDockerfile
	}

	return g.FromDockerContextAndParent(context, parentImageName[0])

}

// FromContextAndParent generates a build strategy ref from a context path and parent image name
func (g *BuildStrategyRefGenerator) FromDockerContextAndParent(context string, parentImageName string) (*app.BuildStrategyRef, error) {
	var parentImage, err = g.imageRefGenerator.FromName(parentImageName)
	if err != nil {
		return nil, err
	}
	return &app.BuildStrategyRef{
		IsDockerBuild: true,
		Base:          parentImage,
		DockerContext: context,
	}, nil
}

// FromSTIBuilderImage generates a build strategy from a builder image ref
func (g *BuildStrategyRefGenerator) FromSTIBuilderImage(image *app.ImageRef) (*app.BuildStrategyRef, error) {
	return &app.BuildStrategyRef{
		IsDockerBuild: false,
		Base:          image,
	}, nil
}

func (g *BuildStrategyRefGenerator) imageForSourceInfo(s *source.Info) (*app.ImageRef, error) {
	var imageName string
	// TODO: More sophisticated matching
	switch s.Platform {
	case "Ruby":
		imageName = "openshift/ruby-20-centos"
	case "JEE":
		imageName = "openshift/wildfly-8-centos"
	case "NodeJS":
		imageName = "openshift/nodejs-0-10-centos"
	default:
		return nil, errors.NoBuilderFound
	}
	return g.imageRefGenerator.FromName(imageName)
}

func (g *BuildStrategyRefGenerator) detectDockerFile(dir string) (contextDir string, found bool, err error) {
	dockerFiles, err := g.dockerfileFinder.Find(dir)
	if err != nil {
		return "", false, err
	}
	if len(dockerFiles) > 1 {
		return "", true, errors.NewMultipleDockerfilesErr(dockerFiles)
	}
	if len(dockerFiles) == 1 {
		return filepath.Dir(dockerFiles[0]), true, nil
	}
	return "", false, nil
}

func (g *BuildStrategyRefGenerator) getSource(srcRef *app.SourceRef) error {
	var err error
	// Clone git repository into a local directory
	if srcRef.Dir, err = ioutil.TempDir("", "gen"); err != nil {
		return err
	}
	if err = g.gitRepository.Clone(srcRef.Dir, srcRef.URL.String()); err != nil {
		return err
	}
	if len(srcRef.Ref) != 0 {
		if err = g.gitRepository.Checkout(srcRef.Dir, srcRef.Ref); err != nil {
			return err
		}
	}
	return nil
}
