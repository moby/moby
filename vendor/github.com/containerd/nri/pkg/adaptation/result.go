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

package adaptation

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/containerd/nri/pkg/api"
)

type result struct {
	request resultRequest
	reply   resultReply
	updates map[string]*ContainerUpdate
	owners  *api.OwningPlugins
}

type resultRequest struct {
	create *CreateContainerRequest
	update *UpdateContainerRequest
}

type resultReply struct {
	adjust *ContainerAdjustment
	update []*ContainerUpdate
}

func collectCreateContainerResult(request *CreateContainerRequest) *result {
	if request.Container.Labels == nil {
		request.Container.Labels = map[string]string{}
	}
	if request.Container.Annotations == nil {
		request.Container.Annotations = map[string]string{}
	}
	if request.Container.Mounts == nil {
		request.Container.Mounts = []*Mount{}
	}
	if request.Container.Env == nil {
		request.Container.Env = []string{}
	}
	if request.Container.Hooks == nil {
		request.Container.Hooks = &Hooks{}
	}
	if request.Container.Rlimits == nil {
		request.Container.Rlimits = []*POSIXRlimit{}
	}
	if request.Container.Linux == nil {
		request.Container.Linux = &LinuxContainer{}
	}
	if request.Container.Linux.Devices == nil {
		request.Container.Linux.Devices = []*LinuxDevice{}
	}
	if request.Container.Linux.Resources == nil {
		request.Container.Linux.Resources = &LinuxResources{}
	}
	if request.Container.Linux.Resources.Memory == nil {
		request.Container.Linux.Resources.Memory = &LinuxMemory{}
	}
	if request.Container.Linux.Resources.Cpu == nil {
		request.Container.Linux.Resources.Cpu = &LinuxCPU{}
	}
	if request.Container.Linux.Resources.Unified == nil {
		request.Container.Linux.Resources.Unified = map[string]string{}
	}
	if request.Container.Linux.Namespaces == nil {
		request.Container.Linux.Namespaces = []*LinuxNamespace{}
	}
	if request.Container.Linux.NetDevices == nil {
		request.Container.Linux.NetDevices = map[string]*LinuxNetDevice{}
	}

	return &result{
		request: resultRequest{
			create: request,
		},
		reply: resultReply{
			adjust: &ContainerAdjustment{
				Annotations: map[string]string{},
				Mounts:      []*Mount{},
				Env:         []*KeyValue{},
				Hooks:       &Hooks{},
				Rlimits:     []*POSIXRlimit{},
				CDIDevices:  []*CDIDevice{},
				Linux: &LinuxContainerAdjustment{
					Devices: []*LinuxDevice{},
					Resources: &LinuxResources{
						Memory:         &LinuxMemory{},
						Cpu:            &LinuxCPU{},
						HugepageLimits: []*HugepageLimit{},
						Unified:        map[string]string{},
					},
					Namespaces: []*LinuxNamespace{},
					NetDevices: map[string]*LinuxNetDevice{},
				},
			},
		},
		updates: map[string]*ContainerUpdate{},
		owners:  api.NewOwningPlugins(),
	}
}

func collectUpdateContainerResult(request *UpdateContainerRequest) *result {
	if request != nil {
		if request.LinuxResources == nil {
			request.LinuxResources = &LinuxResources{}
		}
		if request.LinuxResources.Memory == nil {
			request.LinuxResources.Memory = &LinuxMemory{}
		}
		if request.LinuxResources.Cpu == nil {
			request.LinuxResources.Cpu = &LinuxCPU{}
		}
	}

	return &result{
		request: resultRequest{
			update: request,
		},
		reply: resultReply{
			update: []*ContainerUpdate{},
		},
		updates: map[string]*ContainerUpdate{},
		owners:  api.NewOwningPlugins(),
	}
}

func collectStopContainerResult() *result {
	return collectUpdateContainerResult(nil)
}

func (r *result) createContainerResponse() *CreateContainerResponse {
	return &CreateContainerResponse{
		Adjust: r.reply.adjust,
		Update: r.reply.update,
	}
}

func (r *result) updateContainerResponse() *UpdateContainerResponse {
	requested := r.updates[r.request.update.Container.Id]
	return &UpdateContainerResponse{
		Update: append(r.reply.update, requested),
	}
}

func (r *result) stopContainerResponse() *StopContainerResponse {
	return &StopContainerResponse{
		Update: r.reply.update,
	}
}

