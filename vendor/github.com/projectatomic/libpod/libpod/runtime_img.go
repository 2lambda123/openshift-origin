package libpod

import (
	"context"
	"fmt"
	"io"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/projectatomic/buildah/imagebuildah"
	"github.com/projectatomic/libpod/libpod/common"
	"github.com/projectatomic/libpod/libpod/image"
)

// Runtime API

// CopyOptions contains the options given when pushing or pulling images
type CopyOptions struct {
	// Compression specifies the type of compression which is applied to
	// layer blobs.  The default is to not use compression, but
	// archive.Gzip is recommended.
	Compression archive.Compression
	// DockerRegistryOptions encapsulates settings that affect how we
	// connect or authenticate to a remote registry to which we want to
	// push the image.
	common.DockerRegistryOptions
	// SigningOptions encapsulates settings that control whether or not we
	// strip or add signatures to the image when pushing (uploading) the
	// image to a registry.
	common.SigningOptions

	// SigningPolicyPath this points to a alternative signature policy file, used mainly for testing
	SignaturePolicyPath string
	// AuthFile is the path of the cached credentials file defined by the user
	AuthFile string
	// Writer is the reportWriter for the output
	Writer io.Writer
	// Reference is the name for the image created when a tar archive is imported
	Reference string
	// ImageConfig is the Image spec for the image created when a tar archive is imported
	ImageConfig ociv1.Image
	// ManifestMIMEType is the manifest type of the image when saving to a directory
	ManifestMIMEType string
	// ForceCompress compresses the image layers when saving to a directory using the dir transport if true
	ForceCompress bool
}

// RemoveImage deletes an image from local storage
// Images being used by running containers can only be removed if force=true
func (r *Runtime) RemoveImage(ctx context.Context, img *image.Image, force bool) (string, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return "", ErrRuntimeStopped
	}

	// Get all containers, filter to only those using the image, and remove those containers
	ctrs, err := r.state.AllContainers()
	if err != nil {
		return "", err
	}
	imageCtrs := []*Container{}
	for _, ctr := range ctrs {
		if ctr.config.RootfsImageID == img.ID() {
			imageCtrs = append(imageCtrs, ctr)
		}
	}
	if len(imageCtrs) > 0 && len(img.Names()) <= 1 {
		if force {
			for _, ctr := range imageCtrs {
				if err := r.removeContainer(ctx, ctr, true); err != nil {
					return "", errors.Wrapf(err, "error removing image %s: container %s using image could not be removed", img.ID(), ctr.ID())
				}
			}
		} else {
			return "", fmt.Errorf("could not remove image %s as it is being used by %d containers", img.ID(), len(imageCtrs))
		}
	}

	hasChildren, err := img.IsParent()
	if err != nil {
		return "", err
	}

	if (len(img.Names()) > 1 && !img.InputIsID()) || hasChildren {
		// If the image has multiple reponames, we do not technically delete
		// the image. we figure out which repotag the user is trying to refer
		// to and untag it.
		repoName, err := img.MatchRepoTag(img.InputName)
		if hasChildren && err == image.ErrRepoTagNotFound {
			return "", errors.Errorf("unable to delete %q (cannot be forced) - image has dependent child images", img.ID())
		}
		if err != nil {
			return "", err
		}
		if err := img.UntagImage(repoName); err != nil {
			return "", err
		}
		return fmt.Sprintf("Untagged: %s", repoName), nil
	} else if len(img.Names()) > 1 && img.InputIsID() && !force {
		// If the user requests to delete an image by ID and the image has multiple
		// reponames and no force is applied, we error out.
		return "", fmt.Errorf("unable to delete %s (must force) - image is referred to in multiple tags", img.ID())
	}
	err = img.Remove(force)
	if err != nil && errors.Cause(err) == storage.ErrImageUsedByContainer {
		if errStorage := r.rmStorageContainers(force, img); errStorage == nil {
			// Containers associated with the image should be deleted now,
			// let's try removing the image again.
			err = img.Remove(force)
		} else {
			err = errStorage
		}
	}
	return img.ID(), err
}

// Remove containers that are in storage rather than Podman.
func (r *Runtime) rmStorageContainers(force bool, image *image.Image) error {
	ctrIDs, err := storageContainers(image.ID(), r.store)
	if err != nil {
		return errors.Wrapf(err, "error getting containers for image %q", image.ID())
	}

	if len(ctrIDs) > 0 && !force {
		return storage.ErrImageUsedByContainer
	}

	if len(ctrIDs) > 0 && force {
		if err = removeStorageContainers(ctrIDs, r.store); err != nil {
			return errors.Wrapf(err, "error removing containers %v for image %q", ctrIDs, image.ID())
		}
	}
	return nil
}

// Returns a list of storage containers associated with the given ImageReference
func storageContainers(imageID string, store storage.Store) ([]string, error) {
	ctrIDs := []string{}
	containers, err := store.Containers()
	if err != nil {
		return nil, err
	}
	for _, ctr := range containers {
		if ctr.ImageID == imageID {
			ctrIDs = append(ctrIDs, ctr.ID)
		}
	}
	return ctrIDs, nil
}

// Removes the containers passed in the array.
func removeStorageContainers(ctrIDs []string, store storage.Store) error {
	for _, ctrID := range ctrIDs {
		if err := store.DeleteContainer(ctrID); err != nil {
			return errors.Wrapf(err, "could not remove container %q", ctrID)
		}
	}
	return nil
}

// Build adds the runtime to the imagebuildah call
func (r *Runtime) Build(ctx context.Context, options imagebuildah.BuildOptions, dockerfiles ...string) error {
	return imagebuildah.BuildDockerfiles(ctx, r.store, options, dockerfiles...)
}
