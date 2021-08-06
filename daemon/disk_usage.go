package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/docker/docker/api/server/router/system"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"golang.org/x/sync/errgroup"
)

// SystemDiskUsage returns information about the daemon data disk usage
func (daemon *Daemon) SystemDiskUsage(ctx context.Context, opts system.DiskUsageOptions) (*types.DiskUsage, error) {
	if !atomic.CompareAndSwapInt32(&daemon.diskUsageRunning, 0, 1) {
		return nil, fmt.Errorf("a disk usage operation is already running")
	}
	defer atomic.StoreInt32(&daemon.diskUsageRunning, 0)

	eg, ctx := errgroup.WithContext(ctx)

	var containers []*types.Container
	if opts.Containers {
		eg.Go(func() error {
			var err error
			// Retrieve container list
			containers, err = daemon.Containers(&types.ContainerListOptions{
				Size: true,
				All:  true,
			})
			if err != nil {
				return fmt.Errorf("failed to retrieve container list: %v", err)
			}
			return nil
		})
	}

	var (
		images     []*types.ImageSummary
		layersSize int64
	)
	if opts.Images {
		eg.Go(func() error {
			var err error
			// Get all top images with extra attributes
			images, err = daemon.imageService.Images(ctx, types.ImageListOptions{
				Filters:        filters.NewArgs(),
				SharedSize:     true,
				ContainerCount: true,
			})
			if err != nil {
				return fmt.Errorf("failed to retrieve image list: %v", err)
			}
			return nil
		})
		eg.Go(func() error {
			var err error
			layersSize, err = daemon.imageService.LayerDiskUsage(ctx)
			return err
		})
	}

	var volumes []*types.Volume
	if opts.Volumes {
		eg.Go(func() error {
			var err error
			volumes, err = daemon.volumes.LocalVolumesSize(ctx)
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
