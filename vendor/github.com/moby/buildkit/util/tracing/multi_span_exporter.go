package tracing

import (
	"context"

	"github.com/hashicorp/go-multierror"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type MultiSpanExporter []sdktrace.SpanExporter

func (m MultiSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) (err error) {
	for _, exp := range m {
		if e := exp.ExportSpans(ctx, spans); e != nil {
			if err != nil {
				err = multierror.Append(err, e)
				continue
			}
			err = e
		}
	}
	return err
}

func (m MultiSpanExporter) Shutdown(ctx context.Context) (err error) {
	for _, exp := range m {
		if e := exp.Shutdown(ctx); e != nil {
			if err != nil {
				err = multierror.Append(err, e)
				continue
			}
			err = e
		}
	}
	return err
}
