package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/containerd/log"
	"github.com/moby/extensions"
	"github.com/moby/moby/v2/daemon/internal/otelutil/metrictransform"
	metricsinkv0 "github.com/moby/moby/v2/extpoints/metricsink/v0"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	collectormetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

// startMetricSinks returns a meter provider exporting to each allowed metric
// sink, or nil if no sink is allowed.
// A meter provider cannot gain readers after it is built, so this runs after
// the extension host resolves its providers.
func startMetricSinks(ctx context.Context, resolver extensions.Resolver, allowed []string) (*sdkmetric.MeterProvider, func(context.Context), error) {
	providers, err := metricsinkv0.Point.All(resolver)
	if err != nil {
		return nil, nil, err
	}

	// Readers export with their own contexts, including when flushing.
	// Cancel outstanding exports if flushing runs out of time.
	lifetime, cancel := context.WithCancel(context.WithoutCancel(ctx))
	opts := []sdkmetric.Option{sdkmetric.WithResource(resource.Default())}

	var readers []sdkmetric.Reader

	for _, p := range providers {
		if !slices.Contains(allowed, string(p.Identity.ID)) {
			continue
		}

		exporter := metricSinkExporter{sink: p.Impl, id: p.Identity.ID, lifetime: lifetime}
		reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithTimeout(2*time.Second))
		readers = append(readers, reader)
		opts = append(opts, sdkmetric.WithReader(reader))
	}

	if len(readers) == 0 {
		cancel()
		return nil, func(context.Context) {}, nil
	}

	// MeterProvider.Shutdown flushes readers one at a time; flush them
	// concurrently so a slow sink cannot use up the others' time.
	stop := func(ctx context.Context) {
		defer cancel()

		ctx, cancelFlush := context.WithTimeout(ctx, 5*time.Second)
		defer cancelFlush()

		var wg sync.WaitGroup
		for _, r := range readers {
			wg.Go(func() {
				if err := r.Shutdown(ctx); err != nil {
					log.G(ctx).WithError(err).Warn("Failed to flush extension metric sink")
				}
			})
		}

		wg.Wait()
	}

	return sdkmetric.NewMeterProvider(opts...), stop, nil
}

type metricSinkExporter struct {
	sink     metricsinkv0.Sink
	id       extensions.ExtensionID
	lifetime context.Context
}

func (metricSinkExporter) Temporality(kind sdkmetric.InstrumentKind) metricdata.Temporality {
	return sdkmetric.DefaultTemporalitySelector(kind)
}

func (metricSinkExporter) Aggregation(kind sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return sdkmetric.DefaultAggregationSelector(kind)
}

// ForceFlush and Shutdown do not manage the connection; it belongs to the host.
func (metricSinkExporter) ForceFlush(context.Context) error { return nil }
func (metricSinkExporter) Shutdown(context.Context) error   { return nil }

func (e metricSinkExporter) Export(ctx context.Context, rm *metricdata.ResourceMetrics) error {
	if e.lifetime.Err() != nil {
		return nil
	}

	// Like the OTLP exporters, send what converted and report the rest.
	converted, convErr := metrictransform.ResourceMetrics(rm)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stop := context.AfterFunc(e.lifetime, cancel)
	defer stop()

	resp, err := e.sink.ExportMetrics(ctx, &collectormetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{converted},
	})
	if err != nil {
		return errors.Join(fmt.Errorf("metric sink %q: %w", e.id, err), convErr)
	}

	if partial := resp.GetPartialSuccess(); partial.GetRejectedDataPoints() != 0 || partial.GetErrorMessage() != "" {
		log.G(ctx).WithFields(log.Fields{
			"extension":            e.id,
			"rejected-data-points": partial.GetRejectedDataPoints(),
			"message":              partial.GetErrorMessage(),
		}).Warn("Extension metric sink reported partial success")
	}

	return convErr
}
