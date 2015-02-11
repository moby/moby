package mount

import (
	"errors"

	"github.com/docker/libcontainer/devices"
)

var ErrUnsupported = errors.New("Unsupported method")

type MountConfig struct {
	// NoPivotRoot will use MS_MOVE and a chroot to jail the process into the container's rootfs
	// This is a common option when the container is running in ramdisk
	NoPivotRoot bool `json:"no_pivot_root,omitempty"`

	// PivotDir allows a custom directory inside the container's root filesystem to be used as pivot, when NoPivotRoot is not set.
	// When a custom PivotDir not set, a temporary dir inside the root filesystem will be used. The pivot dir needs to be writeable.
	// This is required when using read only root filesystems. In these cases, a read/writeable path can be (bind) mounted somewhere inside the root filesystem to act as pivot.
	PivotDir string `json:"pivot_dir,omitempty"`

	// ReadonlyFs will remount the container's rootfs as readonly where only externally mounted
	// bind mounts are writtable
	ReadonlyFs bool `json:"readonly_fs,omitempty"`

	// Mounts specify additional source and destination paths that will be mounted inside the container's
	// rootfs and mount namespace if specified
	Mounts []*Mount `json:"mounts,omitempty"`

	// The device nodes that should be automatically created within the container upon container start.  Note, make sure that the node is marked as allowed in the cgroup as well!
	DeviceNodes []*devices.Device `json:"device_nodes,omitempty"`

	MountLabel string `json:"mount_label,omitempty"`
}
