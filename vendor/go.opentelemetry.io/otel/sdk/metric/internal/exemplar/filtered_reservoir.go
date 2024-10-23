// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exemplar // import "go.opentelemetry.io/otel/sdk/metric/internal/exemplar"

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// FilteredReservoir wraps a [Reservoir] with a filter.
type FilteredReservoir[N int64 | float64] interface {
	// Offer accepts the parameters associated with a measurement. The
	// parameters will be stored as an exemplar if the filter decides to
	// sample the measurement.
	//
	// The passed ctx needs to contain any baggage or span that were active
	// when the measurement was made. This information may be used by the
	// Reservoir in making a sampling decision.
	Offer(ctx context.Context, val N, attr []attribute.KeyValue)
	// Collect returns all the held exemplars in the reservoir.
	Collect(dest *[]Exemplar)
}

// filteredReservoir handles the pre-sampled exemplar of measurements made.
type filteredReservoir[N int64 | float64] struct {
	filter    Filter
	reservoir Reservoir
}

// NewFilteredReservoir creates a [FilteredReservoir] which only offers values
// that are allowed by the filter.
func NewFilteredReservoir[N int64 | float64](f Filter, r Reservoir) FilteredReservoir[N] {
	return &filteredReservoir[N]{
		filter:    f,
		reservoir: r,
	}
}

func (f *filteredReservoir[N]) Offer(ctx context.Context, val N, attr []attribute.KeyValue) {
	if f.filter(ctx) {
		// only record the current time if we are sampling this measurment.
		f.reservoir.Offer(ctx, time.Now(), NewValue(val), attr)
	}
}

func (f *filteredReservoir[N]) Collect(dest *[]Exemplar) { f.reservoir.Collect(dest) }
