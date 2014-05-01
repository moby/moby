// +build linux

package restrict

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dotcloud/docker/pkg/system"
)

// This has to be called while the container still has CAP_SYS_ADMIN (to be able to perform mounts).
// However, afterwards, CAP_SYS_ADMIN should be dropped (otherwise the user will be able to revert those changes).
func Restrict() error {
	// remount proc and sys as readonly
	for _, dest := range []string{"proc", "sys"} {
		if err := system.Mount("", dest, "", syscall.MS_REMOUNT|syscall.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("unable to remount %s readonly: %s", dest, err)
		}
	}
	if err := system.Mount("/dev/null", "/proc/kcore", "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("unable to bind-mount /dev/null over /proc/kcore")
	}

	// This weird trick will allow us to mount /proc read-only, while being able to use AppArmor.
	// This is because apparently, loading an AppArmor profile requires write access to /proc/1/attr.
	// So we do another mount of procfs, ensure it's write-able, and bind-mount a subset of it.
	if err := os.Mkdir(".proc", 0700); err != nil {
		return fmt.Errorf("unable to create temporary proc mountpoint .proc: %s", err)
	}
	if err := system.Mount("proc", ".proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("unable to mount proc on temporary proc mountpoint: %s", err)
	}
	if err := system.Mount("proc", ".proc", "", syscall.MS_REMOUNT, ""); err != nil {
		return fmt.Errorf("unable to remount proc read-write: %s", err)
	}
	for _, path := range []string{"attr", "task"} {
		if err := system.Mount(filepath.Join(".proc", "1", path), filepath.Join("proc", "1", path), "", syscall.MS_BIND, ""); err != nil {
			return fmt.Errorf("unable to bind-mount %s: %s", path, err)
		}
	}
	if err := system.Unmount(".proc", 0); err != nil {
		return fmt.Errorf("unable to unmount temporary proc filesystem: %s", err)
	}
	return os.RemoveAll(".proc")
}
