// +build linux

package console

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/docker/libcontainer/label"
	"github.com/dotcloud/docker/pkg/system"
)

// Setup initializes the proper /dev/console inside the rootfs path
func Setup(rootfs, consolePath, mountLabel string) error {
	oldMask := system.Umask(0000)
	defer system.Umask(oldMask)

	if err := os.Chmod(consolePath, 0600); err != nil {
		return err
	}
	if err := os.Chown(consolePath, 0, 0); err != nil {
		return err
	}
	if err := label.SetFileLabel(consolePath, mountLabel); err != nil {
		return fmt.Errorf("set file label %s %s", consolePath, err)
	}

	dest := filepath.Join(rootfs, "dev/console")

	f, err := os.Create(dest)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("create %s %s", dest, err)
	}
	if f != nil {
		f.Close()
	}

	if err := system.Mount(consolePath, dest, "bind", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind %s to %s %s", consolePath, dest, err)
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
