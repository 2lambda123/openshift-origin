package v1beta3

import (
	kapi "k8s.io/kubernetes/pkg/api/v1beta3"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
)

// ImageList is a list of Image objects.
type ImageList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	Items []Image `json:"items"`
}

// ImageStatus is an information about the current status of an Image.
type ImageStatus struct {
	// Phase is the current lifecycle phase of the image.
	Phase string `json:"phase,omitempty" description:"current lifecycle phase of the image"`
}

// Image is an immutable representation of a Docker image and metadata at a point in time.
type Image struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// The string that can be used to pull this image.
	DockerImageReference string `json:"dockerImageReference,omitempty"`
	// Metadata about this image
	DockerImageMetadata runtime.RawExtension `json:"dockerImageMetadata,omitempty"`
	// This attribute conveys the version of the object, which if empty defaults to "1.0"
	DockerImageMetadataVersion string `json:"dockerImageMetadataVersion,omitempty"`
	// The raw JSON of the manifest
	DockerImageManifest string `json:"dockerImageManifest,omitempty"`
	// Finalizers is an opaque list of values that must be empty to permanently remove object from storage
	Finalizers []kapi.FinalizerName `json:"finalizers,omitempty" description:"opaque list of values that must be empty to permanently remove object from storage"`
	// Status describes the current status of an Image
	Status ImageStatus `json:"status,omitempty" description:"current status of an Image"`
}

// ImageStreamList is a list of ImageStream objects.
type ImageStreamList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	Items []ImageStream `json:"items"`
}

// ImageStream stores a mapping of tags to images, metadata overrides that are applied
// when images are tagged in a stream, and an optional reference to a Docker image
// repository on a registry.
type ImageStream struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Spec describes the desired state of this stream
	Spec ImageStreamSpec `json:"spec"`
	// Status describes the current state of this stream
	Status ImageStreamStatus `json:"status,omitempty"`
}

// ImageStreamSpec represents options for ImageStreams.
type ImageStreamSpec struct {
	// Optional, if specified this stream is backed by a Docker repository on this server
	DockerImageRepository string `json:"dockerImageRepository,omitempty"`
	// Tags map arbitrary string values to specific image locators
	Tags []NamedTagReference `json:"tags,omitempty"`
	// Finalizers is an opaque list of values that must be empty to permanently remove object from storage
	Finalizers []kapi.FinalizerName `json:"finalizers,omitempty"`
}

// NamedTagReference specifies optional annotations for images using this tag and an optional reference to an ImageStreamTag, ImageStreamImage, or DockerImage this tag should track.
type NamedTagReference struct {
	Name        string                `json:"name"`
	Annotations map[string]string     `json:"annotations,omitempty"`
	From        *kapi.ObjectReference `json:"from,omitempty"`
}

// These are the valid phases of an image stream.
const (
	// ImageStreamActive means the image stream is available for use in the system
	ImageStreamAvailable string = "Available"
	// ImageStreamTerminating means the image stream is being deleted
	ImageStreamTerminating string = "Terminating"
)

// ImageStreamStatus contains information about the state of this image stream.
type ImageStreamStatus struct {
	// Represents the effective location this stream may be accessed at. May be empty until the server
	// determines where the repository is located
	DockerImageRepository string `json:"dockerImageRepository"`
	// A historical record of images associated with each tag. The first entry in the TagEvent array is
	// the currently tagged image.
	Tags []NamedTagEventList `json:"tags,omitempty"`
	// Phase is the current lifecycle phase of the image stream.
	Phase string `json:"phase" description:"phase is the current lifecycle phase of the image stream"`
}

// NamedTagEventList relates a tag to its image history.
type NamedTagEventList struct {
	Tag   string     `json:"tag"`
	Items []TagEvent `json:"items"`
}

// TagEvent is used by ImageRepositoryStatus to keep a historical record of images associated with a tag.
type TagEvent struct {
	// When the TagEvent was created
	Created util.Time `json:"created"`
	// The string that can be used to pull this image
	DockerImageReference string `json:"dockerImageReference"`
	// The image
	Image string `json:"image"`
}

// ImageStreamMapping represents a mapping from a single tag to a Docker image as
// well as the reference to the Docker image repository the image came from.
type ImageStreamMapping struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// A Docker image.
	Image Image `json:"image"`
	// A string value this image can be located with inside the repository.
	Tag string `json:"tag"`
}

// ImageStreamTag represents an Image that is retrieved by tag name from an ImageStream.
type ImageStreamTag struct {
	Image     `json:",inline"`
	ImageName string `json:"imageName"`
}

// ImageStreamImageList is a list of image stream image objects.
type ImageStreamImageList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Items is a list of images stream images
	Items []ImageStreamImage `json:"items" description:"list of image stream image objects"`
}

// ImageStreamImage represents an Image that is retrieved by image name from an ImageStream.
type ImageStreamImage struct {
	Image     `json:",inline"`
	ImageName string `json:"imageName"`
}

// ImageStreamDeletionList is a list of image stream deletion objects.
type ImageStreamDeletionList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Items is a list of images stream images
	Items []ImageStreamDeletion `json:"items" description:"list of image stream deletion objects"`
}

// ImageStreamDeletion represents an ImageStream that have been deleted from
// etcd store and is awaiting a garbage collection in internal registry.
type ImageStreamDeletion struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`
}

// DockerImageReference points to a Docker image.
type DockerImageReference struct {
	Registry  string
	Namespace string
	Name      string
	Tag       string
	ID        string
}
