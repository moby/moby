// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exemplar // import "go.opentelemetry.io/otel/sdk/metric/internal/exemplar"

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
)

// Drop returns a [FilteredReservoir] that drops all measurements it is offered.
func Drop[N int64 | float64]() FilteredReservoir[N] { return &dropRes[N]{} }

type dropRes[N int64 | float64] struct{}

// Offer does nothing, all measurements offered will be dropped.
func (r *dropRes[N]) Offer(context.Context, N, []attribute.KeyValue) {}

// Collect resets dest. No exemplars will ever be returned.
func (r *dropRes[N]) Collect(dest *[]Exemplar) {
	*dest = (*dest)[:0]
}
