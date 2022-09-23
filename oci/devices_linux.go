package oci // import "github.com/docker/docker/oci"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencontainers/runc/libcontainer/devices"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

// Device transforms a libcontainer devices.Device to a specs.LinuxDevice object.
func Device(d *devices.Device) specs.LinuxDevice {
	return specs.LinuxDevice{
		Type:     string(d.Type),
		Path:     d.Path,
		Major:    d.Major,
		Minor:    d.Minor,
		FileMode: fmPtr(int64(d.FileMode &^ unix.S_IFMT)), // strip file type, as OCI spec only expects file-mode to be included
		UID:      u32Ptr(int64(d.Uid)),
		GID:      u32Ptr(int64(d.Gid)),
	}
}

func deviceCgroup(d *devices.Device) specs.LinuxDeviceCgroup {
	return specs.LinuxDeviceCgroup{
		Allow:  true,
		Type:   string(d.Type),
		Major:  &d.Major,
		Minor:  &d.Minor,
		Access: string(d.Permissions),
	}
}

// DevicesFromPath computes a list of devices and device permissions from paths (pathOnHost and pathInContainer) and cgroup permissions.
func DevicesFromPath(pathOnHost, pathInContainer, cgroupPermissions string) (devs []specs.LinuxDevice, devPermissions []specs.LinuxDeviceCgroup, err error) {
	resolvedPathOnHost := pathOnHost

	// check if it is a symbolic link
	if src, e := os.Lstat(pathOnHost); e == nil && src.Mode()&os.ModeSymlink == os.ModeSymlink {
		if linkedPathOnHost, e := filepath.EvalSymlinks(pathOnHost); e == nil {
			resolvedPathOnHost = linkedPathOnHost
		}
	}

	device, err := devices.DeviceFromPath(resolvedPathOnHost, cgroupPermissions)
	// if there was no error, return the device
	if err == nil {
		device.Path = pathInContainer
		return append(devs, Device(device)), append(devPermissions, deviceCgroup(device)), nil
	}

	// if the device is not a device node
	// try to see if it's a directory holding many devices
	if err == devices.ErrNotADevice {
		// check if it is a directory
		if src, e := os.Stat(resolvedPathOnHost); e == nil && src.IsDir() {
			// mount the internal devices recursively
			// TODO check if additional errors should be handled or logged
			_ = filepath.Walk(resolvedPathOnHost, func(dpath string, f os.FileInfo, _ error) error {
				childDevice, e := devices.DeviceFromPath(dpath, cgroupPermissions)
				if e != nil {
					// ignore the device
					return nil
				}

				// add the device to userSpecified devices
				childDevice.Path = strings.Replace(dpath, resolvedPathOnHost, pathInContainer, 1)
				devs = append(devs, Device(childDevice))
				devPermissions = append(devPermissions, deviceCgroup(childDevice))

				return nil
			})
		}
	}

	if len(devs) > 0 {
		return devs, devPermissions, nil
	}

	return devs, devPermissions, fmt.Errorf("error gathering device information while adding custom device %q: %s", pathOnHost, err)
}
