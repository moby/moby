package daemon

import (
	"fmt"
	"strings"

	"github.com/docker/docker/nat"
	"github.com/docker/docker/runconfig"
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

func mergeLxcConfIntoOptions(hostConfig *runconfig.HostConfig, driverConfig map[string][]string) {
	if hostConfig == nil {
		return
	}

	// merge in the lxc conf options into the generic config map
	if lxcConf := hostConfig.LxcConf; lxcConf != nil {
		lxc := driverConfig["lxc"]
		for _, pair := range lxcConf {
			// because lxc conf gets the driver name lxc.XXXX we need to trim it off
			// and let the lxc driver add it back later if needed
			parts := strings.SplitN(pair.Key, ".", 2)
			lxc = append(lxc, fmt.Sprintf("%s=%s", parts[1], pair.Value))
		}
		driverConfig["lxc"] = lxc
	}
}
