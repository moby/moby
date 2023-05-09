package distribution // import "github.com/docker/docker/api/server/router/distribution"

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func (s *distributionRouter) getDistributionInfo(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")

	image := vars["name"]

	// TODO why is reference.ParseAnyReference() / reference.ParseNormalizedNamed() not using the reference.ErrTagInvalidFormat (and so on) errors?
	ref, err := reference.ParseAnyReference(image)
	if err != nil {
		return errdefs.InvalidParameter(err)
	}
	namedRef, ok := ref.(reference.Named)
	if !ok {
		if _, ok := ref.(reference.Digested); ok {
			// full image ID
			return errors.Errorf("no manifest found for full image ID")
		}
		return errdefs.InvalidParameter(errors.Errorf("unknown image reference format: %s", image))
	}

	// For a search it is not an error if no auth was given. Ignore invalid
	// AuthConfig to increase compatibility with the existing API.
	authConfig, _ := registry.DecodeAuthConfig(r.Header.Get(registry.AuthHeader))
	distrepo, err := s.backend.GetRepository(ctx, namedRef, authConfig)
	if err != nil {
		return err
	}
	blobsrvc := distrepo.Blobs(ctx)

	var distributionInspect registry.DistributionInspect
	if canonicalRef, ok := namedRef.(reference.Canonical); !ok {
		namedRef = reference.TagNameOnly(namedRef)

		taggedRef, ok := namedRef.(reference.NamedTagged)
		if !ok {
			return errdefs.InvalidParameter(errors.Errorf("image reference not tagged: %s", image))
		}

		descriptor, err := distrepo.Tags(ctx).Get(ctx, taggedRef.Tag())
		if err != nil {
			return err
		}
		distributionInspect.Descriptor = ocispec.Descriptor{
			MediaType: descriptor.MediaType,
			Digest:    descriptor.Digest,
			Size:      descriptor.Size,
		}
	} else {
		// TODO(nishanttotla): Once manifests can be looked up as a blob, the
		// descriptor should be set using blobsrvc.Stat(ctx, canonicalRef.Digest())
		// instead of having to manually fill in the fields
		distributionInspect.Descriptor.Digest = canonicalRef.Digest()
	}

	// we have a digest, so we can retrieve the manifest
	mnfstsrvc, err := distrepo.Manifests(ctx)
	if err != nil {
		return err
	}
	mnfst, err := mnfstsrvc.Get(ctx, distributionInspect.Descriptor.Digest)
	if err != nil {
		switch err {
		case reference.ErrReferenceInvalidFormat,
			reference.ErrTagInvalidFormat,
			reference.ErrDigestInvalidFormat,
			reference.ErrNameContainsUppercase,
			reference.ErrNameEmpty,
			reference.ErrNameTooLong,
			reference.ErrNameNotCanonical:
			return errdefs.InvalidParameter(err)
		}
		return err
	}

	mediaType, payload, err := mnfst.Payload()
	if err != nil {
		return err
	}
	// update MediaType because registry might return something incorrect
	distributionInspect.Descriptor.MediaType = mediaType
	if distributionInspect.Descriptor.Size == 0 {
		distributionInspect.Descriptor.Size = int64(len(payload))
	}

	// retrieve platform information depending on the type of manifest
	switch mnfstObj := mnfst.(type) {
	case *manifestlist.DeserializedManifestList:
		for _, m := range mnfstObj.Manifests {
			distributionInspect.Platforms = append(distributionInspect.Platforms, ocispec.Platform{
				Architecture: m.Platform.Architecture,
				OS:           m.Platform.OS,
				OSVersion:    m.Platform.OSVersion,
				OSFeatures:   m.Platform.OSFeatures,
				Variant:      m.Platform.Variant,
			})
		}
	case *schema2.DeserializedManifest:
		configJSON, err := blobsrvc.Get(ctx, mnfstObj.Config.Digest)
		var platform ocispec.Platform
		if err == nil {
			err := json.Unmarshal(configJSON, &platform)
			if err == nil && (platform.OS != "" || platform.Architecture != "") {
				distributionInspect.Platforms = append(distributionInspect.Platforms, platform)
			}
		}
	case *schema1.SignedManifest:
		platform := ocispec.Platform{
			Architecture: mnfstObj.Architecture,
			OS:           "linux",
		}
		distributionInspect.Platforms = append(distributionInspect.Platforms, platform)
	}

	return httputils.WriteJSON(w, http.StatusOK, distributionInspect)
}
