// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metric // import "go.opentelemetry.io/otel/sdk/metric"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel/internal/global"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// ManualReader is a simple Reader that allows an application to
// read metrics on demand.
type ManualReader struct {
	sdkProducer  atomic.Value
	shutdownOnce sync.Once

	mu                sync.Mutex
	isShutdown        bool
	externalProducers atomic.Value

	temporalitySelector TemporalitySelector
	aggregationSelector AggregationSelector
}

// Compile time check the manualReader implements Reader and is comparable.
var _ = map[Reader]struct{}{&ManualReader{}: {}}

// NewManualReader returns a Reader which is directly called to collect metrics.
func NewManualReader(opts ...ManualReaderOption) *ManualReader {
	cfg := newManualReaderConfig(opts)
	r := &ManualReader{
		temporalitySelector: cfg.temporalitySelector,
		aggregationSelector: cfg.aggregationSelector,
	}
	r.externalProducers.Store(cfg.producers)
	return r
}

// register stores the sdkProducer which enables the caller
// to read metrics from the SDK on demand.
func (mr *ManualReader) register(p sdkProducer) {
	// Only register once. If producer is already set, do nothing.
	if !mr.sdkProducer.CompareAndSwap(nil, produceHolder{produce: p.produce}) {
		msg := "did not register manual reader"
		global.Error(errDuplicateRegister, msg)
	}
}

// temporality reports the Temporality for the instrument kind provided.
func (mr *ManualReader) temporality(kind InstrumentKind) metricdata.Temporality {
	return mr.temporalitySelector(kind)
}

// aggregation returns what Aggregation to use for kind.
func (mr *ManualReader) aggregation(kind InstrumentKind) Aggregation { // nolint:revive  // import-shadow for method scoped by type.
	return mr.aggregationSelector(kind)
}

// Shutdown closes any connections and frees any resources used by the reader.
//
// This method is safe to call concurrently.
func (mr *ManualReader) Shutdown(context.Context) error {
	err := ErrReaderShutdown
	mr.shutdownOnce.Do(func() {
		// Any future call to Collect will now return ErrReaderShutdown.
		mr.sdkProducer.Store(produceHolder{
			produce: shutdownProducer{}.produce,
		})
		mr.mu.Lock()
		defer mr.mu.Unlock()
		mr.isShutdown = true
		// release references to Producer(s)
		mr.externalProducers.Store([]Producer{})
		err = nil
	})
	return err
}

// Collect gathers all metric data related to the Reader from
// the SDK and other Producers and stores the result in rm.
//
// Collect will return an error if called after shutdown.
// Collect will return an error if rm is a nil ResourceMetrics.
// Collect will return an error if the context's Done channel is closed.
//
// This method is safe to call concurrently.
func (mr *ManualReader) Collect(ctx context.Context, rm *metricdata.ResourceMetrics) error {
	if rm == nil {
		return errors.New("manual reader: *metricdata.ResourceMetrics is nil")
	}
	p := mr.sdkProducer.Load()
	if p == nil {
		return ErrReaderNotRegistered
	}

	ph, ok := p.(produceHolder)
	if !ok {
		// The atomic.Value is entirely in the periodicReader's control so
		// this should never happen. In the unforeseen case that this does
		// happen, return an error instead of panicking so a users code does
		// not halt in the processes.
		err := fmt.Errorf("manual reader: invalid producer: %T", p)
		return err
	}

	err := ph.produce(ctx, rm)
	if err != nil {
		return err
	}
	var errs []error
	for _, producer := range mr.externalProducers.Load().([]Producer) {
		externalMetrics, err := producer.Produce(ctx)
		if err != nil {
			errs = append(errs, err)
		}
		rm.ScopeMetrics = append(rm.ScopeMetrics, externalMetrics...)
	}

	global.Debug("ManualReader collection", "Data", rm)

	return unifyErrors(errs)
}

// MarshalLog returns logging data about the ManualReader.
func (r *ManualReader) MarshalLog() interface{} {
	r.mu.Lock()
	down := r.isShutdown
	r.mu.Unlock()
	return struct {
		Type       string
		Registered bool
		Shutdown   bool
	}{
		Type:       "ManualReader",
		Registered: r.sdkProducer.Load() != nil,
		Shutdown:   down,
	}
}

// manualReaderConfig contains configuration options for a ManualReader.
type manualReaderConfig struct {
	temporalitySelector TemporalitySelector
	aggregationSelector AggregationSelector
	producers           []Producer
}

// newManualReaderConfig returns a manualReaderConfig configured with options.
func newManualReaderConfig(opts []ManualReaderOption) manualReaderConfig {
	cfg := manualReaderConfig{
		temporalitySelector: DefaultTemporalitySelector,
		aggregationSelector: DefaultAggregationSelector,
	}
	for _, opt := range opts {
		cfg = opt.applyManual(cfg)
	}
	return cfg
}

// ManualReaderOption applies a configuration option value to a ManualReader.
type ManualReaderOption interface {
	applyManual(manualReaderConfig) manualReaderConfig
}

// WithTemporalitySelector sets the TemporalitySelector a reader will use to
// determine the Temporality of an instrument based on its kind. If this
// option is not used, the reader will use the DefaultTemporalitySelector.
func WithTemporalitySelector(selector TemporalitySelector) ManualReaderOption {
	return temporalitySelectorOption{selector: selector}
}

type temporalitySelectorOption struct {
	selector func(instrument InstrumentKind) metricdata.Temporality
}

// applyManual returns a manualReaderConfig with option applied.
func (t temporalitySelectorOption) applyManual(mrc manualReaderConfig) manualReaderConfig {
	mrc.temporalitySelector = t.selector
	return mrc
}

// WithAggregationSelector sets the AggregationSelector a reader will use to
// determine the aggregation to use for an instrument based on its kind. If
// this option is not used, the reader will use the DefaultAggregationSelector
// or the aggregation explicitly passed for a view matching an instrument.
func WithAggregationSelector(selector AggregationSelector) ManualReaderOption {
	return aggregationSelectorOption{selector: selector}
}

type aggregationSelectorOption struct {
	selector AggregationSelector
}

// applyManual returns a manualReaderConfig with option applied.
func (t aggregationSelectorOption) applyManual(c manualReaderConfig) manualReaderConfig {
	c.aggregationSelector = t.selector
	return c
}
