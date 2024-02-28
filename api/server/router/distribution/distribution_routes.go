package distribution // import "github.com/docker/docker/api/server/router/distribution"

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/distribution/reference"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types/registry"
	distributionpkg "github.com/docker/docker/distribution"
	"github.com/docker/docker/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func (s *distributionRouter) getDistributionInfo(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")

	imgName := vars["name"]

	// TODO why is reference.ParseAnyReference() / reference.ParseNormalizedNamed() not using the reference.ErrTagInvalidFormat (and so on) errors?
	ref, err := reference.ParseAnyReference(imgName)
	if err != nil {
		return errdefs.InvalidParameter(err)
	}
	namedRef, ok := ref.(reference.Named)
	if !ok {
		if _, ok := ref.(reference.Digested); ok {
			// full image ID
			return errors.Errorf("no manifest found for full image ID")
		}
		return errdefs.InvalidParameter(errors.Errorf("unknown image reference format: %s", imgName))
	}

	// For a search it is not an error if no auth was given. Ignore invalid
	// AuthConfig to increase compatibility with the existing API.
	authConfig, _ := registry.DecodeAuthConfig(r.Header.Get(registry.AuthHeader))
	repos, err := s.backend.GetRepositories(ctx, namedRef, authConfig)
	if err != nil {
		return err
	}

	// Fetch the manifest; if a mirror is configured, try the mirror first,
	// but continue with upstream on failure.
	//
	// FIXME(thaJeztah): construct "repositories" on-demand;
	// GetRepositories() will attempt to connect to all endpoints (registries),
	// but we may only need the first one if it contains the manifest we're
	// looking for, or if the configured mirror is a pull-through mirror.
	//
	// Logic for this could be implemented similar to "distribution.Pull()",
	// which uses the "pullEndpoints" utility to iterate over the list
	// of endpoints;
	//
	// - https://github.com/moby/moby/blob/12c7411b6b7314bef130cd59f1c7384a7db06d0b/distribution/pull.go#L17-L31
	// - https://github.com/moby/moby/blob/12c7411b6b7314bef130cd59f1c7384a7db06d0b/distribution/pull.go#L76-L152
	var lastErr error
	for _, repo := range repos {
		distributionInspect, err := s.fetchManifest(ctx, repo, namedRef)
		if err != nil {
			lastErr = err
			continue
		}
		return httputils.WriteJSON(w, http.StatusOK, distributionInspect)
	}
	return lastErr
}

func (s *distributionRouter) fetchManifest(ctx context.Context, distrepo distribution.Repository, namedRef reference.Named) (registry.DistributionInspect, error) {
	var distributionInspect registry.DistributionInspect
	if canonicalRef, ok := namedRef.(reference.Canonical); !ok {
		namedRef = reference.TagNameOnly(namedRef)

		taggedRef, ok := namedRef.(reference.NamedTagged)
		if !ok {
			return registry.DistributionInspect{}, errdefs.InvalidParameter(errors.Errorf("image reference not tagged: %s", namedRef))
		}

		descriptor, err := distrepo.Tags(ctx).Get(ctx, taggedRef.Tag())
		if err != nil {
			return registry.DistributionInspect{}, err
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
		return registry.DistributionInspect{}, err
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
			return registry.DistributionInspect{}, errdefs.InvalidParameter(err)
		}
		return registry.DistributionInspect{}, err
	}

	mediaType, payload, err := mnfst.Payload()
	if err != nil {
		return registry.DistributionInspect{}, err
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
		blobStore := distrepo.Blobs(ctx)
		configJSON, err := blobStore.Get(ctx, mnfstObj.Config.Digest)
		var platform ocispec.Platform
		if err == nil {
			err := json.Unmarshal(configJSON, &platform)
			if err == nil && (platform.OS != "" || platform.Architecture != "") {
				distributionInspect.Platforms = append(distributionInspect.Platforms, platform)
			}
		}
	case *schema1.SignedManifest:
		if os.Getenv("DOCKER_ENABLE_DEPRECATED_PULL_SCHEMA_1_IMAGE") == "" {
			return registry.DistributionInspect{}, distributionpkg.DeprecatedSchema1ImageError(namedRef)
		}
		platform := ocispec.Platform{
			Architecture: mnfstObj.Architecture,
			OS:           "linux",
		}
		distributionInspect.Platforms = append(distributionInspect.Platforms, platform)
	}
	return distributionInspect, nil
}
