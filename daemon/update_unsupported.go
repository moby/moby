//go:build !linux && !windows

package daemon

import (
	"github.com/moby/moby/api/types/container"
	libcontainerdtypes "github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
)

func toContainerdResources(container.Resources) *libcontainerdtypes.Resources {
	return &libcontainerdtypes.Resources{}
}
