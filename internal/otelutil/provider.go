package otelutil

import (
	"context"

	"github.com/containerd/log"
	"github.com/moby/buildkit/util/tracing/detect"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func NewTracerProvider(ctx context.Context, allowNoop bool) (trace.TracerProvider, func(context.Context) error) {
	noopShutdown := func(ctx context.Context) error { return nil }

	exp, err := detect.NewSpanExporter(ctx)
	if err != nil {
		log.G(ctx).WithError(err).Warn("Failed to initialize tracing, skipping")
		if allowNoop {
			return noop.NewTracerProvider(), noopShutdown
		}
	}

	if allowNoop && detect.IsNoneSpanExporter(exp) {
		log.G(ctx).Info("OTEL tracing is not configured, using no-op tracer provider")
		return noop.NewTracerProvider(), noopShutdown
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.Default()),
		sdktrace.WithSyncer(detect.Recorder),
		sdktrace.WithBatcher(exp),
	)
	return tp, tp.Shutdown
}
