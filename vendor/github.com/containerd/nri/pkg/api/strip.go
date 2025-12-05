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

//
// Notes:
//   These stripping functions are used in tests to compare values for
//   semantic equality using go-cmp. They reduce their receiver to a
//   unique canonical representation by replacing empty slices, maps,
//   and struct type fields by nil. These are destructive (IOW might
//   alter the receiver) and should only be used for testing.
//
//   TODO(klihub):
//     Starting with 1.36.6, we could use protobuf/proto.CloneOf() to
//     create a deep copy before stripping. However, we can't update
//     beyond 1.35.2 yet, before all supported release branches of our
//     downstream dependencies have bumped their direct dependencies.

// Strip empty fields from a container adjustment, reducing a fully empty
// one to nil. Strip allows comparison of two adjustments for semantic
// equality using go-cmp.
func (a *ContainerAdjustment) Strip() *ContainerAdjustment {
	if a == nil {
		return nil
	}

	empty := true

	if len(a.Annotations) == 0 {
		a.Annotations = nil
	} else {
		empty = false
	}
	if len(a.Mounts) == 0 {
		a.Mounts = nil
	} else {
		empty = false
	}
	if len(a.Env) == 0 {
		a.Env = nil
	} else {
		empty = false
	}
	if len(a.Rlimits) == 0 {
		a.Rlimits = nil
	} else {
		empty = false
	}
	if len(a.CDIDevices) == 0 {
		a.CDIDevices = nil
	} else {
		empty = false
	}
	if len(a.Args) == 0 {
		a.Args = nil
	} else {
		empty = false
	}

	if a.Hooks = a.Hooks.Strip(); a.Hooks != nil {
		empty = false
	}
	if a.Linux = a.Linux.Strip(); a.Linux != nil {
		empty = false
	}

	if empty {
		return nil
	}

	return a
}

// Strip empty fields from a linux container adjustment, reducing a fully
// empty one to nil. Strip allows comparison of two adjustments for semantic
// equality using go-cmp.
func (l *LinuxContainerAdjustment) Strip() *LinuxContainerAdjustment {
	if l == nil {
		return nil
	}

	empty := true

	if len(l.Devices) == 0 {
		l.Devices = nil
	} else {
		empty = false
	}

	if l.Resources = l.Resources.Strip(); l.Resources != nil {
		empty = false
	}

	if l.CgroupsPath != "" {
		empty = false
	}
	if l.OomScoreAdj != nil {
		empty = false
	}

	if empty {
		return nil
	}

	return l
}

// Strip empty fields from Hooks, reducing a fully empty one to nil. Strip
// allows comparison of two Hooks for semantic equality using go-cmp.
func (h *Hooks) Strip() *Hooks {
	if h == nil {
		return nil
	}

	empty := true

	if len(h.Prestart) == 0 {
		h.Prestart = nil
	} else {
		empty = false
	}
	if len(h.CreateRuntime) == 0 {
		h.CreateRuntime = nil
	} else {
		empty = false
	}
	if len(h.CreateContainer) == 0 {
		h.CreateContainer = nil
	} else {
		empty = false
	}
	if len(h.StartContainer) == 0 {
		h.StartContainer = nil
	} else {
		empty = false
	}
	if len(h.Poststart) == 0 {
		h.Poststart = nil
	} else {
		empty = false
	}
	if len(h.Poststop) == 0 {
		h.Poststop = nil
	} else {
		empty = false
	}

	if empty {
		return nil
	}

	return h
}

// Strip empty fields from a linux resources, reducing a fully empty one
// to nil. Strip allows comparison of two sets of resources for semantic
// equality using go-cmp.
func (r *LinuxResources) Strip() *LinuxResources {
	if r == nil {
		return nil
	}

	empty := true

	if r.Memory = r.Memory.Strip(); r.Memory != nil {
		empty = false
	}
	if r.Cpu = r.Cpu.Strip(); r.Cpu != nil {
		empty = false
	}
	if len(r.HugepageLimits) == 0 {
		r.HugepageLimits = nil
	} else {
		empty = false
	}

	if r.BlockioClass != nil {
		empty = false
	}
	if r.RdtClass != nil {
		empty = false
	}
	if len(r.Unified) == 0 {
		r.Unified = nil
	} else {
		empty = false
	}
	if len(r.Devices) == 0 {
		r.Devices = nil
	} else {
		empty = false
	}
	if r.Pids != nil {
		empty = false
	}

	if empty {
		return nil
	}

	return r
}

// Strip empty fields from linux CPU attributes, reducing a fully empty one
// to nil. Strip allows comparison of two sets of attributes for semantic
// equality using go-cmp.
func (c *LinuxCPU) Strip() *LinuxCPU {
	if c == nil {
		return nil
	}

	empty := true

	if c.Shares != nil {
		empty = false
	}
	if c.Quota != nil {
		empty = false
	}
	if c.Period != nil {
		empty = false
	}
	if c.RealtimeRuntime != nil {
		empty = false
	}
	if c.RealtimePeriod != nil {
		empty = false
	}
	if c.Cpus != "" {
		empty = false
	}
	if c.Mems != "" {
		empty = false
	}

	if empty {
		return nil
	}

	return c
}

// Strip empty fields from linux memory attributes, reducing a fully empty
// one to nil. Strip allows comparison of two sets of attributes for semantic
// equality using go-cmp.
func (m *LinuxMemory) Strip() *LinuxMemory {
	if m == nil {
		return nil
	}

	empty := true

	if m.Limit != nil {
		empty = false
	}
	if m.Reservation != nil {
		empty = false
	}
	if m.Swap != nil {
		empty = false
	}
	if m.Kernel != nil {
		empty = false
	}
	if m.KernelTcp != nil {
		empty = false
	}
	if m.Swappiness != nil {
		empty = false
	}
	if m.DisableOomKiller != nil {
		empty = false
	}
	if m.UseHierarchy != nil {
		empty = false
	}

	if empty {
		return nil
	}

	return m
}

// Strip empty fields from a container update, reducing a fully empty one
// to nil. Strip allows comparison of two updates for semantic equality
// using go-cmp.
func (u *ContainerUpdate) Strip() *ContainerUpdate {
	if u == nil {
		return nil
	}

	empty := true

	if u.Linux = u.Linux.Strip(); u.Linux != nil {
		empty = false
	}

	if u.IgnoreFailure {
		empty = false
	}

	if empty {
		return nil
	}

	return u
}

// Strip empty fields from a linux container update, reducing a fully empty
// one to nil. Strip allows comparison of two updates for semantic equality
// using go-cmp.
func (l *LinuxContainerUpdate) Strip() *LinuxContainerUpdate {
	if l == nil {
		return nil
	}

	empty := true

	if l.Resources = l.Resources.Strip(); l.Resources != nil {
		empty = false
	}

	if empty {
		return nil
	}

	return l
}
