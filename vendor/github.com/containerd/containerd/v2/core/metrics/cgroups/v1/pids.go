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
	v1 "github.com/containerd/containerd/v2/core/metrics/types/v1"
	metrics "github.com/docker/go-metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var pidMetrics = []*metric{
	{
		name: "pids",
		help: "The limit to the number of pids allowed",
		unit: metrics.Unit("limit"),
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v1.Metrics) []value {
			if stats.Pids == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Pids.Limit),
				},
			}
		},
	},
	{
		name: "pids",
		help: "The current number of pids",
		unit: metrics.Unit("current"),
		vt:   prometheus.GaugeValue,
		getValues: func(stats *v1.Metrics) []value {
			if stats.Pids == nil {
				return nil
			}
			return []value{
				{
					v: float64(stats.Pids.Current),
				},
			}
		},
	},
}
