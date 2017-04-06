package daemon

import (
	"github.com/Sirupsen/logrus"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/swarmkit/agent/exec"
)

// SetContainerSecretStore sets the secret store backend for the container
func (daemon *Daemon) SetContainerSecretStore(name string, store exec.SecretGetter) error {
	c, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	c.SecretStore = store

	return nil
}

// SetContainerSecretReferences sets the container secret references needed
func (daemon *Daemon) SetContainerSecretReferences(name string, refs []*swarmtypes.SecretReference) error {
	if !secretsSupported() && len(refs) > 0 {
		logrus.Warn("secrets are not supported on this platform")
		return nil
	}

	c, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	c.SecretReferences = refs

	return nil
}
