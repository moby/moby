// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oc2otel // import "go.opentelemetry.io/otel/bridge/opencensus/internal/oc2otel"

import (
	"slices"

	octrace "go.opencensus.io/trace"

	"go.opentelemetry.io/otel/trace"
)

func SpanContext(sc octrace.SpanContext) trace.SpanContext {
	var traceFlags trace.TraceFlags
	if sc.IsSampled() {
		traceFlags = trace.FlagsSampled
	}

	entries := slices.Clone(sc.Tracestate.Entries())
	slices.Reverse(entries)

	tsOtel := trace.TraceState{}
	for _, entry := range entries {
		tsOtel, _ = tsOtel.Insert(entry.Key, entry.Value)
	}

	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID(sc.TraceID),
		SpanID:     trace.SpanID(sc.SpanID),
		TraceFlags: traceFlags,
		TraceState: tsOtel,
	})
}
