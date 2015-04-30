package daemon

import (
	"github.com/docker/docker/runconfig"
)

func (daemon *Daemon) ContainerSet(name string, hostConfig *runconfig.HostConfig) ([]string, error) {
	warnings, err := daemon.verifyHostConfig(hostConfig)
	if err != nil {
		return warnings, err
	}

	container, err := daemon.Get(name)
	if err != nil {
		return warnings, err
	}

	daemon.updateResources(container, hostConfig)

	if err := daemon.Set(container.command); err != nil {
		return warnings, err
	}

	return warnings, nil
}

func (daemon *Daemon) updateResources(container *Container, hostConfig *runconfig.HostConfig) {
	if hostConfig.CpuShares != 0 {
		container.hostConfig.CpuShares = hostConfig.CpuShares
	}
	if hostConfig.CpusetCpus != "" {
		container.hostConfig.CpusetCpus = hostConfig.CpusetCpus
	}
	if hostConfig.CpusetMems != "" {
		container.hostConfig.CpusetMems = hostConfig.CpusetMems
	}
	if hostConfig.Memory != 0 {
		container.hostConfig.Memory = hostConfig.Memory
	}
	if hostConfig.MemorySwap != 0 {
		container.hostConfig.MemorySwap = hostConfig.MemorySwap
	}
	if hostConfig.CpuQuota != 0 {
		container.hostConfig.CpuQuota = hostConfig.CpuQuota
	}

	container.command.Resources.CpuShares = container.hostConfig.CpuShares
	container.command.Resources.CpusetCpus = container.hostConfig.CpusetCpus
	container.command.Resources.CpusetMems = container.hostConfig.CpusetMems
	container.command.Resources.Memory = container.hostConfig.Memory
	container.command.Resources.MemorySwap = container.hostConfig.MemorySwap
	container.command.Resources.CpuQuota = container.hostConfig.CpuQuota
}