func (r *result) apply(response interface{}, plugin string) error {
	switch rpl := response.(type) {
	case *CreateContainerResponse:
		if rpl == nil {
			return nil
		}
		if err := r.adjust(rpl.Adjust, plugin); err != nil {
			return err
		}
		if err := r.update(rpl.Update, plugin); err != nil {
			return err
		}
	case *UpdateContainerResponse:
		if rpl == nil {
			return nil
		}
		if err := r.update(rpl.Update, plugin); err != nil {
			return err
		}
	case *StopContainerResponse:
		if rpl == nil {
			return nil
		}
		if err := r.update(rpl.Update, plugin); err != nil {
			return err
		}
	default:
		return fmt.Errorf("cannot apply response of invalid type %T", response)
	}

	return nil
}

func (r *result) adjust(rpl *ContainerAdjustment, plugin string) error {
	if rpl == nil {
		return nil
	}
	if err := r.adjustAnnotations(rpl.Annotations, plugin); err != nil {
		return err
	}
	if err := r.adjustMounts(rpl.Mounts, plugin); err != nil {
		return err
	}
	if err := r.adjustEnv(rpl.Env, plugin); err != nil {
		return err
	}
	if err := r.adjustArgs(rpl.Args, plugin); err != nil {
		return err
	}
	if err := r.adjustHooks(rpl.Hooks, plugin); err != nil {
		return err
	}
	if rpl.Linux != nil {
		if err := r.adjustDevices(rpl.Linux.Devices, plugin); err != nil {
			return err
		}
		if err := r.adjustResources(rpl.Linux.Resources, plugin); err != nil {
			return err
		}
		if err := r.adjustCgroupsPath(rpl.Linux.CgroupsPath, plugin); err != nil {
			return err
		}
		if err := r.adjustOomScoreAdj(rpl.Linux.OomScoreAdj, plugin); err != nil {
			return err
		}
		if err := r.adjustIOPriority(rpl.Linux.IoPriority, plugin); err != nil {
			return err
		}
		if err := r.adjustSeccompPolicy(rpl.Linux.SeccompPolicy, plugin); err != nil {
			return err
		}
		if err := r.adjustNamespaces(rpl.Linux.Namespaces, plugin); err != nil {
			return err
		}
		if err := r.adjustSysctl(rpl.Linux.Sysctl, plugin); err != nil {
			return err
		}
		if err := r.adjustLinuxNetDevices(rpl.Linux.NetDevices, plugin); err != nil {
			return err
		}
		if err := r.adjustLinuxScheduler(rpl.Linux.Scheduler, plugin); err != nil {
			return err
		}
		if err := r.adjustRdt(rpl.Linux.Rdt, plugin); err != nil {
			return err
		}
	}

	if err := r.adjustRlimits(rpl.Rlimits, plugin); err != nil {
		return err
	}
	if err := r.adjustCDIDevices(rpl.CDIDevices, plugin); err != nil {
		return err
	}

	return nil
}

func (r *result) update(updates []*ContainerUpdate, plugin string) error {
	for _, u := range updates {
		reply, err := r.getContainerUpdate(u, plugin)
		if err != nil {
			return err
		}
		if err := r.updateResources(reply, u, plugin); err != nil && !u.IgnoreFailure {
			return err
		}
	}

	return nil
}

func (r *result) adjustAnnotations(annotations map[string]string, plugin string) error {
	if len(annotations) == 0 {
		return nil
	}

	create, id := r.request.create, r.request.create.Container.Id
	del := map[string]struct{}{}
	for k := range annotations {
		if key, marked := IsMarkedForRemoval(k); marked {
			del[key] = struct{}{}
			delete(annotations, k)
		}
	}

	for k, v := range annotations {
		if _, ok := del[k]; ok {
			r.owners.ClearAnnotation(id, k, plugin)
			delete(create.Container.Annotations, k)
			r.reply.adjust.Annotations[MarkForRemoval(k)] = ""
		}
		if err := r.owners.ClaimAnnotation(id, k, plugin); err != nil {
			return err
		}
		create.Container.Annotations[k] = v
		r.reply.adjust.Annotations[k] = v
		delete(del, k)
	}

	for k := range del {
		r.reply.adjust.Annotations[MarkForRemoval(k)] = ""
	}

	return nil
}

