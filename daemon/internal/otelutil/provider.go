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

func NewTracerProvider(ctx context.Context, allowNoop bool, opts ...sdktrace.TracerProviderOption) *sdktrace.TracerProvider {
	finalOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(resource.Default()),
		sdktrace.WithSpanProcessor(baggagecopy.NewSpanProcessor(func(member baggage.Member) bool { return true })),
	}

	if exp, err := detect.NewSpanExporter(ctx); err != nil {
		log.G(ctx).WithError(err).Warn("Failed to initialize tracing, skipping")
	} else if !detect.IsNoneSpanExporter(exp) {
		finalOpts = append(finalOpts, sdktrace.WithBatcher(exp))
	} else if allowNoop {
		log.G(ctx).Info("OTEL tracing is not configured, using no-op tracer provider")
	}

	if detect.Recorder != nil {
		finalOpts = append(finalOpts, sdktrace.WithSyncer(detect.Recorder))
	}

	finalOpts = append(finalOpts, opts...)
	return sdktrace.NewTracerProvider(finalOpts...)
}
