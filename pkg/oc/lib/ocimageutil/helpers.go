package ocimageutil

import (
	"strings"

	dockerv10 "github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
)

func HasAnnotationTag(tagRef *imagev1.TagReference, searchTag string) bool {
	for _, tag := range strings.Split(tagRef.Annotations["tags"], ",") {
		if tag == searchTag {
			return true
		}
	}
	return false
}

// ShortDockerImageID returns a short form of the provided DockerImage ID for display
func ShortDockerImageID(image *dockerv10.DockerImage, length int) string {
	id := image.ID
	if s, err := imageutil.ParseDigest(id); err == nil {
		id = s.Hex()
	}
	if len(id) > length {
		id = id[:length]
	}
	return id
}
