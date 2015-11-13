package daemon

import (
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/runconfig"
)

// This sets platform-specific fields
func setPlatformSpecificContainerFields(container *Container, contJSONBase *types.ContainerJSONBase) *types.ContainerJSONBase {
	return contJSONBase
}

func addMountPoints(container *Container) []types.MountPoint {
	mountPoints := make([]types.MountPoint, 0, len(container.MountPoints))
	for _, m := range container.MountPoints {
		mountPoints = append(mountPoints, types.MountPoint{
			Name:        m.Name,
			Source:      m.Path(),
			Destination: m.Destination,
			Driver:      m.Driver,
			RW:          m.RW,
		})
	}
	return mountPoints
}

// ContainerInspectPre120 get containers for pre 1.20 APIs.
func (daemon *Daemon) ContainerInspectPre120(name string) (*types.ContainerJSON, error) {
	return daemon.ContainerInspect(name, false)
}

// populateJSONBase is a platform-specific helper function for inspect that
// populates the ContainerJSONBase structure
func populateJSONBase(container *Container, hostConfig *runconfig.HostConfig, containerState *types.ContainerState) *types.ContainerJSONBase {

	return &types.ContainerJSONBase{
		ID:           container.ID,
		Created:      container.Created.Format(time.RFC3339Nano),
		Path:         container.Path,
		Args:         container.Args,
		State:        containerState,
		Image:        container.ImageID,
		LogPath:      container.LogPath,
		Name:         container.Name,
		RestartCount: container.RestartCount,
		Driver:       container.Driver,
		ExecIDs:      container.getExecIDs(),
		HostConfig:   hostConfig,
	}
}
