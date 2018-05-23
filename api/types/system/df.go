package system

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
)

// DiskUsage contains response of Engine API:
// GET "/system/df"
type DiskUsage struct {
	LayersSize  int64
	Images      []*image.Summary
	Containers  []*container.Container
	Volumes     []*volume.Volume
	BuilderSize int64
}
