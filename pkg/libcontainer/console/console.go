// +build linux

package console

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/label"
	"github.com/dotcloud/docker/pkg/system"
	"os"
	"path/filepath"
	"syscall"
)

// Setup initializes the proper /dev/console inside the rootfs path
func Setup(rootfs, consolePath, mountLabel string) error {
	oldMask := system.Umask(0000)
	defer system.Umask(oldMask)

	stat, err := os.Stat(consolePath)
	if err != nil {
		return fmt.Errorf("stat console %s %s", consolePath, err)
	}
	var (
		st   = stat.Sys().(*syscall.Stat_t)
		dest = filepath.Join(rootfs, "dev/console")
	)
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s %s", dest, err)
	}
	if err := os.Chmod(consolePath, 0600); err != nil {
		return err
	}
	if err := os.Chown(consolePath, 0, 0); err != nil {
		return err
	}
	if err := system.Mknod(dest, (st.Mode&^07777)|0600, int(st.Rdev)); err != nil {
		return fmt.Errorf("mknod %s %s", dest, err)
	}
	if err := label.SetFileLabel(consolePath, mountLabel); err != nil {
		return fmt.Errorf("set file label %s %s", dest, err)
	}
	if err := system.Mount(consolePath, dest, "bind", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind %s to %s %s", consolePath, dest, err)
	}
	return nil
}
