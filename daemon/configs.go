package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	"github.com/containerd/containerd/log"
	swarmtypes "github.com/docker/docker/api/types/swarm"
)

// SetContainerConfigReferences sets the container config references needed
func (daemon *Daemon) SetContainerConfigReferences(name string, refs []*swarmtypes.ConfigReference) error {
	if !configsSupported() && len(refs) > 0 {
		log.G(context.TODO()).Warn("configs are not supported on this platform")
		return nil
	}

	c, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}
	c.ConfigReferences = append(c.ConfigReferences, refs...)
	return nil
}
