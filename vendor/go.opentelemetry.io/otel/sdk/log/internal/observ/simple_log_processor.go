// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package observ // import "go.opentelemetry.io/otel/sdk/log/internal/observ"

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk"
	"go.opentelemetry.io/otel/sdk/log/internal/x"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/semconv/v1.39.0/otelconv"
)

const (
	// ScopeName is the name of the instrumentation scope.
	ScopeName = "go.opentelemetry.io/otel/sdk/log/internal/observ"
)

var measureAttrsPool = sync.Pool{
	New: func() any {
		// "component.name" + "component.type" + "error.type"
		const n = 1 + 1 + 1
		s := make([]attribute.KeyValue, 0, n)
		// Return a pointer to a slice instead of a slice itself
		// to avoid allocations on every call.
		return &s
	},
}

// simpleProcessorN is a global 0-based count of the number of simple processor created.
var simpleProcessorN atomic.Int64

// NextSimpleProcessorID returns the next unique ID for a simpleProcessor.
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
	attrs     []attribute.KeyValue
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
		attrs:     attrs,
		addOpts:   addOpts,
	}, nil
}

// LogProcessed records that a log has been processed by the SimpleLogProcessor.
// If err is non-nil, it records the processing error as an attribute.
func (slp *SLP) LogProcessed(ctx context.Context, err error) {
	slp.processed.Add(ctx, 1, slp.addOption(err)...)
}

func (slp *SLP) addOption(err error) []metric.AddOption {
	if err == nil {
		return slp.addOpts
	}
	attrs := measureAttrsPool.Get().(*[]attribute.KeyValue)
	defer func() {
		*attrs = (*attrs)[:0] // reset the slice
		measureAttrsPool.Put(attrs)
	}()

	*attrs = append(*attrs, slp.attrs...)
	*attrs = append(*attrs, semconv.ErrorType(err))

	// Do not inefficiently make a copy of attrs by using
	// WithAttributes instead of WithAttributeSet.
	return []metric.AddOption{metric.WithAttributeSet(attribute.NewSet(*attrs...))}
}
