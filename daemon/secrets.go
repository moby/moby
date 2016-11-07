package daemon

import (
	"github.com/Sirupsen/logrus"
	containertypes "github.com/docker/docker/api/types/container"
)

// SetContainerSecrets sets the container secrets needed
func (daemon *Daemon) SetContainerSecrets(name string, secrets []*containertypes.ContainerSecret) error {
	if !secretsSupported() && len(secrets) > 0 {
		logrus.Warn("secrets are not supported on this platform")
		return nil
	}

	c, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	c.Secrets = secrets

	return nil
}
