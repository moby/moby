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

import "slices"

//
// Notes:
//   Adjustment of metadata that is stored in maps (labels and annotations)
//   currently assumes that a single plugin will never do an add prior to a
//   delete for any key. IOW, it is always assumed that if both a deletion
//   and an addition/setting was recorded for a key then the final desired
//   state is the addition. This seems like a reasonably safe assumption. A
//   removal is usually done only to protect against triggering the conflict
//   in the runtime when a plugin intends to touch a key which is known to
//   have been put there or already modified by another plugin.
//
//   An alternative without this implicit ordering assumption would be to
//   store the adjustment for such data as a sequence of add/del operations
//   in a slice. At the moment that does not seem to be necessary.
//

// AddAnnotation records the addition of the annotation key=value.
func (a *ContainerAdjustment) AddAnnotation(key, value string) {
	a.initAnnotations()
	a.Annotations[key] = value
}

// RemoveAnnotation records the removal of the annotation for the given key.
// Normally it is an error for a plugin to try and alter an annotation
// touched by another plugin. However, this is not an error if the plugin
// removes that annotation prior to touching it.
func (a *ContainerAdjustment) RemoveAnnotation(key string) {
	a.initAnnotations()
	a.Annotations[MarkForRemoval(key)] = ""
}

// AddMount records the addition of a mount to a container.
func (a *ContainerAdjustment) AddMount(m *Mount) {
	a.Mounts = append(a.Mounts, m) // TODO: should we dup m here ?
}

// RemoveMount records the removal of a mount from a container.
// Normally it is an error for a plugin to try and alter a mount
// touched by another plugin. However, this is not an error if the
// plugin removes that mount prior to touching it.
func (a *ContainerAdjustment) RemoveMount(ContainerPath string) {
	a.Mounts = append(a.Mounts, &Mount{
		Destination: MarkForRemoval(ContainerPath),
	})
}

// AddEnv records the addition of an environment variable to a container.
func (a *ContainerAdjustment) AddEnv(key, value string) {
	a.Env = append(a.Env, &KeyValue{
		Key:   key,
		Value: value,
	})
}

// RemoveEnv records the removal of an environment variable from a container.
// Normally it is an error for a plugin to try and alter an environment
// variable touched by another container. However, this is not an error if
// the plugin removes that variable prior to touching it.
func (a *ContainerAdjustment) RemoveEnv(key string) {
	a.Env = append(a.Env, &KeyValue{
		Key: MarkForRemoval(key),
	})
}

// SetArgs overrides the container command with the given arguments.
func (a *ContainerAdjustment) SetArgs(args []string) {
	a.Args = slices.Clone(args)
}

// UpdateArgs overrides the container command with the given arguments.
// It won't fail if another plugin has already set the command line.
func (a *ContainerAdjustment) UpdateArgs(args []string) {
	a.Args = append([]string{""}, args...)
}

// AddHooks records the addition of the given hooks to a container.
func (a *ContainerAdjustment) AddHooks(h *Hooks) {
	a.initHooks()
	if h.Prestart != nil {
		a.Hooks.Prestart = append(a.Hooks.Prestart, h.Prestart...)
	}
	if h.CreateRuntime != nil {
		a.Hooks.CreateRuntime = append(a.Hooks.CreateRuntime, h.CreateRuntime...)
	}
	if h.CreateContainer != nil {
		a.Hooks.CreateContainer = append(a.Hooks.CreateContainer, h.CreateContainer...)
	}
	if h.StartContainer != nil {
		a.Hooks.StartContainer = append(a.Hooks.StartContainer, h.StartContainer...)
	}
	if h.Poststart != nil {
		a.Hooks.Poststart = append(a.Hooks.Poststart, h.Poststart...)
	}
	if h.Poststop != nil {
		a.Hooks.Poststop = append(a.Hooks.Poststop, h.Poststop...)
	}
}

func (a *ContainerAdjustment) AddRlimit(typ string, hard, soft uint64) {
	a.initRlimits()
	a.Rlimits = append(a.Rlimits, &POSIXRlimit{
		Type: typ,
		Hard: hard,
		Soft: soft,
	})
}

// AddDevice records the addition of the given device to a container.
func (a *ContainerAdjustment) AddDevice(d *LinuxDevice) {
	a.initLinux()
	a.Linux.Devices = append(a.Linux.Devices, d) // TODO: should we dup d here ?
}

// RemoveDevice records the removal of a device from a container.
// Normally it is an error for a plugin to try and alter an device
// touched by another container. However, this is not an error if
// the plugin removes that device prior to touching it.
func (a *ContainerAdjustment) RemoveDevice(path string) {
	a.initLinux()
	a.Linux.Devices = append(a.Linux.Devices, &LinuxDevice{
		Path: MarkForRemoval(path),
	})
}

