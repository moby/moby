package ot

import (
	"context"

	"github.com/Microsoft/hcsshim/internal/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	TracerName = "containerd-shim-runhcs-v1"
)

var DefaultSampler = sdktrace.AlwaysSample()

// SetSpanStatus sets the span's OpenTelemetry status based on `err`.
//
// If `err` is non-nil the error is recorded on the span and the status is
// set to `codes.Error`.
//
// If `err` is nil the span is left in its default (Unset) status. The
// exporter treats Unset the same as Ok for level selection (Info), so
// downstream log level does not change. Leaving the status Unset instead of
// calling SetStatus(Ok, "") avoids the OpenTelemetry spec's "Ok is terminal"
// rule, under which a subsequent SetStatus(Error, ...) would be silently
// dropped.
func SetSpanStatus(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// StartSpan wraps otel.Start(), but, if the span is sampling,
// adds a log entry to the context that points to the newly created span.
func StartSpan(ctx context.Context, name string, o ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := otel.Tracer(TracerName)
	ctx, s := tracer.Start(ctx, name, o...)
	return update(ctx, s)
}

// StartSpanWithRemoteParent wraps "go.opentelemetry.io/otel/trace".ContextWithRemoteSpanContext
//
// See StartSpan for more information.
func StartSpanWithRemoteParent(ctx context.Context, name string, parent trace.SpanContext, o ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := otel.Tracer(TracerName)
	ctx = trace.ContextWithRemoteSpanContext(ctx, parent)
	ctx, s := tracer.Start(ctx, name, o...)
	return update(ctx, s)
}

func update(ctx context.Context, s trace.Span) (context.Context, trace.Span) {
	if s.IsRecording() {
		ctx = log.UpdateContext(ctx)
	}

	return ctx, s
}

var WithServerSpanKind = trace.WithSpanKind(trace.SpanKindServer)
var WithClientSpanKind = trace.WithSpanKind(trace.SpanKindClient)

func spanKindToString(sk trace.SpanKind) string {
	switch sk {
	case trace.SpanKindClient:
		return "client"
	case trace.SpanKindServer:
		return "server"
	default:
		return ""
	}
}
