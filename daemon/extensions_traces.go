package daemon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/containerd/log"
	"github.com/moby/extensions"
	tracesinkv0 "github.com/moby/moby/v2/extpoints/tracesink/v0"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func startTraceSinks(ctx context.Context, tp *sdktrace.TracerProvider, resolver extensions.Resolver) (func(context.Context), error) {
	providers, err := tracesinkv0.Point.All(resolver)
	if err != nil {
		return nil, err
	}

	// The batch processor uses its own contexts, including when draining.
	// Cancel outstanding exports if draining runs out of time.
	lifetime, cancel := context.WithCancel(context.WithoutCancel(ctx))
	var processors []sdktrace.SpanProcessor
	stop := func(ctx context.Context) {
		defer cancel()

		ctx, cancelDrain := context.WithTimeout(ctx, 5*time.Second)
		defer cancelDrain()

		var wg sync.WaitGroup
		for _, p := range processors {
			wg.Go(func() {
				if err := p.Shutdown(ctx); err != nil {
					log.G(ctx).WithError(err).Warn("Failed to drain extension trace sink")
				}
			})
		}
		wg.Wait()
	}

	for _, p := range providers {
		exporter, err := otlptrace.New(ctx, traceSinkClient{sink: p.Impl, id: p.Identity.ID, lifetime: lifetime})
		if err != nil {
			stop(ctx)
			return nil, fmt.Errorf("initialize trace sink %q: %w", p.Identity.ID, err)
		}

		processor := sdktrace.NewBatchSpanProcessor(exporter, sdktrace.WithExportTimeout(2*time.Second))
		tp.RegisterSpanProcessor(processor)
		processors = append(processors, processor)
	}

	return stop, nil
}

type traceSinkClient struct {
	sink     tracesinkv0.Sink
	id       extensions.ExtensionID
	lifetime context.Context
}

// Start and Stop do not manage the connection; it belongs to the host.
func (traceSinkClient) Start(context.Context) error { return nil }
func (traceSinkClient) Stop(context.Context) error  { return nil }

func (c traceSinkClient) UploadTraces(ctx context.Context, spans []*tracepb.ResourceSpans) error {
	if c.lifetime.Err() != nil {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(c.lifetime, cancel)
	defer stop()

	resp, err := c.sink.ExportTraces(ctx, &collectortracepb.ExportTraceServiceRequest{ResourceSpans: spans})
	if err != nil {
		return fmt.Errorf("trace sink %q: %w", c.id, err)
	}

	if partial := resp.GetPartialSuccess(); partial.GetRejectedSpans() != 0 || partial.GetErrorMessage() != "" {
		log.G(ctx).WithFields(log.Fields{
			"extension":      c.id,
			"rejected-spans": partial.GetRejectedSpans(),
			"message":        partial.GetErrorMessage(),
		}).Warn("Extension trace sink reported partial success")
	}

	return nil
}
