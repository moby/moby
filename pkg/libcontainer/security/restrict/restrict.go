// +build linux

package restrict

import (
	"fmt"
	"os"
	"syscall"

	"github.com/dotcloud/docker/pkg/system"
)

const defaultMountFlags = syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV

func mountReadonly(path string) error {
	if err := system.Mount("", path, "", syscall.MS_REMOUNT|syscall.MS_RDONLY, ""); err != nil {
		if err == syscall.EINVAL {
			// Probably not a mountpoint, use bind-mount
			if err := system.Mount(path, path, "", syscall.MS_BIND, ""); err != nil {
				return err
			}
			if err := system.Mount(path, path, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_REC|defaultMountFlags, ""); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

// This has to be called while the container still has CAP_SYS_ADMIN (to be able to perform mounts).
// However, afterwards, CAP_SYS_ADMIN should be dropped (otherwise the user will be able to revert those changes).
func Restrict(mounts ...string) error {
	// remount proc and sys as readonly
	for _, dest := range mounts {
		if err := mountReadonly(dest); err != nil {
			return fmt.Errorf("unable to remount %s readonly: %s", dest, err)
		}
	}
	if err := system.Mount("/dev/null", "/proc/kcore", "", syscall.MS_BIND, ""); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to bind-mount /dev/null over /proc/kcore: %s", err)
	}
	return nil
}
