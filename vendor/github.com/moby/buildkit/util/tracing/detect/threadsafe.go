package detect

import (
	"context"
	"sync"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// threadSafeExporterWrapper wraps an OpenTelemetry SpanExporter and makes it thread-safe.
type threadSafeExporterWrapper struct {
	mu       sync.Mutex
	exporter sdktrace.SpanExporter
}

func (tse *threadSafeExporterWrapper) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	tse.mu.Lock()
	defer tse.mu.Unlock()
	return tse.exporter.ExportSpans(ctx, spans)
}

func (tse *threadSafeExporterWrapper) Shutdown(ctx context.Context) error {
	tse.mu.Lock()
	defer tse.mu.Unlock()
	return tse.exporter.Shutdown(ctx)
}
