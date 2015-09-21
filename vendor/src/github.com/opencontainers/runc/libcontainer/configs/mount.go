package configs

import (
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opencontainers/runc/libcontainer/label"
)

type Mount struct {
	// Source path for the mount.
	Source string `json:"source"`

	// Destination path for the mount inside the container.
	Destination string `json:"destination"`

	// Device the mount is for.
	Device string `json:"device"`

	// Mount flags.
	Flags int `json:"flags"`

	// Propagation Flags
	PropagationFlags []int `json:"propagation_flags"`

	// Mount data applied to the mount.
	Data string `json:"data"`

	// Relabel source if set, "z" indicates shared, "Z" indicates unshared.
	Relabel string `json:"relabel"`

	// Optional Command to be run before Source is mounted.
	PremountCmds []Command `json:"premount_cmds"`

	// Optional Command to be run after Source is mounted.
	PostmountCmds []Command `json:"postmount_cmds"`
}

func (m *Mount) Remount(rootfs string) error {
	var (
		dest = m.Destination
	)
	if !strings.HasPrefix(dest, rootfs) {
		dest = filepath.Join(rootfs, dest)
	}

	if err := syscall.Mount(m.Source, dest, m.Device, uintptr(m.Flags|syscall.MS_REMOUNT), ""); err != nil {
		return err
	}
	return nil
}

// Do the mount operation followed by additional mounts required to take care
// of propagation flags.
func (m *Mount) MountPropagate(rootfs string, mountLabel string) error {
	var (
		dest = m.Destination
		data = label.FormatMountLabel(m.Data, mountLabel)
	)
	if !strings.HasPrefix(dest, rootfs) {
		dest = filepath.Join(rootfs, dest)
	}

	if err := syscall.Mount(m.Source, dest, m.Device, uintptr(m.Flags), data); err != nil {
		return err
	}

	for _, pflag := range m.PropagationFlags {
		if err := syscall.Mount("", dest, "", uintptr(pflag), ""); err != nil {
			return err
		}
	}
	return nil
}
