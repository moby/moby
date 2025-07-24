package otelutil

import (
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RecordStatus records the status of a span based on the error provided.
//
// If err is nil, the span status is unmodified. If err is not nil, the span
// takes status Error, and the error message is recorded.
func RecordStatus(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}
