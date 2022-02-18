package daemon // import "github.com/moby/moby/daemon"

import (
	"github.com/moby/moby/api/types/container"
	libcontainerdtypes "github.com/moby/moby/libcontainerd/types"
)

func toContainerdResources(resources container.Resources) *libcontainerdtypes.Resources {
	// We don't support update, so do nothing
	return nil
}
