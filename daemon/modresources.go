package daemon

import (
	"github.com/Sirupsen/logrus"
	derr "github.com/docker/docker/api/errors"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/runconfig"
)

func (daemon *Daemon) ContainerModResources(contID string, hostConfig *runconfig.HostConfig) ([]string, error) {
	logrus.Debugf("Server pinged!!!")

	// Is this needed here?
	if contID == "" {
		return nil, derr.ErrorCodeEmptyConfig
	}

	// Get the container
	if dockerContainer, err := daemon.Get(contID); err == nil {
		logrus.Debugf("Container found")

		resources := new(execdriver.Resources)

		if hostConfig.CPUShares != -1 {
			resources.CPUShares = hostConfig.CPUShares
		}
		if hostConfig.CPUPeriod != -1 {
			resources.CPUPeriod = hostConfig.CPUPeriod
		}
		if hostConfig.CPUQuota != -1 {
			resources.CPUQuota = hostConfig.CPUQuota
		}
		if hostConfig.CpusetCpus != "notset" {
			resources.CpusetCpus = hostConfig.CpusetCpus
		}
		if hostConfig.CpusetMems != "notset" {
			resources.CpusetMems = hostConfig.CpusetMems
		}
		if hostConfig.BlkioWeight != -1 {
			resources.BlkioWeight = hostConfig.BlkioWeight
		}
		if hostConfig.BlkioReadLimit != "notset" {
			resources.BlkioReadLimit = constructBlkioArgs(dockerContainer.hostConfig.Binds, hostConfig.BlkioReadLimit)
		}
		if *(hostConfig.MemorySwappiness) != -1 {
			resources.MemorySwappiness = *hostConfig.MemorySwappiness
		}

		if err := daemon.execDriver.ModifyResources(dockerContainer.ID, resources); err != nil {
			return nil, err
		}

		dockerContainer.hostConfig.Memory = resources.Memory
		dockerContainer.hostConfig.MemorySwap = resources.MemorySwap
		dockerContainer.hostConfig.KernelMemory = resources.KernelMemory
		dockerContainer.hostConfig.CPUShares = resources.CPUShares
		dockerContainer.hostConfig.CpusetCpus = resources.CpusetCpus
		dockerContainer.hostConfig.CpusetMems = resources.CpusetMems
		dockerContainer.hostConfig.CPUPeriod = resources.CPUPeriod
		dockerContainer.hostConfig.CPUQuota = resources.CPUQuota
		dockerContainer.hostConfig.BlkioWeight = resources.BlkioWeight
		if hostConfig.BlkioReadLimit != "notset" {
			dockerContainer.hostConfig.BlkioReadLimit = resources.BlkioReadLimit
		}
		dockerContainer.hostConfig.OomKillDisable = resources.OomKillDisable
		*dockerContainer.hostConfig.MemorySwappiness = resources.MemorySwappiness

		return nil, nil
	} else {
		return nil, err
	}
}
