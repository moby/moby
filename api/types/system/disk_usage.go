package system

import (
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/volume"
)

// DiskUsageObject represents an object type used for disk usage query filtering.
type DiskUsageObject string

const (
	// ContainerObject represents a container DiskUsageObject.
	ContainerObject DiskUsageObject = "container"
	// ImageObject represents an image DiskUsageObject.
	ImageObject DiskUsageObject = "image"
	// VolumeObject represents a volume DiskUsageObject.
	VolumeObject DiskUsageObject = "volume"
	// BuildCacheObject represents a build-cache DiskUsageObject.
	BuildCacheObject DiskUsageObject = "build-cache"
)

// DiskUsage contains response of Engine API:
// GET "/system/df"
type DiskUsage struct {
	LegacyDiskUsage

	ImageUsage      *ImagesDiskUsage     `json:"ImageUsage,omitempty"`
	ContainerUsage  *ContainersDiskUsage `json:"ContainerUsage,omitempty"`
	VolumeUsage     *VolumesDiskUsage    `json:"VolumeUsage,omitempty"`
	BuildCacheUsage *BuildCacheDiskUsage `json:"BuildCacheUsage,omitempty"`
}

type LegacyDiskUsage struct {
	// Deprecated: kept to maintain backwards compatibility with API < v1.52, use [ImagesDiskUsage.TotalSize] instead.
	LayersSize int64 `json:"LayersSize,omitempty"`

	// Deprecated: kept to maintain backwards compatibility with API < v1.52, use [ImagesDiskUsage.Items] instead.
	Images []*image.Summary `json:"Images,omitempty"`

	// Deprecated: kept to maintain backwards compatibility with API < v1.52, use [ContainersDiskUsage.Items] instead.
	Containers []*container.Summary `json:"Containers,omitempty"`

	// Deprecated: kept to maintain backwards compatibility with API < v1.52, use [VolumesDiskUsage.Items] instead.
	Volumes []*volume.Volume `json:"Volumes,omitempty"`

	// Deprecated: kept to maintain backwards compatibility with API < v1.52, use [BuildCacheDiskUsage.Items] instead.
	BuildCache []*build.CacheRecord `json:"BuildCache,omitempty"`
}
