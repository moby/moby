package daemon

import "github.com/docker/docker/api/types"

// This sets platform-specific fields
func setPlatformSpecificContainerFields(container *Container, contJSONBase *types.ContainerJSONBase) *types.ContainerJSONBase {
	return contJSONBase
}

func addMountPoints(container *Container) []types.MountPoint {
	return nil
}
