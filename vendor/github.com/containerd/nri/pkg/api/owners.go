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

// TODO: Add comments to exported methods and functions.
//
//nolint:revive // exported symbols should have comments
package api

import (
	"fmt"
	"strings"
)

//
// Notes:
//   OwningPlugins, FieldOwners and CompoundFieldOwners are not protected
//   against concurrent access and therefore not goroutine safe.
//
//   None of these functions are used by plugins directly. These are used by
//   the runtime adaptation code to track container adjustments and updates
//   requested by plugins, and to detect conflicting requests.
//

func NewOwningPlugins() *OwningPlugins {
	return &OwningPlugins{
		Owners: make(map[string]*FieldOwners),
	}
}

func (o *OwningPlugins) ClaimAnnotation(id, key, plugin string) error {
	return o.mustOwnersFor(id).ClaimAnnotation(key, plugin)
}

func (o *OwningPlugins) ClaimMount(id, destination, plugin string) error {
	return o.mustOwnersFor(id).ClaimMount(destination, plugin)
}

func (o *OwningPlugins) ClaimHooks(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimHooks(plugin)
}

func (o *OwningPlugins) ClaimDevice(id, path, plugin string) error {
	return o.mustOwnersFor(id).ClaimDevice(path, plugin)
}

func (o *OwningPlugins) ClaimNamespace(id, typ, plugin string) error {
	return o.mustOwnersFor(id).ClaimNamespace(typ, plugin)
}

func (o *OwningPlugins) ClaimCdiDevice(id, name, plugin string) error {
	return o.mustOwnersFor(id).ClaimCdiDevice(name, plugin)
}

func (o *OwningPlugins) ClaimEnv(id, name, plugin string) error {
	return o.mustOwnersFor(id).ClaimEnv(name, plugin)
}

func (o *OwningPlugins) ClaimArgs(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimArgs(plugin)
}

func (o *OwningPlugins) ClaimMemLimit(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimMemLimit(plugin)
}

func (o *OwningPlugins) ClaimMemReservation(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimMemReservation(plugin)
}

func (o *OwningPlugins) ClaimMemSwapLimit(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimMemSwapLimit(plugin)
}

func (o *OwningPlugins) ClaimMemKernelLimit(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimMemKernelLimit(plugin)
}

func (o *OwningPlugins) ClaimMemTCPLimit(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimMemTCPLimit(plugin)
}

func (o *OwningPlugins) ClaimMemSwappiness(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimMemSwappiness(plugin)
}

func (o *OwningPlugins) ClaimMemDisableOomKiller(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimMemDisableOomKiller(plugin)
}

func (o *OwningPlugins) ClaimMemUseHierarchy(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimMemUseHierarchy(plugin)
}

func (o *OwningPlugins) ClaimCPUShares(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimCPUShares(plugin)
}

func (o *OwningPlugins) ClaimCPUQuota(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimCPUQuota(plugin)
}

func (o *OwningPlugins) ClaimCPUPeriod(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimCPUPeriod(plugin)
}

func (o *OwningPlugins) ClaimCPURealtimeRuntime(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimCPURealtimeRuntime(plugin)
}

func (o *OwningPlugins) ClaimCPURealtimePeriod(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimCPURealtimePeriod(plugin)
}

func (o *OwningPlugins) ClaimCPUSetCPUs(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimCPUSetCPUs(plugin)
}

func (o *OwningPlugins) ClaimCPUSetMems(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimCPUSetMems(plugin)
}

func (o *OwningPlugins) ClaimPidsLimit(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimPidsLimit(plugin)
}

func (o *OwningPlugins) ClaimHugepageLimit(id, size, plugin string) error {
	return o.mustOwnersFor(id).ClaimHugepageLimit(size, plugin)
}

func (o *OwningPlugins) ClaimBlockioClass(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimBlockioClass(plugin)
}

func (o *OwningPlugins) ClaimRdtClass(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimRdtClass(plugin)
}

