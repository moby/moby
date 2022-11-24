//go:build !windows

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package oci

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/pkg/userns"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

// ErrNotADevice denotes that a file is not a valid linux device.
var ErrNotADevice = errors.New("not a device node")

// Testing dependencies
var (
	osReadDir              = os.ReadDir
	usernsRunningInUserNS  = userns.RunningInUserNS
	overrideDeviceFromPath func(path string) error
)

// HostDevices returns all devices that can be found under /dev directory.
func HostDevices() ([]specs.LinuxDevice, error) {
	return getDevices("/dev", "")
}

func getDevices(path, containerPath string) ([]specs.LinuxDevice, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("error stating device path: %w", err)
	}

	if !stat.IsDir() {
		dev, err := DeviceFromPath(path)
		if err != nil {
			return nil, err
		}
		if containerPath != "" {
			dev.Path = containerPath
		}
		return []specs.LinuxDevice{*dev}, nil
	}

	files, err := osReadDir(path)
	if err != nil {
		return nil, err
	}
	var out []specs.LinuxDevice
	for _, f := range files {
		switch {
		case f.IsDir():
			switch f.Name() {
			// ".lxc" & ".lxd-mounts" added to address https://github.com/lxc/lxd/issues/2825
			// ".udev" added to address https://github.com/opencontainers/runc/issues/2093
			case "pts", "shm", "fd", "mqueue", ".lxc", ".lxd-mounts", ".udev":
				continue
			default:
				var cp string
				if containerPath != "" {
					cp = filepath.Join(containerPath, filepath.Base(f.Name()))
				}
				sub, err := getDevices(filepath.Join(path, f.Name()), cp)
				if err != nil {
					if errors.Is(err, os.ErrPermission) && usernsRunningInUserNS() {
						// ignore the "permission denied" error if running in userns.
						// This allows rootless containers to use devices that are
						// accessible, ignoring devices / subdirectories that are not.
						continue
					}
					return nil, err
				}

				out = append(out, sub...)
				continue
			}
		case f.Name() == "console":
			continue
		default:
			device, err := DeviceFromPath(filepath.Join(path, f.Name()))
			if err != nil {
				if err == ErrNotADevice {
					continue
				}
				if os.IsNotExist(err) {
					continue
				}
				if errors.Is(err, os.ErrPermission) && usernsRunningInUserNS() {
					// ignore the "permission denied" error if running in userns.
					// This allows rootless containers to use devices that are
					// accessible, ignoring devices that are not.
					continue
				}
				return nil, err
			}
			if device.Type == fifoDevice {
				continue
			}
			if containerPath != "" {
				device.Path = filepath.Join(containerPath, filepath.Base(f.Name()))
			}
			out = append(out, *device)
		}
	}
	return out, nil
}

// TODO consider adding these consts to the OCI runtime-spec.
const (
	wildcardDevice = "a" //nolint:nolintlint,unused,varcheck // currently unused, but should be included when upstreaming to OCI runtime-spec.
	blockDevice    = "b"
	charDevice     = "c" // or "u"
	fifoDevice     = "p"
)

// DeviceFromPath takes the path to a device to look up the information about a
// linux device and returns that information as a LinuxDevice struct.
func DeviceFromPath(path string) (*specs.LinuxDevice, error) {
	if overrideDeviceFromPath != nil {
		if err := overrideDeviceFromPath(path); err != nil {
			return nil, err
		}
	}

	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return nil, err
	}

	var (
		devNumber = uint64(stat.Rdev) //nolint:nolintlint,unconvert // the type is 32bit on mips.
		major     = unix.Major(devNumber)
		minor     = unix.Minor(devNumber)
	)

	var (
		devType string
		mode    = stat.Mode
	)

	switch mode & unix.S_IFMT {
	case unix.S_IFBLK:
		devType = blockDevice
	case unix.S_IFCHR:
		devType = charDevice
	case unix.S_IFIFO:
		devType = fifoDevice
	default:
		return nil, ErrNotADevice
	}
	fm := os.FileMode(mode &^ unix.S_IFMT)
	return &specs.LinuxDevice{
		Type:     devType,
		Path:     path,
		Major:    int64(major),
		Minor:    int64(minor),
		FileMode: &fm,
		UID:      &stat.Uid,
		GID:      &stat.Gid,
	}, nil
}
