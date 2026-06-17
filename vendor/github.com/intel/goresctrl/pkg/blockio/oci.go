/*
Copyright 2019-2021 Intel Corporation

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

package blockio

import (
	"fmt"

	oci "github.com/opencontainers/runtime-spec/specs-go"
)

// OciLinuxBlockIO returns OCI LinuxBlockIO structure corresponding to the class.
func OciLinuxBlockIO(class string) (*oci.LinuxBlockIO, error) {
	blockio, ok := classBlockIO[class]
	if !ok {
		return nil, fmt.Errorf("no OCI BlockIO parameters for class %#v", class)
	}
	ociBlockio := oci.LinuxBlockIO{}
	if blockio.Weight != -1 {
		w := uint16(blockio.Weight)
		ociBlockio.Weight = &w
	}
	ociBlockio.WeightDevice = ociLinuxWeightDevices(blockio.WeightDevice)
	ociBlockio.ThrottleReadBpsDevice = ociLinuxThrottleDevices(blockio.ThrottleReadBpsDevice)
	ociBlockio.ThrottleWriteBpsDevice = ociLinuxThrottleDevices(blockio.ThrottleWriteBpsDevice)
	ociBlockio.ThrottleReadIOPSDevice = ociLinuxThrottleDevices(blockio.ThrottleReadIOPSDevice)
	ociBlockio.ThrottleWriteIOPSDevice = ociLinuxThrottleDevices(blockio.ThrottleWriteIOPSDevice)
	return &ociBlockio, nil
}

func ociLinuxWeightDevices(dws DeviceWeights) []oci.LinuxWeightDevice {
	if len(dws) == 0 {
		return nil
	}
	olwds := make([]oci.LinuxWeightDevice, len(dws))
	for i, wd := range dws {
		w := uint16(wd.Weight)
		olwds[i].Major = wd.Major
		olwds[i].Minor = wd.Minor
		olwds[i].Weight = &w
	}
	return olwds
}

func ociLinuxThrottleDevices(drs DeviceRates) []oci.LinuxThrottleDevice {
	if len(drs) == 0 {
		return nil
	}
	oltds := make([]oci.LinuxThrottleDevice, len(drs))
	for i, dr := range drs {
		oltds[i].Major = dr.Major
		oltds[i].Minor = dr.Minor
		oltds[i].Rate = uint64(dr.Rate)
	}
	return oltds
}
