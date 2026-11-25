package otlptrace

import (
	"context"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

type (
	Client interface {
		UploadTraces(ctx context.Context, protoSpans []*tracev1.ResourceSpans) error
	}
	Exporter = otlptrace.Exporter
)

// New creates an otlptrace.Exporter with the more minimal interface
// defined in this package.
func New(ctx context.Context, client Client) (*Exporter, error) {
	return otlptrace.New(ctx, &clientWrapper{client})
}

type clientWrapper struct {
	Client
}

// Start is tied to the lifecycle of the Exporter which isn't relevant
// for our purposes.
func (*clientWrapper) Start(context.Context) error {
	return nil
}

// Stop is tied to the lifecycle of the Exporter which isn't relevant
// for our purposes.
func (*clientWrapper) Stop(context.Context) error {
	return nil
}
