package execdriver

import (
	"github.com/dotcloud/docker/pkg/libcontainer/devices"
)

var (
	DefaultAllowedDevices     = devices.DefaultAllowedDevices
	DefaultAutoCreatedDevices = devices.DefaultAutoCreatedDevices
)