func (r *result) adjustMounts(mounts []*Mount, plugin string) error {
	if len(mounts) == 0 {
		return nil
	}

	create, id := r.request.create, r.request.create.Container.Id

	// first split removals from the rest of adjustments
	add := []*Mount{}
	del := map[string]*Mount{}
	mod := map[string]*Mount{}
	for _, m := range mounts {
		if key, marked := m.IsMarkedForRemoval(); marked {
			del[key] = m
		} else {
			add = append(add, m)
			mod[key] = m
		}
	}

	// next remove marked mounts from collected adjustments
	cleared := []*Mount{}
	for _, m := range r.reply.adjust.Mounts {
		if _, removed := del[m.Destination]; removed {
			r.owners.ClearMount(id, m.Destination, plugin)
			continue
		}
		cleared = append(cleared, m)
	}
	r.reply.adjust.Mounts = cleared

	// next remove marked and modified mounts from container creation request
	cleared = []*Mount{}
	for _, m := range create.Container.Mounts {
		if _, removed := del[m.Destination]; removed {
			continue
		}
		if _, modified := mod[m.Destination]; modified {
			continue
		}
		cleared = append(cleared, m)
	}
	create.Container.Mounts = cleared

	// next, apply additions/modifications to collected adjustments
	for _, m := range add {
		if err := r.owners.ClaimMount(id, m.Destination, plugin); err != nil {
			return err
		}
		r.reply.adjust.Mounts = append(r.reply.adjust.Mounts, m)
	}

	// next, apply deletions with no corresponding additions
	for _, m := range del {
		if _, ok := mod[api.ClearRemovalMarker(m.Destination)]; !ok {
			r.reply.adjust.Mounts = append(r.reply.adjust.Mounts, m)
		}
	}

	// finally, apply additions/modifications to plugin container creation request
	create.Container.Mounts = append(create.Container.Mounts, add...)

	return nil
}

func (r *result) adjustDevices(devices []*LinuxDevice, plugin string) error {
	if len(devices) == 0 {
		return nil
	}

	create, id := r.request.create, r.request.create.Container.Id

	// first split removals from the rest of adjustments
	add := []*LinuxDevice{}
	del := map[string]*LinuxDevice{}
	mod := map[string]*LinuxDevice{}
	for _, d := range devices {
		if key, marked := d.IsMarkedForRemoval(); marked {
			del[key] = d
		} else {
			add = append(add, d)
			mod[key] = d
		}
	}

	// next remove marked devices from collected adjustments
	cleared := []*LinuxDevice{}
	for _, d := range r.reply.adjust.Linux.Devices {
		if _, removed := del[d.Path]; removed {
			r.owners.ClearDevice(id, d.Path, plugin)
			continue
		}
		cleared = append(cleared, d)
	}
	r.reply.adjust.Linux.Devices = cleared

	// next remove marked and modified devices from container creation request
	cleared = []*LinuxDevice{}
	for _, d := range create.Container.Linux.Devices {
		if _, removed := del[d.Path]; removed {
			continue
		}
		if _, modified := mod[d.Path]; modified {
			continue
		}
		cleared = append(cleared, d)
	}
	create.Container.Linux.Devices = cleared

	// next, apply additions/modifications to collected adjustments
	for _, d := range add {
		if err := r.owners.ClaimDevice(id, d.Path, plugin); err != nil {
			return err
		}
		r.reply.adjust.Linux.Devices = append(r.reply.adjust.Linux.Devices, d)
	}

	// finally, apply additions/modifications to plugin container creation request
	create.Container.Linux.Devices = append(create.Container.Linux.Devices, add...)

	return nil
}

func (r *result) adjustNamespaces(namespaces []*LinuxNamespace, plugin string) error {
	if len(namespaces) == 0 {
		return nil
	}

	create, id := r.request.create, r.request.create.Container.Id

	creatensmap := map[string]*LinuxNamespace{}
	for _, n := range create.Container.Linux.Namespaces {
		creatensmap[n.Type] = n
	}

	for _, n := range namespaces {
		if n == nil {
			continue
		}
		key, marked := n.IsMarkedForRemoval()
		if err := r.owners.ClaimNamespace(id, key, plugin); err != nil {
			return err
		}
		if marked {
			delete(creatensmap, key)
		} else {
			creatensmap[key] = n
		}
		r.reply.adjust.Linux.Namespaces = append(r.reply.adjust.Linux.Namespaces, n)
	}

	create.Container.Linux.Namespaces = slices.Collect(maps.Values(creatensmap))

	return nil
}

