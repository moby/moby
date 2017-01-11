// +build !linux

package oci

import (
	"errors"

	"github.com/opencontainers/runc/libcontainer/configs"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Device transforms a libcontainer configs.Device to a specs.Device object.
// Not implemented
func Device(d *configs.Device) specs.Device { return specs.Device{} }

// DevicesFromPath computes a list of devices and device permissions from paths (pathOnHost and pathInContainer) and cgroup permissions.
// Not implemented
func DevicesFromPath(pathOnHost, pathInContainer, cgroupPermissions string) (devs []specs.Device, devPermissions []specs.DeviceCgroup, err error) {
	return nil, nil, errors.New("oci/devices: unsupported platform")
}
