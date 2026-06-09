// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metric // import "go.opentelemetry.io/otel/sdk/metric"

import (
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// batcher splits metrics into batches.
type batcher struct {
	size int
}

// splitResourceMetrics splits a metricdata.ResourceMetrics into multiple ResourceMetrics, sequentially,
// ensuring no ResourceMetrics has more than `size` data points. It does not mutate the `src` object.
func (b batcher) splitResourceMetrics(src *metricdata.ResourceMetrics) []*metricdata.ResourceMetrics {
	if b.size <= 0 || len(src.ScopeMetrics) == 0 {
		return []*metricdata.ResourceMetrics{src}
	}
	var batches []*metricdata.ResourceMetrics
	var currentBatch *metricdata.ResourceMetrics
	currentPoints := 0

	for i := 0; i < len(src.ScopeMetrics); i++ {
		sm := src.ScopeMetrics[i]

		take := b.size - currentPoints
		smChunks := b.splitScopeMetrics(sm, take)

		for _, chunk := range smChunks {
			if currentBatch == nil {
				currentBatch = &metricdata.ResourceMetrics{Resource: src.Resource}
				batches = append(batches, currentBatch)
			}
			currentBatch.ScopeMetrics = append(currentBatch.ScopeMetrics, chunk)
			currentPoints += scopeMetricsDPC(chunk)

			if currentPoints == b.size {
				currentBatch = nil
				currentPoints = 0
			}
		}
	}
	return batches
}

// splitScopeMetrics splits a metricdata.ScopeMetrics into chunks. The first chunk will have at most firstSize data points.
func (b batcher) splitScopeMetrics(sm metricdata.ScopeMetrics, firstSize int) []metricdata.ScopeMetrics {
	smPoints := scopeMetricsDPC(sm)
	if smPoints <= firstSize {
		return []metricdata.ScopeMetrics{sm}
	}

	var chunks []metricdata.ScopeMetrics
	var currentChunk *metricdata.ScopeMetrics
	currentPoints := 0
	targetSize := firstSize

	for i := 0; i < len(sm.Metrics); i++ {
		m := sm.Metrics[i]

		take := targetSize - currentPoints
		mChunks := b.splitMetric(m, take)

		for _, mc := range mChunks {
			if currentChunk == nil {
				chunks = append(chunks, metricdata.ScopeMetrics{Scope: sm.Scope})
				currentChunk = &chunks[len(chunks)-1]
			}
			currentChunk.Metrics = append(currentChunk.Metrics, mc)
			currentPoints += metricDPC(mc)

			if currentPoints == targetSize {
				currentChunk = nil
				currentPoints = 0
				targetSize = b.size
			}
		}
	}
	return chunks
}

// splitMetric splits a metricdata.Metrics into chunks. The first chunk will have at most firstSize data points.
func (b batcher) splitMetric(m metricdata.Metrics, firstSize int) []metricdata.Metrics {
	mPoints := metricDPC(m)
	if mPoints <= firstSize {
		return []metricdata.Metrics{m}
	}

	var chunks []metricdata.Metrics
	mRemaining := mPoints
	mOffset := 0
	take := firstSize

	for mRemaining > 0 {
		if take > mRemaining {
			take = mRemaining
		}
		chunks = append(chunks, copyMetricData(m, mOffset, take))
		mRemaining -= take
		mOffset += take
		take = b.size
	}
	return chunks
}

// copyMetricData creates a copy of the metricdata.Metrics with the specified offset and number of datapoints to take.
func copyMetricData(m metricdata.Metrics, offset, take int) metricdata.Metrics {
	dest := metricdata.Metrics{
		Name:        m.Name,
		Description: m.Description,
		Unit:        m.Unit,
	}
	switch a := m.Data.(type) {
	case metricdata.Gauge[int64]:
		dest.Data = metricdata.Gauge[int64]{DataPoints: a.DataPoints[offset : offset+take]}
	case metricdata.Gauge[float64]:
		dest.Data = metricdata.Gauge[float64]{DataPoints: a.DataPoints[offset : offset+take]}
	case metricdata.Sum[int64]:
		dest.Data = metricdata.Sum[int64]{
			DataPoints:  a.DataPoints[offset : offset+take],
			Temporality: a.Temporality,
			IsMonotonic: a.IsMonotonic,
		}
	case metricdata.Sum[float64]:
		dest.Data = metricdata.Sum[float64]{
			DataPoints:  a.DataPoints[offset : offset+take],
			Temporality: a.Temporality,
			IsMonotonic: a.IsMonotonic,
		}
	case metricdata.Histogram[int64]:
		dest.Data = metricdata.Histogram[int64]{
			DataPoints:  a.DataPoints[offset : offset+take],
			Temporality: a.Temporality,
		}
	case metricdata.Histogram[float64]:
		dest.Data = metricdata.Histogram[float64]{
			DataPoints:  a.DataPoints[offset : offset+take],
			Temporality: a.Temporality,
		}
	case metricdata.ExponentialHistogram[int64]:
		dest.Data = metricdata.ExponentialHistogram[int64]{
			DataPoints:  a.DataPoints[offset : offset+take],
			Temporality: a.Temporality,
		}
	case metricdata.ExponentialHistogram[float64]:
		dest.Data = metricdata.ExponentialHistogram[float64]{
			DataPoints:  a.DataPoints[offset : offset+take],
			Temporality: a.Temporality,
		}
	case metricdata.Summary:
		dest.Data = metricdata.Summary{DataPoints: a.DataPoints[offset : offset+take]}
	}
	return dest
}

// scopeMetricsDPC calculates the total number of data points in the metricdata.ScopeMetrics.
func scopeMetricsDPC(sm metricdata.ScopeMetrics) int {
	dataPointCount := 0
	ms := sm.Metrics
	for k := range ms {
		dataPointCount += metricDPC(ms[k])
	}
	return dataPointCount
}

// metricDPC calculates the total number of data points in the metricdata.Metrics.
func metricDPC(m metricdata.Metrics) int {
	switch a := m.Data.(type) {
	case metricdata.Gauge[int64]:
		return len(a.DataPoints)
	case metricdata.Gauge[float64]:
		return len(a.DataPoints)
	case metricdata.Sum[int64]:
		return len(a.DataPoints)
	case metricdata.Sum[float64]:
		return len(a.DataPoints)
	case metricdata.Histogram[int64]:
		return len(a.DataPoints)
	case metricdata.Histogram[float64]:
		return len(a.DataPoints)
	case metricdata.ExponentialHistogram[int64]:
		return len(a.DataPoints)
	case metricdata.ExponentialHistogram[float64]:
		return len(a.DataPoints)
	case metricdata.Summary:
		return len(a.DataPoints)
	}
	return 0
}
