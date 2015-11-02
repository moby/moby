package daemon

import (
	"net/url"
	// "strconv"

	"github.com/Sirupsen/logrus"
	derr "github.com/docker/docker/api/errors"
	"github.com/docker/docker/daemon/execdriver"
	// "github.com/docker/docker/api/types"
	// "github.com/docker/docker/graph/tags"
	// "github.com/docker/docker/image"
	// "github.com/docker/docker/pkg/parsers"
	// "github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
)

func (daemon *Daemon) ContainerModResources(httpForm url.Values, hostConfig *runconfig.HostConfig) ([]string, error) {
	logrus.Debugf("Server pinged!!!")

	contID := httpForm.Get("ID")
	// if val, ok := dict["foo"]; ok {
	//     //do something here
	// }

	if contID == "" {
		return nil, derr.ErrorCodeEmptyConfig
	}

	logrus.Debugf("In modresourcs")

	// Get the container
	dockerContainer, err := daemon.Get(contID)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Container found")

	// libContContainer := daemon.execDriver.activeContainers[dockerContainer.ID]

	logrus.Debugf("libcontainer Container found")

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
		MemorySwappiness: -1,
	}

	// if CPUShares := httpForm.Get("CPUShares"); CPUShares != "" {
	// 	dockerContainer.hostConfig.CPUShares, _ = strconv.ParseInt(CPUShares, 10, 64)
	// 	resources.CPUShares = dockerContainer.hostConfig.CPUShares
	// }
	// if CPUPeriod := httpForm.Get("CPUPeriod"); CPUPeriod != "" {
	// 	dockerContainer.hostConfig.CPUPeriod, _ = strconv.ParseInt(CPUPeriod, 10, 64)
	// 	resources.CPUPeriod = dockerContainer.hostConfig.CPUPeriod
	// }
	// if CPUQuota := httpForm.Get("CPUQuota"); CPUQuota != "" {
	// 	dockerContainer.hostConfig.CPUQuota, _ = strconv.ParseInt(CPUQuota, 10, 64)
	// 	resources.CPUQuota = dockerContainer.hostConfig.CPUQuota
	// }
	// if CpusetCpus := httpForm.Get("CpusetCpus"); CpusetCpus != "" {
	// 	dockerContainer.hostConfig.CpusetCpus = CpusetCpus
	// 	resources.CpusetCpus = dockerContainer.hostConfig.CpusetCpus
	// }
	// if CpusetMems := httpForm.Get("CpusetMems"); CpusetMems != "" {
	// 	dockerContainer.hostConfig.CpusetMems = CpusetMems
	// 	resources.CpusetMems = dockerContainer.hostConfig.CpusetMems
	// }
	// if BlkioWeight := httpForm.Get("BlkioWeight"); BlkioWeight != "" {
	// 	dockerContainer.hostConfig.BlkioWeight, _ = strconv.ParseInt(BlkioWeight, 10, 64)
	// 	resources.BlkioWeight = dockerContainer.hostConfig.BlkioWeight
	// }
	// if BlkioReadLimit := httpForm.Get("BlkioReadLimit"); BlkioReadLimit != "" {
	// 	dockerContainer.hostConfig.BlkioReadLimit = BlkioReadLimit
	// 	resources.BlkioReadLimit = dockerContainer.hostConfig.BlkioReadLimit
	// }
	if hostConfig.CPUShares != 0 {
		dockerContainer.hostConfig.CPUShares = hostConfig.CPUShares
		resources.CPUShares = hostConfig.CPUShares
	}

	daemon.execDriver.ModifyResources(dockerContainer.ID, resources)

	return nil, nil
}
