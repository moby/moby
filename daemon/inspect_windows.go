package daemon

import (
	"github.com/docker/docker/api/types/container"
	containerpkg "github.com/docker/docker/daemon/container"
)

// This sets platform-specific fields
func setPlatformSpecificContainerFields(container *containerpkg.Container, contJSONBase *container.ContainerJSONBase) *container.ContainerJSONBase {
	return contJSONBase
}