// AddCDIDevice records the addition of the given CDI device to a container.
func (a *ContainerAdjustment) AddCDIDevice(d *CDIDevice) {
	a.CDIDevices = append(a.CDIDevices, d) // TODO: should we dup d here ?
}

// AddOrReplaceNamespace records the addition or replacement of the given namespace to a container.
func (a *ContainerAdjustment) AddOrReplaceNamespace(n *LinuxNamespace) {
	a.initLinuxNamespaces()
	a.Linux.Namespaces = append(a.Linux.Namespaces, n) // TODO: should we dup n here ?
}

// RemoveNamespace records the removal of the given namespace from a container.
func (a *ContainerAdjustment) RemoveNamespace(n *LinuxNamespace) {
	a.initLinuxNamespaces()
	a.Linux.Namespaces = append(a.Linux.Namespaces, &LinuxNamespace{
		Type: MarkForRemoval(n.Type),
	})
}

// SetLinuxMemoryLimit records setting the memory limit for a container.
func (a *ContainerAdjustment) SetLinuxMemoryLimit(value int64) {
	a.initLinuxResourcesMemory()
	a.Linux.Resources.Memory.Limit = Int64(value)
}

// SetLinuxMemoryReservation records setting the memory reservation for a container.
func (a *ContainerAdjustment) SetLinuxMemoryReservation(value int64) {
	a.initLinuxResourcesMemory()
	a.Linux.Resources.Memory.Reservation = Int64(value)
}

// SetLinuxMemorySwap records records setting the memory swap limit for a container.
func (a *ContainerAdjustment) SetLinuxMemorySwap(value int64) {
	a.initLinuxResourcesMemory()
	a.Linux.Resources.Memory.Swap = Int64(value)
}

// SetLinuxMemoryKernel records setting the memory kernel limit for a container.
func (a *ContainerAdjustment) SetLinuxMemoryKernel(value int64) {
	a.initLinuxResourcesMemory()
	a.Linux.Resources.Memory.Kernel = Int64(value)
}

// SetLinuxMemoryKernelTCP records setting the memory kernel TCP limit for a container.
func (a *ContainerAdjustment) SetLinuxMemoryKernelTCP(value int64) {
	a.initLinuxResourcesMemory()
	a.Linux.Resources.Memory.KernelTcp = Int64(value)
}

// SetLinuxMemorySwappiness records setting the memory swappiness for a container.
func (a *ContainerAdjustment) SetLinuxMemorySwappiness(value uint64) {
	a.initLinuxResourcesMemory()
	a.Linux.Resources.Memory.Swappiness = UInt64(value)
}

// SetLinuxMemoryDisableOomKiller records disabling the OOM killer for a container.
func (a *ContainerAdjustment) SetLinuxMemoryDisableOomKiller() {
	a.initLinuxResourcesMemory()
	a.Linux.Resources.Memory.DisableOomKiller = Bool(true)
}

// SetLinuxMemoryUseHierarchy records enabling hierarchical memory accounting for a container.
func (a *ContainerAdjustment) SetLinuxMemoryUseHierarchy() {
	a.initLinuxResourcesMemory()
	a.Linux.Resources.Memory.UseHierarchy = Bool(true)
}

// SetLinuxCPUShares records setting the scheduler's CPU shares for a container.
func (a *ContainerAdjustment) SetLinuxCPUShares(value uint64) {
	a.initLinuxResourcesCPU()
	a.Linux.Resources.Cpu.Shares = UInt64(value)
}

// SetLinuxCPUQuota records setting the scheduler's CPU quota for a container.
func (a *ContainerAdjustment) SetLinuxCPUQuota(value int64) {
	a.initLinuxResourcesCPU()
	a.Linux.Resources.Cpu.Quota = Int64(value)
}

// SetLinuxCPUPeriod records setting the scheduler's CPU period for a container.
func (a *ContainerAdjustment) SetLinuxCPUPeriod(value int64) {
	a.initLinuxResourcesCPU()
	a.Linux.Resources.Cpu.Period = UInt64(value)
}

// SetLinuxCPURealtimeRuntime records setting the scheduler's realtime runtime for a container.
func (a *ContainerAdjustment) SetLinuxCPURealtimeRuntime(value int64) {
	a.initLinuxResourcesCPU()
	a.Linux.Resources.Cpu.RealtimeRuntime = Int64(value)
}

// SetLinuxCPURealtimePeriod records setting the scheduler's realtime period for a container.
func (a *ContainerAdjustment) SetLinuxCPURealtimePeriod(value uint64) {
	a.initLinuxResourcesCPU()
	a.Linux.Resources.Cpu.RealtimePeriod = UInt64(value)
}

// SetLinuxCPUSetCPUs records setting the cpuset CPUs for a container.
func (a *ContainerAdjustment) SetLinuxCPUSetCPUs(value string) {
	a.initLinuxResourcesCPU()
	a.Linux.Resources.Cpu.Cpus = value
}

