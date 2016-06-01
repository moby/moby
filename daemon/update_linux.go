// +build linux

package daemon

import (
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/engine-api/types/container"
)

func toContainerdResources(resources container.Resources) libcontainerd.Resources {
	var r libcontainerd.Resources
	r.BlkioWeight = uint32(resources.BlkioWeight)
	r.CpuShares = uint32(resources.CPUShares)
	r.CpuPeriod = uint32(resources.CPUPeriod)
	r.CpuQuota = uint32(resources.CPUQuota)
	r.CpusetCpus = resources.CpusetCpus
	r.CpusetMems = resources.CpusetMems
	r.MemoryLimit = uint32(resources.Memory)
	if resources.MemorySwap > 0 {
		r.MemorySwap = uint32(resources.MemorySwap)
	}
	r.MemoryReservation = uint32(resources.MemoryReservation)
	r.KernelMemoryLimit = uint32(resources.KernelMemory)
	return r
}
