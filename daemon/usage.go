package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	"github.com/docker/docker/api/server/router/system"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
	"golang.org/x/sync/errgroup"
)

func (daemon *Daemon) containersUsage(ctx context.Context) ([]*types.ContainerUsage, error) {
	var us []*types.ContainerUsage
	if err := daemon.rangeContainers(&types.ContainerListOptions{
		Size: true,
		All:  true,
	}, func(s *container.Snapshot, _ *listContext) bool {
		sizeRw, sizeRootFs := daemon.imageService.GetContainerLayerSize(s.ID)
		us = append(us, &types.ContainerUsage{
			ID:         s.ID,
			Names:      s.Names,
			SizeRw:     sizeRw,
			SizeRootFs: sizeRootFs,
		})
		return true
	}); err != nil {
		return nil, err
	}
	return us, nil
}

// ContainerUsage returns information about container usage.
func (daemon *Daemon) ContainersUsage(ctx context.Context) ([]*types.ContainerUsage, error) {
	v, err := daemon.containersUsageSingleton.Do(ctx)
	if err != nil {
		return nil, err
	}
	return v.([]*types.ContainerUsage), nil
}

// SystemDiskUsage returns information about the daemon data disk usage.
// The caller must not mutate returned values, since
func (daemon *Daemon) SystemDiskUsage(ctx context.Context, opts system.DiskUsageOptions) (*types.DiskUsage, error) {
	eg, ctx := errgroup.WithContext(ctx)

	var containers []*types.ContainerUsage
	if opts.Containers {
		eg.Go(func() error {
			var err error
			containers, err = daemon.ContainersUsage(ctx)
			return err
		})
	}

	var (
		images     []*types.ImageUsage
		layersSize int64
	)
	if opts.Images {
		eg.Go(func() error {
			var err error
			images, err = daemon.imageService.ImagesUsage(ctx)
			return err
		})
		eg.Go(func() error {
			var err error
			layersSize, err = daemon.imageService.LayersUsage(ctx)
			return err
		})
	}

	var volumes []*types.VolumeUsage
	if opts.Volumes {
		eg.Go(func() error {
			var err error
			volumes, err = daemon.volumes.LocalVolumesUsage(ctx)
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
