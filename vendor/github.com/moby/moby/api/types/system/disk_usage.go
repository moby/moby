package system

import (
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/volume"
)

// DiskUsage contains response of Engine API for API 1.49 and greater:
// GET "/system/df"
type DiskUsage struct {
	Images     *image.DiskUsage
	Containers *container.DiskUsage
	Volumes    *volume.DiskUsage
	BuildCache *build.CacheDiskUsage
}
