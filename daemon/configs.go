package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/sirupsen/logrus"
)

// SetContainerConfigReferences sets the container config references needed
func (daemon *Daemon) SetContainerConfigReferences(ctx context.Context, name string, refs []*swarmtypes.ConfigReference) error {
	if !configsSupported() && len(refs) > 0 {
		logrus.Warn("configs are not supported on this platform")
		return nil
	}

	c, err := daemon.GetContainer(ctx, name)
	if err != nil {
		return err
	}
	c.ConfigReferences = append(c.ConfigReferences, refs...)
	return nil
}
