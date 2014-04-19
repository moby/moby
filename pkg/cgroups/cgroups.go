package cgroups

import (
	"errors"
)

var (
	ErrNotFound = errors.New("mountpoint not found")
)

type Cgroup struct {
	Name   string `json:"name,omitempty"`
	Parent string `json:"parent,omitempty"`

	DeviceAccess bool   `json:"device_access,omitempty"` // name of parent cgroup or slice
	Memory       int64  `json:"memory,omitempty"`        // Memory limit (in bytes)
	MemorySwap   int64  `json:"memory_swap,omitempty"`   // Total memory usage (memory + swap); set `-1' to disable swap
	CpuShares    int64  `json:"cpu_shares,omitempty"`    // CPU shares (relative weight vs. other containers)
	CpusetCpus   string `json:"cpuset_cpus,omitempty"`   // CPU to use

	UnitProperties [][2]string `json:"unit_properties,omitempty"` // systemd unit properties
}

type ActiveCgroup interface {
	Cleanup() error
}

func Apply(c *Cgroup, pid int) (ActiveCgroup, error) {
	// We have two implementation of cgroups support, one is based on
	// systemd and the dbus api, and one is based on raw cgroup fs operations
	// following the pre-single-writer model docs at:
	// http://www.freedesktop.org/wiki/Software/systemd/PaxControlGroups/

	if useSystemd() {
		return systemdApply(c, pid)
	} else {
		return rawApply(c, pid)
	}
}