func (r *result) adjustSysctl(sysctl map[string]string, plugin string) error {
	if len(sysctl) == 0 {
		return nil
	}

	create, id := r.request.create, r.request.create.Container.Id
	del := map[string]struct{}{}
	for k := range sysctl {
		if key, marked := IsMarkedForRemoval(k); marked {
			del[key] = struct{}{}
			delete(sysctl, k)
		}
	}

	for k, v := range sysctl {
		if _, ok := del[k]; ok {
			r.owners.ClearSysctl(id, k, plugin)
			delete(create.Container.Linux.Sysctl, k)
			r.reply.adjust.Linux.Sysctl[MarkForRemoval(k)] = ""
		}
		if err := r.owners.ClaimSysctl(id, k, plugin); err != nil {
			return err
		}
		create.Container.Linux.Sysctl[k] = v
		r.reply.adjust.Linux.Sysctl[k] = v
		delete(del, k)
	}

	for k := range del {
		r.reply.adjust.Annotations[MarkForRemoval(k)] = ""
	}

	return nil
}

func (r *result) adjustRdt(rdt *LinuxRdt, plugin string) error {
	if r == nil {
		return nil
	}

	r.initAdjustRdt()

	id := r.request.create.Container.Id

	if rdt.GetRemove() {
		r.owners.ClearRdt(id, plugin)
		r.reply.adjust.Linux.Rdt = &LinuxRdt{
			// Propagate the remove request (if not overridden below).
			Remove: true,
		}
	}

	if v := rdt.GetClosId(); v != nil {
		if err := r.owners.ClaimRdtClosID(id, plugin); err != nil {
			return err
		}
		r.reply.adjust.Linux.Rdt.ClosId = String(v.GetValue())
		r.reply.adjust.Linux.Rdt.Remove = false
	}
	if v := rdt.GetSchemata(); v != nil {
		if err := r.owners.ClaimRdtSchemata(id, plugin); err != nil {
			return err
		}
		r.reply.adjust.Linux.Rdt.Schemata = RepeatedString(v.GetValue())
		r.reply.adjust.Linux.Rdt.Remove = false
	}
	if v := rdt.GetEnableMonitoring(); v != nil {
		if err := r.owners.ClaimRdtEnableMonitoring(id, plugin); err != nil {
			return err
		}
		r.reply.adjust.Linux.Rdt.EnableMonitoring = Bool(v.GetValue())
		r.reply.adjust.Linux.Rdt.Remove = false
	}

	return nil
}

func (r *result) adjustCDIDevices(devices []*CDIDevice, plugin string) error {
	if len(devices) == 0 {
		return nil
	}

	// Notes:
	//   CDI devices are opaque references, typically to vendor specific
	//   devices. They get resolved to actual devices and potential related
	//   mounts, environment variables, etc. in the runtime. Unlike with
	//   devices, we only allow CDI device references to be added to a
	//   container, not removed. We pass them unresolved to the runtime and
	//   have them resolved there. Also unlike with devices, we don't include
	//   CDI device references in creation requests. However, since there
	//   is typically a strong ownership and a single related management entity
	//   per vendor/class for these devices we do treat multiple injection of
	//   the same CDI device reference as an error here.

	id := r.request.create.Container.Id

	// apply additions to collected adjustments
	for _, d := range devices {
		if err := r.owners.ClaimCdiDevice(id, d.Name, plugin); err != nil {
			return err
		}
		r.reply.adjust.CDIDevices = append(r.reply.adjust.CDIDevices, d)
	}

	return nil
}

func (r *result) adjustEnv(env []*KeyValue, plugin string) error {
	if len(env) == 0 {
		return nil
	}

	create, id := r.request.create, r.request.create.Container.Id

	// first split removals from the rest of adjustments
	add := []*KeyValue{}
	del := map[string]struct{}{}
	mod := map[string]struct{}{}
	for _, e := range env {
		if key, marked := e.IsMarkedForRemoval(); marked {
			del[key] = struct{}{}
		} else {
			add = append(add, e)
			mod[key] = struct{}{}
		}
	}

	// next remove marked environment variables from collected adjustments
	cleared := []*KeyValue{}
	for _, e := range r.reply.adjust.Env {
		if _, removed := del[e.Key]; removed {
			r.owners.ClearEnv(id, e.Key, plugin)
			continue
		}
		cleared = append(cleared, e)
	}
	r.reply.adjust.Env = cleared

	// next remove marked and modified environment from container creation request
	clearedEnv := []string{}
	for _, e := range create.Container.Env {
		key, _ := splitEnvVar(e)
		if _, removed := del[key]; removed {
			continue
		}
		if _, modified := mod[key]; modified {
			continue
		}
		clearedEnv = append(clearedEnv, e)
	}
	create.Container.Env = clearedEnv

	// next, apply additions/modifications to collected adjustments
	for _, e := range add {
		if err := r.owners.ClaimEnv(id, e.Key, plugin); err != nil {
			return err
		}
		r.reply.adjust.Env = append(r.reply.adjust.Env, e)
	}

	// finally, apply additions/modifications to plugin container creation request
	for _, e := range add {
		create.Container.Env = append(create.Container.Env, e.ToOCI())
	}

	return nil
}

