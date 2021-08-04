package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/server/router/system"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/volume"
	"golang.org/x/sync/errgroup"
)

// ContainerDiskUsage returns information about container data disk usage.
func (daemon *Daemon) ContainerDiskUsage(ctx context.Context) ([]*types.ContainerUsage, error) {
	ch := daemon.usage.DoChan("ContainerDiskUsage", func() (interface{}, error) {
		// Retrieve container list
		containers, err := daemon.Containers(&types.ContainerListOptions{
			Size: true,
			All:  true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve container list: %v", err)
		}
		us := make([]*types.ContainerUsage, 0, len(containers))
		for _, c := range containers {
			us = append(us, &types.ContainerUsage{
				ID:         c.ID,
				Image:      c.Image,
				ImageID:    c.ImageID,
				Command:    c.Command,
				Created:    c.Created,
				Ports:      c.Ports,
				Names:      c.Names,
				SizeRw:     c.SizeRw,
				SizeRootFs: c.SizeRootFs,
				Labels:     c.Labels,
				State:      c.State,
				Status:     c.Status,
				Mounts:     c.Mounts,
			})
		}
		return us, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}
		return res.Val.([]*types.ContainerUsage), nil
	}
}

// SystemDiskUsage returns information about the daemon data disk usage.
// Callers must not mutate contents of the returned fields.
func (daemon *Daemon) SystemDiskUsage(ctx context.Context, opts system.DiskUsageOptions) (*types.DiskUsage, error) {
	eg, ctx := errgroup.WithContext(ctx)

	var containers []*types.ContainerUsage
	if opts.Containers {
		eg.Go(func() error {
			var err error
			containers, err = daemon.ContainerDiskUsage(ctx)
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
			images, err = daemon.imageService.ImageDiskUsage(ctx)
			return err
		})
		eg.Go(func() error {
			var err error
			layersSize, err = daemon.imageService.LayerDiskUsage(ctx)
			return err
		})
	}

	var volumes []*volume.Volume
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
