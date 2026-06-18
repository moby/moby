// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package observ // import "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal/observ"

import metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"

// countDataPoints counts the total number of data points in a ResourceMetrics.
func countDataPoints(rm *metricpb.ResourceMetrics) int64 {
	if rm == nil {
		return 0
	}

	var total int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch data := m.Data.(type) {
			case *metricpb.Metric_Gauge:
				if data.Gauge != nil {
					total += int64(len(data.Gauge.DataPoints))
				}
			case *metricpb.Metric_Sum:
				if data.Sum != nil {
					total += int64(len(data.Sum.DataPoints))
				}
			case *metricpb.Metric_Histogram:
				if data.Histogram != nil {
					total += int64(len(data.Histogram.DataPoints))
				}
			case *metricpb.Metric_ExponentialHistogram:
				if data.ExponentialHistogram != nil {
					total += int64(len(data.ExponentialHistogram.DataPoints))
				}
			case *metricpb.Metric_Summary:
				if data.Summary != nil {
					total += int64(len(data.Summary.DataPoints))
				}
			}
		}
	}
	return total
}
