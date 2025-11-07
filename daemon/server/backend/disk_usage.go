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

	// Verbose indicates whether to include detailed information.
	Verbose bool
}

// DiskUsage contains the information returned by the backend for the
// GET "/system/df" endpoint.
type DiskUsage struct {
	Images     *ImageDiskUsage
	Containers *ContainerDiskUsage
	Volumes    *VolumeDiskUsage
	BuildCache *build.DiskUsage
}

// ContainerDiskUsage contains disk usage for containers.
type ContainerDiskUsage = container.DiskUsage

// ImageDiskUsage contains disk usage for images.
type ImageDiskUsage = image.DiskUsage

// VolumeDiskUsage contains disk usage for volumes.
type VolumeDiskUsage = volume.DiskUsage
