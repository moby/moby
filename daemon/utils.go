package daemon

import (
	"github.com/dotcloud/docker/nat"
	"github.com/dotcloud/docker/pkg/namesgenerator"
	"github.com/dotcloud/docker/runconfig"
)

func migratePortMappings(config *runconfig.Config, hostConfig *runconfig.HostConfig) error {
	if config.PortSpecs != nil {
		ports, bindings, err := nat.ParsePortSpecs(config.PortSpecs)
		if err != nil {
			return err
		}
		config.PortSpecs = nil
		if len(bindings) > 0 {
			if hostConfig == nil {
				hostConfig = &runconfig.HostConfig{}
			}
			hostConfig.PortBindings = bindings
		}

		if config.ExposedPorts == nil {
			config.ExposedPorts = make(nat.PortSet, len(ports))
		}
		for k, v := range ports {
			config.ExposedPorts[k] = v
		}
	}
	return nil
}

type checker struct {
	daemon *Daemon
}

func (c *checker) Exists(name string) bool {
	return c.daemon.containerGraph.Exists("/" + name)
}

// Generate a random and unique name
func generateRandomName(daemon *Daemon) (string, error) {
	return namesgenerator.GenerateRandomName(&checker{daemon})
}
