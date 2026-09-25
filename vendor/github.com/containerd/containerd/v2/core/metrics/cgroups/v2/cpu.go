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

var cpuMetrics = []*metric{
	{
		name: "cpu_usage_usec",
		help: "Total cpu usage (cgroup v2)",
		unit: metrics.Unit("microseconds"),
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.CPU == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.CPU.UsageUsec),
				},
			}
		},
	},
	{
		name: "cpu_user_usec",
		help: "Current cpu usage in user space (cgroup v2)",
		unit: metrics.Unit("microseconds"),
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.CPU == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.CPU.UserUsec),
				},
			}
		},
	},
	{
		name: "cpu_system_usec",
		help: "Current cpu usage in kernel space (cgroup v2)",
		unit: metrics.Unit("microseconds"),
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.CPU == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.CPU.SystemUsec),
				},
			}
		},
	},
	{
		name: "cpu_nr_periods",
		help: "Current cpu number of periods (only if controller is enabled)",
		unit: metrics.Total,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.CPU == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.CPU.NrPeriods),
				},
			}
		},
	},
	{
		name: "cpu_nr_throttled",
		help: "Total number of times tasks have been throttled (only if controller is enabled)",
		unit: metrics.Total,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.CPU == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.CPU.NrThrottled),
				},
			}
		},
	},
	{
		name: "cpu_throttled_usec",
		help: "Total time duration for which tasks have been throttled. (only if controller is enabled)",
		unit: metrics.Unit("microseconds"),
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v2.Metrics) []value {
			if stats.CPU == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.CPU.ThrottledUsec),
				},
			}
		},
	},
}
