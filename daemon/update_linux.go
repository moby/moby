package daemon

import (
	"time"

	"github.com/moby/moby/api/types/container"
	libcontainerdtypes "github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func toContainerdResources(resources container.Resources) (*libcontainerdtypes.Resources, error) {
	var r libcontainerdtypes.Resources

	// little helper to lazily initialize the BlockIO struct only if needed
	blockIO := func() *specs.LinuxBlockIO {
		if r.BlockIO == nil {
			r.BlockIO = &specs.LinuxBlockIO{}
		}
		return r.BlockIO
	}

	weightDevices, err := getBlkioWeightDevices(resources)
	if err != nil {
		return nil, err
	}
	if resources.BlkioWeightDevice != nil {
		blockIO().WeightDevice = weightDevices
	}

	readBpsDevices, err := getBlkioThrottleDevices(resources.BlkioDeviceReadBps)
	if err != nil {
		return nil, err
	}
	if resources.BlkioDeviceReadBps != nil {
		blockIO().ThrottleReadBpsDevice = readBpsDevices
	}

	writeBpsDevices, err := getBlkioThrottleDevices(resources.BlkioDeviceWriteBps)
	if err != nil {
		return nil, err
	}
	if resources.BlkioDeviceWriteBps != nil {
		blockIO().ThrottleWriteBpsDevice = writeBpsDevices
	}

	readIOpsDevices, err := getBlkioThrottleDevices(resources.BlkioDeviceReadIOps)
	if err != nil {
		return nil, err
	}
	if resources.BlkioDeviceReadIOps != nil {
		blockIO().ThrottleReadIOPSDevice = readIOpsDevices
	}

	writeIOpsDevices, err := getBlkioThrottleDevices(resources.BlkioDeviceWriteIOps)
	if err != nil {
		return nil, err
	}
	if resources.BlkioDeviceWriteIOps != nil {
		blockIO().ThrottleWriteIOPSDevice = writeIOpsDevices
	}

	if resources.BlkioWeight != 0 {
		blockIO().Weight = &resources.BlkioWeight
	}

	cpu := specs.LinuxCPU{
		Cpus: resources.CpusetCpus,
		Mems: resources.CpusetMems,
	}
	if resources.CPUShares != 0 {
		shares := uint64(resources.CPUShares)
		cpu.Shares = &shares
	}

	var (
		period uint64
		quota  int64
	)
	if resources.NanoCPUs != 0 {
		period = uint64(100 * time.Millisecond / time.Microsecond)
		quota = resources.NanoCPUs * int64(period) / 1e9
	}
	if quota == 0 && resources.CPUQuota != 0 {
		quota = resources.CPUQuota
	}
	if period == 0 && resources.CPUPeriod != 0 {
		period = uint64(resources.CPUPeriod)
	}

	if period != 0 {
		cpu.Period = &period
	}
	if quota != 0 {
		cpu.Quota = &quota
	}

	if cpu != (specs.LinuxCPU{}) {
		r.CPU = &cpu
	}

	var memory specs.LinuxMemory
	if resources.Memory != 0 {
		memory.Limit = &resources.Memory
	}
	if resources.MemoryReservation != 0 {
		memory.Reservation = &resources.MemoryReservation
	}
	if resources.MemorySwap > 0 {
		memory.Swap = &resources.MemorySwap
	}

	if memory != (specs.LinuxMemory{}) {
		r.Memory = &memory
	}

	r.Pids = getPidsLimit(resources)
	return &r, nil
}