func splitEnvVar(s string) (string, string) {
	split := strings.SplitN(s, "=", 2)
	if len(split) < 1 {
		return "", ""
	}
	if len(split) != 2 {
		return split[0], ""
	}
	return split[0], split[1]
}

func (r *result) adjustArgs(args []string, plugin string) error {
	if len(args) == 0 {
		return nil
	}

	create, id := r.request.create, r.request.create.Container.Id

	if args[0] == "" {
		r.owners.ClearArgs(id, plugin)
		args = args[1:]
	}

	if err := r.owners.ClaimArgs(id, plugin); err != nil {
		return err
	}

	r.reply.adjust.Args = slices.Clone(args)
	create.Container.Args = r.reply.adjust.Args

	return nil
}

func (r *result) adjustHooks(hooks *Hooks, plugin string) error {
	if hooks == nil {
		return nil
	}

	reply := r.reply.adjust
	container := r.request.create.Container
	claim := false

	if h := hooks.Prestart; len(h) > 0 {
		reply.Hooks.Prestart = append(reply.Hooks.Prestart, h...)
		container.Hooks.Prestart = append(container.Hooks.Prestart, h...)
		claim = true
	}
	if h := hooks.Poststart; len(h) > 0 {
		reply.Hooks.Poststart = append(reply.Hooks.Poststart, h...)
		container.Hooks.Poststart = append(container.Hooks.Poststart, h...)
		claim = true
	}
	if h := hooks.Poststop; len(h) > 0 {
		reply.Hooks.Poststop = append(reply.Hooks.Poststop, h...)
		container.Hooks.Poststop = append(container.Hooks.Poststop, h...)
		claim = true
	}
	if h := hooks.CreateRuntime; len(h) > 0 {
		reply.Hooks.CreateRuntime = append(reply.Hooks.CreateRuntime, h...)
		container.Hooks.CreateRuntime = append(container.Hooks.CreateRuntime, h...)
		claim = true
	}
	if h := hooks.CreateContainer; len(h) > 0 {
		reply.Hooks.CreateContainer = append(reply.Hooks.CreateContainer, h...)
		container.Hooks.CreateContainer = append(container.Hooks.CreateContainer, h...)
		claim = true
	}
	if h := hooks.StartContainer; len(h) > 0 {
		reply.Hooks.StartContainer = append(reply.Hooks.StartContainer, h...)
		container.Hooks.StartContainer = append(container.Hooks.StartContainer, h...)
		claim = true
	}

	if claim {
		r.owners.ClaimHooks(container.Id, plugin)
	}

	return nil
}

func (r *result) adjustMemoryResource(mem, targetContainer, targetReply *LinuxMemory, id, plugin string) error {
	if mem == nil {
		return nil
	}

	if v := mem.GetLimit(); v != nil {
		if err := r.owners.ClaimMemLimit(id, plugin); err != nil {
			return err
		}
		targetContainer.Limit = Int64(v.GetValue())
		if targetReply != nil {
			targetReply.Limit = Int64(v.GetValue())
		}
	}
	if v := mem.GetReservation(); v != nil {
		if err := r.owners.ClaimMemReservation(id, plugin); err != nil {
			return err
		}
		targetContainer.Reservation = Int64(v.GetValue())
		if targetReply != nil {
			targetReply.Reservation = Int64(v.GetValue())
		}
	}
	if v := mem.GetSwap(); v != nil {
		if err := r.owners.ClaimMemSwapLimit(id, plugin); err != nil {
			return err
		}
		targetContainer.Swap = Int64(v.GetValue())
		if targetReply != nil {
			targetReply.Swap = Int64(v.GetValue())
		}
	}
	if v := mem.GetKernel(); v != nil {
		if err := r.owners.ClaimMemKernelLimit(id, plugin); err != nil {
			return err
		}
		targetContainer.Kernel = Int64(v.GetValue())
		if targetReply != nil {
			targetReply.Kernel = Int64(v.GetValue())
		}
	}
	if v := mem.GetKernelTcp(); v != nil {
		if err := r.owners.ClaimMemTCPLimit(id, plugin); err != nil {
			return err
		}
		targetContainer.KernelTcp = Int64(v.GetValue())
		if targetReply != nil {
			targetReply.KernelTcp = Int64(v.GetValue())
		}
	}
	if v := mem.GetSwappiness(); v != nil {
		if err := r.owners.ClaimMemSwappiness(id, plugin); err != nil {
			return err
		}
		targetContainer.Swappiness = UInt64(v.GetValue())
		if targetReply != nil {
			targetReply.Swappiness = UInt64(v.GetValue())
		}
	}
	if v := mem.GetDisableOomKiller(); v != nil {
		if err := r.owners.ClaimMemDisableOomKiller(id, plugin); err != nil {
			return err
		}
		targetContainer.DisableOomKiller = Bool(v.GetValue())
		if targetReply != nil {
			targetReply.DisableOomKiller = Bool(v.GetValue())
		}
	}
	if v := mem.GetUseHierarchy(); v != nil {
		if err := r.owners.ClaimMemUseHierarchy(id, plugin); err != nil {
			return err
		}
		targetContainer.UseHierarchy = Bool(v.GetValue())
		if targetReply != nil {
			targetReply.UseHierarchy = Bool(v.GetValue())
		}
	}

	return nil
}

