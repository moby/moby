package daemon // import "github.com/docker/docker/daemon"

import (
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/container"
	"github.com/sirupsen/logrus"
)

// SetContainerConfigReferences sets the container config references needed
func (daemon *Daemon) SetContainerConfigReferences(name string, refs []*swarmtypes.ConfigReference) error {
	if !configsSupported() && len(refs) > 0 {
		logrus.Warn("configs are not supported on this platform")
		return nil
	}

	c, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	for _, ref := range refs {
		c.ConfigReferences = append(c.ConfigReferences, &container.ConfigReference{ConfigReference: ref})
	}

	return nil
}
