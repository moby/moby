package daemon

import (
	"context"
	"fmt"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/internal/filters"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/daemon/server/imagebackend"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// containerDiskUsage obtains information about container data disk usage
// and makes sure that only one calculation is performed at the same time.
func (daemon *Daemon) containerDiskUsage(ctx context.Context, verbose bool) (*backend.ContainerDiskUsage, error) {
	res, _, err := daemon.usageContainers.Do(ctx, struct{}{}, func(ctx context.Context) (*backend.ContainerDiskUsage, error) {
		// Retrieve container list
		containers, err := daemon.Containers(ctx, &backend.ContainerListOptions{
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

		activeCount := int64(len(containers))

		du := &backend.ContainerDiskUsage{TotalCount: activeCount}
		for _, ctr := range containers {
			du.TotalSize += ctr.SizeRw
			if !isActive(ctr) {
				du.Reclaimable += ctr.SizeRw
				activeCount--
			}
		}

		du.ActiveCount = activeCount

		if verbose {
			du.Items = containers
		}

		return du, nil
	})
	return res, err
}

// imageDiskUsage obtains information about image data disk usage from image service
// and makes sure that only one calculation is performed at the same time.
func (daemon *Daemon) imageDiskUsage(ctx context.Context, verbose bool) (*backend.ImageDiskUsage, error) {
	du, _, err := daemon.usageImages.Do(ctx, struct{}{}, func(ctx context.Context) (*backend.ImageDiskUsage, error) {
		// Get all top images with extra attributes
		images, err := daemon.imageService.Images(ctx, imagebackend.ListOptions{
			Filters:    filters.NewArgs(),
			SharedSize: true,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve image list")
		}

		reclaimable, _, err := daemon.usageLayer.Do(ctx, struct{}{}, func(ctx context.Context) (int64, error) {
			return daemon.imageService.ImageDiskUsage(ctx)
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to calculate image disk usage")
		}

		activeCount := int64(len(images))

		du := &backend.ImageDiskUsage{TotalCount: activeCount, TotalSize: reclaimable}
		for _, i := range images {
			if i.Containers == 0 {
				activeCount--
				if i.Size == -1 || i.SharedSize == -1 {
					continue
				}
				reclaimable -= i.Size - i.SharedSize
			}
		}

		du.Reclaimable = reclaimable
		du.ActiveCount = activeCount

		if verbose {
			du.Items = images
		}

		return du, nil
	})

	return du, err
}

// localVolumesSize obtains information about volume disk usage from volumes service
// and makes sure that only one size calculation is performed at the same time.
func (daemon *Daemon) localVolumesSize(ctx context.Context, verbose bool) (*backend.VolumeDiskUsage, error) {
	volumes, _, err := daemon.usageVolumes.Do(ctx, struct{}{}, func(ctx context.Context) (*backend.VolumeDiskUsage, error) {
		volumes, err := daemon.volumes.LocalVolumesSize(ctx)
		if err != nil {
			return nil, err
		}

		activeCount := int64(len(volumes))

		du := &backend.VolumeDiskUsage{TotalCount: activeCount}
		for _, v := range volumes {
			if v.UsageData.Size != -1 {
				if v.UsageData.RefCount == 0 {
					du.Reclaimable += v.UsageData.Size
					activeCount--
				}
				du.TotalSize += v.UsageData.Size
			}
		}

		du.ActiveCount = activeCount

		if verbose {
			du.Items = volumes
		}

		return du, nil
	})
	return volumes, err
}

// SystemDiskUsage returns information about the daemon data disk usage.
// Callers must not mutate contents of the returned fields.
func (daemon *Daemon) SystemDiskUsage(ctx context.Context, opts backend.DiskUsageOptions) (*backend.DiskUsage, error) {
	eg, ctx := errgroup.WithContext(ctx)

	du := &backend.DiskUsage{}
	if opts.Containers {
		eg.Go(func() (err error) {
			du.Containers, err = daemon.containerDiskUsage(ctx, opts.Verbose)
			return err
		})
	}

	if opts.Images {
		eg.Go(func() (err error) {
			du.Images, err = daemon.imageDiskUsage(ctx, opts.Verbose)
			return err
		})
	}

	if opts.Volumes {
		eg.Go(func() (err error) {
			du.Volumes, err = daemon.localVolumesSize(ctx, opts.Verbose)
			return err
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return du, nil
}
