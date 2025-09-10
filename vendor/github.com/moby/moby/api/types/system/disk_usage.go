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
	LayersSize  int64
	Images      []*image.Summary
	Containers  []*container.Summary
	Volumes     []*volume.Volume
	BuildCache  []*build.CacheRecord
	BuilderSize int64 `json:",omitempty"` // Deprecated: deprecated in API 1.38, and no longer used since API 1.40.
}
