package daemon

import (
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/libcontainerd"
)

func toContainerdResources(resources container.Resources) libcontainerd.Resources {
	var r libcontainerd.Resources
	return r
}
