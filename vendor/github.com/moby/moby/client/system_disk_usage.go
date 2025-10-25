package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
)

// DiskUsageOptions holds parameters for system disk usage query.
type DiskUsageOptions struct {
	// Containers controls whether container disk usage should be computed.
	Containers bool

	// Images controls whether image disk usage should be computed.
	Images bool

	// BuildCache controls whether build cache disk usage should be computed.
	BuildCache bool

	// Volumes controls whether volume disk usage should be computed.
	Volumes bool
}

// DiskUsageResult holds the result of a DiskUsage query.
type DiskUsageResult struct {
	DiskUsage system.DiskUsage

	Containers ContainerDiskUsage
	Images     ImageDiskUsage
	BuildCache BuildCacheDiskUsage
	Volumes    VolumeDiskUsage
}

type ContainerDiskUsage struct {
	Items []container.Summary
}

type ImageDiskUsage struct {
	TotalSize int64
	Items     []image.Summary
}

type VolumeDiskUsage struct {
	Items []volume.Volume
}

type BuildCacheDiskUsage struct {
	Items []build.CacheRecord
}

// DiskUsage requests the current data usage from the daemon
func (cli *Client) DiskUsage(ctx context.Context, options DiskUsageOptions) (DiskUsageResult, error) {
	var (
		query url.Values
		once  sync.Once

		initQuery = func() {
			query = url.Values{}
		}
	)

	if options.Containers {
		once.Do(initQuery)
		query.Add("type", string(system.ContainerObject))
	}
	if options.Images {
		once.Do(initQuery)
		query.Add("type", string(system.ImageObject))
	}
	if options.BuildCache {
		once.Do(initQuery)
		query.Add("type", string(system.BuildCacheObject))
	}
	if options.Volumes {
		once.Do(initQuery)
		query.Add("type", string(system.VolumeObject))
	}

	resp, err := cli.get(ctx, "/system/df", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return DiskUsageResult{}, err
	}

	var du system.DiskUsage
	if err := json.NewDecoder(resp.Body).Decode(&du); err != nil {
		return DiskUsageResult{}, fmt.Errorf("Error retrieving disk usage: %v", err)
	}

	result := DiskUsageResult{
		DiskUsage: du,
		Containers: ContainerDiskUsage{
			Items: make([]container.Summary, 0, len(du.Containers)),
		},
		Images: ImageDiskUsage{
			TotalSize: du.LayersSize,
			Items:     make([]image.Summary, 0, len(du.Images)),
		},
		BuildCache: BuildCacheDiskUsage{
			Items: make([]build.CacheRecord, 0, len(du.BuildCache)),
		},
		Volumes: VolumeDiskUsage{
			Items: make([]volume.Volume, 0, len(du.Volumes)),
		},
	}

	for _, c := range du.Containers {
		result.Containers.Items = append(result.Containers.Items, *c)
	}

	for _, i := range du.Images {
		result.Images.Items = append(result.Images.Items, *i)
	}

	for _, b := range du.BuildCache {
		result.BuildCache.Items = append(result.BuildCache.Items, *b)
	}

	for _, v := range du.Volumes {
		result.Volumes.Items = append(result.Volumes.Items, *v)
	}

	return result, nil
}
