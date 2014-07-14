package mount

import (
	"errors"

	"github.com/docker/libcontainer/devices"
)

type MountConfig struct {
	// NoPivotRoot will use MS_MOVE and a chroot to jail the process into the container's rootfs
	// This is a common option when the container is running in ramdisk
	NoPivotRoot bool `json:"no_pivot_root,omitempty"`

	// ReadonlyFs will remount the container's rootfs as readonly where only externally mounted
	// bind mounts are writtable
	ReadonlyFs bool `json:"readonly_fs,omitempty"`

	// Mounts specify additional source and destination paths that will be mounted inside the container's
	// rootfs and mount namespace if specified
	Mounts Mounts `json:"mounts,omitempty"`

	// The device nodes that should be automatically created within the container upon container start.  Note, make sure that the node is marked as allowed in the cgroup as well!
	DeviceNodes []*devices.Device `json:"device_nodes,omitempty"`

	MountLabel string `json:"mount_label,omitempty"`
}

type Mount struct {
	Type        string `json:"type,omitempty"`
	Source      string `json:"source,omitempty"`      // Source path, in the host namespace
	Destination string `json:"destination,omitempty"` // Destination path, in the container
	Writable    bool   `json:"writable,omitempty"`
	Relabel     string `json:"relabel,omitempty"` // Relabel source if set, "z" indicates shared, "Z" indicates unshared
	Private     bool   `json:"private,omitempty"`
}

type Mounts []Mount

var ErrUnsupported = errors.New("Unsupported method")

func (s Mounts) OfType(t string) Mounts {
	out := Mounts{}
	for _, m := range s {
		if m.Type == t {
			out = append(out, m)
		}
	}
	return out
}
