// +build linux

package nodes

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dotcloud/docker/pkg/libcontainer/devices"
	"github.com/dotcloud/docker/pkg/system"
)

// Create the device nodes in the container.
func CreateDeviceNodes(rootfs string, nodesToCreate []devices.Device) error {
	oldMask := system.Umask(0000)
	defer system.Umask(oldMask)

	for _, node := range nodesToCreate {
		if err := CreateDeviceNode(rootfs, node); err != nil {
			return err
		}
	}
	return nil
}

// Creates the device node in the rootfs of the container.
func CreateDeviceNode(rootfs string, node devices.Device) error {
	var (
		dest   = filepath.Join(rootfs, node.Path)
		parent = filepath.Dir(dest)
	)

	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}

	fileMode := node.FileMode
	switch node.Type {
	case 'c':
		fileMode |= syscall.S_IFCHR
	case 'b':
		fileMode |= syscall.S_IFBLK
	default:
		return fmt.Errorf("%c is not a valid device type for device %s", node.Type, node.Path)
	}

	if err := system.Mknod(dest, uint32(fileMode), devices.Mkdev(node.MajorNumber, node.MinorNumber)); err != nil && !os.IsExist(err) {
		return fmt.Errorf("mknod %s %s", node.Path, err)
	}
	return nil
}

func getDeviceNodes(path string) ([]string, error) {
	out := []string{}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if f.IsDir() && f.Name() != "pts" && f.Name() != "shm" {
			sub, err := getDeviceNodes(filepath.Join(path, f.Name()))
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
	return getDeviceNodes("/dev")
}
