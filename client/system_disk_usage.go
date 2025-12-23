package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client/pkg/versions"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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
	// ActiveCount is the number of active containers.
	ActiveCount int64

	// TotalCount is the total number of containers.
	TotalCount int64

	// Reclaimable is the amount of disk space that can be reclaimed.
	Reclaimable int64

	// TotalSize is the total disk space used by all containers.
	TotalSize int64

	// Items holds detailed information about each container.
	Items []container.Summary
}

// ImagesDiskUsage contains disk usage information for images.
type ImagesDiskUsage struct {
	// ActiveCount is the number of active images.
	ActiveCount int64

	// TotalCount is the total number of images.
	TotalCount int64

	// Reclaimable is the amount of disk space that can be reclaimed.
	Reclaimable int64

	// TotalSize is the total disk space used by all images.
	TotalSize int64

	// Items holds detailed information about each image.
	Items []image.Summary
}

// VolumesDiskUsage contains disk usage information for volumes.
type VolumesDiskUsage struct {
	// ActiveCount is the number of active volumes.
	ActiveCount int64

	// TotalCount is the total number of volumes.
	TotalCount int64

	// Reclaimable is the amount of disk space that can be reclaimed.
	Reclaimable int64

	// TotalSize is the total disk space used by all volumes.
	TotalSize int64

	// Items holds detailed information about each volume.
	Items []volume.Volume
}

// BuildCacheDiskUsage contains disk usage information for build cache.
type BuildCacheDiskUsage struct {
	// ActiveCount is the number of active build cache records.
	ActiveCount int64

	// TotalCount is the total number of build cache records.
	TotalCount int64

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

	if versions.LessThan(cli.version, "1.52") {
		// Generate result from a legacy response.
		var du legacyDiskUsage
		if err := json.NewDecoder(resp.Body).Decode(&du); err != nil {
			return DiskUsageResult{}, fmt.Errorf("retrieving disk usage: %v", err)
		}

		return diskUsageResultFromLegacyAPI(&du), nil
	}

	var du system.DiskUsage
	if err := json.NewDecoder(resp.Body).Decode(&du); err != nil {
		return DiskUsageResult{}, fmt.Errorf("retrieving disk usage: %v", err)
	}

	var r DiskUsageResult
	if idu := du.ImageUsage; idu != nil {
		r.Images = ImagesDiskUsage{
			ActiveCount: idu.ActiveCount,
			Reclaimable: idu.Reclaimable,
			TotalCount:  idu.TotalCount,
			TotalSize:   idu.TotalSize,
		}

		if options.Verbose {
			r.Images.Items = slices.Clone(idu.Items)
		}
	}

	if cdu := du.ContainerUsage; cdu != nil {
		r.Containers = ContainersDiskUsage{
			ActiveCount: cdu.ActiveCount,
			Reclaimable: cdu.Reclaimable,
			TotalCount:  cdu.TotalCount,
			TotalSize:   cdu.TotalSize,
		}

		if options.Verbose {
			r.Containers.Items = slices.Clone(cdu.Items)
		}
	}

	if bdu := du.BuildCacheUsage; bdu != nil {
		r.BuildCache = BuildCacheDiskUsage{
			ActiveCount: bdu.ActiveCount,
			Reclaimable: bdu.Reclaimable,
			TotalCount:  bdu.TotalCount,
			TotalSize:   bdu.TotalSize,
		}

		if options.Verbose {
			r.BuildCache.Items = slices.Clone(bdu.Items)
		}
	}

	if vdu := du.VolumeUsage; vdu != nil {
		r.Volumes = VolumesDiskUsage{
			ActiveCount: vdu.ActiveCount,
			Reclaimable: vdu.Reclaimable,
			TotalCount:  vdu.TotalCount,
			TotalSize:   vdu.TotalSize,
		}

		if options.Verbose {
			r.Volumes.Items = slices.Clone(vdu.Items)
		}
	}

	return r, nil
}

// legacyDiskUsage is the response as was used by API < v1.52.
type legacyDiskUsage struct {
	LayersSize int64               `json:"LayersSize,omitempty"`
	Images     []image.Summary     `json:"Images,omitzero"`
	Containers []container.Summary `json:"Containers,omitzero"`
	Volumes    []volume.Volume     `json:"Volumes,omitzero"`
	BuildCache []build.CacheRecord `json:"BuildCache,omitzero"`
}

func diskUsageResultFromLegacyAPI(du *legacyDiskUsage) DiskUsageResult {
	return DiskUsageResult{
		Images:     imageDiskUsageFromLegacyAPI(du),
		Containers: containerDiskUsageFromLegacyAPI(du),
		BuildCache: buildCacheDiskUsageFromLegacyAPI(du),
		Volumes:    volumeDiskUsageFromLegacyAPI(du),
	}
}

func imageDiskUsageFromLegacyAPI(du *legacyDiskUsage) ImagesDiskUsage {
	idu := ImagesDiskUsage{
		TotalSize:  du.LayersSize,
		TotalCount: int64(len(du.Images)),
		Items:      du.Images,
	}

	for _, i := range idu.Items {
		if i.Containers > 0 {
			idu.ActiveCount++
		} else if i.Size != -1 && i.SharedSize != -1 {
			// Only count reclaimable size if we have size information
			idu.Reclaimable += (i.Size - i.SharedSize)
			// Also include the size of image index if it was included
			if i.Descriptor != nil && i.Descriptor.MediaType == ocispec.MediaTypeImageIndex {
				idu.Reclaimable += i.Descriptor.Size
			}
		}
	}

	return idu
}

func containerDiskUsageFromLegacyAPI(du *legacyDiskUsage) ContainersDiskUsage {
	cdu := ContainersDiskUsage{
		TotalCount: int64(len(du.Containers)),
		Items:      du.Containers,
	}

	var used int64
	for _, c := range cdu.Items {
		cdu.TotalSize += c.SizeRw
		switch c.State {
		case container.StateRunning, container.StatePaused, container.StateRestarting:
			cdu.ActiveCount++
			used += c.SizeRw
		case container.StateCreated, container.StateRemoving, container.StateExited, container.StateDead:
			// not active
		}
	}

	cdu.Reclaimable = cdu.TotalSize - used
	return cdu
}

func buildCacheDiskUsageFromLegacyAPI(du *legacyDiskUsage) BuildCacheDiskUsage {
	bdu := BuildCacheDiskUsage{
		TotalCount: int64(len(du.BuildCache)),
		Items:      du.BuildCache,
	}

	var used int64
	for _, b := range du.BuildCache {
		if !b.Shared {
			bdu.TotalSize += b.Size
		}

		if b.InUse {
			bdu.ActiveCount++
			if !b.Shared {
				used += b.Size
			}
		}
	}

	bdu.Reclaimable = bdu.TotalSize - used
	return bdu
}

func volumeDiskUsageFromLegacyAPI(du *legacyDiskUsage) VolumesDiskUsage {
	vdu := VolumesDiskUsage{
		TotalCount: int64(len(du.Volumes)),
		Items:      du.Volumes,
	}

	var used int64
	for _, v := range vdu.Items {
		// Ignore volumes with no usage data
		if v.UsageData != nil {
			if v.UsageData.RefCount > 0 {
				vdu.ActiveCount++
				used += v.UsageData.Size
			}
			if v.UsageData.Size > 0 {
				vdu.TotalSize += v.UsageData.Size
			}
		}
	}

	vdu.Reclaimable = vdu.TotalSize - used
	return vdu
}
