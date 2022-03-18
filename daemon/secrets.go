package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/sirupsen/logrus"
)

// SetContainerSecretReferences sets the container secret references needed
func (daemon *Daemon) SetContainerSecretReferences(ctx context.Context, name string, refs []*swarmtypes.SecretReference) error {
	if !secretsSupported() && len(refs) > 0 {
		logrus.Warn("secrets are not supported on this platform")
		return nil
	}

	c, err := daemon.GetContainer(ctx, name)
	if err != nil {
		return err
	}

	c.SecretReferences = refs

	return nil
}
