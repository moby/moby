// +build linux

package restrict

import (
	"fmt"
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
	return nil
}
