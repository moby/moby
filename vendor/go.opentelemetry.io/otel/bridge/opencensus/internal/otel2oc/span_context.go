// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package otel2oc provides conversions from OpenTelemetry to OpenCensus.
package otel2oc // import "go.opentelemetry.io/otel/bridge/opencensus/internal/otel2oc"

import (
	octrace "go.opencensus.io/trace"
	"go.opencensus.io/trace/tracestate"

	"go.opentelemetry.io/otel/trace"
)

func SpanContext(sc trace.SpanContext) octrace.SpanContext {
	var to octrace.TraceOptions
	if sc.IsSampled() {
		// OpenCensus doesn't expose functions to directly set sampled
		to = 0x1
	}

	entries := make([]tracestate.Entry, 0, sc.TraceState().Len())
	sc.TraceState().Walk(func(key, value string) bool {
		entries = append(entries, tracestate.Entry{Key: key, Value: value})
		return true
	})
	tsOc, _ := tracestate.New(nil, entries...)

	return octrace.SpanContext{
		TraceID:      octrace.TraceID(sc.TraceID()),
		SpanID:       octrace.SpanID(sc.SpanID()),
		TraceOptions: to,
		Tracestate:   tsOc,
	}
}
