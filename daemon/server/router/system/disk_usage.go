package system

import (
	buildtypes "github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
)

// diskUsageCompat is used to provide API responses with backward-compatibility
// for API < v1.52, which used a different format. For API v1.52, we return
// both "old" and "new" responses if the client did not explicitly opt in to
// using the new format (through the use of the "verbose" query-parameter).
type diskUsageCompat struct {
	*legacyDiskUsage
	*system.DiskUsage
}

// legacyDiskUsage is the response as was used by API < v1.52.
type legacyDiskUsage struct {
	LayersSize int64                    `json:"LayersSize,omitempty"`
	Images     []image.Summary          `json:"Images,omitzero"`
	Containers []container.Summary      `json:"Containers,omitzero"`
	Volumes    []volume.Volume          `json:"Volumes,omitzero"`
	BuildCache []buildtypes.CacheRecord `json:"BuildCache,omitzero"`
}
