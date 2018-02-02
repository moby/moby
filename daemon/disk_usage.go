package daemon // import "github.com/docker/docker/daemon"

import (
	"fmt"
	"sync/atomic"

	"golang.org/x/net/context"

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

	// Get all local volumes
	allVolumes := []*types.Volume{}
	getLocalVols := func(v volume.Volume) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if d, ok := v.(volume.DetailedVolume); ok {
				// skip local volumes with mount options since these could have external
				// mounted filesystems that will be slow to enumerate.
				if len(d.Options()) > 0 {
					return nil
				}
			}
			name := v.Name()
			refs := daemon.volumes.Refs(v)

			tv := volumeToAPIType(v)
			sz, err := directory.Size(v.Path())
			if err != nil {
				logrus.Warnf("failed to determine size of volume %v", name)
				sz = -1
			}
			tv.UsageData = &types.VolumeUsageData{Size: sz, RefCount: int64(len(refs))}
			allVolumes = append(allVolumes, tv)
		}

		return nil
	}

	err = daemon.traverseLocalVolumes(getLocalVols)
	if err != nil {
		return nil, err
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
