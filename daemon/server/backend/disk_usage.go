package backend

import (
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/volume"
)

// DiskUsageOptions holds parameters for system disk usage query.
type DiskUsageOptions struct {
	// Containers controls whether container disk usage should be computed.
	Containers bool

	// Images controls whether image disk usage should be computed.
	Images bool

	// Volumes controls whether volume disk usage should be computed.
	Volumes bool
}

// DiskUsage contains the information returned by the backend for the
// GET "/system/df" endpoint.
type DiskUsage struct {
	Images     *image.DiskUsage
	Containers *ContainerDiskUsage
	Volumes    *volume.DiskUsage
	BuildCache *BuildCacheDiskUsage
}

// BuildCacheDiskUsage contains disk usage for the build cache.
type BuildCacheDiskUsage struct {
	TotalSize   int64
	Reclaimable int64
	Items       []*build.CacheRecord
}

// ContainerDiskUsage contains disk usage for containers.
type ContainerDiskUsage struct {
	TotalSize   int64
	Reclaimable int64
	Items       []*container.Summary
}
