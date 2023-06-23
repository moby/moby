package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	"github.com/containerd/containerd/log"
	swarmtypes "github.com/docker/docker/api/types/swarm"
)

// SetContainerSecretReferences sets the container secret references needed
func (daemon *Daemon) SetContainerSecretReferences(name string, refs []*swarmtypes.SecretReference) error {
	if !secretsSupported() && len(refs) > 0 {
		log.G(context.TODO()).Warn("secrets are not supported on this platform")
		return nil
	}

	c, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	c.SecretReferences = refs

	return nil
}
