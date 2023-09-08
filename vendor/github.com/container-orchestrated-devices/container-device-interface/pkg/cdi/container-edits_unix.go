//go:build !windows
// +build !windows

/*
   Copyright © 2021 The CDI Authors

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

package cdi

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

const (
	blockDevice = "b"
	charDevice  = "c" // or "u"
	fifoDevice  = "p"
)

// deviceInfoFromPath takes the path to a device and returns its type,
// major and minor device numbers.
//
// It was adapted from https://github.com/opencontainers/runc/blob/v1.1.9/libcontainer/devices/device_unix.go#L30-L69
func deviceInfoFromPath(path string) (devType string, major, minor int64, _ error) {
	var stat unix.Stat_t
	err := unix.Lstat(path, &stat)
	if err != nil {
		return "", 0, 0, err
	}
	switch stat.Mode & unix.S_IFMT {
	case unix.S_IFBLK:
		devType = blockDevice
	case unix.S_IFCHR:
		devType = charDevice
	case unix.S_IFIFO:
		devType = fifoDevice
	default:
		return "", 0, 0, errors.New("not a device node")
	}
	devNumber := uint64(stat.Rdev) //nolint:unconvert // Rdev is uint32 on e.g. MIPS.
	return devType, int64(unix.Major(devNumber)), int64(unix.Minor(devNumber)), nil
}

// fillMissingInfo fills in missing mandatory attributes from the host device.
func (d *DeviceNode) fillMissingInfo() error {
	if d.HostPath == "" {
		d.HostPath = d.Path
	}

	if d.Type != "" && (d.Major != 0 || d.Type == "p") {
		return nil
	}

	deviceType, major, minor, err := deviceInfoFromPath(d.HostPath)
	if err != nil {
		return fmt.Errorf("failed to stat CDI host device %q: %w", d.HostPath, err)
	}

	if d.Type == "" {
		d.Type = deviceType
	} else {
		if d.Type != deviceType {
			return fmt.Errorf("CDI device (%q, %q), host type mismatch (%s, %s)",
				d.Path, d.HostPath, d.Type, deviceType)
		}
	}
	if d.Major == 0 && d.Type != "p" {
		d.Major = major
		d.Minor = minor
	}

	return nil
}
