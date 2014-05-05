// +build linux

package nodes

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/system"
	"os"
	"path/filepath"
	"syscall"
)

// Default list of device nodes to copy
var DefaultNodes = []string{
	"null",
	"zero",
	"full",
	"random",
	"urandom",
	"tty",
}

// CopyN copies the device node from the host into the rootfs
func CopyN(rootfs string, nodesToCopy []string) error {
	oldMask := system.Umask(0000)
	defer system.Umask(oldMask)

	for _, node := range nodesToCopy {
		if err := Copy(rootfs, node); err != nil {
			return err
		}
	}
	return nil
}

func Copy(rootfs, node string) error {
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