func (o *OwningPlugins) ClaimRdtClosID(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimRdtClosID(plugin)
}

func (o *OwningPlugins) ClaimRdtSchemata(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimRdtSchemata(plugin)
}

func (o *OwningPlugins) ClaimRdtEnableMonitoring(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimRdtEnableMonitoring(plugin)
}

func (o *OwningPlugins) ClaimCgroupsUnified(id, key, plugin string) error {
	return o.mustOwnersFor(id).ClaimCgroupsUnified(key, plugin)
}

func (o *OwningPlugins) ClaimCgroupsPath(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimCgroupsPath(plugin)
}

func (o *OwningPlugins) ClaimOomScoreAdj(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimOomScoreAdj(plugin)
}

func (o *OwningPlugins) ClaimLinuxScheduler(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimLinuxScheduler(plugin)
}

func (o *OwningPlugins) ClaimRlimit(id, typ, plugin string) error {
	return o.mustOwnersFor(id).ClaimRlimit(typ, plugin)
}

func (o *OwningPlugins) ClaimIOPriority(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimIOPriority(plugin)
}

func (o *OwningPlugins) ClaimSeccompPolicy(id, plugin string) error {
	return o.mustOwnersFor(id).ClaimSeccompPolicy(plugin)
}

func (o *OwningPlugins) ClaimSysctl(id, key, plugin string) error {
	return o.mustOwnersFor(id).ClaimSysctl(key, plugin)
}

func (o *OwningPlugins) ClaimLinuxNetDevice(id, path, plugin string) error {
	return o.mustOwnersFor(id).ClaimLinuxNetDevice(path, plugin)
}

func (o *OwningPlugins) ClearAnnotation(id, key, plugin string) {
	o.mustOwnersFor(id).ClearAnnotation(key, plugin)
}

func (o *OwningPlugins) ClearMount(id, key, plugin string) {
	o.mustOwnersFor(id).ClearMount(key, plugin)
}

func (o *OwningPlugins) ClearDevice(id, key, plugin string) {
	o.mustOwnersFor(id).ClearDevice(key, plugin)
}

func (o *OwningPlugins) ClearEnv(id, key, plugin string) {
	o.mustOwnersFor(id).ClearEnv(key, plugin)
}

func (o *OwningPlugins) ClearArgs(id, plugin string) {
	o.mustOwnersFor(id).ClearArgs(plugin)
}

func (o *OwningPlugins) ClearSysctl(id, key, plugin string) {
	o.mustOwnersFor(id).ClearSysctl(key, plugin)
}

func (o *OwningPlugins) ClearLinuxNetDevice(id, path, plugin string) {
	o.mustOwnersFor(id).ClearLinuxNetDevice(path, plugin)
}

func (o *OwningPlugins) ClearRdt(id, plugin string) {
	o.mustOwnersFor(id).ClearRdt(plugin)
}

func (o *OwningPlugins) AnnotationOwner(id, key string) (string, bool) {
	return o.ownersFor(id).compoundOwner(Field_Annotations.Key(), key)
}

func (o *OwningPlugins) MountOwner(id, destination string) (string, bool) {
	return o.ownersFor(id).compoundOwner(Field_Mounts.Key(), destination)
}

func (o *OwningPlugins) HooksOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_OciHooks.Key())
}

func (o *OwningPlugins) DeviceOwner(id, path string) (string, bool) {
	return o.ownersFor(id).compoundOwner(Field_Devices.Key(), path)
}

func (o *OwningPlugins) NamespaceOwner(id, path string) (string, bool) {
	return o.ownersFor(id).compoundOwner(Field_Namespace.Key(), path)
}

func (o *OwningPlugins) NamespaceOwners(id string) (map[string]string, bool) {
	return o.ownersFor(id).compoundOwnerMap(Field_Namespace.Key())
}

func (o *OwningPlugins) EnvOwner(id, name string) (string, bool) {
	return o.ownersFor(id).compoundOwner(Field_Env.Key(), name)
}

