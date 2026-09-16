/*
   Copyright 2026 The CDI Authors

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

package ociedit

import (
	"errors"
	"slices"
	"strings"

	oci "github.com/opencontainers/runtime-spec/specs-go"
)

// SpecEditor is the internal boundary between CDI edits and OCI spec mutation.
// It deliberately models only the operations CDI needs, so a future external
// generator can be wired in without leaking that dependency elsewhere.
type SpecEditor interface {
	AddMultipleProcessEnv([]string)
	RemoveDevice(string)
	AddDevice(oci.LinuxDevice)
	AddLinuxResourcesDevice(bool, string, *int64, *int64, string)
	SetLinuxNetDevice(string, *oci.LinuxNetDevice)
	RemoveMount(string)
	AddMount(oci.Mount)
	Mounts() []oci.Mount
	SetMounts([]oci.Mount)
	AddPreStartHook(oci.Hook)
	AddPostStartHook(oci.Hook)
	AddPostStopHook(oci.Hook)
	AddCreateRuntimeHook(oci.Hook)
	AddCreateContainerHook(oci.Hook)
	AddStartContainerHook(oci.Hook)
	SetLinuxIntelRdt(*oci.LinuxIntelRdt)
	AddProcessAdditionalGID(uint32)
	ClearLinuxDevices()
}

// NewSpecEditor returns CDI's native OCI spec editor.
func NewSpecEditor(spec *oci.Spec) (SpecEditor, error) {
	if spec == nil {
		return nil, errors.New("can't edit nil OCI Spec")
	}

	envCache := map[string]int{}
	if spec.Process != nil {
		envCache = createEnvCacheMap(spec.Process.Env)
	}
	return &nativeSpecEditor{
		spec:   spec,
		envMap: envCache,
	}, nil
}

type nativeSpecEditor struct {
	spec   *oci.Spec
	envMap map[string]int
}

func createEnvCacheMap(env []string) map[string]int {
	envMap := make(map[string]int, len(env))
	for i, val := range env {
		val, _, _ = strings.Cut(val, "=")
		envMap[val] = i
	}
	return envMap
}

func (e *nativeSpecEditor) initProcess() {
	if e.spec.Process == nil {
		e.spec.Process = &oci.Process{}
	}
}

func (e *nativeSpecEditor) initHooks() {
	if e.spec.Hooks == nil {
		e.spec.Hooks = &oci.Hooks{}
	}
}

func (e *nativeSpecEditor) initLinux() {
	if e.spec.Linux == nil {
		e.spec.Linux = &oci.Linux{}
	}
}

func (e *nativeSpecEditor) initLinuxResources() {
	e.initLinux()
	if e.spec.Linux.Resources == nil {
		e.spec.Linux.Resources = &oci.LinuxResources{}
	}
}

func (e *nativeSpecEditor) initLinuxNetDevices() {
	e.initLinux()
	if e.spec.Linux.NetDevices == nil {
		e.spec.Linux.NetDevices = map[string]oci.LinuxNetDevice{}
	}
}

func (e *nativeSpecEditor) AddMultipleProcessEnv(envs []string) {
	e.initProcess()

	for _, val := range envs {
		key, _, _ := strings.Cut(val, "=")
		e.addEnv(val, key)
	}
}

func (e *nativeSpecEditor) addEnv(env, key string) {
	if idx, ok := e.envMap[key]; ok {
		e.spec.Process.Env[idx] = env
		return
	}

	e.spec.Process.Env = append(e.spec.Process.Env, env)
	e.envMap[key] = len(e.spec.Process.Env) - 1
}

func (e *nativeSpecEditor) RemoveDevice(path string) {
	if e.spec == nil || e.spec.Linux == nil || e.spec.Linux.Devices == nil {
		return
	}

	for i, device := range e.spec.Linux.Devices {
		if device.Path == path {
			e.spec.Linux.Devices = append(e.spec.Linux.Devices[:i], e.spec.Linux.Devices[i+1:]...)
			return
		}
	}
}

func (e *nativeSpecEditor) AddDevice(device oci.LinuxDevice) {
	e.initLinux()

	for i, dev := range e.spec.Linux.Devices {
		if dev.Path == device.Path {
			e.spec.Linux.Devices[i] = device
			return
		}
	}

	e.spec.Linux.Devices = append(e.spec.Linux.Devices, device)
}

func (e *nativeSpecEditor) AddLinuxResourcesDevice(allow bool, devType string, major, minor *int64, access string) {
	e.initLinuxResources()
	e.spec.Linux.Resources.Devices = append(e.spec.Linux.Resources.Devices, oci.LinuxDeviceCgroup{
		Allow:  allow,
		Type:   devType,
		Major:  major,
		Minor:  minor,
		Access: access,
	})
}

func (e *nativeSpecEditor) SetLinuxNetDevice(hostIf string, netDev *oci.LinuxNetDevice) {
	if netDev == nil {
		return
	}

	e.initLinuxNetDevices()
	e.spec.Linux.NetDevices[hostIf] = *netDev
}

func (e *nativeSpecEditor) RemoveMount(dest string) {
	for i, mount := range e.spec.Mounts {
		if mount.Destination == dest {
			e.spec.Mounts = append(e.spec.Mounts[:i], e.spec.Mounts[i+1:]...)
			return
		}
	}
}

func (e *nativeSpecEditor) AddMount(mnt oci.Mount) {
	e.spec.Mounts = append(e.spec.Mounts, mnt)
}

func (e *nativeSpecEditor) Mounts() []oci.Mount {
	return e.spec.Mounts
}

func (e *nativeSpecEditor) SetMounts(mounts []oci.Mount) {
	e.spec.Mounts = mounts
}

func (e *nativeSpecEditor) AddPreStartHook(hook oci.Hook) {
	e.initHooks()
	e.spec.Hooks.Prestart = append(e.spec.Hooks.Prestart, hook) //nolint:staticcheck // CDI still supports OCI prestart hooks.
}

func (e *nativeSpecEditor) AddPostStartHook(hook oci.Hook) {
	e.initHooks()
	e.spec.Hooks.Poststart = append(e.spec.Hooks.Poststart, hook)
}

func (e *nativeSpecEditor) AddPostStopHook(hook oci.Hook) {
	e.initHooks()
	e.spec.Hooks.Poststop = append(e.spec.Hooks.Poststop, hook)
}

func (e *nativeSpecEditor) AddCreateRuntimeHook(hook oci.Hook) {
	e.initHooks()
	e.spec.Hooks.CreateRuntime = append(e.spec.Hooks.CreateRuntime, hook)
}

func (e *nativeSpecEditor) AddCreateContainerHook(hook oci.Hook) {
	e.initHooks()
	e.spec.Hooks.CreateContainer = append(e.spec.Hooks.CreateContainer, hook)
}

func (e *nativeSpecEditor) AddStartContainerHook(hook oci.Hook) {
	e.initHooks()
	e.spec.Hooks.StartContainer = append(e.spec.Hooks.StartContainer, hook)
}

func (e *nativeSpecEditor) SetLinuxIntelRdt(rdt *oci.LinuxIntelRdt) {
	e.initLinux()
	e.spec.Linux.IntelRdt = rdt
}

func (e *nativeSpecEditor) AddProcessAdditionalGID(gid uint32) {
	e.initProcess()
	if slices.Contains(e.spec.Process.User.AdditionalGids, gid) {
		return
	}
	e.spec.Process.User.AdditionalGids = append(e.spec.Process.User.AdditionalGids, gid)
}

func (e *nativeSpecEditor) ClearLinuxDevices() {
	if e.spec == nil || e.spec.Linux == nil || e.spec.Linux.Devices == nil {
		return
	}
	e.spec.Linux.Devices = []oci.LinuxDevice{}
}
