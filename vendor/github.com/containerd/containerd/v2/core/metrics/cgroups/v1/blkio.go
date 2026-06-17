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

var blkioMetrics = []*metric{
	{
		name:   "blkio_io_merged_recursive",
		help:   "The blkio io merged recursive",
		unit:   metrics.Total,
		vt:     prometheus.GaugeValue,
		labels: []string{"op", "device", "major", "minor"},
		getValues: func(stats *v1.Metrics) []value {
			if stats.Blkio == nil {
				return nil
			}
			return blkioValues(stats.Blkio.IoMergedRecursive)
		},
	},
	{
		name:   "blkio_io_queued_recursive",
		help:   "The blkio io queued recursive",
		unit:   metrics.Total,
		vt:     prometheus.GaugeValue,
		labels: []string{"op", "device", "major", "minor"},
		getValues: func(stats *v1.Metrics) []value {
			if stats.Blkio == nil {
				return nil
			}
			return blkioValues(stats.Blkio.IoQueuedRecursive)
		},
	},
	{
		name:   "blkio_io_service_bytes_recursive",
		help:   "The blkio io service bytes recursive",
		unit:   metrics.Bytes,
		vt:     prometheus.GaugeValue,
		labels: []string{"op", "device", "major", "minor"},
		getValues: func(stats *v1.Metrics) []value {
			if stats.Blkio == nil {
				return nil
			}
			return blkioValues(stats.Blkio.IoServiceBytesRecursive)
		},
	},
	{
		name:   "blkio_io_service_time_recursive",
		help:   "The blkio io service time recursive",
		unit:   metrics.Total,
		vt:     prometheus.GaugeValue,
		labels: []string{"op", "device", "major", "minor"},
		getValues: func(stats *v1.Metrics) []value {
			if stats.Blkio == nil {
				return nil
			}
			return blkioValues(stats.Blkio.IoServiceTimeRecursive)
		},
	},
	{
		name:   "blkio_io_serviced_recursive",
		help:   "The blkio io serviced recursive",
		unit:   metrics.Total,
		vt:     prometheus.GaugeValue,
		labels: []string{"op", "device", "major", "minor"},
		getValues: func(stats *v1.Metrics) []value {
			if stats.Blkio == nil {
				return nil
			}
			return blkioValues(stats.Blkio.IoServicedRecursive)
		},
	},
	{
		name:   "blkio_io_time_recursive",
		help:   "The blkio io time recursive",
		unit:   metrics.Total,
		vt:     prometheus.GaugeValue,
		labels: []string{"op", "device", "major", "minor"},
		getValues: func(stats *v1.Metrics) []value {
			if stats.Blkio == nil {
				return nil
			}
			return blkioValues(stats.Blkio.IoTimeRecursive)
		},
	},
	{
		name:   "blkio_sectors_recursive",
		help:   "The blkio sectors recursive",
		unit:   metrics.Total,
		vt:     prometheus.GaugeValue,
		labels: []string{"op", "device", "major", "minor"},
		getValues: func(stats *v1.Metrics) []value {
			if stats.Blkio == nil {
				return nil
			}
			return blkioValues(stats.Blkio.SectorsRecursive)
		},
	},
}

func blkioValues(l []*v1.BlkIOEntry) []value {
	var out []value
	for _, e := range l {
		out = append(out, value{
			v: float64(e.Value),
			l: []string{e.Op, e.Device, strconv.FormatUint(e.Major, 10), strconv.FormatUint(e.Minor, 10)},
		})
	}
	return out
}
