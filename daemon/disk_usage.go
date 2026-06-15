package daemon

import (
	"context"
	"fmt"

	"github.com/containerd/log"
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
	res, _, err := daemon.usageContainers.Do(ctx, verbose, func(ctx context.Context) (*backend.ContainerDiskUsage, error) {
		// Retrieve container list
		containers, err := daemon.Containers(ctx, &backend.ContainerListOptions{
			Size: true,
			All:  true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve container list: %v", err)
		}

		du := &backend.ContainerDiskUsage{
			ActiveCount: int64(len(containers)),
			TotalCount:  int64(len(containers)),
		}
		for i := range containers {
			du.TotalSize += containers[i].SizeRw
			switch containers[i].State {
			case container.StateRunning, container.StatePaused, container.StateRestarting:
				// active
			default:
				du.Reclaimable += containers[i].SizeRw
				du.ActiveCount--
			}

			// Remove image manifest descriptor from the result as it should not be included.
			// https://github.com/moby/moby/pull/49407#discussion_r1954396666
			containers[i].ImageManifestDescriptor = nil
		}

		if verbose {
			du.Items = containers
			for _, c := range containers {
				log.G(ctx).WithFields(log.Fields{
					"container":   c.ID,
					"image":       c.Image,
					"image_id":    c.ImageID,
					"names":       c.Names,
					"rw_size":     c.SizeRw,
					"rootfs_size": c.SizeRootFs,
					"state":       c.State,
				}).Debug("container disk usage item")
			}
		}

		log.G(ctx).WithFields(log.Fields{
			"active_count": du.ActiveCount,
			"total_count":  du.TotalCount,
			"total_size":   du.TotalSize,
			"reclaimable":  du.Reclaimable,
			"verbose":      verbose,
		}).Debug("container disk usage summary")

		return du, nil
	})
	return res, err
}

// imageDiskUsage obtains information about image data disk usage from image service
// and makes sure that only one calculation is performed at the same time.
func (daemon *Daemon) imageDiskUsage(ctx context.Context, verbose bool) (*backend.ImageDiskUsage, error) {
	du, _, err := daemon.usageImages.Do(ctx, verbose, func(ctx context.Context) (*backend.ImageDiskUsage, error) {
		// Get all top images with extra attributes
		images, err := daemon.imageService.Images(ctx, imagebackend.ListOptions{
			Filters:    filters.NewArgs(),
			SharedSize: true,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve image list")
		}

		totalSize, _, err := daemon.usageLayer.Do(ctx, struct{}{}, func(ctx context.Context) (int64, error) {
			return daemon.imageService.ImageDiskUsage(ctx)
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to calculate image disk usage")
		}

		du := &backend.ImageDiskUsage{
			TotalCount: int64(len(images)),
			TotalSize:  totalSize,
		}

		for _, img := range images {
			switch {
			case img.Containers < 0:
				// No container-count information available; skip (assume it's in use).
			case img.Containers > 0:
				du.ActiveCount++
			case img.Containers == 0 && img.Size != -1 && img.SharedSize != -1:
				reclaimable := img.Size - img.SharedSize
				du.Reclaimable += reclaimable
			}
		}

		if verbose {
			du.Items = images
			for _, img := range images {
				log.G(ctx).WithFields(log.Fields{
					"image":       img.ID,
					"repo_tags":   img.RepoTags,
					"size":        img.Size,
					"shared_size": img.SharedSize,
					"containers":  img.Containers,
				}).Debug("image disk usage item")
			}
		}

		log.G(ctx).WithFields(log.Fields{
			"active_count": du.ActiveCount,
			"total_count":  du.TotalCount,
			"total_size":   du.TotalSize,
			"reclaimable":  du.Reclaimable,
			"verbose":      verbose,
		}).Debug("image disk usage summary")

		return du, nil
	})

	return du, err
}

// localVolumesSize obtains information about volume disk usage from volumes service
// and makes sure that only one size calculation is performed at the same time.
func (daemon *Daemon) localVolumesSize(ctx context.Context, verbose bool) (*backend.VolumeDiskUsage, error) {
	volumes, _, err := daemon.usageVolumes.Do(ctx, verbose, func(ctx context.Context) (*backend.VolumeDiskUsage, error) {
		volumes, err := daemon.volumes.LocalVolumesSize(ctx)
		if err != nil {
			return nil, err
		}

		du := &backend.VolumeDiskUsage{
			ActiveCount: int64(len(volumes)),
			TotalCount:  int64(len(volumes)),
		}
		for _, v := range volumes {
			if v.UsageData.Size != -1 {
				du.TotalSize += v.UsageData.Size
				if v.UsageData.RefCount == 0 {
					du.Reclaimable += v.UsageData.Size
					du.ActiveCount--
				}
			}
		}

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

	fields := log.Fields{
		"containers_requested": opts.Containers,
		"images_requested":     opts.Images,
		"volumes_requested":    opts.Volumes,
		"verbose":              opts.Verbose,
	}
	if du.Containers != nil {
		fields["container_active_count"] = du.Containers.ActiveCount
		fields["container_total_count"] = du.Containers.TotalCount
		fields["container_total_size"] = du.Containers.TotalSize
		fields["container_reclaimable"] = du.Containers.Reclaimable
	}
	if du.Images != nil {
		fields["image_active_count"] = du.Images.ActiveCount
		fields["image_total_count"] = du.Images.TotalCount
		fields["image_total_size"] = du.Images.TotalSize
		fields["image_reclaimable"] = du.Images.Reclaimable
	}
	if du.Volumes != nil {
		fields["volume_active_count"] = du.Volumes.ActiveCount
		fields["volume_total_count"] = du.Volumes.TotalCount
		fields["volume_total_size"] = du.Volumes.TotalSize
		fields["volume_reclaimable"] = du.Volumes.Reclaimable
	}
	log.G(ctx).WithFields(fields).Debug("system disk usage backend result")

	return du, nil
}
