/*
Copyright © 2021 The CDI Authors

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

package cdi

import (
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

// toOCI returns the opencontainers runtime Spec Hook for this Hook.
func (h *Hook) toOCI() spec.Hook {
	return spec.Hook{
		Path:    h.Path,
		Args:    h.Args,
		Env:     h.Env,
		Timeout: h.Timeout,
	}
}

// toOCI returns the opencontainers runtime Spec Mount for this Mount.
func (m *Mount) toOCI() spec.Mount {
	return spec.Mount{
		Source:      m.HostPath,
		Destination: m.ContainerPath,
		Options:     m.Options,
		Type:        m.Type,
	}
}

// toOCI returns the opencontainers runtime Spec LinuxDevice for this DeviceNode.
func (d *DeviceNode) toOCI() spec.LinuxDevice {
	return spec.LinuxDevice{
		Path:     d.Path,
		Type:     d.Type,
		Major:    d.Major,
		Minor:    d.Minor,
		FileMode: d.FileMode,
		UID:      d.UID,
		GID:      d.GID,
	}
}

// toOCI returns the opencontainers runtime Spec LinuxIntelRdt for this IntelRdt config.
func (i *IntelRdt) toOCI() *spec.LinuxIntelRdt {
	return &spec.LinuxIntelRdt{
		ClosID:           i.ClosID,
		L3CacheSchema:    i.L3CacheSchema,
		MemBwSchema:      i.MemBwSchema,
		Schemata:         i.Schemata,
		EnableMonitoring: i.EnableMonitoring,
	}
}

// toOCI returns the opencontainers runtime Spec LinuxNetDevice for this LinuxNetDevice.
func (d *LinuxNetDevice) toOCI() *spec.LinuxNetDevice {
	return &spec.LinuxNetDevice{
		Name: d.Name,
	}
}
