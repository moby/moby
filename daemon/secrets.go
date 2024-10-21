package daemon // import "github.com/docker/docker/daemon"

import (
	swarmtypes "github.com/docker/docker/api/types/swarm"
)

// SetContainerSecretReferences sets the container secret references needed
func (daemon *Daemon) SetContainerSecretReferences(name string, refs []*swarmtypes.SecretReference) error {
	c, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}
	c.SecretReferences = refs
	return nil
}
