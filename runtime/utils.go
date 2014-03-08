package runtime

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
	runtime *Runtime
}

func (c *checker) Exists(name string) bool {
	return c.runtime.containerGraph.Exists("/" + name)
}

// Generate a random and unique name
func generateRandomName(runtime *Runtime) (string, error) {
	return namesgenerator.GenerateRandomName(&checker{runtime})
}
