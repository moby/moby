// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aggregate // import "go.opentelemetry.io/otel/sdk/metric/internal/aggregate"

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// now is used to return the current local time while allowing tests to
// override the default time.Now function.
var now = time.Now

// Measure receives measurements to be aggregated.
type Measure[N int64 | float64] func(context.Context, N, attribute.Set)

// ComputeAggregation stores the aggregate of measurements into dest and
// returns the number of aggregate data-points output.
type ComputeAggregation func(dest *metricdata.Aggregation) int

// Builder builds an aggregate function.
type Builder[N int64 | float64] struct {
	// Temporality is the temporality used for the returned aggregate function.
	//
	// If this is not provided a default of cumulative will be used (except for
	// the last-value aggregate function where delta is the only appropriate
	// temporality).
	Temporality metricdata.Temporality
	// Filter is the attribute filter the aggregate function will use on the
	// input of measurements.
	Filter attribute.Filter
}

func (b Builder[N]) filter(f Measure[N]) Measure[N] {
	if b.Filter != nil {
		fltr := b.Filter // Copy to make it immutable after assignment.
		return func(ctx context.Context, n N, a attribute.Set) {
			fAttr, _ := a.Filter(fltr)
			f(ctx, n, fAttr)
		}
	}
	return f
}

// LastValue returns a last-value aggregate function input and output.
//
// The Builder.Temporality is ignored and delta is use always.
func (b Builder[N]) LastValue() (Measure[N], ComputeAggregation) {
	// Delta temporality is the only temporality that makes semantic sense for
	// a last-value aggregate.
	lv := newLastValue[N]()

	return b.filter(lv.measure), func(dest *metricdata.Aggregation) int {
		// Ignore if dest is not a metricdata.Gauge. The chance for memory
		// reuse of the DataPoints is missed (better luck next time).
		gData, _ := (*dest).(metricdata.Gauge[N])
		lv.computeAggregation(&gData.DataPoints)
		*dest = gData

		return len(gData.DataPoints)
	}
}

// PrecomputedSum returns a sum aggregate function input and output. The
// arguments passed to the input are expected to be the precomputed sum values.
func (b Builder[N]) PrecomputedSum(monotonic bool) (Measure[N], ComputeAggregation) {
	s := newPrecomputedSum[N](monotonic)
	switch b.Temporality {
	case metricdata.DeltaTemporality:
		return b.filter(s.measure), s.delta
	default:
		return b.filter(s.measure), s.cumulative
	}
}

// Sum returns a sum aggregate function input and output.
func (b Builder[N]) Sum(monotonic bool) (Measure[N], ComputeAggregation) {
	s := newSum[N](monotonic)
	switch b.Temporality {
	case metricdata.DeltaTemporality:
		return b.filter(s.measure), s.delta
	default:
		return b.filter(s.measure), s.cumulative
	}
}

// ExplicitBucketHistogram returns a histogram aggregate function input and
// output.
func (b Builder[N]) ExplicitBucketHistogram(boundaries []float64, noMinMax, noSum bool) (Measure[N], ComputeAggregation) {
	h := newHistogram[N](boundaries, noMinMax, noSum)
	switch b.Temporality {
	case metricdata.DeltaTemporality:
		return b.filter(h.measure), h.delta
	default:
		return b.filter(h.measure), h.cumulative
	}
}

// ExponentialBucketHistogram returns a histogram aggregate function input and
// output.
func (b Builder[N]) ExponentialBucketHistogram(maxSize, maxScale int32, noMinMax, noSum bool) (Measure[N], ComputeAggregation) {
	h := newExponentialHistogram[N](maxSize, maxScale, noMinMax, noSum)
	switch b.Temporality {
	case metricdata.DeltaTemporality:
		return b.filter(h.measure), h.delta
	default:
		return b.filter(h.measure), h.cumulative
	}
}

// reset ensures s has capacity and sets it length. If the capacity of s too
// small, a new slice is returned with the specified capacity and length.
func reset[T any](s []T, length, capacity int) []T {
	if cap(s) < capacity {
		return make([]T, length, capacity)
	}
	return s[:length]
}
