// +build linux

package mount

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/system"
	"syscall"
)

func MsMoveRoot(rootfs string) error {
	if err := system.Mount(rootfs, "/", "", syscall.MS_MOVE, ""); err != nil {
		return fmt.Errorf("mount move %s into / %s", rootfs, err)
	}
	if err := system.Chroot("."); err != nil {
		return fmt.Errorf("chroot . %s", err)
	}
	return system.Chdir("/")
}
