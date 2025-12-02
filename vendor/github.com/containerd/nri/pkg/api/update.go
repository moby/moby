/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package api

//nolint
// SetContainerId sets the id of the container to update.
func (u *ContainerUpdate) SetContainerId(id string) {
	u.ContainerId = id
}

// SetLinuxMemoryLimit records setting the memory limit for a container.
func (u *ContainerUpdate) SetLinuxMemoryLimit(value int64) {
	u.initLinuxResourcesMemory()
	u.Linux.Resources.Memory.Limit = Int64(value)
}

// SetLinuxMemoryReservation records setting the memory reservation for a container.
func (u *ContainerUpdate) SetLinuxMemoryReservation(value int64) {
	u.initLinuxResourcesMemory()
	u.Linux.Resources.Memory.Reservation = Int64(value)
}

// SetLinuxMemorySwap records records setting the memory swap limit for a container.
func (u *ContainerUpdate) SetLinuxMemorySwap(value int64) {
	u.initLinuxResourcesMemory()
	u.Linux.Resources.Memory.Swap = Int64(value)
}

// SetLinuxMemoryKernel records setting the memory kernel limit for a container.
func (u *ContainerUpdate) SetLinuxMemoryKernel(value int64) {
	u.initLinuxResourcesMemory()
	u.Linux.Resources.Memory.Kernel = Int64(value)
}

// SetLinuxMemoryKernelTCP records setting the memory kernel TCP limit for a container.
func (u *ContainerUpdate) SetLinuxMemoryKernelTCP(value int64) {
	u.initLinuxResourcesMemory()
	u.Linux.Resources.Memory.KernelTcp = Int64(value)
}

// SetLinuxMemorySwappiness records setting the memory swappiness for a container.
func (u *ContainerUpdate) SetLinuxMemorySwappiness(value uint64) {
	u.initLinuxResourcesMemory()
	u.Linux.Resources.Memory.Swappiness = UInt64(value)
}

// SetLinuxMemoryDisableOomKiller records disabling the OOM killer for a container.
func (u *ContainerUpdate) SetLinuxMemoryDisableOomKiller() {
	u.initLinuxResourcesMemory()
	u.Linux.Resources.Memory.DisableOomKiller = Bool(true)
}

// SetLinuxMemoryUseHierarchy records enabling hierarchical memory accounting for a container.
func (u *ContainerUpdate) SetLinuxMemoryUseHierarchy() {
	u.initLinuxResourcesMemory()
	u.Linux.Resources.Memory.UseHierarchy = Bool(true)
}

// SetLinuxCPUShares records setting the scheduler's CPU shares for a container.
func (u *ContainerUpdate) SetLinuxCPUShares(value uint64) {
	u.initLinuxResourcesCPU()
	u.Linux.Resources.Cpu.Shares = UInt64(value)
}

// SetLinuxCPUQuota records setting the scheduler's CPU quota for a container.
func (u *ContainerUpdate) SetLinuxCPUQuota(value int64) {
	u.initLinuxResourcesCPU()
	u.Linux.Resources.Cpu.Quota = Int64(value)
}

// SetLinuxCPUPeriod records setting the scheduler's CPU period for a container.
func (u *ContainerUpdate) SetLinuxCPUPeriod(value int64) {
	u.initLinuxResourcesCPU()
	u.Linux.Resources.Cpu.Period = UInt64(value)
}

// SetLinuxCPURealtimeRuntime records setting the scheduler's realtime runtime for a container.
func (u *ContainerUpdate) SetLinuxCPURealtimeRuntime(value int64) {
	u.initLinuxResourcesCPU()
	u.Linux.Resources.Cpu.RealtimeRuntime = Int64(value)
}

// SetLinuxCPURealtimePeriod records setting the scheduler's realtime period for a container.
func (u *ContainerUpdate) SetLinuxCPURealtimePeriod(value uint64) {
	u.initLinuxResourcesCPU()
	u.Linux.Resources.Cpu.RealtimePeriod = UInt64(value)
}

// SetLinuxCPUSetCPUs records setting the cpuset CPUs for a container.
func (u *ContainerUpdate) SetLinuxCPUSetCPUs(value string) {
	u.initLinuxResourcesCPU()
	u.Linux.Resources.Cpu.Cpus = value
}

// SetLinuxCPUSetMems records setting the cpuset memory for a container.
func (u *ContainerUpdate) SetLinuxCPUSetMems(value string) {
	u.initLinuxResourcesCPU()
	u.Linux.Resources.Cpu.Mems = value
}

// SetLinuxPidLimits records setting the pid max number for a container.
func (u *ContainerUpdate) SetLinuxPidLimits(value int64) {
	u.initLinuxResourcesPids()
	u.Linux.Resources.Pids.Limit = value
}

// AddLinuxHugepageLimit records adding a hugepage limit for a container.
func (u *ContainerUpdate) AddLinuxHugepageLimit(pageSize string, value uint64) {
	u.initLinuxResources()
	u.Linux.Resources.HugepageLimits = append(u.Linux.Resources.HugepageLimits,
		&HugepageLimit{
			PageSize: pageSize,
			Limit:    value,
		})
}

// SetLinuxBlockIOClass records setting the Block I/O class for a container.
func (u *ContainerUpdate) SetLinuxBlockIOClass(value string) {
	u.initLinuxResources()
	u.Linux.Resources.BlockioClass = String(value)
}

// SetLinuxRDTClass records setting the RDT class for a container.
func (u *ContainerUpdate) SetLinuxRDTClass(value string) {
	u.initLinuxResources()
	u.Linux.Resources.RdtClass = String(value)
}

// AddLinuxUnified sets a cgroupv2 unified resource.
func (u *ContainerUpdate) AddLinuxUnified(key, value string) {
	u.initLinuxResourcesUnified()
	u.Linux.Resources.Unified[key] = value
}

// SetIgnoreFailure marks an Update as ignored for failures.
// Such updates will not prevent the related container operation
// from succeeding if the update fails.
func (u *ContainerUpdate) SetIgnoreFailure() {
	u.IgnoreFailure = true
}

//
// Initializing a container update.
//

func (u *ContainerUpdate) initLinux() {
	if u.Linux == nil {
		u.Linux = &LinuxContainerUpdate{}
	}
}

func (u *ContainerUpdate) initLinuxResources() {
	u.initLinux()
	if u.Linux.Resources == nil {
		u.Linux.Resources = &LinuxResources{}
	}
}

func (u *ContainerUpdate) initLinuxResourcesMemory() {
	u.initLinuxResources()
	if u.Linux.Resources.Memory == nil {
		u.Linux.Resources.Memory = &LinuxMemory{}
	}
}

func (u *ContainerUpdate) initLinuxResourcesCPU() {
	u.initLinuxResources()
	if u.Linux.Resources.Cpu == nil {
		u.Linux.Resources.Cpu = &LinuxCPU{}
	}
}

func (u *ContainerUpdate) initLinuxResourcesUnified() {
	u.initLinuxResources()
	if u.Linux.Resources.Unified == nil {
		u.Linux.Resources.Unified = make(map[string]string)
	}
}

func (u *ContainerUpdate) initLinuxResourcesPids() {
	u.initLinuxResources()
	if u.Linux.Resources.Pids == nil {
		u.Linux.Resources.Pids = &LinuxPids{}
	}
}