func (o *OwningPlugins) ArgsOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_Args.Key())
}

func (o *OwningPlugins) MemLimitOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_MemLimit.Key())
}

func (o *OwningPlugins) MemReservationOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_MemReservation.Key())
}

func (o *OwningPlugins) MemSwapLimitOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_MemSwapLimit.Key())
}

func (o *OwningPlugins) MemKernelLimitOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_MemKernelLimit.Key())
}

func (o *OwningPlugins) MemTCPLimitOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_MemTCPLimit.Key())
}

func (o *OwningPlugins) MemSwappinessOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_MemSwappiness.Key())
}

func (o *OwningPlugins) MemDisableOomKillerOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_MemDisableOomKiller.Key())
}

func (o *OwningPlugins) MemUseHierarchyOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_MemUseHierarchy.Key())
}

func (o *OwningPlugins) CPUSharesOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_CPUShares.Key())
}

func (o *OwningPlugins) CPUQuotaOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_CPUQuota.Key())
}

func (o *OwningPlugins) CPUPeriodOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_CPUPeriod.Key())
}

func (o *OwningPlugins) CPURealtimeRuntimeOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_CPURealtimeRuntime.Key())
}

func (o *OwningPlugins) CPURealtimePeriodOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_CPURealtimePeriod.Key())
}

func (o *OwningPlugins) CPUSetCPUsOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_CPUSetCPUs.Key())
}

func (o *OwningPlugins) CPUSetMemsOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_CPUSetMems.Key())
}

func (o *OwningPlugins) PidsLimitOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_PidsLimit.Key())
}

func (o *OwningPlugins) HugepageLimitOwner(id, size string) (string, bool) {
	return o.ownersFor(id).compoundOwner(Field_HugepageLimits.Key(), size)
}

func (o *OwningPlugins) BlockioClassOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_BlockioClass.Key())
}

func (o *OwningPlugins) RdtClassOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_RdtClass.Key())
}

func (o *OwningPlugins) RdtClosIDOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_RdtClosID.Key())
}

func (o *OwningPlugins) RdtSchemataOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_RdtSchemata.Key())
}

func (o *OwningPlugins) RdtEnableMonitoringOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_RdtEnableMonitoring.Key())
}

func (o *OwningPlugins) CgroupsUnifiedOwner(id, key string) (string, bool) {
	return o.ownersFor(id).compoundOwner(Field_CgroupsUnified.Key(), key)
}

func (o *OwningPlugins) CgroupsPathOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_CgroupsPath.Key())
}

func (o *OwningPlugins) OomScoreAdjOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_OomScoreAdj.Key())
}

func (o *OwningPlugins) LinuxScheduler(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_LinuxSched.Key())
}

func (o *OwningPlugins) RlimitOwner(id, typ string) (string, bool) {
	return o.ownersFor(id).compoundOwner(Field_Rlimits.Key(), typ)
}

func (o *OwningPlugins) IOPriorityOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_IoPriority.Key())
}

func (o *OwningPlugins) SeccompPolicyOwner(id string) (string, bool) {
	return o.ownersFor(id).simpleOwner(Field_SeccompPolicy.Key())
}

func (o *OwningPlugins) SysctlOwner(id, key string) (string, bool) {
	return o.ownersFor(id).compoundOwner(Field_Sysctl.Key(), key)
}

func (o *OwningPlugins) LinuxNetDeviceOwner(id, path string) (string, bool) {
	return o.ownersFor(id).compoundOwner(Field_LinuxNetDevices.Key(), path)
}

func (o *OwningPlugins) mustOwnersFor(id string) *FieldOwners {
	f, ok := o.Owners[id]
	if !ok {
		f = NewFieldOwners()
		o.Owners[id] = f
	}
	return f
}

func (o *OwningPlugins) ownersFor(id string) *FieldOwners {
	f, ok := o.Owners[id]
	if !ok {
		return nil
	}
	return f
}

