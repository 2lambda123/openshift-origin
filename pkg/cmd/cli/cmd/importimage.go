package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	importImageLong = `
Import tag and image information from an external Docker image repository

Only image streams that have a value set for spec.dockerImageRepository and/or
spec.Tags may have tag and image information imported.`

	importImageExample = `  $ %[1]s import-image mystream`
)

// NewCmdImportImage implements the OpenShift cli import-image command.
func NewCmdImportImage(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &ImportImageOptions{}
	cmd := &cobra.Command{
		Use:        "import-image IMAGESTREAM[:TAG]",
		Short:      "Imports images from a Docker registry",
		Long:       importImageLong,
		Example:    fmt.Sprintf(importImageExample, fullName),
		SuggestFor: []string{"image"},
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, args, out))
			kcmdutil.CheckErr(opts.Validate(cmd))
			kcmdutil.CheckErr(opts.Run())
		},
	}
	cmd.Flags().StringVar(&opts.From, "from", "", "A Docker image repository to import images from")
	cmd.Flags().BoolVar(&opts.Confirm, "confirm", false, "If true, allow the image stream import location to be set or changed")
	cmd.Flags().BoolVar(&opts.All, "all", false, "If true, import all tags from the provided source on creation or if --from is specified")
	cmd.Flags().BoolVar(&opts.Insecure, "insecure", false, "If true, allow importing from registries that have invalid HTTPS certificates or are hosted via HTTP")

	return cmd
}

// ImageImportOptions contains all the necessary information to perform an import.
type ImportImageOptions struct {
	// user set values
	From     string
	Confirm  bool
	All      bool
	Insecure bool

	// internal values
	Namespace string
	Name      string
	Tag       string
	Target    string

	// helpers
	out      io.Writer
	osClient client.Interface
	kClient  kclient.Interface
	isClient client.ImageStreamInterface
}