func (r *result) adjustCPUResource(cpu, targetContainer, targetReply *LinuxCPU, id, plugin string) error {
	if cpu == nil {
		return nil
	}

	if v := cpu.GetShares(); v != nil {
		if err := r.owners.ClaimCPUShares(id, plugin); err != nil {
			return err
		}
		targetContainer.Shares = UInt64(v.GetValue())
		if targetReply != nil {
			targetReply.Shares = UInt64(v.GetValue())
		}
	}
	if v := cpu.GetQuota(); v != nil {
		if err := r.owners.ClaimCPUQuota(id, plugin); err != nil {
			return err
		}
		targetContainer.Quota = Int64(v.GetValue())
		if targetReply != nil {
			targetReply.Quota = Int64(v.GetValue())
		}
	}
	if v := cpu.GetPeriod(); v != nil {
		if err := r.owners.ClaimCPUPeriod(id, plugin); err != nil {
			return err
		}
		targetContainer.Period = UInt64(v.GetValue())
		if targetReply != nil {
			targetReply.Period = UInt64(v.GetValue())
		}
	}
	if v := cpu.GetRealtimeRuntime(); v != nil {
		if err := r.owners.ClaimCPURealtimeRuntime(id, plugin); err != nil {
			return err
		}
		targetContainer.RealtimeRuntime = Int64(v.GetValue())
		if targetReply != nil {
			targetReply.RealtimeRuntime = Int64(v.GetValue())
		}
	}
	if v := cpu.GetRealtimePeriod(); v != nil {
		if err := r.owners.ClaimCPURealtimePeriod(id, plugin); err != nil {
			return err
		}
		targetContainer.RealtimePeriod = UInt64(v.GetValue())
		if targetReply != nil {
			targetReply.RealtimePeriod = UInt64(v.GetValue())
		}
	}
	if v := cpu.GetCpus(); v != "" {
		if err := r.owners.ClaimCPUSetCPUs(id, plugin); err != nil {
			return err
		}
		targetContainer.Cpus = v
		if targetReply != nil {
			targetReply.Cpus = v
		}
	}
	if v := cpu.GetMems(); v != "" {
		if err := r.owners.ClaimCPUSetMems(id, plugin); err != nil {
			return err
		}
		targetContainer.Mems = v
		if targetReply != nil {
			targetReply.Mems = v
		}
	}

	return nil
}

