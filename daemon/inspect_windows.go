package daemon

import "github.com/docker/docker/api/types"

// This sets platform-specific fields
func setPlatformSpecificContainerFields(container *Container, contJSONBase *types.ContainerJSONBase) *types.ContainerJSONBase {
	return contJSONBase
}

func addMountPoints(container *Container) []types.MountPoint {
	return nil
}

// ContainerInspectPre120 get containers for pre 1.20 APIs.
func (daemon *Daemon) ContainerInspectPre120(name string) (*types.ContainerJSON, error) {
	return daemon.ContainerInspect(name, false)
}
