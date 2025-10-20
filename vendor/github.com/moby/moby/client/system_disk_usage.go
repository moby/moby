package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
)

// DiskUsageOptions holds parameters for [Client.DiskUsage] operations.
type DiskUsageOptions struct {
	// Containers controls whether container disk usage should be computed.
	Containers bool

	// Images controls whether image disk usage should be computed.
	Images bool

	// BuildCache controls whether build cache disk usage should be computed.
	BuildCache bool

	// Volumes controls whether volume disk usage should be computed.
	Volumes bool

	// Verbose enables more detailed disk usage information.
	Verbose bool
}

// DiskUsageResult is the result of [Client.DiskUsage] operations.
type DiskUsageResult struct {
	// Containers holds container disk usage information.
	Containers ContainersDiskUsage

	// Images holds image disk usage information.
	Images ImagesDiskUsage

	// BuildCache holds build cache disk usage information.
	BuildCache BuildCacheDiskUsage

	// Volumes holds volume disk usage information.
	Volumes VolumesDiskUsage
}

// ContainersDiskUsage contains disk usage information for containers.
type ContainersDiskUsage struct {
	// ActiveContainers is the number of active containers.
	ActiveContainers int64

	// TotalContainers is the total number of containers.
	TotalContainers int64

	// Reclaimable is the amount of disk space that can be reclaimed.
	Reclaimable int64

	// TotalSize is the total disk space used by all containers.
	TotalSize int64

	// Items holds detailed information about each container.
	Items []container.Summary
}

// ImagesDiskUsage contains disk usage information for images.
type ImagesDiskUsage struct {
	// ActiveImages is the number of active images.
	ActiveImages int64

	// TotalImages is the total number of images.
	TotalImages int64

	// Reclaimable is the amount of disk space that can be reclaimed.
	Reclaimable int64

	// TotalSize is the total disk space used by all images.
	TotalSize int64

	// Items holds detailed information about each image.
	Items []image.Summary
}

// VolumesDiskUsage contains disk usage information for volumes.
type VolumesDiskUsage struct {
	// ActiveVolumes is the number of active volumes.
	ActiveVolumes int64

	// TotalVolumes is the total number of volumes.
	TotalVolumes int64

	// Reclaimable is the amount of disk space that can be reclaimed.
	Reclaimable int64

	// TotalSize is the total disk space used by all volumes.
	TotalSize int64

	// Items holds detailed information about each volume.
	Items []volume.Volume
}

// BuildCacheDiskUsage contains disk usage information for build cache.
type BuildCacheDiskUsage struct {
	// ActiveBuildCacheRecords is the number of active build cache records.
	ActiveBuildCacheRecords int64

	// TotalBuildCacheRecords is the total number of build cache records.
	TotalBuildCacheRecords int64

	// Reclaimable is the amount of disk space that can be reclaimed.
	Reclaimable int64

	// TotalSize is the total disk space used by all build cache records.
	TotalSize int64

	// Items holds detailed information about each build cache record.
	Items []build.CacheRecord
}

// DiskUsage requests the current data usage from the daemon.
func (cli *Client) DiskUsage(ctx context.Context, options DiskUsageOptions) (DiskUsageResult, error) {
	query := url.Values{}

	for _, t := range []struct {
		flag   bool
		sysObj system.DiskUsageObject
	}{
		{options.Containers, system.ContainerObject},
		{options.Images, system.ImageObject},
		{options.Volumes, system.VolumeObject},
		{options.BuildCache, system.BuildCacheObject},
	} {
		if t.flag {
			query.Add("type", string(t.sysObj))
		}
	}

	if options.Verbose {
		query.Set("verbose", "1")
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

	var (
		r              DiskUsageResult
		imagesFrom     = []*image.Summary{}
		containersFrom = []*container.Summary{}
		volumesFrom    = []*volume.Volume{}
		buildCacheFrom = []*build.CacheRecord{}
	)

	if du.ImageUsage != nil {
		r.Images = ImagesDiskUsage{
			ActiveImages: du.ImageUsage.ActiveImages,
			Reclaimable:  du.ImageUsage.Reclaimable,
			TotalImages:  du.ImageUsage.TotalImages,
			TotalSize:    du.ImageUsage.TotalSize,
		}

		if options.Verbose {
			imagesFrom = du.ImageUsage.Items
		}
	} else {
		// Fallback for legacy response.
		r.Images = ImagesDiskUsage{
			TotalSize: du.LayersSize,
		}

		if du.Images != nil && options.Verbose {
			imagesFrom = du.Images
		}
	}

	r.Images.Items = make([]image.Summary, len(imagesFrom))
	for i, ii := range imagesFrom {
		r.Images.Items[i] = *ii
	}

	if du.ContainerUsage != nil {
		r.Containers = ContainersDiskUsage{
			ActiveContainers: du.ContainerUsage.ActiveContainers,
			Reclaimable:      du.ContainerUsage.Reclaimable,
			TotalContainers:  du.ContainerUsage.TotalContainers,
			TotalSize:        du.ContainerUsage.TotalSize,
		}

		if options.Verbose {
			containersFrom = du.ContainerUsage.Items
		}
	} else if du.Containers != nil && options.Verbose {
		// Fallback for legacy response.
		containersFrom = du.Containers
	}

	r.Containers.Items = make([]container.Summary, len(containersFrom))
	for i, c := range containersFrom {
		r.Containers.Items[i] = *c
	}

	if du.BuildCacheUsage != nil {
		r.BuildCache = BuildCacheDiskUsage{
			ActiveBuildCacheRecords: du.BuildCacheUsage.ActiveBuildCacheRecords,
			Reclaimable:             du.BuildCacheUsage.Reclaimable,
			TotalBuildCacheRecords:  du.BuildCacheUsage.TotalBuildCacheRecords,
			TotalSize:               du.BuildCacheUsage.TotalSize,
		}

		if options.Verbose {
			buildCacheFrom = du.BuildCacheUsage.Items
		}
	} else if du.BuildCache != nil && options.Verbose {
		// Fallback for legacy response.
		buildCacheFrom = du.BuildCache
	}

	r.BuildCache.Items = make([]build.CacheRecord, len(buildCacheFrom))
	for i, b := range buildCacheFrom {
		r.BuildCache.Items[i] = *b
	}

	if du.VolumeUsage != nil {
		r.Volumes = VolumesDiskUsage{
			ActiveVolumes: du.VolumeUsage.ActiveVolumes,
			Reclaimable:   du.VolumeUsage.Reclaimable,
			TotalSize:     du.VolumeUsage.TotalSize,
			TotalVolumes:  du.VolumeUsage.TotalVolumes,
		}

		if options.Verbose {
			volumesFrom = du.VolumeUsage.Items
		}
	} else if du.Volumes != nil && options.Verbose {
		// Fallback for legacy response.
		volumesFrom = du.Volumes
	}

	r.Volumes.Items = make([]volume.Volume, len(volumesFrom))
	for i, v := range volumesFrom {
		r.Volumes.Items[i] = *v
	}

	return r, nil
}
