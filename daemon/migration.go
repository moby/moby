package daemon

import (
	"context"
	"encoding/json"

	"github.com/containerd/containerd/content"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/container"
	"github.com/docker/docker/image"
	"github.com/docker/docker/internal/multierror"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func migrateContainerOS(ctx context.Context,
	migration platformReader,
	ctr *container.Container,
) {
	deduced, err := deduceContainerPlatform(ctx, migration, ctr)
	if err != nil {
		log.G(ctx).WithFields(log.Fields{
			"container": ctr.ID,
			"error":     err,
		}).Warn("failed to deduce the container architecture")
		ctr.ImagePlatform.OS = ctr.OS //nolint:staticcheck // ignore SA1019
		return
	}

	ctr.ImagePlatform = deduced
}

type platformReader interface {
	ReadPlatformFromConfigByImageManifest(ctx context.Context, desc ocispec.Descriptor) (ocispec.Platform, error)
	ReadPlatformFromImage(ctx context.Context, id image.ID) (ocispec.Platform, error)
}

// deduceContainerPlatform tries to deduce `ctr`'s platform.
// If both `ctr.OS` and `ctr.ImageManifest` are empty, assume the image comes
// from a pre-OS times and use the host platform to match the behavior of
// [container.FromDisk].
// Otherwise:
// - `ctr.ImageManifest.Platform` is used, if it exists and is not empty.
// - The platform from the manifest's config is used, if `ctr.ImageManifest` exists
// and we're able to load its config from the content store.
// - The platform found by loading the image from the image service by ID (using
// `ctr.ImageID`) is used – this looks for the best *present* matching manifest in
// the store.
func deduceContainerPlatform(
	ctx context.Context,
	migration platformReader,
	ctr *container.Container,
) (ocispec.Platform, error) {
	if ctr.OS == "" && ctr.ImageManifest == nil { //nolint:staticcheck // ignore SA1019 because we are testing deprecated field migration
		return platforms.DefaultSpec(), nil
	}

	var errs []error
	isValidPlatform := func(p ocispec.Platform) bool {
		return p.OS != "" && p.Architecture != ""
	}

	if ctr.ImageManifest != nil {
		if ctr.ImageManifest.Platform != nil {
			return *ctr.ImageManifest.Platform, nil
		}

		if ctr.ImageManifest != nil {
			p, err := migration.ReadPlatformFromConfigByImageManifest(ctx, *ctr.ImageManifest)
			if err != nil {
				errs = append(errs, err)
			} else {
				if isValidPlatform(p) {
					return p, nil
				}
				errs = append(errs, errors.New("malformed image config obtained by ImageManifestDescriptor"))
			}
		}
	}

	if ctr.ImageID != "" {
		p, err := migration.ReadPlatformFromImage(ctx, ctr.ImageID)
		if err != nil {
			errs = append(errs, err)
		} else {
			if isValidPlatform(p) {
				return p, nil
			}
			errs = append(errs, errors.New("malformed image config obtained by image id"))
		}
	}

	return ocispec.Platform{}, errors.Wrap(multierror.Join(errs...), "cannot deduce the container platform")
}

type daemonPlatformReader struct {
	imageService ImageService
	content      content.Provider
}

func (r daemonPlatformReader) ReadPlatformFromConfigByImageManifest(
	ctx context.Context,
	desc ocispec.Descriptor,
) (ocispec.Platform, error) {
	b, err := content.ReadBlob(ctx, r.content, desc)
	if err != nil {
		return ocispec.Platform{}, err
	}

	var mfst ocispec.Manifest
	if err := json.Unmarshal(b, &mfst); err != nil {
		return ocispec.Platform{}, err
	}

	b, err = content.ReadBlob(ctx, r.content, mfst.Config)
	if err != nil {
		return ocispec.Platform{}, err
	}

	var plat ocispec.Platform
	if err := json.Unmarshal(b, &plat); err != nil {
		return ocispec.Platform{}, err
	}

	return plat, nil
}

func (r daemonPlatformReader) ReadPlatformFromImage(ctx context.Context, id image.ID) (ocispec.Platform, error) {
	img, err := r.imageService.GetImage(ctx, id.String(), backend.GetImageOpts{})
	if err != nil {
		return ocispec.Platform{}, err
	}

	return img.Platform(), nil
}
