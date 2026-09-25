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

package v2

import (
	v2 "github.com/containerd/cgroups/v3/cgroup2/stats"
)

type (
	// Metrics alias
	Metrics = v2.Metrics
	// MemoryStat alias
	MemoryStat = v2.MemoryStat
	// CPUStat alias
	CPUStat = v2.CPUStat
	// PidsStat alias
	PidsStat = v2.PidsStat
	// IOStat alias
	IOStat = v2.IOStat
)
