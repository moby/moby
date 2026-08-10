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
	v2 "github.com/containerd/containerd/v2/core/metrics/types/v2"
	metrics "github.com/docker/go-metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var memoryMetrics = []*metric{
	{
		name: "memory_usage",
		help: "Current memory usage (cgroup v2)",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Usage),
				},
			}
		},
	},
	{
		name: "memory_usage_limit",
		help: "Current memory usage limit (cgroup v2)",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.UsageLimit),
				},
			}
		},
	},
	{
		name: "memory_swap_usage",
		help: "Current swap usage (cgroup v2)",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.SwapUsage),
				},
			}
		},
	},
	{
		name: "memory_swap_limit",
		help: "Current swap usage limit (cgroup v2)",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.SwapLimit),
				},
			}
		},
	},

	{
		name: "memory_file_mapped",
		help: "The file_mapped amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.FileMapped),
				},
			}
		},
	},
	{
		name: "memory_file_dirty",
		help: "The file_dirty amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.FileDirty),
				},
			}
		},
	},
	{
		name: "memory_file_writeback",
		help: "The file_writeback amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.FileWriteback),
				},
			}
		},
	},
	{
		name: "memory_pgactivate",
		help: "The pgactivate amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Pgactivate),
				},
			}
		},
	},
	{
		name: "memory_pgdeactivate",
		help: "The pgdeactivate amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Pgdeactivate),
				},
			}
		},
	},
	{
		name: "memory_pgfault",
		help: "The pgfault amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Pgfault),
				},
			}
		},
	},
	{
		name: "memory_pgmajfault",
		help: "The pgmajfault amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Pgmajfault),
				},
			}
		},
	},
	{
		name: "memory_pglazyfree",
		help: "The pglazyfree amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Pglazyfree),
				},
			}
		},
	},
	{
		name: "memory_pgrefill",
		help: "The pgrefill amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Pgrefill),
				},
			}
		},
	},
	{
		name: "memory_pglazyfreed",
		help: "The pglazyfreed amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Pglazyfreed),
				},
			}
		},
	},
	{
		name: "memory_pgscan",
		help: "The pgscan amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Pgscan),
				},
			}
		},
	},
	{
		name: "memory_pgsteal",
		help: "The pgsteal amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Pgsteal),
				},
			}
		},
	},
	{
		name: "memory_inactive_anon",
		help: "The inactive_anon amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.InactiveAnon),
				},
			}
		},
	},
	{
		name: "memory_active_anon",
		help: "The active_anon amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.ActiveAnon),
				},
			}
		},
	},
	{
		name: "memory_inactive_file",
		help: "The inactive_file amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.InactiveFile),
				},
			}
		},
	},
	{
		name: "memory_active_file",
		help: "The active_file amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.ActiveFile),
				},
			}
		},
	},
	{
		name: "memory_unevictable",
		help: "The unevictable amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Unevictable),
				},
			}
		},
	},
	{
		name: "memory_anon",
		help: "The anon amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Anon),
				},
			}
		},
	},
	{
		name: "memory_file",
		help: "The file amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.File),
				},
			}
		},
	},
	{
		name: "memory_kernel_stack",
		help: "The kernel_stack amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.KernelStack),
				},
			}
		},
	},
	{
		name: "memory_slab",
		help: "The slab amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Slab),
				},
			}
		},
	},
	{
		name: "memory_sock",
		help: "The sock amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Sock),
				},
			}
		},
	},
	{
		name: "memory_shmem",
		help: "The shmem amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.Shmem),
				},
			}
		},
	},
	{
		name: "memory_anon_thp",
		help: "The anon_thp amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.AnonThp),
				},
			}
		},
	},
	{
		name: "memory_slab_reclaimable",
		help: "The slab_reclaimable amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.SlabReclaimable),
				},
			}
		},
	},
	{
		name: "memory_slab_unreclaimable",
		help: "The slab_unreclaimable amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.SlabUnreclaimable),
				},
			}
		},
	},
	{
		name: "memory_workingset_refault",
		help: "The workingset_refault amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.WorkingsetRefault),
				},
			}
		},
	},
	{
		name: "memory_workingset_activate",
		help: "The workingset_activate amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.WorkingsetActivate),
				},
			}
		},
	},
	{
		name: "memory_workingset_nodereclaim",
		help: "The workingset_nodereclaim amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.WorkingsetNodereclaim),
				},
			}
		},
	},
	{
		name: "memory_thp_fault_alloc",
		help: "The thp_fault_alloc amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.ThpFaultAlloc),
				},
			}
		},
	},
	{
		name: "memory_thp_collapse_alloc",
		help: "The thp_collapse_alloc amount",
		unit: metrics.Bytes,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.Memory == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Memory.ThpCollapseAlloc),
				},
			}
		},
	},
	{
		name: "memory_oom",
		help: "The number of times a container has received an oom event",
		unit: metrics.Total,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.MemoryEvents == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.MemoryEvents.Oom),
				},
			}
		},
	},
}
