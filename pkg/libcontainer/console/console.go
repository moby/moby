// +build linux

package console

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dotcloud/docker/pkg/label"
	"github.com/dotcloud/docker/pkg/system"
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
	return nil
}

func OpenAndDup(consolePath string) error {
	slave, err := system.OpenTerminal(consolePath, syscall.O_RDWR)
	if err != nil {
		return fmt.Errorf("open terminal %s", err)
	}
	if err := system.Dup2(slave.Fd(), 0); err != nil {
		return err
	}
	if err := system.Dup2(slave.Fd(), 1); err != nil {
		return err
	}
	return system.Dup2(slave.Fd(), 2)
}
