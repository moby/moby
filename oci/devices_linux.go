package oci // import "github.com/docker/docker/oci"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	coci "github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func deviceCgroup(d *specs.LinuxDevice, permissions string) specs.LinuxDeviceCgroup {
	return specs.LinuxDeviceCgroup{
		Allow:  true,
		Type:   d.Type,
		Major:  &d.Major,
		Minor:  &d.Minor,
		Access: permissions,
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

	device, err := coci.DeviceFromPath(resolvedPathOnHost)
	// if there was no error, return the device
	if err == nil {
		device.Path = pathInContainer
		return append(devs, *device), append(devPermissions, deviceCgroup(device, cgroupPermissions)), nil
	}

	// if the device is not a device node
	// try to see if it's a directory holding many devices
	if err == coci.ErrNotADevice {
		// check if it is a directory
		if src, e := os.Stat(resolvedPathOnHost); e == nil && src.IsDir() {
			// mount the internal devices recursively
			// TODO check if additional errors should be handled or logged
			_ = filepath.WalkDir(resolvedPathOnHost, func(dpath string, f os.DirEntry, _ error) error {
				childDevice, e := coci.DeviceFromPath(dpath)
				if e != nil {
					// ignore the device
					return nil
				}

				// add the device to userSpecified devices
				childDevice.Path = strings.Replace(dpath, resolvedPathOnHost, pathInContainer, 1)
				devs = append(devs, *childDevice)
				devPermissions = append(devPermissions, deviceCgroup(childDevice, cgroupPermissions))

				return nil
			})
		}
	}

	if len(devs) > 0 {
		return devs, devPermissions, nil
	}

	return devs, devPermissions, fmt.Errorf("error gathering device information while adding custom device %q: %s", pathOnHost, err)
}
