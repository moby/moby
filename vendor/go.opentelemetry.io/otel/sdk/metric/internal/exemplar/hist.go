// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exemplar // import "go.opentelemetry.io/otel/sdk/metric/internal/exemplar"

import (
	"context"
	"slices"
	"sort"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// Histogram returns a [Reservoir] that samples the last measurement that falls
// within a histogram bucket. The histogram bucket upper-boundaries are define
// by bounds.
//
// The passed bounds will be sorted by this function.
func Histogram(bounds []float64) Reservoir {
	slices.Sort(bounds)
	return &histRes{
		bounds:  bounds,
		storage: newStorage(len(bounds) + 1),
	}
}

type histRes struct {
	*storage

	// bounds are bucket bounds in ascending order.
	bounds []float64
}

func (r *histRes) Offer(ctx context.Context, t time.Time, v Value, a []attribute.KeyValue) {
	var x float64
	switch v.Type() {
	case Int64ValueType:
		x = float64(v.Int64())
	case Float64ValueType:
		x = v.Float64()
	default:
		panic("unknown value type")
	}
	r.store[sort.SearchFloat64s(r.bounds, x)] = newMeasurement(ctx, t, v, a)
}
