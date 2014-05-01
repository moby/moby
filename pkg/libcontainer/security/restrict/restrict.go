// +build linux

package restrict

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dotcloud/docker/pkg/system"
)

// "restrictions" are container paths (files, directories, whatever) that have to be masked.
// maskPath is a "safe" path to be mounted over maskedPath. It can take two special values:
// - if it is "", then nothing is mounted;
// - if it is "EMPTY", then an empty directory is mounted instead.
// If remountRO is true then the maskedPath is remounted read-only (regardless of whether a maskPath was used).
type restriction struct {
	maskedPath string
	maskPath   string
	remountRO  bool
}

var restrictions = []restriction{
	{"/proc", "", true},
	{"/sys", "", true},
	{"/proc/kcore", "/dev/null", false},
}

// This has to be called while the container still has CAP_SYS_ADMIN (to be able to perform mounts).
// However, afterwards, CAP_SYS_ADMIN should be dropped (otherwise the user will be able to revert those changes).
// "empty" should be the path to an empty directory.
func Restrict(rootfs, empty string) error {
	for _, restriction := range restrictions {
		dest := filepath.Join(rootfs, restriction.maskedPath)
		if restriction.maskPath != "" {
			var source string
			if restriction.maskPath == "EMPTY" {
				source = empty
			} else {
				source = filepath.Join(rootfs, restriction.maskPath)
			}
			if err := system.Mount(source, dest, "", syscall.MS_BIND, ""); err != nil {
				return fmt.Errorf("unable to bind-mount %s over %s: %s", source, dest, err)
			}
		}
		if restriction.remountRO {
			if err := system.Mount("", dest, "", syscall.MS_REMOUNT|syscall.MS_RDONLY, ""); err != nil {
				return fmt.Errorf("unable to remount %s readonly: %s", dest, err)
			}
		}
	}

	// This weird trick will allow us to mount /proc read-only, while being able to use AppArmor.
	// This is because apparently, loading an AppArmor profile requires write access to /proc/1/attr.
	// So we do another mount of procfs, ensure it's write-able, and bind-mount a subset of it.
	tmpProcPath := filepath.Join(rootfs, ".proc")
	if err := os.Mkdir(tmpProcPath, 0700); err != nil {
		return fmt.Errorf("unable to create temporary proc mountpoint %s: %s", tmpProcPath, err)
	}
	if err := system.Mount("proc", tmpProcPath, "proc", 0, ""); err != nil {
		return fmt.Errorf("unable to mount proc on temporary proc mountpoint: %s", err)
	}
	if err := system.Mount("proc", tmpProcPath, "", syscall.MS_REMOUNT, ""); err != nil {
		return fmt.Errorf("unable to remount proc read-write: %s", err)
	}
	rwAttrPath := filepath.Join(rootfs, ".proc", "1", "attr")
	roAttrPath := filepath.Join(rootfs, "proc", "1", "attr")
	if err := system.Mount(rwAttrPath, roAttrPath, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("unable to bind-mount %s on %s: %s", rwAttrPath, roAttrPath, err)
	}
	if err := system.Unmount(tmpProcPath, 0); err != nil {
		return fmt.Errorf("unable to unmount temporary proc filesystem: %s", err)
	}
	return nil
}
