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

// FromOCILinuxNetDevice returns a LinuxNetDevice for the given OCI LinuxNetDevice.
func FromOCILinuxNetDevice(o rspec.LinuxNetDevice) *LinuxNetDevice {
	return &LinuxNetDevice{
		Name: o.Name,
	}
}

// FromOCILinuxNetDevices returns LinuxNetDevice's for the given OCI LinuxNetDevice's.
func FromOCILinuxNetDevices(o map[string]rspec.LinuxNetDevice) map[string]*LinuxNetDevice {
	if len(o) == 0 {
		return nil
	}

	devices := make(map[string]*LinuxNetDevice, len(o))
	for host, dev := range o {
		devices[host] = FromOCILinuxNetDevice(dev)
	}

	return devices
}

// ToOCI returns the OCI LinuxNetDevice corresponding to the LinuxNetDevice.
func (d *LinuxNetDevice) ToOCI() rspec.LinuxNetDevice {
	if d == nil {
		return rspec.LinuxNetDevice{}
	}

	return rspec.LinuxNetDevice{
		Name: d.Name,
	}
}

// ToOCILinuxNetDevices returns the OCI LinuxNetDevice's corresponding to the LinuxNetDevice's.
func ToOCILinuxNetDevices(devices map[string]*LinuxNetDevice) map[string]rspec.LinuxNetDevice {
	if devices == nil {
		return nil
	}

	o := make(map[string]rspec.LinuxNetDevice, len(devices))
	for host, dev := range devices {
		o[host] = dev.ToOCI()
	}

	return o
}
