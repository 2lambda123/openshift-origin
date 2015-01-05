package main

import (
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/fsouza/go-dockerclient"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	dh "github.com/openshift/origin/pkg/cmd/util/docker"
	config "github.com/openshift/origin/pkg/config/api"
	appgen "github.com/openshift/origin/pkg/generate/app"
	gen "github.com/openshift/origin/pkg/generate/generator"
	"github.com/openshift/origin/pkg/generate/source"
)

type Input struct {
	name,
	sourceDir,
	sourceURL,
	dockerContext,
	builderImage,
	registry,
	outputImage string
}

const DefaultRegistry = "172.30.17.3:5001"

func main() {
	cfg := clientcmd.NewConfig()
	dockerHelper := dh.NewHelper()
	input := Input{}
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", "gen-app", clientcmd.ConfigSyntax),
		Short: "Generate an application configuration",
		Long:  "Generate an application configuration",
		Run: func(c *cobra.Command, args []string) {
			_, osClient, err := cfg.Clients()
			if err != nil {
				osClient = nil
			}
			dockerClient, _, err := dockerHelper.GetClient()
			if err != nil {
				osClient = nil
			}
			GenerateApp(input, osClient, dockerClient)
		},
	}

	flag := cmd.Flags()
	flag.StringVar(&input.name, "name", "", "Set name to use for generated application artifacts")
	flag.StringVar(&input.sourceDir, "source-dir", "", "Set the source directory for the application build")
	flag.StringVar(&input.sourceURL, "source-url", "", "Set the source URL")
	flag.StringVar(&input.dockerContext, "context", "", "Context path for Dockerfile if creating a Docker build")
	flag.StringVar(&input.builderImage, "builder-image", "", "Image to use for STI build")
	flag.StringVar(&input.outputImage, "output-image", "", "Image name to use for output")
	flag.StringVar(&input.registry, "docker-registry", "", "Set the docker registry to use (host:port). If not specified, uses "+DefaultRegistry)
	cfg.Bind(flag)
	dockerHelper.InstallFlags(flag)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err)
		os.Exit(1)
	}
}

func GenerateApp(input Input, client client.Interface, dockerClient *docker.Client) {
	// Get a SourceRef
	var srcRef *appgen.SourceRef
	var err error
	srcRefGen := gen.NewSourceRefGenerator()
	if len(input.sourceURL) > 0 {
		if srcRef, err = srcRefGen.FromGitURL(input.sourceURL); err != nil {
			exitWithError(err)
		}
	} else {
		if len(input.sourceDir) == 0 {
			if input.sourceDir, err = os.Getwd(); err != nil {
				exitWithError(err)
			}
		}
		if srcRef, err = srcRefGen.FromDirectory(input.sourceDir); err != nil {
			exitWithError(err)
		}
	}

	// Get a BuildStrategyRef
	var strategyRef *appgen.BuildStrategyRef
	strategyRefGen := gen.NewBuildStrategyRefGenerator(source.DefaultDetectors)
	imageRefGen := gen.NewImageRefGenerator()
	if len(input.dockerContext) > 0 {
		if strategyRef, err = strategyRefGen.FromSourceRefAndDockerContext(*srcRef, input.dockerContext); err != nil {
			exitWithError(err)
		}
	} else if len(input.builderImage) > 0 {
		builderRef, err := imageRefGen.FromName(input.builderImage)
		if err != nil {
			exitWithError(err)
		}
		if strategyRef, err = strategyRefGen.FromSTIBuilderImage(builderRef); err != nil {
			exitWithError(err)
		}
	} else {
		if strategyRef, err = strategyRefGen.FromSourceRef(*srcRef); err != nil {
			exitWithError(err)
		}
	}

	// Get an ImageRef for Output
	outputImage := input.outputImage
	if len(outputImage) == 0 {
		var ok bool
		if outputImage, ok = srcRef.SuggestName(); !ok {
			exitWithError(fmt.Errorf("Cannot suggest a name for the output image, please specify one in the command line using --output-image=\"<name>\""))
		}
	}
	outputRef, err := imageRefGen.FromName(outputImage)
	if err != nil {
		exitWithError(err)
	}
	registry := input.registry
	if len(registry) == 0 {
		registry = DefaultRegistry
	}
	outputRef.Registry = registry

	// Get a BuildRef
	buildRef := appgen.BuildRef{
		Source:   srcRef,
		Strategy: strategyRef,
		Output:   outputRef,
	}

	// Get a DeploymentConfigRef
	var imageInfo *appgen.ImageInfo
	imageInfoGen := gen.NewImageInfoGenerator(nil)
	if strategyRef.IsDockerBuild {
		imageInfo = imageInfoGen.FromDockerfile(*outputRef, srcRef.Dir, strategyRef.DockerContext)
	} else {
		imageInfo = imageInfoGen.FromSTIImageRef(*strategyRef.Base)
	}
	deployRef := appgen.DeploymentConfigRef{
		Images: []*appgen.ImageInfo{imageInfo},
	}

	// Generate OpenShift resources
	config := config.Config{}
	bldcfg, err := buildRef.BuildConfig()
	if err != nil {
		exitWithError(err)
	}
	imgrepo, err := outputRef.ImageRepository()
	if err != nil {
		exitWithError(err)
	}
	deploycfg, err := deployRef.DeploymentConfig()
	if err != nil {
		exitWithError(err)
	}
	addToConfig(&config, bldcfg)
	addToConfig(&config, imgrepo)
	addToConfig(&config, deploycfg)
	services, err := imageInfo.Services()
	if err != nil {
		exitWithError(err)
	}
	for _, service := range services {
		addToConfig(&config, service)
	}

	result, err := latest.Codec.Encode(&config)
	if err != nil {
		exitWithError(err)
	}
	fmt.Println(string(result))
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "%v", err)
	os.Exit(1)
}

func addToConfig(cfg *config.Config, object runtime.Object) {
	json, err := latest.Codec.Encode(object)
	if err != nil {
		exitWithError(err)
	}
	cfg.Items = append(cfg.Items, runtime.RawExtension{RawJSON: json})
}
