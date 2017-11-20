package server

import (
	"strings"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/api/errcode"
	disterrors "github.com/docker/distribution/registry/api/v2"
	quotautil "github.com/openshift/origin/pkg/quota/util"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"

	"github.com/openshift/origin/pkg/dockerregistry/server/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	"github.com/openshift/origin/pkg/image/importer"
)

func getNamespaceName(resourceName string) (string, string, error) {
	repoParts := strings.SplitN(resourceName, "/", 2)
	if len(repoParts) != 2 {
		return "", "", ErrNamespaceRequired
	}
	ns := repoParts[0]
	if len(ns) == 0 {
		return "", "", ErrNamespaceRequired
	}
	name := repoParts[1]
	if len(name) == 0 {
		return "", "", ErrNamespaceRequired
	}
	return ns, name, nil
}

// effectiveCreateOptions find out what the blob creation options are going to do by dry-running them.
func effectiveCreateOptions(options []distribution.BlobCreateOption) (*distribution.CreateOptions, error) {
	opts := &distribution.CreateOptions{}
	for _, createOptions := range options {
		err := createOptions.Apply(opts)
		if err != nil {
			return nil, err
		}
	}
	return opts, nil
}

func isImageManaged(image *imageapiv1.Image) bool {
	managed, ok := image.ObjectMeta.Annotations[imageapi.ManagedByOpenShiftAnnotation]
	return ok && managed == "true"
}

// wrapKStatusErrorOnGetImage transforms the given kubernetes status error into a distribution one. Upstream
// handler do not allow us to propagate custom error messages except for ErrManifetUnknownRevision. All the
// other errors will result in an internal server error with details made out of returned error.
func wrapKStatusErrorOnGetImage(repoName string, dgst digest.Digest, err error) error {
	switch {
	case kerrors.IsNotFound(err):
		// This is the only error type we can propagate unchanged to the client.
		return distribution.ErrManifestUnknownRevision{
			Name:     repoName,
			Revision: dgst,
		}
	case err != nil:
		// We don't turn this error to distribution error on purpose: Upstream manifest handler wraps any
		// error but distribution.ErrManifestUnknownRevision with errcode.ErrorCodeUnknown. If we wrap the
		// original error with distribution.ErrorCodeUnknown, the "unknown error" will appear twice in the
		// resulting error message.
		return err
	}

	return nil
}

// getImportContext loads secrets for given repository and returns a context for getting distribution clients
// to remote repositories.
func getImportContext(
	ctx context.Context,
	osClient client.ImageStreamSecretsNamespacer,
	namespace, name string,
) importer.RepositoryRetriever {
	secrets, err := osClient.ImageStreamSecrets(namespace).Secrets(name, metav1.ListOptions{})
	if err != nil {
		context.GetLogger(ctx).Errorf("error getting secrets for repository %s/%s: %v", namespace, name, err)
		secrets = &kapiv1.SecretList{}
	}
	credentials := importer.NewCredentialsForSecrets(secrets.Items)
	return importer.NewContext(secureTransport, insecureTransport).WithCredentials(credentials)
}

// cachedImageStreamGetter wraps a master API client for getting image streams with a cache.
type cachedImageStreamGetter struct {
	ctx               context.Context
	namespace         string
	name              string
	isNamespacer      client.ImageStreamsNamespacer
	cachedImageStream *imageapiv1.ImageStream
}

func (g *cachedImageStreamGetter) get() (*imageapiv1.ImageStream, error) {
	if g.cachedImageStream != nil {
		context.GetLogger(g.ctx).Debugf("(*cachedImageStreamGetter).getImageStream: returning cached copy")
		return g.cachedImageStream, nil
	}
	is, err := g.isNamespacer.ImageStreams(g.namespace).Get(g.name, metav1.GetOptions{})
	if err != nil {
		context.GetLogger(g.ctx).Errorf("failed to get image stream: %v", err)
		switch {
		case kerrors.IsNotFound(err):
			return nil, disterrors.ErrorCodeNameUnknown.WithDetail(err)
		case kerrors.IsForbidden(err), kerrors.IsUnauthorized(err), quotautil.IsErrorQuotaExceeded(err):
			return nil, errcode.ErrorCodeDenied.WithDetail(err)
		default:
			return nil, errcode.ErrorCodeUnknown.WithDetail(err)
		}
	}

	context.GetLogger(g.ctx).Debugf("(*cachedImageStreamGetter).getImageStream: got image stream %s/%s", is.Namespace, is.Name)
	g.cachedImageStream = is
	return is, nil
}

func (g *cachedImageStreamGetter) cacheImageStream(is *imageapiv1.ImageStream) {
	context.GetLogger(g.ctx).Debugf("(*cachedImageStreamGetter).cacheImageStream: got image stream %s/%s", is.Namespace, is.Name)
	g.cachedImageStream = is
}
