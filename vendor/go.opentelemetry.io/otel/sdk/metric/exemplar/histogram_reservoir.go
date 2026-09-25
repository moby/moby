// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exemplar

import (
	"context"
	"slices"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/internal/reservoir"
)

// HistogramReservoirProvider is a provider of [HistogramReservoir].
func HistogramReservoirProvider(bounds []float64) ReservoirProvider {
	cp := slices.Clone(bounds)
	slices.Sort(cp)
	return func(attribute.Set) Reservoir {
		return NewHistogramReservoir(cp)
	}
}

type bucket struct {
	mu sync.Mutex
	nt nextTracker
	measurement
}

// NewHistogramReservoir returns a [HistogramReservoir] that samples
// measurements that fall within a histogram bucket using Algorithm L. The
// histogram bucket upper-boundaries are defined by bounds.
//
// The passed bounds must be sorted before calling this function.
func NewHistogramReservoir(bounds []float64) *HistogramReservoir {
	buckets := make([]bucket, len(bounds)+1)
	for i := range buckets {
		buckets[i].nt.k = 1
		buckets[i].nt.reset()
	}
	return &HistogramReservoir{
		bounds:  bounds,
		buckets: buckets,
	}
}

var _ Reservoir = &HistogramReservoir{}

// HistogramReservoir is a [Reservoir] that samples
// measurements that fall within a histogram bucket using Algorithm L. The
// histogram bucket upper-boundaries are defined by bounds.
type HistogramReservoir struct {
	reservoir.ConcurrentSafe
	// bounds are bucket bounds in ascending order.
	bounds  []float64
	buckets []bucket
}

// Offer accepts the parameters associated with a measurement. The
// parameters will be stored as an exemplar if the Reservoir decides to
// sample the measurement.
//
// The passed ctx needs to contain any baggage or span that were active
// when the measurement was made. This information may be used by the
// Reservoir in making a sampling decision.
//
// The time t is the time when the measurement was made. The v and a
// parameters are the value and dropped (filtered) attributes of the
// measurement respectively.
func (r *HistogramReservoir) Offer(ctx context.Context, t time.Time, v Value, a []attribute.KeyValue) {
	var n float64
	switch v.Type() {
	case Int64ValueType:
		n = float64(v.Int64())
	case Float64ValueType:
		n = v.Float64()
	default:
		panic("unknown value type")
	}

	b := &r.buckets[sort.SearchFloat64s(r.bounds, n)]

	b.mu.Lock()
	defer b.mu.Unlock()

	sampled, _ := b.nt.shouldSample()
	if sampled {
		b.store(ctx, t, v, a)
	}
}

// Collect returns all the held exemplars.
//
// The stored exemplars are preserved after this call, but the sampling state is reset.
func (r *HistogramReservoir) Collect(dest *[]Exemplar) {
	*dest = reset(*dest, len(r.buckets), len(r.buckets))
	var n int
	for i := range r.buckets {
		b := &r.buckets[i]
		b.mu.Lock()
		if b.exemplar(&(*dest)[n]) {
			n++
		}
		b.nt.reset()
		b.mu.Unlock()
	}
	*dest = (*dest)[:n]
}
