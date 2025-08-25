package daemon

import (
	"context"
	"fmt"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/daemon/server/imagebackend"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// containerDiskUsage obtains information about container data disk usage
// and makes sure that only one calculation is performed at the same time.
func (daemon *Daemon) containerDiskUsage(ctx context.Context) (*backend.ContainerDiskUsage, error) {
	res, _, err := daemon.usageContainers.Do(ctx, struct{}{}, func(ctx context.Context) (*backend.ContainerDiskUsage, error) {
		// Retrieve container list
		containers, err := daemon.Containers(ctx, &container.ListOptions{
			Size: true,
			All:  true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve container list: %v", err)
		}

		// Remove image manifest descriptor from the result as it should not be included.
		// https://github.com/moby/moby/pull/49407#discussion_r1954396666
		for _, c := range containers {
			c.ImageManifestDescriptor = nil
		}

		isActive := func(ctr *container.Summary) bool {
			return ctr.State == container.StateRunning ||
				ctr.State == container.StatePaused ||
				ctr.State == container.StateRestarting
		}

		du := &backend.ContainerDiskUsage{Items: containers}
		for _, ctr := range du.Items {
			du.TotalSize += ctr.SizeRw
			if !isActive(ctr) {
				du.Reclaimable += ctr.SizeRw
			}
		}
		return du, nil
	})
	return res, err
}

// imageDiskUsage obtains information about image data disk usage from image service
// and makes sure that only one calculation is performed at the same time.
func (daemon *Daemon) imageDiskUsage(ctx context.Context) ([]*image.Summary, error) {
	imgs, _, err := daemon.usageImages.Do(ctx, struct{}{}, func(ctx context.Context) ([]*image.Summary, error) {
		// Get all top images with extra attributes
		imgs, err := daemon.imageService.Images(ctx, imagebackend.ListOptions{
			Filters:    filters.NewArgs(),
			SharedSize: true,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve image list")
		}
		return imgs, nil
	})

	return imgs, err
}

// localVolumesSize obtains information about volume disk usage from volumes service
// and makes sure that only one size calculation is performed at the same time.
func (daemon *Daemon) localVolumesSize(ctx context.Context) (*backend.VolumeDiskUsage, error) {
	volumes, _, err := daemon.usageVolumes.Do(ctx, struct{}{}, func(ctx context.Context) (*backend.VolumeDiskUsage, error) {
		volumes, err := daemon.volumes.LocalVolumesSize(ctx)
		if err != nil {
			return nil, err
		}

		du := &backend.VolumeDiskUsage{Items: volumes}
		for _, v := range du.Items {
			if v.UsageData.Size != -1 {
				if v.UsageData.RefCount == 0 {
					du.Reclaimable += v.UsageData.Size
				}
				du.TotalSize += v.UsageData.Size
			}
		}
		return du, nil
	})
	return volumes, err
}

// layerDiskUsage obtains information about layer disk usage from image service
// and makes sure that only one size calculation is performed at the same time.
func (daemon *Daemon) layerDiskUsage(ctx context.Context) (int64, error) {
	usage, _, err := daemon.usageLayer.Do(ctx, struct{}{}, func(ctx context.Context) (usage int64, err error) {
		return daemon.imageService.ImageDiskUsage(ctx)
	})
	return usage, err
}

// SystemDiskUsage returns information about the daemon data disk usage.
// Callers must not mutate contents of the returned fields.
func (daemon *Daemon) SystemDiskUsage(ctx context.Context, opts backend.DiskUsageOptions) (*backend.DiskUsage, error) {
	eg, ctx := errgroup.WithContext(ctx)

	du := &backend.DiskUsage{}
	if opts.Containers {
		eg.Go(func() (err error) {
			du.Containers, err = daemon.containerDiskUsage(ctx)
			return err
		})
	}

	var (
		layersSize int64
		images     []*image.Summary
	)
	if opts.Images {
		eg.Go(func() (err error) {
			images, err = daemon.imageDiskUsage(ctx)
			return err
		})
		eg.Go(func() (err error) {
			layersSize, err = daemon.layerDiskUsage(ctx)
			return err
		})
	}

	if opts.Volumes {
		eg.Go(func() (err error) {
			du.Volumes, err = daemon.localVolumesSize(ctx)
			return err
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	if opts.Images {
		reclaimable := layersSize
		for _, i := range images {
			if i.Containers != 0 {
				if i.Size == -1 || i.SharedSize == -1 {
					continue
				}
				reclaimable -= i.Size - i.SharedSize
			}
		}

		du.Images = &backend.ImageDiskUsage{
			TotalSize:   layersSize,
			Reclaimable: reclaimable,
			Items:       images,
		}
	}
	return du, nil
}