func NewFieldOwners() *FieldOwners {
	return &FieldOwners{
		Simple:   make(map[int32]string),
		Compound: make(map[int32]*CompoundFieldOwners),
	}
}

func (f *FieldOwners) IsCompoundConflict(field int32, key, plugin string) error {
	m, ok := f.Compound[field]
	if !ok {
		f.Compound[field] = NewCompoundFieldOwners()
		return nil
	}

	other, claimed := m.Owners[key]
	if !claimed {
		return nil
	}

	clearer, ok := IsMarkedForRemoval(other)
	if ok {
		if clearer == plugin {
			return nil
		}
		other = clearer
	}

	return f.Conflict(field, plugin, other, key)
}

func (f *FieldOwners) IsSimpleConflict(field int32, plugin string) error {
	other, claimed := f.Simple[field]
	if !claimed {
		return nil
	}

	clearer, ok := IsMarkedForRemoval(other)
	if ok {
		if clearer == plugin {
			return nil
		}
		other = clearer
	}

	return f.Conflict(field, plugin, other)
}

func (f *FieldOwners) claimCompound(field int32, entry, plugin string) error {
	if err := f.IsCompoundConflict(field, entry, plugin); err != nil {
		return err
	}

	f.Compound[field].Owners[entry] = plugin
	return nil
}

func (f *FieldOwners) claimSimple(field int32, plugin string) error {
	if err := f.IsSimpleConflict(field, plugin); err != nil {
		return err
	}

	f.Simple[field] = plugin
	return nil
}

func (f *FieldOwners) ClaimAnnotation(key, plugin string) error {
	return f.claimCompound(Field_Annotations.Key(), key, plugin)
}

func (f *FieldOwners) ClaimMount(destination, plugin string) error {
	return f.claimCompound(Field_Mounts.Key(), destination, plugin)
}

func (f *FieldOwners) ClaimHooks(plugin string) error {
	plugins := plugin

	if current, ok := f.simpleOwner(Field_OciHooks.Key()); ok {
		f.clearSimple(Field_OciHooks.Key(), plugin)
		plugins = current + "," + plugin
	}

	f.claimSimple(Field_OciHooks.Key(), plugins)
	return nil
}

func (f *FieldOwners) ClaimDevice(path, plugin string) error {
	return f.claimCompound(Field_Devices.Key(), path, plugin)
}

func (f *FieldOwners) ClaimCdiDevice(name, plugin string) error {
	return f.claimCompound(Field_CdiDevices.Key(), name, plugin)
}

func (f *FieldOwners) ClaimNamespace(typ, plugin string) error {
	return f.claimCompound(Field_Namespace.Key(), typ, plugin)
}

func (f *FieldOwners) ClaimEnv(name, plugin string) error {
	return f.claimCompound(Field_Env.Key(), name, plugin)
}

func (f *FieldOwners) ClaimArgs(plugin string) error {
	return f.claimSimple(Field_Args.Key(), plugin)
}

func (f *FieldOwners) ClaimMemLimit(plugin string) error {
	return f.claimSimple(Field_MemLimit.Key(), plugin)
}

func (f *FieldOwners) ClaimMemReservation(plugin string) error {
	return f.claimSimple(Field_MemReservation.Key(), plugin)
}

func (f *FieldOwners) ClaimMemSwapLimit(plugin string) error {
	return f.claimSimple(Field_MemSwapLimit.Key(), plugin)
}

func (f *FieldOwners) ClaimMemKernelLimit(plugin string) error {
	return f.claimSimple(Field_MemKernelLimit.Key(), plugin)
}

func (f *FieldOwners) ClaimMemTCPLimit(plugin string) error {
	return f.claimSimple(Field_MemTCPLimit.Key(), plugin)
}

func (f *FieldOwners) ClaimMemSwappiness(plugin string) error {
	return f.claimSimple(Field_MemSwappiness.Key(), plugin)
}

