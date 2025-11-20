// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opencensus // import "go.opentelemetry.io/otel/bridge/opencensus"

import (
	octrace "go.opencensus.io/trace"

	"go.opentelemetry.io/otel/bridge/opencensus/internal"
	"go.opentelemetry.io/otel/bridge/opencensus/internal/oc2otel"
	"go.opentelemetry.io/otel/bridge/opencensus/internal/otel2oc"
	"go.opentelemetry.io/otel/trace"
)

// InstallTraceBridge installs the OpenCensus trace bridge, which overwrites
// the global OpenCensus tracer implementation. Once the bridge is installed,
// spans recorded using OpenCensus are redirected to the OpenTelemetry SDK.
func InstallTraceBridge(opts ...TraceOption) {
	octrace.DefaultTracer = newTraceBridge(opts)
}

func newTraceBridge(opts []TraceOption) octrace.Tracer {
	cfg := newTraceConfig(opts)
	return internal.NewTracer(
		cfg.tp.Tracer(scopeName, trace.WithInstrumentationVersion(Version())),
	)
}

// OTelSpanContextToOC converts from an OpenTelemetry SpanContext to an
// OpenCensus SpanContext, and handles any incompatibilities with the global
// error handler.
func OTelSpanContextToOC(sc trace.SpanContext) octrace.SpanContext {
	return otel2oc.SpanContext(sc)
}

// OCSpanContextToOTel converts from an OpenCensus SpanContext to an
// OpenTelemetry SpanContext.
func OCSpanContextToOTel(sc octrace.SpanContext) trace.SpanContext {
	return oc2otel.SpanContext(sc)
}
