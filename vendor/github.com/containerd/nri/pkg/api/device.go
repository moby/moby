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

package api

import (
	rspec "github.com/opencontainers/runtime-spec/specs-go"
)

// FromOCILinuxDevices returns a device slice from an OCI runtime Spec.
func FromOCILinuxDevices(o []rspec.LinuxDevice) []*LinuxDevice {
	var devices []*LinuxDevice
	for _, d := range o {
		devices = append(devices, &LinuxDevice{
			Path:     d.Path,
			Type:     d.Type,
			Major:    d.Major,
			Minor:    d.Minor,
			FileMode: FileMode(d.FileMode),
			Uid:      UInt32(d.UID),
			Gid:      UInt32(d.GID),
		})
	}
	return devices
}

// ToOCI returns the linux devices for an OCI runtime Spec.
func (d *LinuxDevice) ToOCI() rspec.LinuxDevice {
	if d == nil {
		return rspec.LinuxDevice{}
	}

	return rspec.LinuxDevice{
		Path:     d.Path,
		Type:     d.Type,
		Major:    d.Major,
		Minor:    d.Minor,
		FileMode: d.FileMode.Get(),
		UID:      d.Uid.Get(),
		GID:      d.Gid.Get(),
	}
}

// AccessString returns an OCI access string for the device.
func (d *LinuxDevice) AccessString() string {
	r, w, m := "r", "w", ""

	if mode := d.FileMode.Get(); mode != nil {
		perm := mode.Perm()
		if (perm & 0444) != 0 {
			r = "r"
		}
		if (perm & 0222) != 0 {
			w = "w"
		}
	}
	if d.Type == "b" {
		m = "m"
	}

	return r + w + m
}

// Cmp returns true if the devices are equal.
func (d *LinuxDevice) Cmp(v *LinuxDevice) bool {
	if v == nil {
		return false
	}
	return d.Major != v.Major || d.Minor != v.Minor
}

// IsMarkedForRemoval checks if a LinuxDevice is marked for removal.
func (d *LinuxDevice) IsMarkedForRemoval() (string, bool) {
	key, marked := IsMarkedForRemoval(d.Path)
	return key, marked
}