func (f *FieldOwners) ClaimMemDisableOomKiller(plugin string) error {
	return f.claimSimple(Field_MemDisableOomKiller.Key(), plugin)
}

func (f *FieldOwners) ClaimMemUseHierarchy(plugin string) error {
	return f.claimSimple(Field_MemUseHierarchy.Key(), plugin)
}

func (f *FieldOwners) ClaimCPUShares(plugin string) error {
	return f.claimSimple(Field_CPUShares.Key(), plugin)
}

func (f *FieldOwners) ClaimCPUQuota(plugin string) error {
	return f.claimSimple(Field_CPUQuota.Key(), plugin)
}

func (f *FieldOwners) ClaimCPUPeriod(plugin string) error {
	return f.claimSimple(Field_CPUPeriod.Key(), plugin)
}

func (f *FieldOwners) ClaimCPURealtimeRuntime(plugin string) error {
	return f.claimSimple(Field_CPURealtimeRuntime.Key(), plugin)
}

func (f *FieldOwners) ClaimCPURealtimePeriod(plugin string) error {
	return f.claimSimple(Field_CPURealtimePeriod.Key(), plugin)
}

func (f *FieldOwners) ClaimCPUSetCPUs(plugin string) error {
	return f.claimSimple(Field_CPUSetCPUs.Key(), plugin)
}

func (f *FieldOwners) ClaimCPUSetMems(plugin string) error {
	return f.claimSimple(Field_CPUSetMems.Key(), plugin)
}

func (f *FieldOwners) ClaimPidsLimit(plugin string) error {
	return f.claimSimple(Field_PidsLimit.Key(), plugin)
}

func (f *FieldOwners) ClaimHugepageLimit(size, plugin string) error {
	return f.claimCompound(Field_HugepageLimits.Key(), size, plugin)
}

func (f *FieldOwners) ClaimBlockioClass(plugin string) error {
	return f.claimSimple(Field_BlockioClass.Key(), plugin)
}

func (f *FieldOwners) ClaimRdtClass(plugin string) error {
	return f.claimSimple(Field_RdtClass.Key(), plugin)
}

func (f *FieldOwners) ClaimRdtClosID(plugin string) error {
	return f.claimSimple(Field_RdtClosID.Key(), plugin)
}

func (f *FieldOwners) ClaimRdtSchemata(plugin string) error {
	return f.claimSimple(Field_RdtSchemata.Key(), plugin)
}

func (f *FieldOwners) ClaimRdtEnableMonitoring(plugin string) error {
	return f.claimSimple(Field_RdtEnableMonitoring.Key(), plugin)
}

func (f *FieldOwners) ClaimCgroupsUnified(key, plugin string) error {
	return f.claimCompound(Field_CgroupsUnified.Key(), key, plugin)
}

func (f *FieldOwners) ClaimCgroupsPath(plugin string) error {
	return f.claimSimple(Field_CgroupsPath.Key(), plugin)
}

func (f *FieldOwners) ClaimOomScoreAdj(plugin string) error {
	return f.claimSimple(Field_OomScoreAdj.Key(), plugin)
}

func (f *FieldOwners) ClaimLinuxScheduler(plugin string) error {
	return f.claimSimple(Field_LinuxSched.Key(), plugin)
}

func (f *FieldOwners) ClaimRlimit(typ, plugin string) error {
	return f.claimCompound(Field_Rlimits.Key(), typ, plugin)
}

func (f *FieldOwners) ClaimIOPriority(plugin string) error {
	return f.claimSimple(Field_IoPriority.Key(), plugin)
}

func (f *FieldOwners) ClaimSeccompPolicy(plugin string) error {
	return f.claimSimple(Field_SeccompPolicy.Key(), plugin)
}

func (f *FieldOwners) ClaimSysctl(key, plugin string) error {
	return f.claimCompound(Field_Sysctl.Key(), key, plugin)
}

