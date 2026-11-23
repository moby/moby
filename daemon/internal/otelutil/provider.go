package otelutil

import (
	"context"

	"github.com/containerd/log"
	"github.com/moby/buildkit/util/tracing/detect"
	"go.opentelemetry.io/contrib/processors/baggagecopy"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// NewTracerProvider returns a tracer provider exporting to the OTLP exporter
// configured in the environment.
// If allowNoop is set, it returns nil when no exporter is configured.
func NewTracerProvider(ctx context.Context, allowNoop bool) *sdktrace.TracerProvider {
	exp, err := detect.NewSpanExporter(ctx)
	if err != nil {
		log.G(ctx).WithError(err).Warn("Failed to initialize tracing, skipping")
		if allowNoop {
			return nil
		}
	}

	if allowNoop && detect.IsNoneSpanExporter(exp) {
		return nil
	}

	return NewRecordingTracerProvider(sdktrace.WithBatcher(exp))
}

// NewRecordingTracerProvider returns a tracer provider that records spans for
// BuildKit, plus any span processors in opts.
func NewRecordingTracerProvider(opts ...sdktrace.TracerProviderOption) *sdktrace.TracerProvider {
	return sdktrace.NewTracerProvider(append([]sdktrace.TracerProviderOption{
		sdktrace.WithResource(resource.Default()),
		sdktrace.WithSyncer(detect.Recorder),
		sdktrace.WithSpanProcessor(baggagecopy.NewSpanProcessor(func(member baggage.Member) bool { return true })),
	}, opts...)...)
}
