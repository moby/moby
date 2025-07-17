package daemon

import (
	swarmtypes "github.com/moby/moby/api/types/swarm"
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
