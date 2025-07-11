package daemon

import (
	containerpkg "github.com/docker/docker/daemon/container"
	"github.com/moby/moby/api/types/container"
)

// This sets platform-specific fields
func setPlatformSpecificContainerFields(container *containerpkg.Container, contJSONBase *container.ContainerJSONBase) *container.ContainerJSONBase {
	return contJSONBase
}
