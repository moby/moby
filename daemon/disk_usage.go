package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/directory"
	"github.com/docker/docker/volume"
	"github.com/sirupsen/logrus"
)

// SystemDiskUsage returns information about the daemon data disk usage
func (daemon *Daemon) SystemDiskUsage(ctx context.Context) (*types.DiskUsage, error) {
	if !atomic.CompareAndSwapInt32(&daemon.diskUsageRunning, 0, 1) {
		return nil, fmt.Errorf("a disk usage operation is already running")
	}
	defer atomic.StoreInt32(&daemon.diskUsageRunning, 0)

	// Retrieve container list
	allContainers, err := daemon.Containers(&types.ContainerListOptions{
		Size: true,
		All:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve container list: %v", err)
	}

	// Get all top images with extra attributes
	allImages, err := daemon.imageService.Images(filters.NewArgs(), false, true)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve image list: %v", err)
	}

	volumes, err := daemon.volumes.FilterByDriver(volume.DefaultDriverName)
	if err != nil {
		return nil, err
	}

	var allVolumes []*types.Volume
	for _, v := range volumes {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if d, ok := v.(volume.DetailedVolume); ok {
			if len(d.Options()) > 0 {
				// skip local volumes with mount options since these could have external
				// mounted filesystems that will be slow to enumerate.
				continue
			}
		}

		name := v.Name()
		refs := daemon.volumes.Refs(v)

		tv := volumeToAPIType(v)
		sz, err := directory.Size(ctx, v.Path())
		if err != nil {
			logrus.Warnf("failed to determine size of volume %v", name)
			sz = -1
		}
		tv.UsageData = &types.VolumeUsageData{Size: sz, RefCount: int64(len(refs))}
		allVolumes = append(allVolumes, tv)
	}

	allLayersSize, err := daemon.imageService.LayerDiskUsage(ctx)
	if err != nil {
		return nil, err
	}

	return &types.DiskUsage{
		LayersSize: allLayersSize,
		Containers: allContainers,
		Volumes:    allVolumes,
		Images:     allImages,
	}, nil
}
