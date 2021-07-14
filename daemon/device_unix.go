// +build !windows

package daemon

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/pkg/userns"
	"github.com/opencontainers/runc/libcontainer/devices"
)

// Testing dependencies
var (
	ioutilReadDir         = ioutil.ReadDir
	usernsRunningInUserNS = userns.RunningInUserNS
)

// HostDevices returns all devices that can be found under /dev directory.
// Based on HostDevices from runc (libcontainer/devices/device_unix.go).
func HostDevices() ([]*devices.Device, error) {
	return GetDevices("/dev")
}

// GetDevices recursively traverses a directory specified by path
// and returns all devices found there.
// Based on GetDevices from runc (libcontainer/devices/device_unix.go).
func GetDevices(path string) ([]*devices.Device, error) {
	files, err := ioutilReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrPermission) && usernsRunningInUserNS() {
			// ignore the "permission denied" error if running in userns
			return nil, nil
		}
		return nil, err
	}
	var out []*devices.Device
	for _, f := range files {
		switch {
		case f.IsDir():
			switch f.Name() {
			// ".lxc" & ".lxd-mounts" added to address https://github.com/lxc/lxd/issues/2825
			// ".udev" added to address https://github.com/opencontainers/runc/issues/2093
			case "pts", "shm", "fd", "mqueue", ".lxc", ".lxd-mounts", ".udev":
				continue
			default:
				sub, err := GetDevices(filepath.Join(path, f.Name()))
				if err != nil {
					return nil, err
				}

				if sub != nil {
					out = append(out, sub...)
				}
				continue
			}
		case f.Name() == "console":
			continue
		}
		device, err := devices.DeviceFromPath(filepath.Join(path, f.Name()), "rwm")
		if err != nil {
			if errors.Is(err, devices.ErrNotADevice) {
				continue
			}
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if device.Type == devices.FifoDevice {
			continue
		}
		out = append(out, device)
	}
	return out, nil
}
