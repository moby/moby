package daemon

import (
	"github.com/Microsoft/opengcs/client"
	"github.com/docker/docker/container"
	"github.com/docker/docker/libcontainerd"
)

func (daemon *Daemon) getLibcontainerdCreateOptions(container *container.Container) ([]libcontainerd.CreateOption, error) {
	createOptions := []libcontainerd.CreateOption{}

	// LCOW options.
	if container.Platform == "linux" {
		config := &client.Config{}
		if err := config.GenerateDefault(daemon.configStore.GraphOptions); err != nil {
			return nil, err
		}
		// Override from user-supplied options.
		for k, v := range container.HostConfig.StorageOpt {
			switch k {
			case "lcow.kirdpath":
				config.KirdPath = v
			case "lcow.kernel":
				config.KernelFile = v
			case "lcow.initrd":
				config.InitrdFile = v
			case "lcow.vhdx":
				config.Vhdx = v
			case "lcow.bootparameters":
				config.BootParameters = v
			}
		}
		if err := config.Validate(); err != nil {
			return nil, err
		}
		lcowOpts := &libcontainerd.LCOWOption{
			Config: config,
		}
		createOptions = append(createOptions, lcowOpts)
	}

	return createOptions, nil
}
