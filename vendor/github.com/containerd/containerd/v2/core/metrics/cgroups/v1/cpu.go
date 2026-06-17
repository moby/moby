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
	"strconv"

	v1 "github.com/containerd/containerd/v2/core/metrics/types/v1"
	metrics "github.com/docker/go-metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var cpuMetrics = []*metric{
	{
		name: "cpu_total",
		help: "The total cpu time",
		unit: metrics.Nanoseconds,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v1.Metrics) []value {
			if stats.CPU == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.CPU.Usage.Total),
				},
			}
		},
	},
	{
		name: "cpu_kernel",
		help: "The total kernel cpu time",
		unit: metrics.Nanoseconds,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v1.Metrics) []value {
			if stats.CPU == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.CPU.Usage.Kernel),
				},
			}
		},
	},
	{
		name: "cpu_user",
		help: "The total user cpu time",
		unit: metrics.Nanoseconds,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v1.Metrics) []value {
			if stats.CPU == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.CPU.Usage.User),
				},
			}
		},
	},
	{
		name:   "per_cpu",
		help:   "The total cpu time per cpu",
		unit:   metrics.Nanoseconds,
		vt:     prometheus.GaugeValue,
		labels: []string{"cpu"},
		getValues: func(stats *v1.Metrics) []value {
			if stats.CPU == nil {
				return nil
			}
			var out []value
			for i, v := range stats.CPU.Usage.PerCPU {
				out = append(out, value{
					v: float64(v),
					l: []string{strconv.Itoa(i)},
				})
			}
			return out
		},
	},
	{
		name: "cpu_throttle_periods",
		help: "The total cpu throttle periods",
		unit: metrics.Total,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v1.Metrics) []value {
			if stats.CPU == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.CPU.Throttling.Periods),
				},
			}
		},
	},
	{
		name: "cpu_throttled_periods",
		help: "The total cpu throttled periods",
		unit: metrics.Total,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v1.Metrics) []value {
			if stats.CPU == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.CPU.Throttling.ThrottledPeriods),
				},
			}
		},
	},
	{
		name: "cpu_throttled_time",
		help: "The total cpu throttled time",
		unit: metrics.Nanoseconds,
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v1.Metrics) []value {
			if stats.CPU == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.CPU.Throttling.ThrottledTime),
				},
			}
		},
	},
}
