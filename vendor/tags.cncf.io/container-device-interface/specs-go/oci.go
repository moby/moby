package specs

import (
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

// ToOCI returns the opencontainers runtime Spec Hook for this Hook.
func (h *Hook) ToOCI() spec.Hook {
	return spec.Hook{
		Path:    h.Path,
		Args:    h.Args,
		Env:     h.Env,
		Timeout: h.Timeout,
	}
}

// ToOCI returns the opencontainers runtime Spec Mount for this Mount.
func (m *Mount) ToOCI() spec.Mount {
	return spec.Mount{
		Source:      m.HostPath,
		Destination: m.ContainerPath,
		Options:     m.Options,
		Type:        m.Type,
	}
}

// ToOCI returns the opencontainers runtime Spec LinuxDevice for this DeviceNode.
func (d *DeviceNode) ToOCI() spec.LinuxDevice {
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
