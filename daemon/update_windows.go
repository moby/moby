package daemon

import (
	libcontainerdtypes "github.com/docker/docker/daemon/internal/libcontainerd/types"
	"github.com/moby/moby/api/types/container"
)

func toContainerdResources(resources container.Resources) *libcontainerdtypes.Resources {
	// We don't support update, so do nothing
	return nil
}
