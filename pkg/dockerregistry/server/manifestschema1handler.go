package server

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/reference"
	"github.com/docker/libtrust"
)

func unmarshalManifestSchema1(content []byte, signatures [][]byte) (distribution.Manifest, error) {
	// prefer signatures from the manifest
	if _, err := libtrust.ParsePrettySignature(content, "signatures"); err == nil {
		sm := schema1.SignedManifest{Canonical: content}
		if err = json.Unmarshal(content, &sm); err == nil {
			return &sm, nil
		}
	}

	jsig, err := libtrust.NewJSONSignature(content, signatures...)
	if err != nil {
		return nil, err
	}

	// Extract the pretty JWS
	content, err = jsig.PrettySignature("signatures")
	if err != nil {
		return nil, err
	}

	var sm schema1.SignedManifest
	if err = json.Unmarshal(content, &sm); err != nil {
		return nil, err
	}
	return &sm, nil
}

type manifestSchema1Handler struct {
	repo     *repository
	manifest *schema1.SignedManifest
}

var _ ManifestHandler = &manifestSchema1Handler{}

func (h *manifestSchema1Handler) Config(ctx context.Context) ([]byte, error) {
	return nil, nil
}

func (h *manifestSchema1Handler) Digest() (digest.Digest, error) {
	return digest.FromBytes(h.manifest.Canonical), nil
}

func (h *manifestSchema1Handler) Manifest() distribution.Manifest {
	return h.manifest
}

func (h *manifestSchema1Handler) Payload() (mediaType string, payload []byte, canonical []byte, err error) {
	mt, payload, err := h.manifest.Payload()
	return mt, payload, h.manifest.Canonical, err
}

func (h *manifestSchema1Handler) Verify(ctx context.Context, skipDependencyVerification bool) error {
	var errs distribution.ErrManifestVerification

	// we want to verify that referenced blobs exist locally or accessible over
	// pullthroughBlobStore. The base image of this image can be remote repository
	// and since we use pullthroughBlobStore all the layer existence checks will be
	// successful. This means that the docker client will not attempt to send them
	// to us as it will assume that the registry has them.
	repo := h.repo

	if len(path.Join(h.repo.config.registryAddr, h.manifest.Name)) > reference.NameTotalLengthMax {
		errs = append(errs,
			distribution.ErrManifestNameInvalid{
				Name:   h.manifest.Name,
				Reason: fmt.Errorf("<registry-host>/<manifest-name> must not be more than %d characters", reference.NameTotalLengthMax),
			})
	}

	if !reference.NameRegexp.MatchString(h.manifest.Name) {
		errs = append(errs,
			distribution.ErrManifestNameInvalid{
				Name:   h.manifest.Name,
				Reason: fmt.Errorf("invalid manifest name format"),
			})
	}

	manifestReferences := h.manifest.References()

	if len(h.manifest.History) != len(manifestReferences) {
		errs = append(errs, fmt.Errorf("mismatched history and manifests references %d != %d",
			len(h.manifest.History), len(manifestReferences)))
	}

	if _, err := schema1.Verify(h.manifest); err != nil {
		switch err {
		case libtrust.ErrMissingSignatureKey, libtrust.ErrInvalidJSONContent, libtrust.ErrMissingSignatureKey:
			errs = append(errs, distribution.ErrManifestUnverified{})
		default:
			if err.Error() == "invalid signature" {
				errs = append(errs, distribution.ErrManifestUnverified{})
			} else {
				errs = append(errs, err)
			}
		}
	}

	if skipDependencyVerification {
		if len(errs) > 0 {
			return errs
		}
		return nil
	}

	var realSizes []int64

	for _, fsLayer := range manifestReferences {
		desc, err := repo.Blobs(ctx).Stat(ctx, fsLayer.Digest)
		if err != nil {
			if err != distribution.ErrBlobUnknown {
				errs = append(errs, err)
				continue
			}

			// On error here, we always append unknown blob errors.
			errs = append(errs, distribution.ErrManifestBlobUnknown{Digest: fsLayer.Digest})
			continue
		}
		realSizes = append(realSizes, desc.Size)
	}

	if len(errs) > 0 {
		return errs
	}

	v1Compatibility := struct {
		Size int64 `json:"size,omitempty"`
	}{}

	for i, obj := range h.manifest.History {
		v1Compatibility.Size = 0
		if err := json.Unmarshal([]byte(obj.V1Compatibility), &v1Compatibility); err != nil {
			errs = append(errs, err)
			continue
		}
		if !isEmptyDigest(manifestReferences[i].Digest) && realSizes[i] != v1Compatibility.Size {
			errs = append(errs, ErrManifestBlobBadSize{
				Digest:       manifestReferences[i].Digest,
				ActualSize:   realSizes[i],
				ManifestSize: v1Compatibility.Size,
			})
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}
