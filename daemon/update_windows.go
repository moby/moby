// +build windows

package daemon

import (
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/libcontainerd"
)

func toContainerdResources(resources container.Resources) *libcontainerd.Resources {
	// We don't support update, so do nothing
	return nil
}
