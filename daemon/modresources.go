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

	logrus.Debugf("In modresourcs")

	// Get the container
	if dockerContainer, err := daemon.Get(contID); err == nil {
		logrus.Debugf("Container found")

		// libContContainer := daemon.execDriver.activeContainers[dockerContainer.ID]

		logrus.Debugf("libcontainer Container found")

		if hostConfig.CPUShares != -1 {
			dockerContainer.hostConfig.CPUShares = hostConfig.CPUShares
		}
		if hostConfig.CPUPeriod != -1 {
			dockerContainer.hostConfig.CPUPeriod = hostConfig.CPUPeriod
		}
		if hostConfig.CPUQuota != -1 {
			dockerContainer.hostConfig.CPUQuota = hostConfig.CPUQuota
		}
		if hostConfig.CpusetCpus != "notset" {
			dockerContainer.hostConfig.CpusetCpus = hostConfig.CpusetCpus
		}
		if hostConfig.CpusetMems != "notset" {
			dockerContainer.hostConfig.CpusetMems = hostConfig.CpusetMems
		}
		if hostConfig.BlkioWeight != -1 {
			dockerContainer.hostConfig.BlkioWeight = hostConfig.BlkioWeight
		}
		if hostConfig.BlkioReadLimit != "notset" {
			dockerContainer.hostConfig.BlkioReadLimit = hostConfig.BlkioReadLimit
		}
		if *(hostConfig.MemorySwappiness) != -1 {
			dockerContainer.hostConfig.MemorySwappiness = hostConfig.MemorySwappiness
		}

		resources := &execdriver.Resources{
			Memory:           dockerContainer.hostConfig.Memory,
			MemorySwap:       dockerContainer.hostConfig.MemorySwap,
			KernelMemory:     dockerContainer.hostConfig.KernelMemory,
			CPUShares:        dockerContainer.hostConfig.CPUShares,
			CpusetCpus:       dockerContainer.hostConfig.CpusetCpus,
			CpusetMems:       dockerContainer.hostConfig.CpusetMems,
			CPUPeriod:        dockerContainer.hostConfig.CPUPeriod,
			CPUQuota:         dockerContainer.hostConfig.CPUQuota,
			BlkioWeight:      dockerContainer.hostConfig.BlkioWeight,
			BlkioReadLimit:   constructBlkioArgs(dockerContainer.hostConfig.Binds, dockerContainer.hostConfig.BlkioReadLimit),
			OomKillDisable:   dockerContainer.hostConfig.OomKillDisable,
			MemorySwappiness: *(dockerContainer.hostConfig.MemorySwappiness),
		}

		if err := daemon.execDriver.ModifyResources(dockerContainer.ID, resources); err != nil {
			return nil, err
		}

		logrus.Debugf("Resources object:\n%v\n", resources)

		return nil, nil
	} else {
		return nil, err
	}
}
