//go:build !windows

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
	"os"

	"golang.org/x/sys/unix"
)

const (
	blockDevice = "b"
	charDevice  = "c" // or "u"
	fifoDevice  = "p"
)

type deviceInfo struct {
	// cgroup properties
	deviceType string
	major      int64
	minor      int64

	// device node properties
	fileMode os.FileMode
}

// deviceInfoFromPath takes the path to a device and returns its type,
// major and minor device numbers.
//
// It was adapted from https://github.com/opencontainers/runc/blob/v1.1.9/libcontainer/devices/device_unix.go#L30-L69
func deviceInfoFromPath(path string) (*deviceInfo, error) {
	var stat unix.Stat_t
	err := unix.Lstat(path, &stat)
	if err != nil {
		return nil, err
	}

	var devType string
	switch stat.Mode & unix.S_IFMT {
	case unix.S_IFBLK:
		devType = blockDevice
	case unix.S_IFCHR:
		devType = charDevice
	case unix.S_IFIFO:
		devType = fifoDevice
	default:
		return nil, errors.New("not a device node")
	}
	devNumber := uint64(stat.Rdev) //nolint:unconvert // Rdev is uint32 on e.g. MIPS.

	di := deviceInfo{
		deviceType: devType,
		major:      int64(unix.Major(devNumber)),
		minor:      int64(unix.Minor(devNumber)),
		fileMode:   os.FileMode(stat.Mode &^ unix.S_IFMT),
	}

	return &di, nil
}

// fillMissingInfo fills in missing mandatory attributes from the host device.
func (d *DeviceNode) fillMissingInfo() error {
	hasMinimalSpecification := d.Type != "" && (d.Major != 0 || d.Type == fifoDevice)

	// Ensure that the host path and the container path match.
	if d.HostPath == "" {
		d.HostPath = d.Path
	}

	// Try to extract the device info from the host path.
	di, err := deviceInfoFromPath(d.HostPath)
	if err != nil {
		// The error is only considered fatal if the device is not already
		// minimally specified since it is allowed for a device vendor to fully
		// specify a device node specification.
		if !hasMinimalSpecification {
			return fmt.Errorf("failed to stat CDI host device %q: %w", d.HostPath, err)
		}
		return nil
	}

	// Even for minimally-specified device nodes, we update the file mode if
	// required. This is useful for rootless containers where device node
	// requests may be treated as bind mounts.
	if d.FileMode == nil {
		d.FileMode = &di.fileMode
	}

	// If the device is minimally specified, we make no further updates and
	// don't perform additional checks.
	if hasMinimalSpecification {
		return nil
	}

	if d.Type == "" {
		d.Type = di.deviceType
	}
	if d.Type != di.deviceType {
		return fmt.Errorf("CDI device (%q, %q), host type mismatch (%s, %s)",
			d.Path, d.HostPath, d.Type, di.deviceType)
	}

	// For a fifoDevice, we do not update the major and minor number.
	if d.Type == fifoDevice {
		return nil
	}

	// Update the major and minor number for the device node if required.
	if d.Major == 0 {
		d.Major = di.major
		d.Minor = di.minor
	}

	return nil
}
