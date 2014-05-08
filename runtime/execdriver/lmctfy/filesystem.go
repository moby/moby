package lmctfy

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/system"
	"github.com/dotcloud/docker/runtime/execdriver"
	"os"
	"path/filepath"
	"syscall"
)

func createDefaultPaths(rootfs string) error {
	// make /proc, /sysfs, /dev/shm, /dev/pts, /dev/ptmx
	for _, dir := range []string{
		"/proc",
		"/sysfs",
		"/dev/shm",
		"/dev/pts",
	} {
		if err := os.MkdirAll(filepath.Join(rootfs, dir), 0700); err != nil {
			return err
		}
	}
	return nil
}

// copyDevNodes mknods the hosts devices so the new container has access to them
func copyDevNodes(rootfs string) error {
	oldMask := system.Umask(0000)
	defer system.Umask(oldMask)

	for _, node := range []string{
		"null",
		"zero",
		"full",
		"ptmx",
		"random",
		"urandom",
		"tty",
	} {
		if err := copyDevNode(rootfs, node); err != nil {
			return err
		}
	}
	return nil
}

func copyDevNode(rootfs, node string) error {
	stat, err := os.Stat(filepath.Join("/dev", node))
	if err != nil {
		return err
	}
	var (
		dest = filepath.Join(rootfs, "dev", node)
		st   = stat.Sys().(*syscall.Stat_t)
	)
	if err := system.Mknod(dest, st.Mode, int(st.Rdev)); err != nil && !os.IsExist(err) {
		return fmt.Errorf("copy %s %s", node, err)
	}
	return nil
}

func createDevConsole(rootfs, console string) error {
	stat, err := os.Stat(console)
	if err != nil {
		return fmt.Errorf("stat console %s %s", console, err)
	}
	var (
		st   = stat.Sys().(*syscall.Stat_t)
		dest = filepath.Join(rootfs, "dev/console")
	)
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s %s", dest, err)
	}
	if err := os.Chmod(console, 0600); err != nil {
		return err
	}
	if err := os.Chown(console, 0, 0); err != nil {
		return err
	}
	if err := system.Mknod(dest, (st.Mode&^07777)|0600, int(st.Rdev)); err != nil {
		return fmt.Errorf("mknod %s %s", dest, err)
	}
	return nil
}

func performFsSetup(c *execdriver.Command) error {
	if err := createDefaultPaths(c.Rootfs); err != nil {
		return err
	}
	if err := copyDevNodes(c.Rootfs); err != nil {
		return err
	}
	if c.Console != "" {
		if err := createDevConsole(c.Rootfs, c.Console); err != nil {
			return err
		}
	}
	return nil
}