// SetLinuxCPUSetMems records setting the cpuset memory for a container.
func (a *ContainerAdjustment) SetLinuxCPUSetMems(value string) {
	a.initLinuxResourcesCPU()
	a.Linux.Resources.Cpu.Mems = value
}

// SetLinuxPidLimits records setting the pid max number for a container.
func (a *ContainerAdjustment) SetLinuxPidLimits(value int64) {
	a.initLinuxResourcesPids()
	a.Linux.Resources.Pids.Limit = value
}

// AddLinuxHugepageLimit records adding a hugepage limit for a container.
func (a *ContainerAdjustment) AddLinuxHugepageLimit(pageSize string, value uint64) {
	a.initLinuxResources()
	a.Linux.Resources.HugepageLimits = append(a.Linux.Resources.HugepageLimits,
		&HugepageLimit{
			PageSize: pageSize,
			Limit:    value,
		})
}

// SetLinuxBlockIOClass records setting the Block I/O class for a container.
func (a *ContainerAdjustment) SetLinuxBlockIOClass(value string) {
	a.initLinuxResources()
	a.Linux.Resources.BlockioClass = String(value)
}

// SetLinuxRDTClass records setting the RDT class for a container.
func (a *ContainerAdjustment) SetLinuxRDTClass(value string) {
	a.initLinuxResources()
	a.Linux.Resources.RdtClass = String(value)
}

// AddLinuxUnified sets a cgroupv2 unified resource.
func (a *ContainerAdjustment) AddLinuxUnified(key, value string) {
	a.initLinuxResourcesUnified()
	a.Linux.Resources.Unified[key] = value
}

// SetLinuxCgroupsPath records setting the cgroups path for a container.
func (a *ContainerAdjustment) SetLinuxCgroupsPath(value string) {
	a.initLinux()
	a.Linux.CgroupsPath = value
}

// SetLinuxOomScoreAdj records setting the kernel's Out-Of-Memory (OOM) killer score for a container.
func (a *ContainerAdjustment) SetLinuxOomScoreAdj(value *int) {
	a.initLinux()
	a.Linux.OomScoreAdj = Int(value) // using Int(value) from ./options.go to optionally allocate a pointer to normalized copy of value
}

// SetLinuxIOPriority records setting the I/O priority for a container.
func (a *ContainerAdjustment) SetLinuxIOPriority(ioprio *LinuxIOPriority) {
	a.initLinux()
	a.Linux.IoPriority = ioprio
}

// SetLinuxSeccompPolicy overrides the container seccomp policy with the given arguments.
func (a *ContainerAdjustment) SetLinuxSeccompPolicy(seccomp *LinuxSeccomp) {
	a.initLinux()
	a.Linux.SeccompPolicy = seccomp
}

//
// Initializing a container adjustment and container update.
//

func (a *ContainerAdjustment) initAnnotations() {
	if a.Annotations == nil {
		a.Annotations = make(map[string]string)
	}
}

func (a *ContainerAdjustment) initHooks() {
	if a.Hooks == nil {
		a.Hooks = &Hooks{}
	}
}

func (a *ContainerAdjustment) initRlimits() {
	if a.Rlimits == nil {
		a.Rlimits = []*POSIXRlimit{}
	}
}

func (a *ContainerAdjustment) initLinux() {
	if a.Linux == nil {
		a.Linux = &LinuxContainerAdjustment{}
	}
}

func (a *ContainerAdjustment) initLinuxNamespaces() {
	a.initLinux()
	if a.Linux.Namespaces == nil {
		a.Linux.Namespaces = []*LinuxNamespace{}
	}
}

func (a *ContainerAdjustment) initLinuxResources() {
	a.initLinux()
	if a.Linux.Resources == nil {
		a.Linux.Resources = &LinuxResources{}
	}
}

func (a *ContainerAdjustment) initLinuxResourcesMemory() {
	a.initLinuxResources()
	if a.Linux.Resources.Memory == nil {
		a.Linux.Resources.Memory = &LinuxMemory{}
	}
}

func (a *ContainerAdjustment) initLinuxResourcesCPU() {
	a.initLinuxResources()
	if a.Linux.Resources.Cpu == nil {
		a.Linux.Resources.Cpu = &LinuxCPU{}
	}
}

func (a *ContainerAdjustment) initLinuxResourcesPids() {
	a.initLinuxResources()
	if a.Linux.Resources.Pids == nil {
		a.Linux.Resources.Pids = &LinuxPids{}
	}
}

func (a *ContainerAdjustment) initLinuxResourcesUnified() {
	a.initLinuxResources()
	if a.Linux.Resources.Unified == nil {
		a.Linux.Resources.Unified = make(map[string]string)
	}
}
