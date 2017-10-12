package image

import (
	"context"
	"net/url"

	"github.com/docker/distribution/digest"

	"github.com/openshift/origin/pkg/image/importer"
)

// getImageManifestByIDFromRegistry retrieves the image manifest from the registry using the basic
// authentication using the image ID.
func getImageManifestByIDFromRegistry(registry *url.URL, repositoryName, imageID, username, password string, insecure bool) ([]byte, error) {
	ctx := context.Background()

	credentials := importer.NewBasicCredentials()
	credentials.Add(registry, username, password)

	repo, err := importer.NewContext(importer.NewTransportRetriever()).
		WithCredentials(credentials).
		Repository(ctx, registry, repositoryName, insecure)
	if err != nil {
		return nil, err
	}

	manifests, err := repo.Manifests(ctx, nil)
	if err != nil {
		return nil, err
	}

	manifest, err := manifests.Get(ctx, digest.Digest(imageID))
	if err != nil {
		return nil, err
	}
	_, manifestPayload, err := manifest.Payload()
	if err != nil {
		return nil, err
	}

	return manifestPayload, nil
}
