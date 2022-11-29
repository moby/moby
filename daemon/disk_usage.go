package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/server/router/system"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// containerDiskUsage obtains information about container data disk usage
// and makes sure that only one calculation is performed at the same time.
func (daemon *Daemon) containerDiskUsage(ctx context.Context) ([]*types.Container, error) {
	res, _, err := daemon.usageContainers.Do(ctx, struct{}{}, func(ctx context.Context) ([]*types.Container, error) {
		// Retrieve container list
		//
		// FIXME(thaJeztah): unlike "master", the 22.06 / 23.0.0 branch does not yet pass through the context to daemon.Containers
		containers, err := daemon.Containers(&types.ContainerListOptions{
			Size: true,
			All:  true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve container list: %v", err)
		}
		return containers, nil
	})
	return res, err
}

// imageDiskUsage obtains information about image data disk usage from image service
// and makes sure that only one calculation is performed at the same time.
func (daemon *Daemon) imageDiskUsage(ctx context.Context) ([]*types.ImageSummary, error) {
	imgs, _, err := daemon.usageImages.Do(ctx, struct{}{}, func(ctx context.Context) ([]*types.ImageSummary, error) {
		// Get all top images with extra attributes
		imgs, err := daemon.imageService.Images(ctx, types.ImageListOptions{
			Filters:        filters.NewArgs(),
			SharedSize:     true,
			ContainerCount: true,
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
func (daemon *Daemon) localVolumesSize(ctx context.Context) ([]*volume.Volume, error) {
	volumes, _, err := daemon.usageVolumes.Do(ctx, struct{}{}, func(ctx context.Context) ([]*volume.Volume, error) {
		volumes, err := daemon.volumes.LocalVolumesSize(ctx)
		if err != nil {
			return nil, err
		}
		return volumes, nil
	})
	return volumes, err
}

// layerDiskUsage obtains information about layer disk usage from image service
// and makes sure that only one size calculation is performed at the same time.
func (daemon *Daemon) layerDiskUsage(ctx context.Context) (int64, error) {
	usage, _, err := daemon.usageLayer.Do(ctx, struct{}{}, func(ctx context.Context) (int64, error) {
		usage, err := daemon.imageService.LayerDiskUsage(ctx)
		if err != nil {
			return 0, err
		}
		return usage, nil
	})
	return usage, err
}

// SystemDiskUsage returns information about the daemon data disk usage.
// Callers must not mutate contents of the returned fields.
func (daemon *Daemon) SystemDiskUsage(ctx context.Context, opts system.DiskUsageOptions) (*types.DiskUsage, error) {
	eg, ctx := errgroup.WithContext(ctx)

	var containers []*types.Container
	if opts.Containers {
		eg.Go(func() error {
			var err error
			containers, err = daemon.containerDiskUsage(ctx)
			return err
		})
	}

	var (
		images     []*types.ImageSummary
		layersSize int64
	)
	if opts.Images {
		eg.Go(func() error {
			var err error
			images, err = daemon.imageDiskUsage(ctx)
			return err
		})
		eg.Go(func() error {
			var err error
			layersSize, err = daemon.layerDiskUsage(ctx)
			return err
		})
	}

	var volumes []*volume.Volume
	if opts.Volumes {
		eg.Go(func() error {
			var err error
			volumes, err = daemon.localVolumesSize(ctx)
			return err
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return &types.DiskUsage{
		LayersSize: layersSize,
		Containers: containers,
		Volumes:    volumes,
		Images:     images,
	}, nil
}
