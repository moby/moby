// +build linux

package mount

import (
	"github.com/dotcloud/docker/pkg/system"
	"syscall"
)

func RemountProc() error {
	if err := system.Unmount("/proc", syscall.MNT_DETACH); err != nil {
		return err
	}
	if err := system.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), ""); err != nil {
		return err
	}
	return nil
}

func RemountSys() error {
	if err := system.Unmount("/sys", syscall.MNT_DETACH); err != nil {
		if err != syscall.EINVAL {
			return err
		}
	} else {
		if err := system.Mount("sysfs", "/sys", "sysfs", uintptr(defaultMountFlags), ""); err != nil {
			return err
		}
	}
	return nil
}