func (f *FieldOwners) ClaimLinuxNetDevice(path, plugin string) error {
	return f.claimCompound(Field_LinuxNetDevices.Key(), path, plugin)
}

func (f *FieldOwners) clearCompound(field int32, key, plugin string) {
	m, ok := f.Compound[field]
	if !ok {
		m = NewCompoundFieldOwners()
		f.Compound[field] = m
	}

	m.Owners[key] = MarkForRemoval(plugin)
}

func (f *FieldOwners) clearSimple(field int32, plugin string) {
	f.Simple[field] = MarkForRemoval(plugin)
}

func (f *FieldOwners) ClearAnnotation(key, plugin string) {
	f.clearCompound(Field_Annotations.Key(), key, plugin)
}

func (f *FieldOwners) ClearMount(destination, plugin string) {
	f.clearCompound(Field_Mounts.Key(), destination, plugin)
}

func (f *FieldOwners) ClearDevice(path, plugin string) {
	f.clearCompound(Field_Devices.Key(), path, plugin)
}

func (f *FieldOwners) ClearEnv(name, plugin string) {
	f.clearCompound(Field_Env.Key(), name, plugin)
}

func (f *FieldOwners) ClearArgs(plugin string) {
	f.clearSimple(Field_Args.Key(), plugin)
}

func (f *FieldOwners) ClearSysctl(key, plugin string) {
	f.clearCompound(Field_Sysctl.Key(), key, plugin)
}

func (f *FieldOwners) ClearLinuxNetDevice(key, plugin string) {
	f.clearCompound(Field_LinuxNetDevices.Key(), key, plugin)
}

func (f *FieldOwners) ClearRdt(plugin string) {
	f.clearSimple(Field_RdtClosID.Key(), plugin)
	f.clearSimple(Field_RdtSchemata.Key(), plugin)
	f.clearSimple(Field_RdtEnableMonitoring.Key(), plugin)
}

func (f *FieldOwners) Conflict(field int32, plugin, other string, qualifiers ...string) error {
	return fmt.Errorf("plugins %q and %q both tried to set %s",
		plugin, other, qualify(field, qualifiers...))
}

func (f *FieldOwners) compoundOwnerMap(field int32) (map[string]string, bool) {
	if f == nil {
		return nil, false
	}

	m, ok := f.Compound[field]
	if !ok {
		return nil, false
	}

	return m.Owners, true
}

func (f *FieldOwners) compoundOwner(field int32, key string) (string, bool) {
	if f == nil {
		return "", false
	}

	m, ok := f.Compound[field]
	if !ok {
		return "", false
	}

	plugin, ok := m.Owners[key]
	return plugin, ok
}

func (f *FieldOwners) simpleOwner(field int32) (string, bool) {
	if f == nil {
		return "", false
	}

	plugin, ok := f.Simple[field]
	return plugin, ok
}

func (f *FieldOwners) AnnotationOwner(key string) (string, bool) {
	return f.compoundOwner(Field_Annotations.Key(), key)
}

func (f *FieldOwners) MountOwner(destination string) (string, bool) {
	return f.compoundOwner(Field_Mounts.Key(), destination)
}

func (f *FieldOwners) DeviceOwner(path string) (string, bool) {
	return f.compoundOwner(Field_Devices.Key(), path)
}

func (f *FieldOwners) NamespaceOwner(typ string) (string, bool) {
	return f.compoundOwner(Field_Devices.Key(), typ)
}

func (f *FieldOwners) EnvOwner(name string) (string, bool) {
	return f.compoundOwner(Field_Env.Key(), name)
}

func (f *FieldOwners) ArgsOwner() (string, bool) {
	return f.simpleOwner(Field_Args.Key())
}

func (f *FieldOwners) MemLimitOwner() (string, bool) {
	return f.simpleOwner(Field_MemLimit.Key())
}

func (f *FieldOwners) MemReservationOwner() (string, bool) {
	return f.simpleOwner(Field_MemReservation.Key())
}

