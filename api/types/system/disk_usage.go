package system

import (
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
)

// DiskUsage contains response of Engine API for API 1.49 and greater:
// GET "/system/df"
type DiskUsage struct {
	Images     *image.DiskUsage
	Containers *container.DiskUsage
	Volumes    *volume.DiskUsage
	BuildCache *build.CacheDiskUsage
}