func (r *result) adjustResources(resources *LinuxResources, plugin string) error {
	if resources == nil {
		return nil
	}

	create, id := r.request.create, r.request.create.Container.Id
	container := create.Container.Linux.Resources
	reply := r.reply.adjust.Linux.Resources

	if err := r.adjustMemoryResource(resources.Memory, container.Memory, reply.Memory, id, plugin); err != nil {
		return err
	}

	if err := r.adjustCPUResource(resources.Cpu, container.Cpu, reply.Cpu, id, plugin); err != nil {
		return err
	}

	for _, l := range resources.HugepageLimits {
		if err := r.owners.ClaimHugepageLimit(id, l.PageSize, plugin); err != nil {
			return err
		}
		container.HugepageLimits = append(container.HugepageLimits, l)
		reply.HugepageLimits = append(reply.HugepageLimits, l)
	}

	for _, d := range resources.Devices {
		container.Devices = append(container.Devices, d)
		reply.Devices = append(reply.Devices, d)
	}

	if len(resources.Unified) != 0 {
		for k, v := range resources.Unified {
			if err := r.owners.ClaimCgroupsUnified(id, k, plugin); err != nil {
				return err
			}
			container.Unified[k] = v
			reply.Unified[k] = v
		}
	}

	if v := resources.GetBlockioClass(); v != nil {
		if err := r.owners.ClaimBlockioClass(id, plugin); err != nil {
			return err
		}
		container.BlockioClass = String(v.GetValue())
		reply.BlockioClass = String(v.GetValue())
	}
	if v := resources.GetRdtClass(); v != nil {
		if err := r.owners.ClaimRdtClass(id, plugin); err != nil {
			return err
		}
		container.RdtClass = String(v.GetValue())
		reply.RdtClass = String(v.GetValue())
	}
	if v := resources.GetPids(); v != nil {
		if err := r.owners.ClaimPidsLimit(id, plugin); err != nil {
			return err
		}
		pidv := &api.LinuxPids{
			Limit: v.GetLimit(),
		}
		container.Pids = pidv
		reply.Pids = pidv
	}
	return nil
}

func (r *result) adjustCgroupsPath(path, plugin string) error {
	if path == "" {
		return nil
	}

	create, id := r.request.create, r.request.create.Container.Id

	if err := r.owners.ClaimCgroupsPath(id, plugin); err != nil {
		return err
	}

	create.Container.Linux.CgroupsPath = path
	r.reply.adjust.Linux.CgroupsPath = path

	return nil
}

func (r *result) adjustOomScoreAdj(OomScoreAdj *OptionalInt, plugin string) error {
	if OomScoreAdj == nil {
		return nil
	}

	create, id := r.request.create, r.request.create.Container.Id

	if err := r.owners.ClaimOomScoreAdj(id, plugin); err != nil {
		return err
	}

	create.Container.Linux.OomScoreAdj = OomScoreAdj
	r.reply.adjust.Linux.OomScoreAdj = OomScoreAdj

	return nil
}

func (r *result) adjustIOPriority(priority *LinuxIOPriority, plugin string) error {
	if priority == nil {
		return nil
	}

	create, id := r.request.create, r.request.create.Container.Id

	if err := r.owners.ClaimIOPriority(id, plugin); err != nil {
		return err
	}

	create.Container.Linux.IoPriority = priority
	r.reply.adjust.Linux.IoPriority = priority

	return nil
}

func (r *result) adjustSeccompPolicy(adjustment *LinuxSeccomp, plugin string) error {
	if adjustment == nil {
		return nil
	}
	create, id := r.request.create, r.request.create.Container.Id

	if err := r.owners.ClaimSeccompPolicy(id, plugin); err != nil {
		return err
	}

	create.Container.Linux.SeccompPolicy = adjustment
	r.reply.adjust.Linux.SeccompPolicy = adjustment

	return nil
}

func (r *result) adjustLinuxScheduler(sch *LinuxScheduler, plugin string) error {
	if sch == nil {
		return nil
	}

	create, id := r.request.create, r.request.create.Container.Id

	if err := r.owners.ClaimLinuxScheduler(id, plugin); err != nil {
		return err
	}

	create.Container.Linux.Scheduler = sch
	r.reply.adjust.Linux.Scheduler = sch

	return nil
}

func (r *result) adjustRlimits(rlimits []*POSIXRlimit, plugin string) error {
	create, id, adjust := r.request.create, r.request.create.Container.Id, r.reply.adjust
	for _, l := range rlimits {
		if err := r.owners.ClaimRlimit(id, l.Type, plugin); err != nil {
			return err
		}

		create.Container.Rlimits = append(create.Container.Rlimits, l)
		adjust.Rlimits = append(adjust.Rlimits, l)
	}
	return nil
}

func (r *result) adjustLinuxNetDevices(devices map[string]*LinuxNetDevice, plugin string) error {
	if len(devices) == 0 {
		return nil
	}

	create, id := r.request.create, r.request.create.Container.Id
	del := map[string]struct{}{}
	for k := range devices {
		if key, marked := IsMarkedForRemoval(k); marked {
			del[key] = struct{}{}
			delete(devices, k)
		}
	}

	for k, v := range devices {
		if _, ok := del[k]; ok {
			r.owners.ClearLinuxNetDevice(id, k, plugin)
			delete(create.Container.Linux.NetDevices, k)
			r.reply.adjust.Linux.NetDevices[MarkForRemoval(k)] = nil
		}
		if err := r.owners.ClaimLinuxNetDevice(id, k, plugin); err != nil {
			return err
		}
		create.Container.Linux.NetDevices[k] = v
		r.reply.adjust.Linux.NetDevices[k] = v
		delete(del, k)
	}

	for k := range del {
		r.reply.adjust.Linux.NetDevices[MarkForRemoval(k)] = nil
	}

	return nil
}

