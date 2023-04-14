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
		logrus.WithField("image", name).WithField("filteredOut", filteredOut).Debug("filtering image")
		if filteredOut {
			delete(imagesToPrune, name)
		}
	}

	containers := i.containers.List()

	var errs error
	// Exclude images used by existing containers
	for _, ctr := range containers {
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

	logrus.WithField("images", imagesToPrune).Debug("pruning")

	for _, img := range imagesToPrune {
		blobs := []ocispec.Descriptor{}

		err = containerdimages.Walk(ctx, presentChildrenHandler(store, containerdimages.HandlerFunc(
			func(_ context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
				blobs = append(blobs, desc)
				return nil, nil
			})),
			img.Target)

		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		err = is.Delete(ctx, img.Name, containerdimages.SynchronousDelete())
		if err != nil && !cerrdefs.IsNotFound(err) {
			errs = multierror.Append(errs, err)
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