func (f *FieldOwners) MemSwapLimitOwner() (string, bool) {
	return f.simpleOwner(Field_MemSwapLimit.Key())
}

func (f *FieldOwners) MemKernelLimitOwner() (string, bool) {
	return f.simpleOwner(Field_MemKernelLimit.Key())
}

func (f *FieldOwners) MemTCPLimitOwner() (string, bool) {
	return f.simpleOwner(Field_MemTCPLimit.Key())
}

func (f *FieldOwners) MemSwappinessOwner() (string, bool) {
	return f.simpleOwner(Field_MemSwappiness.Key())
}

func (f *FieldOwners) MemDisableOomKillerOwner() (string, bool) {
	return f.simpleOwner(Field_MemDisableOomKiller.Key())
}

func (f *FieldOwners) MemUseHierarchyOwner() (string, bool) {
	return f.simpleOwner(Field_MemUseHierarchy.Key())
}

func (f *FieldOwners) CPUSharesOwner() (string, bool) {
	return f.simpleOwner(Field_CPUShares.Key())
}

func (f *FieldOwners) CPUQuotaOwner() (string, bool) {
	return f.simpleOwner(Field_CPUQuota.Key())
}

func (f *FieldOwners) CPUPeriodOwner() (string, bool) {
	return f.simpleOwner(Field_CPUPeriod.Key())
}

func (f *FieldOwners) CPURealtimeRuntimeOwner() (string, bool) {
	return f.simpleOwner(Field_CPURealtimeRuntime.Key())
}

func (f *FieldOwners) CPURealtimePeriodOwner() (string, bool) {
	return f.simpleOwner(Field_CPURealtimePeriod.Key())
}

func (f *FieldOwners) CPUSetCPUsOwner() (string, bool) {
	return f.simpleOwner(Field_CPUSetCPUs.Key())
}

func (f *FieldOwners) CPUSetMemsOwner() (string, bool) {
	return f.simpleOwner(Field_CPUSetMems.Key())
}

func (f *FieldOwners) PidsLimitOwner() (string, bool) {
	return f.simpleOwner(Field_PidsLimit.Key())
}

func (f *FieldOwners) HugepageLimitOwner(size string) (string, bool) {
	return f.compoundOwner(Field_HugepageLimits.Key(), size)
}

func (f *FieldOwners) BlockioClassOwner() (string, bool) {
	return f.simpleOwner(Field_BlockioClass.Key())
}

func (f *FieldOwners) RdtClassOwner() (string, bool) {
	return f.simpleOwner(Field_RdtClass.Key())
}

func (f *FieldOwners) RdtSchemataOwner() (string, bool) {
	return f.simpleOwner(Field_RdtSchemata.Key())
}

func (f *FieldOwners) RdtEnableMonitoringOwner() (string, bool) {
	return f.simpleOwner(Field_RdtEnableMonitoring.Key())
}

func (f *FieldOwners) CgroupsUnifiedOwner(key string) (string, bool) {
	return f.compoundOwner(Field_CgroupsUnified.Key(), key)
}

func (f *FieldOwners) CgroupsPathOwner() (string, bool) {
	return f.simpleOwner(Field_CgroupsPath.Key())
}

func (f *FieldOwners) OomScoreAdjOwner() (string, bool) {
	return f.simpleOwner(Field_OomScoreAdj.Key())
}

func (f *FieldOwners) RlimitOwner(typ string) (string, bool) {
	return f.compoundOwner(Field_Rlimits.Key(), typ)
}

func qualify(field int32, qualifiers ...string) string {
	return Field(field).String() + " " + strings.Join(append([]string{}, qualifiers...), " ")
}

func NewCompoundFieldOwners() *CompoundFieldOwners {
	return &CompoundFieldOwners{
		Owners: make(map[string]string),
	}
}

func (f Field) Key() int32 {
	return int32(f)
}
