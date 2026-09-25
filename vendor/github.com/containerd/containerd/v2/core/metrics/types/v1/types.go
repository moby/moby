//go:build linux

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

package v1

import (
	v1 "github.com/containerd/cgroups/v3/cgroup1/stats"
)

type (
	// Metrics alias
	Metrics = v1.Metrics
	// BlkIOEntry alias
	BlkIOEntry = v1.BlkIOEntry
	// MemoryStat alias
	MemoryStat = v1.MemoryStat
	// CPUStat alias
	CPUStat = v1.CPUStat
	// CPUUsage alias
	CPUUsage = v1.CPUUsage
	// BlkIOStat alias
	BlkIOStat = v1.BlkIOStat
	// PidsStat alias
	PidsStat = v1.PidsStat
	// RdmaStat alias
	RdmaStat = v1.RdmaStat
	// RdmaEntry alias
	RdmaEntry = v1.RdmaEntry
	// HugetlbStat alias
	HugetlbStat = v1.HugetlbStat
)
