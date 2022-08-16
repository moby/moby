//go:build !darwin && !windows
// +build !darwin,!windows

package mount

import (
	"fmt"
	"sort"

	"github.com/moby/sys/mountinfo"
	"golang.org/x/sys/unix"
)

// Mount will mount filesystem according to the specified configuration.
// Options must be specified like the mount or fstab unix commands:
// "opt1=val1,opt2=val2". See flags.go for supported option flags.
func Mount(device, target, mType, options string) error {
	flag, data := parseOptions(options)
	return mount(device, target, mType, uintptr(flag), data)
}

// Unmount lazily unmounts a filesystem on supported platforms, otherwise does
// a normal unmount. If target is not a mount point, no error is returned.
func Unmount(target string) error {
	err := unix.Unmount(target, mntDetach)
	if err == nil || err == unix.EINVAL { //nolint:errorlint // unix errors are bare
		// Ignore "not mounted" error here. Note the same error
		// can be returned if flags are invalid, so this code
		// assumes that the flags value is always correct.
		return nil
	}

	return &mountError{
		op:     "umount",
		target: target,
		flags:  uintptr(mntDetach),
		err:    err,
	}
}

// RecursiveUnmount unmounts the target and all mounts underneath, starting
// with the deepest mount first. The argument does not have to be a mount
// point itself.
func RecursiveUnmount(target string) error {
	// Fast path, works if target is a mount point that can be unmounted.
	// On Linux, mntDetach flag ensures a recursive unmount.  For other
	// platforms, if there are submounts, we'll get EBUSY (and fall back
	// to the slow path). NOTE we do not ignore EINVAL here as target might
	// not be a mount point itself (but there can be mounts underneath).
	if err := unix.Unmount(target, mntDetach); err == nil {
		return nil
	}

	// Slow path: get all submounts, sort, unmount one by one.
	mounts, err := mountinfo.GetMounts(mountinfo.PrefixFilter(target))
	if err != nil {
		return err
	}

	// Make the deepest mount be first
	sort.Slice(mounts, func(i, j int) bool {
		return len(mounts[i].Mountpoint) > len(mounts[j].Mountpoint)
	})

	var (
		suberr    error
		lastMount = len(mounts) - 1
	)
	for i, m := range mounts {
		err = Unmount(m.Mountpoint)
		if err != nil {
			if i == lastMount {
				if suberr != nil {
					return fmt.Errorf("%w (possible cause: %s)", err, suberr)
				}
				return err
			}
			// This is a submount, we can ignore the error for now,
			// the final unmount will fail if this is a real problem.
			// With that in mind, the _first_ failed unmount error
			// might be the real error cause, so let's keep it.
			if suberr == nil {
				suberr = err
			}
		}
	}
	return nil
}
