// +build !linux

package nodes

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/devices"
)

func GetHostDeviceNodes() ([]string, error) {
	return nil, libcontainer.ErrUnsupported
}

func CreateDeviceNodes(rootfs string, nodesToCreate []devices.Device) error {
	return libcontainer.ErrUnsupported
}
