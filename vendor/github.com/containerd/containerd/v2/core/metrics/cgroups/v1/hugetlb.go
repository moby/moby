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

var hugetlbMetrics = []*metric{
	{
		name:   "hugetlb_usage",
		help:   "The hugetlb usage",
		unit:   metrics.Bytes,
		vt:     prometheus.GaugeValue,
		labels: []string{"page"},
		getValues: func(stats *v1.Metrics) []value {
			if stats.Hugetlb == nil {
				return nil
			}
			var out []value
			for _, v := range stats.Hugetlb {
				out = append(out, value{
					v: float64(v.Usage),
					l: []string{v.Pagesize},
				})
			}
			return out
		},
	},
	{
		name:   "hugetlb_failcnt",
		help:   "The hugetlb failcnt",
		unit:   metrics.Total,
		vt:     prometheus.GaugeValue,
		labels: []string{"page"},
		getValues: func(stats *v1.Metrics) []value {
			if stats.Hugetlb == nil {
				return nil
			}
			var out []value
			for _, v := range stats.Hugetlb {
				out = append(out, value{
					v: float64(v.Failcnt),
					l: []string{v.Pagesize},
				})
			}
			return out
		},
	},
	{
		name:   "hugetlb_max",
		help:   "The hugetlb maximum usage",
		unit:   metrics.Bytes,
		vt:     prometheus.GaugeValue,
		labels: []string{"page"},
		getValues: func(stats *v1.Metrics) []value {
			if stats.Hugetlb == nil {
				return nil
			}
			var out []value
			for _, v := range stats.Hugetlb {
				out = append(out, value{
					v: float64(v.Max),
					l: []string{v.Pagesize},
				})
			}
			return out
		},
	},
}
