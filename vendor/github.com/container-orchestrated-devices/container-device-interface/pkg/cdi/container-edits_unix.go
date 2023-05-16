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
	"fmt"

	runc "github.com/opencontainers/runc/libcontainer/devices"
)

// fillMissingInfo fills in missing mandatory attributes from the host device.
func (d *DeviceNode) fillMissingInfo() error {
	if d.HostPath == "" {
		d.HostPath = d.Path
	}

	if d.Type != "" && (d.Major != 0 || d.Type == "p") {
		return nil
	}

	hostDev, err := runc.DeviceFromPath(d.HostPath, "rwm")
	if err != nil {
		return fmt.Errorf("failed to stat CDI host device %q: %w", d.HostPath, err)
	}

	if d.Type == "" {
		d.Type = string(hostDev.Type)
	} else {
		if d.Type != string(hostDev.Type) {
			return fmt.Errorf("CDI device (%q, %q), host type mismatch (%s, %s)",
				d.Path, d.HostPath, d.Type, string(hostDev.Type))
		}
	}
	if d.Major == 0 && d.Type != "p" {
		d.Major = hostDev.Major
		d.Minor = hostDev.Minor
	}

	return nil
}
