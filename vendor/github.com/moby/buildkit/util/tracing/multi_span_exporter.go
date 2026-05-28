package tracing

import (
	"context"
	stderrors "errors"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type MultiSpanExporter []sdktrace.SpanExporter

func (m MultiSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	var errs []error
	for _, exp := range m {
		if e := exp.ExportSpans(ctx, spans); e != nil {
			errs = append(errs, e)
		}
	}
	return stderrors.Join(errs...)
}

func (m MultiSpanExporter) Shutdown(ctx context.Context) error {
	var errs []error
	for _, exp := range m {
		if e := exp.Shutdown(ctx); e != nil {
			errs = append(errs, e)
		}
	}
	return stderrors.Join(errs...)
}