func (r *result) updateResources(reply, u *ContainerUpdate, plugin string) error {
	if u.Linux == nil || u.Linux.Resources == nil {
		return nil
	}

	var resources *LinuxResources
	request, id := r.request.update, u.ContainerId

	// operate on a copy: we won't touch anything on (ignored) failures
	if request != nil && request.Container.Id == id {
		resources = request.LinuxResources.Copy()
	} else {
		resources = reply.Linux.Resources.Copy()
	}

	if err := r.adjustMemoryResource(u.Linux.Resources.Memory, resources.Memory, nil, id, plugin); err != nil {
		return err
	}

	if err := r.adjustCPUResource(u.Linux.Resources.Cpu, resources.Cpu, nil, id, plugin); err != nil {
		return err
	}

	for _, l := range u.Linux.Resources.HugepageLimits {
		if err := r.owners.ClaimHugepageLimit(id, l.PageSize, plugin); err != nil {
			return err
		}
		resources.HugepageLimits = append(resources.HugepageLimits, l)
	}

	if len(u.Linux.Resources.Unified) != 0 {
		if resources.Unified == nil {
			resources.Unified = make(map[string]string)
		}
		for k, v := range u.Linux.Resources.Unified {
			if err := r.owners.ClaimCgroupsUnified(id, k, plugin); err != nil {
				return err
			}
			resources.Unified[k] = v
		}
	}

	if v := u.Linux.Resources.GetBlockioClass(); v != nil {
		if err := r.owners.ClaimBlockioClass(id, plugin); err != nil {
			return err
		}
		resources.BlockioClass = String(v.GetValue())
	}
	if v := u.Linux.Resources.GetRdtClass(); v != nil {
		if err := r.owners.ClaimRdtClass(id, plugin); err != nil {
			return err
		}
		resources.RdtClass = String(v.GetValue())
	}
	if v := resources.GetPids(); v != nil {
		if err := r.owners.ClaimPidsLimit(id, plugin); err != nil {
			return err
		}
		resources.Pids = &api.LinuxPids{
			Limit: v.GetLimit(),
		}
	}

	// update request/reply from copy on success
	reply.Linux.Resources = resources.Copy()

	if request != nil && request.Container.Id == id {
		request.LinuxResources = resources.Copy()
	}

	return nil
}

func (r *result) getContainerUpdate(u *ContainerUpdate, plugin string) (*ContainerUpdate, error) {
	id := u.ContainerId
	if r.request.create != nil && r.request.create.Container != nil {
		if r.request.create.Container.Id == id {
			return nil, fmt.Errorf("plugin %q asked update of %q during creation",
				plugin, id)
		}
	}

	if update, ok := r.updates[id]; ok {
		update.IgnoreFailure = update.IgnoreFailure && u.IgnoreFailure
		return update, nil
	}

	update := &ContainerUpdate{
		ContainerId: id,
		Linux: &LinuxContainerUpdate{
			Resources: &LinuxResources{
				Memory:         &LinuxMemory{},
				Cpu:            &LinuxCPU{},
				HugepageLimits: []*HugepageLimit{},
				Unified:        map[string]string{},
			},
		},
		IgnoreFailure: u.IgnoreFailure,
	}

	r.updates[id] = update

	// for update requests delay appending the requested container (in the response getter)
	if r.request.update == nil || r.request.update.Container.Id != id {
		r.reply.update = append(r.reply.update, update)
	}

	return update, nil
}

func (r *result) initAdjust() {
	if r.reply.adjust == nil {
		r.reply.adjust = &ContainerAdjustment{}
	}
}

func (r *result) initAdjustLinux() {
	r.initAdjust()
	if r.reply.adjust.Linux == nil {
		r.reply.adjust.Linux = &LinuxContainerAdjustment{}
	}
}

func (r *result) initAdjustRdt() {
	r.initAdjustLinux()
	if r.reply.adjust.Linux.Rdt == nil {
		r.reply.adjust.Linux.Rdt = &LinuxRdt{}
	}
}
