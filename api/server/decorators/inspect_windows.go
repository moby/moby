package decorators

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon"
)

// setPlatformSpecificContainerFields sets fields only available in windows hosts.
func setPlatformSpecificContainerFields(container *daemon.Container, contJSONBase *types.ContainerJSONBase) *types.ContainerJSONBase {
	return contJSONBase
}

// getMountPoints transforms mount points to be serialized by the API.
// Mounts are not supported on Windows servers.
func getMountPoints(container *daemon.Container) []types.MountPoint {
	return nil
}
