// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package observ

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk"
	"go.opentelemetry.io/otel/sdk/log/internal/x"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/semconv/v1.43.0/otelconv"
)

const (
	// SchemaURL is the schema URL of the instrumentation.
	SchemaURL = semconv.SchemaURL
)

// ErrQueueFull is the attribute value for the "queue_full" error type.
var ErrQueueFull = otelconv.SDKProcessorLogProcessed{}.AttrErrorType("queue_full")

// BLPComponentName returns the component name attribute for a
// BatchLogProcessor with the given ID.
func BLPComponentName(id int64) attribute.KeyValue {
	t := otelconv.ComponentTypeBatchingLogProcessor
	name := fmt.Sprintf("%s/%d", t, id)
	return semconv.OTelComponentName(name)
}

// BLP is the instrumentation for an OTel SDK BatchLogProcessor.
type BLP struct {
	reg metric.Registration

	processed              metric.Int64Counter
	processedOpts          []metric.AddOption
	processedQueueFullOpts []metric.AddOption
}

// NewBLP creates a new BatchLogProcessor instrumentation.
// Returns nil if observability is not enabled.
func NewBLP(id int64, qLen func() int64, qMax int64) (*BLP, error) {
	if !x.Observability.Enabled() {
		return nil, nil
	}
	if qLen == nil {
		return nil, errors.New("BLP qLen must not be nil")
	}

	meter := otel.GetMeterProvider().Meter(
		ScopeName,
		metric.WithInstrumentationVersion(sdk.Version()),
		metric.WithSchemaURL(SchemaURL),
	)

	qCap, err := otelconv.NewSDKProcessorLogQueueCapacity(meter)
	if err != nil {
		return nil, fmt.Errorf("failed to create BLP queue capacity metric: %w", err)
	}
	qCapInst := qCap.Inst()

	qSize, err := otelconv.NewSDKProcessorLogQueueSize(meter)
	if err != nil {
		return nil, fmt.Errorf("failed to create BLP queue size metric: %w", err)
	}
	qSizeInst := qSize.Inst()

	cmpntT := semconv.OTelComponentTypeBatchingLogProcessor
	cmpnt := BLPComponentName(id)
	set := attribute.NewSet(cmpnt, cmpntT)

	// Register callback for async metrics
	obsOpts := []metric.ObserveOption{metric.WithAttributeSet(set)}
	reg, err := meter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			o.ObserveInt64(qSizeInst, qLen(), obsOpts...)
			o.ObserveInt64(qCapInst, qMax, obsOpts...)
			return nil
		},
		qSizeInst,
		qCapInst,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register BLP queue size/capacity callback: %w", err)
	}

	processed, err := otelconv.NewSDKProcessorLogProcessed(meter)
	if err != nil {
		_ = reg.Unregister()
		return nil, fmt.Errorf("failed to create BLP processed logs metric: %w", err)
	}

	processedOpts := []metric.AddOption{metric.WithAttributeSet(set)}
	setWithError := attribute.NewSet(cmpnt, cmpntT, ErrQueueFull)
	processedQueueFullOpts := []metric.AddOption{metric.WithAttributeSet(setWithError)}

	return &BLP{
		reg:                    reg,
		processed:              processed.Inst(),
		processedOpts:          processedOpts,
		processedQueueFullOpts: processedQueueFullOpts,
	}, nil
}

func (b *BLP) Shutdown() error {
	if b == nil || b.reg == nil {
		return nil
	}
	return b.reg.Unregister()
}

func (b *BLP) Processed(ctx context.Context, n int64) {
	if b.processed.Enabled(ctx) {
		b.processed.Add(ctx, n, b.processedOpts...)
	}
}

func (b *BLP) ProcessedQueueFull(ctx context.Context, n int64) {
	if b.processed.Enabled(ctx) {
		b.processed.Add(ctx, n, b.processedQueueFullOpts...)
	}
}
