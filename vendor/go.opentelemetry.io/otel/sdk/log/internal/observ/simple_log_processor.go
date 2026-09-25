// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package observ

import (
	"context"
	"fmt"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk"
	"go.opentelemetry.io/otel/sdk/log/internal/x"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/semconv/v1.43.0/otelconv"
)

const (
	// ScopeName is the name of the instrumentation scope.
	ScopeName = "go.opentelemetry.io/otel/sdk/log/internal/observ"
)

// simpleProcessorN is a global zero-based count of the number of simple
// processors created.
var simpleProcessorN atomic.Int64

// NextSimpleProcessorID returns the next unique ID for a simple processor.
func NextSimpleProcessorID() int64 {
	const inc = 1
	return simpleProcessorN.Add(inc) - inc
}

// SetSimpleProcessorID sets the exporter ID counter to v and returns the previous
// value.
//
// This function is useful for testing purposes, allowing you to reset the
// counter. It should not be used in production code.
func SetSimpleProcessorID(v int64) int64 {
	return simpleProcessorN.Swap(v)
}

// GetSLPComponentName returns the component name attribute for a
// SimpleLogProcessor with the given ID.
func GetSLPComponentName(id int64) attribute.KeyValue {
	t := otelconv.ComponentTypeSimpleLogProcessor
	name := fmt.Sprintf("%s/%d", t, id)
	return semconv.OTelComponentName(name)
}

// SLP is the instrumentation for an OTel SDK SimpleLogProcessor.
type SLP struct {
	processed metric.Int64Counter
	addOpts   []metric.AddOption
}

// NewSLP returns instrumentation for an OTel SDK SimpleLogProcessor with the
// provided ID.
//
// If the experimental observability is disabled, nil is returned.
func NewSLP(id int64) (*SLP, error) {
	if !x.Observability.Enabled() {
		return nil, nil
	}

	meter := otel.GetMeterProvider()
	mt := meter.Meter(
		ScopeName,
		metric.WithInstrumentationVersion(sdk.Version()),
		metric.WithSchemaURL(semconv.SchemaURL),
	)

	p, err := otelconv.NewSDKProcessorLogProcessed(mt)
	if err != nil {
		err = fmt.Errorf("failed to create a processed log metric: %w", err)
		return nil, err
	}

	name := GetSLPComponentName(id)
	componentType := p.AttrComponentType(otelconv.ComponentTypeSimpleLogProcessor)
	attrs := []attribute.KeyValue{name, componentType}
	addOpts := []metric.AddOption{metric.WithAttributeSet(attribute.NewSet(attrs...))}

	return &SLP{
		processed: p.Inst(),
		addOpts:   addOpts,
	}, nil
}

// LogProcessed records that a log record has been submitted to the exporter by
// the SimpleLogProcessor. Per the semantic conventions, this count is recorded
// at submission time and MUST NOT be affected by the export outcome.
func (slp *SLP) LogProcessed(ctx context.Context) {
	if slp.processed.Enabled(ctx) {
		slp.processed.Add(ctx, 1, slp.addOpts...)
	}
}
