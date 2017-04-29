package daemon

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/exec"
)

// This sets platform-specific fields
func setPlatformSpecificContainerFields(container *container.Container, contJSONBase *types.ContainerJSONBase) *types.ContainerJSONBase {
	return contJSONBase
}

func addMountPoints(container *container.Container) []types.MountPoint {
	mountPoints := make([]types.MountPoint, 0, len(container.MountPoints))
	for _, m := range container.MountPoints {
		mountPoints = append(mountPoints, types.MountPoint{
			Type:        m.Type,
			Name:        m.Name,
			Source:      m.Path(),
			Destination: m.Destination,
			Driver:      m.Driver,
			RW:          m.RW,
		})
	}
	return mountPoints
}

// containerInspectPre120 get containers for pre 1.20 APIs.
func (daemon *Daemon) containerInspectPre120(name string) (*types.ContainerJSON, error) {
	return daemon.ContainerInspectCurrent(name, false)
}

func inspectExecProcessConfig(e *exec.Config) *backend.ExecProcessConfig {
	return &backend.ExecProcessConfig{
		Tty:        e.Tty,
		Entrypoint: e.Entrypoint,
		Arguments:  e.Args,
	}
}
