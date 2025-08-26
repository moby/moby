package daemon

import (
	"github.com/moby/moby/api/types/container"
	containerpkg "github.com/moby/moby/v2/daemon/container"
)

// This sets platform-specific fields
func setPlatformSpecificContainerFields(ctr *containerpkg.Container, resp *container.InspectResponse) *container.InspectResponse {
	return resp
}
