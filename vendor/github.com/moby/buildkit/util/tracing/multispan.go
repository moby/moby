package tracing

import (
	opentracing "github.com/opentracing/opentracing-go"
)

// MultiSpan allows shared tracing to multiple spans.
// TODO: This is a temporary solution and doesn't really support shared tracing yet. Instead the first always wins.

type MultiSpan struct {
	opentracing.Span
}

func NewMultiSpan() *MultiSpan {
	return &MultiSpan{}
}

func (ms *MultiSpan) Add(s opentracing.Span) {
	if ms.Span == nil {
		ms.Span = s
	}
}
