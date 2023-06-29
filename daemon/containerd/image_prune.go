package containerd

import (
	"context"

	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/errdefs"
	"github.com/hashicorp/go-multierror"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var imagesAcceptedFilters = map[string]bool{
	"dangling": true,
	"label":    true,
	"label!":   true,
	"until":    true,
}

// errPruneRunning is returned when a prune request is received while
// one is in progress
var errPruneRunning = errdefs.Conflict(errors.New("a prune operation is already running"))

// ImagesPrune removes unused images
func (i *ImageService) ImagesPrune(ctx context.Context, fltrs filters.Args) (*types.ImagesPruneReport, error) {
	if !i.pruneRunning.CompareAndSwap(false, true) {
		return nil, errPruneRunning
	}
	defer i.pruneRunning.Store(false)

	err := fltrs.Validate(imagesAcceptedFilters)
	if err != nil {
		return nil, err
	}

	danglingOnly, err := fltrs.GetBoolOrDefault("dangling", false)
	if err != nil {
		return nil, err
	}
	// dangling=false will filter out dangling images like in image list.
	// Remove it, because in this context dangling=false means that we're
	// pruning NOT ONLY dangling (`docker image prune -a`) instead of NOT DANGLING.
	// This will be handled by the danglingOnly parameter of pruneUnused.
	for _, v := range fltrs.Get("dangling") {
		fltrs.Del("dangling", v)
	}

	_, filterFunc, err := i.setupFilters(ctx, fltrs)
	if err != nil {
		return nil, err
	}

	return i.pruneUnused(ctx, filterFunc, danglingOnly)
}

func (i *ImageService) pruneUnused(ctx context.Context, filterFunc imageFilterFunc, danglingOnly bool) (*types.ImagesPruneReport, error) {
	report := types.ImagesPruneReport{}
	is := i.client.ImageService()
	store := i.client.ContentStore()

	allImages, err := i.client.ImageService().List(ctx)
	if err != nil {
		return nil, err
	}

	imagesToPrune := map[string]containerdimages.Image{}
	for _, img := range allImages {
		if !danglingOnly || isDanglingImage(img) {
			imagesToPrune[img.Name] = img
		}
	}

	// Apply filters
	for name, img := range imagesToPrune {
		filteredOut := !filterFunc(img)
		logrus.WithFields(logrus.Fields{
			"image":       name,
			"filteredOut": filteredOut,
		}).Debug("filtering image")

		if filteredOut {
			delete(imagesToPrune, name)
		}
	}

	containers := i.containers.List()

	var errs error
	// Exclude images used by existing containers
	for _, ctr := range containers {
		// If the original image was deleted, make sure we don't delete the dangling image
		delete(imagesToPrune, danglingImageName(ctr.ImageID.Digest()))

		// Config.Image is the image reference passed by user.
		// For example: container created by `docker run alpine` will have Image="alpine"
		// Warning: This doesn't handle truncated ids:
		//          `docker run 124c7d2` will have Image="124c7d270790"
		ref, err := reference.ParseNormalizedNamed(ctr.Config.Image)
		logrus.WithFields(logrus.Fields{
			"ctr":          ctr.ID,
			"image":        ref,
			"nameParseErr": err,
		}).Debug("filtering container's image")

		if err == nil {
			name := reference.TagNameOnly(ref)
			delete(imagesToPrune, name.String())
		}
	}

	possiblyDeletedConfigs := map[digest.Digest]struct{}{}

	// Workaround for https://github.com/moby/buildkit/issues/3797
	defer func() {
		if err := i.unleaseSnapshotsFromDeletedConfigs(context.Background(), possiblyDeletedConfigs); err != nil {
			errs = multierror.Append(errs, err)
		}
	}()

	for _, img := range imagesToPrune {
		logrus.WithField("image", img).Debug("pruning image")

		blobs := []ocispec.Descriptor{}

		err := i.walkPresentChildren(ctx, img.Target, func(_ context.Context, desc ocispec.Descriptor) error {
			blobs = append(blobs, desc)
			if containerdimages.IsConfigType(desc.MediaType) {
				possiblyDeletedConfigs[desc.Digest] = struct{}{}
			}
			return nil
		})
		if err != nil {
			errs = multierror.Append(errs, err)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return &report, errs
			}
			continue
		}
		err = is.Delete(ctx, img.Name, containerdimages.SynchronousDelete())
		if err != nil && !cerrdefs.IsNotFound(err) {
			errs = multierror.Append(errs, err)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return &report, errs
			}
			continue
		}

		report.ImagesDeleted = append(report.ImagesDeleted,
			types.ImageDeleteResponseItem{
				Untagged: img.Name,
			},
		)

		// Check which blobs have been deleted and sum their sizes
		for _, blob := range blobs {
			_, err := store.ReaderAt(ctx, blob)

			if cerrdefs.IsNotFound(err) {
				report.ImagesDeleted = append(report.ImagesDeleted,
					types.ImageDeleteResponseItem{
						Deleted: blob.Digest.String(),
					},
				)
				report.SpaceReclaimed += uint64(blob.Size)
			}
		}
	}

	return &report, errs
}

// unleaseSnapshotsFromDeletedConfigs removes gc.ref.snapshot content label from configs that are not
// referenced by any of the existing images.
// This is a temporary solution to the rootfs snapshot not being deleted when there's a buildkit history
// item referencing an image config.
func (i *ImageService) unleaseSnapshotsFromDeletedConfigs(ctx context.Context, possiblyDeletedConfigs map[digest.Digest]struct{}) error {
	is := i.client.ImageService()
	store := i.client.ContentStore()

	all, err := is.List(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list images during snapshot lease removal")
	}

	var errs error
	for _, img := range all {
		err := i.walkPresentChildren(ctx, img.Target, func(_ context.Context, desc ocispec.Descriptor) error {
			if containerdimages.IsConfigType(desc.MediaType) {
				delete(possiblyDeletedConfigs, desc.Digest)
			}
			return nil
		})
		if err != nil {
			errs = multierror.Append(errs, err)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return errs
			}
			continue
		}
	}

	// At this point, all configs that are used by any image has been removed from the slice
	for cfgDigest := range possiblyDeletedConfigs {
		info, err := store.Info(ctx, cfgDigest)
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				logrus.WithField("config", cfgDigest).Debug("config already gone")
			} else {
				errs = multierror.Append(errs, err)
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return errs
				}
			}
			continue
		}

		label := "containerd.io/gc.ref.snapshot." + i.StorageDriver()

		delete(info.Labels, label)
		_, err = store.Update(ctx, info, "labels."+label)
		if err != nil {
			errs = multierror.Append(errs, errors.Wrapf(err, "failed to remove gc.ref.snapshot label from %s", cfgDigest))
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return errs
			}
		}
	}

	return errs
}
