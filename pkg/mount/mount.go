package mount // import "github.com/docker/docker/pkg/mount"

import (
	"sort"
	"strings"

	"syscall"

	"github.com/sirupsen/logrus"
)

// GetMounts retrieves a list of mounts for the current running process.
func GetMounts() ([]*Info, error) {
	return parseMountTable()
}

// Mounted determines if a specified mountpoint has been mounted.
// On Linux it looks at /proc/self/mountinfo.
func Mounted(mountpoint string) (bool, error) {
	entries, err := parseMountTable()
	if err != nil {
		return false, err
	}

	// Search the table for the mountpoint
	for _, e := range entries {
		if e.Mountpoint == mountpoint {
			return true, nil
		}
	}
	return false, nil
}

// Mount will mount filesystem according to the specified configuration, on the
// condition that the target path is *not* already mounted. Options must be
// specified like the mount or fstab unix commands: "opt1=val1,opt2=val2". See
// flags.go for supported option flags.
func Mount(device, target, mType, options string) error {
	flag, _ := parseOptions(options)
	if flag&REMOUNT != REMOUNT {
		if mounted, err := Mounted(target); err != nil || mounted {
			return err
		}
	}
	return ForceMount(device, target, mType, options)
}

// ForceMount will mount a filesystem according to the specified configuration,
// *regardless* if the target path is not already mounted. Options must be
// specified like the mount or fstab unix commands: "opt1=val1,opt2=val2". See
// flags.go for supported option flags.
func ForceMount(device, target, mType, options string) error {
	flag, data := parseOptions(options)
	return mount(device, target, mType, uintptr(flag), data)
}

// Unmount lazily unmounts a filesystem on supported platforms, otherwise
// does a normal unmount.
func Unmount(target string) error {
	if mounted, err := Mounted(target); err != nil || !mounted {
		return err
	}
	return unmount(target, mntDetach)
}

// RecursiveUnmount unmounts the target and all mounts underneath, starting with
// the deepsest mount first.
func RecursiveUnmount(target string) error {
	mounts, err := GetMounts()
	if err != nil {
		return err
	}

	// Make the deepest mount be first
	sort.Sort(sort.Reverse(byMountpoint(mounts)))

	for i, m := range mounts {
		if !strings.HasPrefix(m.Mountpoint, target) {
			continue
		}
		logrus.Debugf("Trying to unmount %s", m.Mountpoint)
		err = unmount(m.Mountpoint, mntDetach)
		if err != nil {
			// If the error is EINVAL either this whole package is wrong (invalid flags passed to unmount(2)) or this is
			// not a mountpoint (which is ok in this case).
			// Meanwhile calling `Mounted()` is very expensive.
			//
			// We've purposefully used `syscall.EINVAL` here instead of `unix.EINVAL` to avoid platform branching
			// Since `EINVAL` is defined for both Windows and Linux in the `syscall` package (and other platforms),
			//   this is nicer than defining a custom value that we can refer to in each platform file.
			if err == syscall.EINVAL {
				continue
			}
			if i == len(mounts)-1 {
				if mounted, e := Mounted(m.Mountpoint); e != nil || mounted {
					return err
				}
				continue
			}
			// This is some submount, we can ignore this error for now, the final unmount will fail if this is a real problem
			logrus.WithError(err).Warnf("Failed to unmount submount %s", m.Mountpoint)
			continue
		}

		logrus.Debugf("Unmounted %s", m.Mountpoint)
	}
	return nil
}
