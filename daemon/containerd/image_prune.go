package containerd

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ImagesPrune removes unused images
// TODO: handle pruneFilters
func (i *ImageService) ImagesPrune(ctx context.Context, pruneFilters filters.Args) (*types.ImagesPruneReport, error) {
	is := i.client.ImageService()
	store := i.client.ContentStore()

	images, err := is.List(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to list images")
	}

	platform := platforms.DefaultStrict()
	report := types.ImagesPruneReport{}
	toDelete := map[digest.Digest]uint64{}
	errs := []error{}

	for _, img := range images {
		err := getContentDigestsWithSizes(ctx, img, store, platform, toDelete)
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}

	for digest, size := range toDelete {
		report.SpaceReclaimed += size
		report.ImagesDeleted = append(report.ImagesDeleted,
			types.ImageDeleteResponseItem{
				Deleted: digest.String(),
			},
		)
	}

	for _, img := range images {
		err = is.Delete(ctx, img.Name, containerdimages.SynchronousDelete())
		if err != nil && !cerrdefs.IsNotFound(err) {
			errs = append(errs, err)
			continue
		}

		report.ImagesDeleted = append(report.ImagesDeleted,
			types.ImageDeleteResponseItem{
				Untagged: img.Name,
			},
		)
	}

	if len(errs) > 0 {
		return &report, combineErrors(errs)
	}

	return &report, nil
}

func getContentDigestsWithSizes(ctx context.Context, img containerdimages.Image, store content.Store, platform platforms.MatchComparer, toDelete map[digest.Digest]uint64) error {
	return containerdimages.Walk(ctx, containerdimages.Handlers(containerdimages.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if desc.Size < 0 {
			return nil, fmt.Errorf("invalid size %v in %v (%v)", desc.Size, desc.Digest, desc.MediaType)
		}
		toDelete[desc.Digest] = uint64(desc.Size)
		return nil, nil
	}), containerdimages.LimitManifests(containerdimages.FilterPlatforms(containerdimages.ChildrenHandler(store), platform), platform, 1)), img.Target)
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