// Complete turns a partially defined ImportImageOptions into a solvent structure
// which can be validated and used for aa import.
func (o *ImportImageOptions) Complete(f *clientcmd.Factory, args []string, out io.Writer) error {
	if len(args) > 0 {
		o.Target = args[0]
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace

	osClient, kClient, err := f.Clients()
	if err != nil {
		return err
	}
	o.osClient = osClient
	o.kClient = kClient
	o.isClient = osClient.ImageStreams(namespace)
	o.out = out

	return nil
}

// Validate ensures that a ImportImageOptions is valid and can be used to execute
// an import.
func (o *ImportImageOptions) Validate(cmd *cobra.Command) error {
	if len(o.Target) == 0 {
		return kcmdutil.UsageError(cmd, "you must specify the name of an image stream")
	}

	targetRef, err := imageapi.ParseDockerImageReference(o.Target)
	switch {
	case err != nil:
		return fmt.Errorf("the image name must be a valid Docker image pull spec or reference to an image stream (e.g. myregistry/myteam/image:tag)")
	case len(targetRef.ID) > 0:
		return fmt.Errorf("to import images by ID, use the 'tag' command")
	case len(targetRef.Tag) != 0 && o.All:
		// error out
		return fmt.Errorf("cannot specify a tag %q as well as --all", o.Target)
	case len(targetRef.Tag) == 0 && !o.All:
		// apply the default tag
		targetRef.Tag = imageapi.DefaultImageTag
	}
	o.Name = targetRef.Name
	o.Tag = targetRef.Tag

	return nil
}

// Run contains all the necessary functionality for the OpenShift cli import-image command.
func (o *ImportImageOptions) Run() error {
	stream, isi, err := o.createImageImport()
	if err != nil {
		return err
	}

	// TODO: add dry-run
	result, err := o.isClient.Import(isi)
	switch {
	case err == client.ErrImageStreamImportUnsupported:
	case err != nil:
		return err
	default:
		fmt.Fprint(o.out, "The import completed successfully.\n\n")

		// optimization, use the image stream returned by the call
		d := describe.ImageStreamDescriber{OSClient: o.osClient, KubeClient: o.kClient}
		info, err := d.Describe(o.Namespace, stream.Name)
		if err != nil {
			return err
		}

		fmt.Fprintln(o.out, info)

		if r := result.Status.Repository; r != nil && len(r.AdditionalTags) > 0 {
			fmt.Fprintf(o.out, "\ninfo: The remote repository contained %d additional tags which were not imported: %s\n", len(r.AdditionalTags), strings.Join(r.AdditionalTags, ", "))
		}
		return nil
	}

	// Legacy path, remove when support for older importers is removed
	delete(stream.Annotations, imageapi.DockerImageRepositoryCheckAnnotation)
	if o.Insecure {
		if stream.Annotations == nil {
			stream.Annotations = make(map[string]string)
		}
		stream.Annotations[imageapi.InsecureRepositoryAnnotation] = "true"
	}

	if stream.CreationTimestamp.IsZero() {
		stream, err = o.isClient.Create(stream)
	} else {
		stream, err = o.isClient.Update(stream)
	}
	if err != nil {
		return err
	}

	fmt.Fprintln(o.out, "Importing (ctrl+c to stop waiting) ...")

	resourceVersion := stream.ResourceVersion
	updatedStream, err := o.waitForImport(resourceVersion)
	if err != nil {
		if _, ok := err.(importError); ok {
			return err
		}
		return fmt.Errorf("unable to determine if the import completed successfully - please run 'oc describe -n %s imagestream/%s' to see if the tags were updated as expected: %v", stream.Namespace, stream.Name, err)
	}

	fmt.Fprint(o.out, "The import completed successfully.\n\n")

	d := describe.ImageStreamDescriber{OSClient: o.osClient, KubeClient: o.kClient}
	info, err := d.Describe(updatedStream.Namespace, updatedStream.Name)
	if err != nil {
		return err
	}

	fmt.Fprintln(o.out, info)
	return nil
}

// TODO: move to image/api as a helper
type importError struct {
	annotation string
}

func (e importError) Error() string {
	return fmt.Sprintf("unable to import image: %s", e.annotation)
}

func (o *ImportImageOptions) waitForImport(resourceVersion string) (*imageapi.ImageStream, error) {
	streamWatch, err := o.isClient.Watch(kapi.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.name", o.Name), ResourceVersion: resourceVersion})
	if err != nil {
		return nil, err
	}
	defer streamWatch.Stop()

	for {
		select {
		case event, ok := <-streamWatch.ResultChan():
			if !ok {
				return nil, fmt.Errorf("image stream watch ended prematurely")
			}

			switch event.Type {
			case watch.Modified:
				s, ok := event.Object.(*imageapi.ImageStream)
				if !ok {
					continue
				}
				annotation, ok := s.Annotations[imageapi.DockerImageRepositoryCheckAnnotation]
				if !ok {
					continue
				}

				if _, err := time.Parse(time.RFC3339, annotation); err == nil {
					return s, nil
				}
				return nil, importError{annotation}

			case watch.Deleted:
				return nil, fmt.Errorf("the image stream was deleted")
			case watch.Error:
				return nil, fmt.Errorf("error watching image stream")
			}
		}
	}
}

func (o *ImportImageOptions) createImageImport() (*imageapi.ImageStream, *imageapi.ImageStreamImport, error) {
	stream, err := o.isClient.Get(o.Name)
	from := o.From
	tag := o.Tag
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, nil, err
		}

		// the stream is new
		if !o.Confirm {
			return nil, nil, fmt.Errorf("no image stream named %q exists, pass --confirm to create and import", o.Name)
		}
		if len(from) == 0 {
			from = o.Target
		}
		if o.All {
			stream = &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: o.Name},
				Spec:       imageapi.ImageStreamSpec{DockerImageRepository: from},
			}
		} else {
			stream = &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: o.Name},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						tag: {
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: from,
							},
						},
					},
				},
			}
		}

	} else {
		// the stream already exists
		if len(stream.Spec.DockerImageRepository) == 0 && len(stream.Spec.Tags) == 0 {
			return nil, nil, fmt.Errorf("image stream has not defined anything to import")
		}

		if o.All {
			// importing a whole repository
			// TODO soltysh: write tests to cover all the possible usecases!!!
			if len(from) == 0 {
				if len(stream.Spec.DockerImageRepository) == 0 {
					// FIXME soltysh:
					return nil, nil, fmt.Errorf("flag --all is applicable only to images with spec.dockerImageRepository defined")
				}
				from = stream.Spec.DockerImageRepository
			}
			if from != stream.Spec.DockerImageRepository {
				if !o.Confirm {
					if len(stream.Spec.DockerImageRepository) == 0 {
						return nil, nil, fmt.Errorf("the image stream does not currently import an entire Docker repository, pass --confirm to update")
					}
					return nil, nil, fmt.Errorf("the image stream has a different import spec %q, pass --confirm to update", stream.Spec.DockerImageRepository)
				}
				stream.Spec.DockerImageRepository = from
			}

		} else {
			// importing a single tag

			// follow any referential tags to the destination
			finalTag, existing, ok, multiple := imageapi.FollowTagReference(stream, tag)
			if !ok && multiple {
				return nil, nil, fmt.Errorf("tag %q on the image stream is a reference to %q, which does not exist", tag, finalTag)
			}

			if ok {
				// disallow changing an existing tag
				if existing.From == nil || existing.From.Kind != "DockerImage" {
					return nil, nil, fmt.Errorf("tag %q already exists - you must use the 'tag' command if you want to change the source to %q", tag, from)
				}
				if len(from) != 0 && from != existing.From.Name {
					if multiple {
						return nil, nil, fmt.Errorf("the tag %q points to the tag %q which points to %q - use the 'tag' command if you want to change the source to %q", tag, finalTag, existing.From.Name, from)
					}
					return nil, nil, fmt.Errorf("the tag %q points to %q - use the 'tag' command if you want to change the source to %q", tag, existing.From.Name, from)
				}

				// set the target item to import
				from = existing.From.Name
				if multiple {
					tag = finalTag
				}

				// clear the legacy annotation
				delete(existing.Annotations, imageapi.DockerImageRepositoryCheckAnnotation)
				// reset the generation
				zero := int64(0)
				existing.Generation = &zero

			} else {
				// create a new tag
				if len(from) == 0 {
					from = stream.Spec.DockerImageRepository
				}
				existing = &imageapi.TagReference{
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: from,
					},
				}
			}
			stream.Spec.Tags[tag] = *existing
		}
	}

	if len(from) == 0 {
		// catch programmer error
		return nil, nil, fmt.Errorf("unexpected error, from is empty")
	}

	// Attempt the new, direct import path
	isi := &imageapi.ImageStreamImport{
		ObjectMeta: kapi.ObjectMeta{
			Name:            stream.Name,
			Namespace:       o.Namespace,
			ResourceVersion: stream.ResourceVersion,
		},
		Spec: imageapi.ImageStreamImportSpec{Import: true},
	}
	if o.All {
		isi.Spec.Repository = &imageapi.RepositoryImportSpec{
			From: kapi.ObjectReference{
				Kind: "DockerImage",
				Name: from,
			},
			ImportPolicy: imageapi.TagImportPolicy{Insecure: o.Insecure},
		}
	} else {
		isi.Spec.Images = append(isi.Spec.Images, imageapi.ImageImportSpec{
			From: kapi.ObjectReference{
				Kind: "DockerImage",
				Name: from,
			},
			To:           &kapi.LocalObjectReference{Name: tag},
			ImportPolicy: imageapi.TagImportPolicy{Insecure: o.Insecure},
		})
	}

	return stream, isi, nil
}
