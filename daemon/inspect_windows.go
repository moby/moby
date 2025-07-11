package daemon

import (
	"github.com/moby/moby/api/types/container"
	containerpkg "github.com/moby/moby/v2/daemon/container"
)

// This sets platform-specific fields
func setPlatformSpecificContainerFields(container *containerpkg.Container, contJSONBase *container.ContainerJSONBase) *container.ContainerJSONBase {
	return contJSONBase
}
