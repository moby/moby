// +build linux

package nodes

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dotcloud/docker/pkg/system"
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
func CopyN(rootfs string, nodesToCopy []string, shouldExist bool) error {
	oldMask := system.Umask(0000)
	defer system.Umask(oldMask)

	for _, node := range nodesToCopy {
		if err := Copy(rootfs, node, shouldExist); err != nil {
			return err
		}
	}
	return nil
}

// Copy copies the device node into the rootfs.  If the node
// on the host system does not exist and the boolean flag is passed
// an error will be returned
func Copy(rootfs, node string, shouldExist bool) error {
	stat, err := os.Stat(filepath.Join("/dev", node))
	if err != nil {
		if os.IsNotExist(err) && !shouldExist {
			return nil
		}
		return err
	}

	var (
		dest = filepath.Join(rootfs, "dev", node)
		st   = stat.Sys().(*syscall.Stat_t)
	)

	if err := system.Mknod(dest, st.Mode, int(st.Rdev)); err != nil && !os.IsExist(err) {
		return fmt.Errorf("mknod %s %s", node, err)
	}
	return nil
}

func GetHostDeviceNodes() ([]string, error) {
	files, err := ioutil.ReadDir("/dev")
	if err != nil {
		return nil, err
	}

	out := []string{}
	for _, f := range files {
		if f.Mode()&os.ModeDevice == os.ModeDevice {
			out = append(out, f.Name())
		}
	}
	return out, nil
}
