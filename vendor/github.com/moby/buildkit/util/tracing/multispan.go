package tracing

import (
	"go.opentelemetry.io/otel/trace"
)

// MultiSpan allows shared tracing to multiple spans.
// TODO: This is a temporary solution and doesn't really support shared tracing yet. Instead the first always wins.

type MultiSpan struct {
	trace.Span
}

func NewMultiSpan() *MultiSpan {
	return &MultiSpan{}
}

func (ms *MultiSpan) Add(s trace.Span) {
	if ms.Span == nil {
		ms.Span = s
	}
}
