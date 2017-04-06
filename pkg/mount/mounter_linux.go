package mount

import (
	"syscall"
)

const (
	// ptypes is the set propagation types.
	ptypes = syscall.MS_SHARED | syscall.MS_PRIVATE | syscall.MS_SLAVE | syscall.MS_UNBINDABLE

	// pflags is the full set valid flags for a change propagation call.
	pflags = ptypes | syscall.MS_REC | syscall.MS_SILENT

	// broflags is the combination of bind and read only
	broflags = syscall.MS_BIND | syscall.MS_RDONLY
)

// isremount returns true if either device name or flags identify a remount request, false otherwise.
func isremount(device string, flags uintptr) bool {
	switch {
	// We treat device "" and "none" as a remount request to provide compatibility with
	// requests that don't explicitly set MS_REMOUNT such as those manipulating bind mounts.
	case flags&syscall.MS_REMOUNT != 0, device == "", device == "none":
		return true
	default:
		return false
	}
}

func mount(device, target, mType string, flags uintptr, data string) error {
	oflags := flags &^ ptypes
	if !isremount(device, flags) {
		// Initial call applying all non-propagation flags.
		if err := syscall.Mount(device, target, mType, oflags, data); err != nil {
			return err
		}
	}

	if flags&ptypes != 0 {
		// Change the propagation type.
		if err := syscall.Mount("", target, "", flags&pflags, ""); err != nil {
			return err
		}
	}

	if oflags&broflags == broflags {
		// Remount the bind to apply read only.
		return syscall.Mount("", target, "", oflags|syscall.MS_REMOUNT, "")
	}

	return nil
}

func unmount(target string, flag int) error {
	return syscall.Unmount(target, flag)
}
