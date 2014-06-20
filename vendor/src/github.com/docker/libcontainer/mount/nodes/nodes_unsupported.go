// +build !linux

package nodes

import (
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/devices"
)

func CreateDeviceNodes(rootfs string, nodesToCreate []*devices.Device) error {
	return libcontainer.ErrUnsupported
}
