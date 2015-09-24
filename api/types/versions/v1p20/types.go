// Package v1p20 provides specific API types for the API version 1, patch 20.
package v1p20

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/runconfig"
)

// ContainerJSON is a backcompatibility struct for the API 1.20
type ContainerJSON struct {
	*types.ContainerJSONBase
	Mounts []types.MountPoint
	Config *ContainerConfig
}

// ContainerConfig is a backcompatibility struct used in ContainerJSON for the API 1.20
type ContainerConfig struct {
	*runconfig.Config
	// backward compatibility, it lives now in HostConfig
	VolumeDriver string
}

// StatsJSON is a backcompatibility struct used in Stats for API prior to 1.21
type StatsJSON struct {
	types.Stats
	Network types.NetworkStats `json:"network,omitempty"`
}
