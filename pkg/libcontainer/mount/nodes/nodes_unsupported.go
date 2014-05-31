// +build !linux

package nodes

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/devices"
)

func CreateDeviceNodes(rootfs string, nodesToCreate []*devices.Device) error {
	return libcontainer.ErrUnsupported
}
