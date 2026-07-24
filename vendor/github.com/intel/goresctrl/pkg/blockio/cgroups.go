// Copyright 2020-2021 Intel Corporation. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package blockio

// BlockIOParameters contains cgroups blockio controller parameters.
//
// Effects of Weight and Rate values in SetBlkioParameters():
// Value  |  Effect
// -------+-------------------------------------------------------------------
//
//	  -1  |  Do not write to cgroups, value is missing.
//	   0  |  Write to cgroups, will clear the setting as specified in cgroups blkio interface.
//	other |  Write to cgroups, sets the value.
type BlockIOParameters struct {
	Weight                  int64
	WeightDevice            DeviceWeights
	ThrottleReadBpsDevice   DeviceRates
	ThrottleWriteBpsDevice  DeviceRates
	ThrottleReadIOPSDevice  DeviceRates
	ThrottleWriteIOPSDevice DeviceRates
}

// DeviceWeight contains values for
// - blkio.[io-scheduler].weight
type DeviceWeight struct {
	Major  int64
	Minor  int64
	Weight int64
}

// DeviceRate contains values for
// - blkio.throttle.read_bps_device
// - blkio.throttle.write_bps_device
// - blkio.throttle.read_iops_device
// - blkio.throttle.write_iops_device
type DeviceRate struct {
	Major int64
	Minor int64
	Rate  int64
}

// DeviceWeights contains weights for devices.
type DeviceWeights []DeviceWeight

// DeviceRates contains throttling rates for devices.
type DeviceRates []DeviceRate

// NewBlockIOParameters creates new BlockIOParameters instance.
func NewBlockIOParameters() BlockIOParameters {
	return BlockIOParameters{
		Weight: -1,
	}
}

// DeviceParameters interface provides functions common to DeviceWeights and DeviceRates.
type DeviceParameters interface {
	Append(maj, min, val int64)
	Update(maj, min, val int64)
}

// Append appends (major, minor, value) to DeviceWeights slice.
func (w *DeviceWeights) Append(maj, min, val int64) {
	*w = append(*w, DeviceWeight{Major: maj, Minor: min, Weight: val})
}

// Append appends (major, minor, value) to DeviceRates slice.
func (r *DeviceRates) Append(maj, min, val int64) {
	*r = append(*r, DeviceRate{Major: maj, Minor: min, Rate: val})
}

// Update updates device weight in DeviceWeights slice, or appends it if not found.
func (w *DeviceWeights) Update(maj, min, val int64) {
	for index, devWeight := range *w {
		if devWeight.Major == maj && devWeight.Minor == min {
			(*w)[index].Weight = val
			return
		}
	}
	w.Append(maj, min, val)
}

// Update updates device rate in DeviceRates slice, or appends it if not found.
func (r *DeviceRates) Update(maj, min, val int64) {
	for index, devRate := range *r {
		if devRate.Major == maj && devRate.Minor == min {
			(*r)[index].Rate = val
			return
		}
	}
	r.Append(maj, min, val)
}
