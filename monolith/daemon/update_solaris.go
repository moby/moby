package daemon

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/monolith/libcontainerd"
)

func toContainerdResources(resources container.Resources) libcontainerd.Resources {
	var r libcontainerd.Resources
	return r
}
