// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exemplar

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// measurement is a measurement made by a telemetry system.
type measurement struct {
	// FilteredAttributes are the attributes dropped during the measurement.
	FilteredAttributes []attribute.KeyValue
	// Time is the time when the measurement was made.
	Time time.Time
	// Value is the value of the measurement.
	Value Value
	// SpanContext is the SpanContext active when a measurement was made.
	SpanContext trace.SpanContext

	valid bool
}

func (m *measurement) store(ctx context.Context, ts time.Time, v Value, droppedAttr []attribute.KeyValue) {
	m.FilteredAttributes = droppedAttr
	m.Time = ts
	m.Value = v
	m.SpanContext = trace.SpanContextFromContext(ctx)
	m.valid = true
}

// exemplar returns m as an [Exemplar].
// returns true if it populated the exemplar.
func (m *measurement) exemplar(dest *Exemplar) bool {
	if !m.valid {
		return false
	}

	dest.FilteredAttributes = m.FilteredAttributes
	dest.Time = m.Time
	dest.Value = m.Value

	sc := m.SpanContext
	if sc.HasTraceID() {
		traceID := sc.TraceID()
		dest.TraceID = traceID[:]
	} else {
		dest.TraceID = dest.TraceID[:0]
	}

	if sc.HasSpanID() {
		spanID := sc.SpanID()
		dest.SpanID = spanID[:]
	} else {
		dest.SpanID = dest.SpanID[:0]
	}
	return true
}

func reset[T any](s []T, length, capacity int) []T {
	if cap(s) < capacity {
		return make([]T, length, capacity)
	}
	return s[:length]
}
