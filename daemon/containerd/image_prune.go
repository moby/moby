package containerd

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/containerd/containerd"
	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/errdefs"
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
func (i *ImageService) ImagesPrune(ctx context.Context, pruneFilters filters.Args) (*types.ImagesPruneReport, error) {
	if !atomic.CompareAndSwapInt32(&i.pruneRunning, 0, 1) {
		return nil, errPruneRunning
	}
	defer atomic.StoreInt32(&i.pruneRunning, 0)

	err := pruneFilters.Validate(imagesAcceptedFilters)
	if err != nil {
		return nil, err
	}

	danglingOnly := true
	if pruneFilters.Contains("dangling") {
		if pruneFilters.ExactMatch("dangling", "false") || pruneFilters.ExactMatch("dangling", "0") {
			danglingOnly = false
		} else if !pruneFilters.ExactMatch("dangling", "true") && !pruneFilters.ExactMatch("dangling", "1") {
			return nil, fmt.Errorf("invalid dangling filter value: %q", pruneFilters.Get("dangling"))
		}
	}

	filterFunc, err := i.setupFilters(ctx, pruneFilters)
	if err != nil {
		return nil, err
	}

	if !danglingOnly {
		r, errs := i.pruneUnused(ctx, filterFunc)
		if len(errs) > 0 {
			return &r, combineErrors(errs)
		}

		return &r, nil
	} else {
		// In containerd dangling content is automatically deleted by the GC.
		// So running prune with dangling=true is mostly a no-op, unless there
		// was some action performed which didn't invoke the GC immediately.
		report := types.ImagesPruneReport{}
		// Trigger GC.
		ls := i.client.LeasesService()
		lease, err := ls.Create(ctx)
		if err != nil {
			return &report, err
		}
		err = ls.Delete(ctx, lease, leases.SynchronousDelete)
		return &report, err
	}
}

func (i *ImageService) pruneUnused(ctx context.Context, filterFunc imageFilterFunc) (types.ImagesPruneReport, []error) {
	report := types.ImagesPruneReport{}
	is := i.client.ImageService()
	store := i.client.ContentStore()

	allImages, err := i.client.ListImages(ctx)
	if err != nil {
		return report, []error{err}
	}

	imagesToPrune := map[string]containerd.Image{}
	for _, img := range allImages {
		imagesToPrune[img.Name()] = img
	}

	errs := []error{}

	// Apply filters
	for name, img := range imagesToPrune {
		filteredOut := !filterFunc(img)
		logrus.WithField("image", name).WithField("filteredOut", filteredOut).Debug("filtering image")
		if filteredOut {
			delete(imagesToPrune, name)
		}
	}

	cs := i.client.ContainerService()
	containers, err := cs.List(ctx)
	if err != nil {
		return report, []error{err}
	}

	// Exclude images that are used by existing containers from prune
	for _, container := range containers {
		logrus.WithField("container", container.ID).WithField("image", container.Image).Debug("filtering container's image")
		if container.Image != "" {
			delete(imagesToPrune, container.Image)
		}
	}

	logrus.WithField("images", imagesToPrune).Debug("pruning")

	for _, img := range imagesToPrune {
		blobs := []ocispec.Descriptor{}

		err = containerdimages.Walk(ctx, containerdimages.Handlers(
			i.presentChildrenHandler(),
			containerdimages.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) (subdescs []ocispec.Descriptor, err error) {
				blobs = append(blobs, desc)
				return nil, nil
			}),
		), img.Target())

		if err != nil {
			errs = append(errs, err)
			continue
		}
		err = is.Delete(ctx, img.Name(), containerdimages.SynchronousDelete())
		if err != nil && !cerrdefs.IsNotFound(err) {
			errs = append(errs, err)
			continue
		}

		report.ImagesDeleted = append(report.ImagesDeleted,
			types.ImageDeleteResponseItem{
				Untagged: img.Name(),
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
	return report, errs
}

func combineErrors(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}

	errString := ""
	for _, err := range errs {
		if errString != "" {
			errString += "\n"
		}
		errString += err.Error()
	}

	return errors.Errorf("Multiple errors encountered:\n%s", errString)
}
