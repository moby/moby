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
	"/dev/null",
	"/dev/zero",
	"/dev/full",
	"/dev/random",
	"/dev/urandom",
	"/dev/tty",
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
	stat, err := os.Stat(node)
	if err != nil {
		if os.IsNotExist(err) && !shouldExist {
			return nil
		}
		return err
	}

	var (
		dest   = filepath.Join(rootfs, node)
		st     = stat.Sys().(*syscall.Stat_t)
		parent = filepath.Dir(dest)
	)

	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}

	if err := system.Mknod(dest, st.Mode, int(st.Rdev)); err != nil && !os.IsExist(err) {
		return fmt.Errorf("mknod %s %s", node, err)
	}
	return nil
}

func getNodes(path string) ([]string, error) {
	out := []string{}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if f.IsDir() && f.Name() != "pts" && f.Name() != "shm" {
			sub, err := getNodes(filepath.Join(path, f.Name()))
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		} else if f.Mode()&os.ModeDevice == os.ModeDevice {
			out = append(out, filepath.Join(path, f.Name()))
		}
	}
	return out, nil
}

func GetHostDeviceNodes() ([]string, error) {
	return getNodes("/dev")
}
