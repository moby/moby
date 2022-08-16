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

package metric // import "go.opentelemetry.io/otel/metric"

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/number"
	"go.opentelemetry.io/otel/metric/sdkapi"
)

// ErrSDKReturnedNilImpl is returned when a new `MeterImpl` returns nil.
var ErrSDKReturnedNilImpl = errors.New("SDK returned a nil implementation")

// Int64ObserverFunc is a type of callback that integral
// observers run.
type Int64ObserverFunc func(context.Context, Int64ObserverResult)

// Float64ObserverFunc is a type of callback that floating point
// observers run.
type Float64ObserverFunc func(context.Context, Float64ObserverResult)

// BatchObserverFunc is a callback argument for use with any
// Observer instrument that will be reported as a batch of
// observations.
type BatchObserverFunc func(context.Context, BatchObserverResult)

// Int64ObserverResult is passed to an observer callback to capture
// observations for one asynchronous integer metric instrument.
type Int64ObserverResult struct {
	instrument sdkapi.AsyncImpl
	function   func([]attribute.KeyValue, ...Observation)
}

// Float64ObserverResult is passed to an observer callback to capture
// observations for one asynchronous floating point metric instrument.
type Float64ObserverResult struct {
	instrument sdkapi.AsyncImpl
	function   func([]attribute.KeyValue, ...Observation)
}

// BatchObserverResult is passed to a batch observer callback to
// capture observations for multiple asynchronous instruments.
type BatchObserverResult struct {
	function func([]attribute.KeyValue, ...Observation)
}

// Observe captures a single integer value from the associated
// instrument callback, with the given labels.
func (ir Int64ObserverResult) Observe(value int64, labels ...attribute.KeyValue) {
	ir.function(labels, sdkapi.NewObservation(ir.instrument, number.NewInt64Number(value)))
}

// Observe captures a single floating point value from the associated
// instrument callback, with the given labels.
func (fr Float64ObserverResult) Observe(value float64, labels ...attribute.KeyValue) {
	fr.function(labels, sdkapi.NewObservation(fr.instrument, number.NewFloat64Number(value)))
}

// Observe captures a multiple observations from the associated batch
// instrument callback, with the given labels.
func (br BatchObserverResult) Observe(labels []attribute.KeyValue, obs ...Observation) {
	br.function(labels, obs...)
}

var _ sdkapi.AsyncSingleRunner = (*Int64ObserverFunc)(nil)
var _ sdkapi.AsyncSingleRunner = (*Float64ObserverFunc)(nil)
var _ sdkapi.AsyncBatchRunner = (*BatchObserverFunc)(nil)

// newInt64AsyncRunner returns a single-observer callback for integer Observer instruments.
func newInt64AsyncRunner(c Int64ObserverFunc) sdkapi.AsyncSingleRunner {
	return &c
}

// newFloat64AsyncRunner returns a single-observer callback for floating point Observer instruments.
func newFloat64AsyncRunner(c Float64ObserverFunc) sdkapi.AsyncSingleRunner {
	return &c
}

// newBatchAsyncRunner returns a batch-observer callback use with multiple Observer instruments.
func newBatchAsyncRunner(c BatchObserverFunc) sdkapi.AsyncBatchRunner {
	return &c
}

// AnyRunner implements AsyncRunner.
func (*Int64ObserverFunc) AnyRunner() {}

// AnyRunner implements AsyncRunner.
func (*Float64ObserverFunc) AnyRunner() {}

// AnyRunner implements AsyncRunner.
func (*BatchObserverFunc) AnyRunner() {}

// Run implements AsyncSingleRunner.
func (i *Int64ObserverFunc) Run(ctx context.Context, impl sdkapi.AsyncImpl, function func([]attribute.KeyValue, ...Observation)) {
	(*i)(ctx, Int64ObserverResult{
		instrument: impl,
		function:   function,
	})
}

// Run implements AsyncSingleRunner.
func (f *Float64ObserverFunc) Run(ctx context.Context, impl sdkapi.AsyncImpl, function func([]attribute.KeyValue, ...Observation)) {
	(*f)(ctx, Float64ObserverResult{
		instrument: impl,
		function:   function,
	})
}

// Run implements AsyncBatchRunner.
func (b *BatchObserverFunc) Run(ctx context.Context, function func([]attribute.KeyValue, ...Observation)) {
	(*b)(ctx, BatchObserverResult{
		function: function,
	})
}

// wrapInt64GaugeObserverInstrument converts an AsyncImpl into Int64GaugeObserver.
func wrapInt64GaugeObserverInstrument(asyncInst sdkapi.AsyncImpl, err error) (Int64GaugeObserver, error) {
	common, err := checkNewAsync(asyncInst, err)
	return Int64GaugeObserver{asyncInstrument: common}, err
}

// wrapFloat64GaugeObserverInstrument converts an AsyncImpl into Float64GaugeObserver.
func wrapFloat64GaugeObserverInstrument(asyncInst sdkapi.AsyncImpl, err error) (Float64GaugeObserver, error) {
	common, err := checkNewAsync(asyncInst, err)
	return Float64GaugeObserver{asyncInstrument: common}, err
}

// wrapInt64CounterObserverInstrument converts an AsyncImpl into Int64CounterObserver.
func wrapInt64CounterObserverInstrument(asyncInst sdkapi.AsyncImpl, err error) (Int64CounterObserver, error) {
	common, err := checkNewAsync(asyncInst, err)
	return Int64CounterObserver{asyncInstrument: common}, err
}

// wrapFloat64CounterObserverInstrument converts an AsyncImpl into Float64CounterObserver.
func wrapFloat64CounterObserverInstrument(asyncInst sdkapi.AsyncImpl, err error) (Float64CounterObserver, error) {
	common, err := checkNewAsync(asyncInst, err)
	return Float64CounterObserver{asyncInstrument: common}, err
}

// wrapInt64UpDownCounterObserverInstrument converts an AsyncImpl into Int64UpDownCounterObserver.
func wrapInt64UpDownCounterObserverInstrument(asyncInst sdkapi.AsyncImpl, err error) (Int64UpDownCounterObserver, error) {
	common, err := checkNewAsync(asyncInst, err)
	return Int64UpDownCounterObserver{asyncInstrument: common}, err
}

// wrapFloat64UpDownCounterObserverInstrument converts an AsyncImpl into Float64UpDownCounterObserver.
func wrapFloat64UpDownCounterObserverInstrument(asyncInst sdkapi.AsyncImpl, err error) (Float64UpDownCounterObserver, error) {
	common, err := checkNewAsync(asyncInst, err)
	return Float64UpDownCounterObserver{asyncInstrument: common}, err
}

// BatchObserver represents an Observer callback that can report
// observations for multiple instruments.
type BatchObserver struct {
	meter  Meter
	runner sdkapi.AsyncBatchRunner
}

// Int64GaugeObserver is a metric that captures a set of int64 values at a
// point in time.
type Int64GaugeObserver struct {
	asyncInstrument
}

// Float64GaugeObserver is a metric that captures a set of float64 values
// at a point in time.
type Float64GaugeObserver struct {
	asyncInstrument
}

// Int64CounterObserver is a metric that captures a precomputed sum of
// int64 values at a point in time.
type Int64CounterObserver struct {
	asyncInstrument
}

// Float64CounterObserver is a metric that captures a precomputed sum of
// float64 values at a point in time.
type Float64CounterObserver struct {
	asyncInstrument
}

// Int64UpDownCounterObserver is a metric that captures a precomputed sum of
// int64 values at a point in time.
type Int64UpDownCounterObserver struct {
	asyncInstrument
}

// Float64UpDownCounterObserver is a metric that captures a precomputed sum of
// float64 values at a point in time.
type Float64UpDownCounterObserver struct {
	asyncInstrument
}

// Observation returns an Observation, a BatchObserverFunc
// argument, for an asynchronous integer instrument.
// This returns an implementation-level object for use by the SDK,
// users should not refer to this.
func (i Int64GaugeObserver) Observation(v int64) Observation {
	return sdkapi.NewObservation(i.instrument, number.NewInt64Number(v))
}

// Observation returns an Observation, a BatchObserverFunc
// argument, for an asynchronous integer instrument.
// This returns an implementation-level object for use by the SDK,
// users should not refer to this.
func (f Float64GaugeObserver) Observation(v float64) Observation {
	return sdkapi.NewObservation(f.instrument, number.NewFloat64Number(v))
}

// Observation returns an Observation, a BatchObserverFunc
// argument, for an asynchronous integer instrument.
// This returns an implementation-level object for use by the SDK,
// users should not refer to this.
func (i Int64CounterObserver) Observation(v int64) Observation {
	return sdkapi.NewObservation(i.instrument, number.NewInt64Number(v))
}

// Observation returns an Observation, a BatchObserverFunc
// argument, for an asynchronous integer instrument.
// This returns an implementation-level object for use by the SDK,
// users should not refer to this.
func (f Float64CounterObserver) Observation(v float64) Observation {
	return sdkapi.NewObservation(f.instrument, number.NewFloat64Number(v))
}

// Observation returns an Observation, a BatchObserverFunc
// argument, for an asynchronous integer instrument.
// This returns an implementation-level object for use by the SDK,
// users should not refer to this.
func (i Int64UpDownCounterObserver) Observation(v int64) Observation {
	return sdkapi.NewObservation(i.instrument, number.NewInt64Number(v))
}

// Observation returns an Observation, a BatchObserverFunc
// argument, for an asynchronous integer instrument.
// This returns an implementation-level object for use by the SDK,
// users should not refer to this.
func (f Float64UpDownCounterObserver) Observation(v float64) Observation {
	return sdkapi.NewObservation(f.instrument, number.NewFloat64Number(v))
}

// syncInstrument contains a SyncImpl.
type syncInstrument struct {
	instrument sdkapi.SyncImpl
}

// asyncInstrument contains a AsyncImpl.
type asyncInstrument struct {
	instrument sdkapi.AsyncImpl
}

// AsyncImpl implements AsyncImpl.
func (a asyncInstrument) AsyncImpl() sdkapi.AsyncImpl {
	return a.instrument
}

// SyncImpl returns the implementation object for synchronous instruments.
func (s syncInstrument) SyncImpl() sdkapi.SyncImpl {
	return s.instrument
}

func (s syncInstrument) float64Measurement(value float64) Measurement {
	return sdkapi.NewMeasurement(s.instrument, number.NewFloat64Number(value))
}

func (s syncInstrument) int64Measurement(value int64) Measurement {
	return sdkapi.NewMeasurement(s.instrument, number.NewInt64Number(value))
}

func (s syncInstrument) directRecord(ctx context.Context, number number.Number, labels []attribute.KeyValue) {
	s.instrument.RecordOne(ctx, number, labels)
}

// checkNewAsync receives an AsyncImpl and potential
// error, and returns the same types, checking for and ensuring that
// the returned interface is not nil.
func checkNewAsync(instrument sdkapi.AsyncImpl, err error) (asyncInstrument, error) {
	if instrument == nil {
		if err == nil {
			err = ErrSDKReturnedNilImpl
		}
		instrument = sdkapi.NewNoopAsyncInstrument()
	}
	return asyncInstrument{
		instrument: instrument,
	}, err
}

// checkNewSync receives an SyncImpl and potential
// error, and returns the same types, checking for and ensuring that
// the returned interface is not nil.
func checkNewSync(instrument sdkapi.SyncImpl, err error) (syncInstrument, error) {
	if instrument == nil {
		if err == nil {
			err = ErrSDKReturnedNilImpl
		}
		// Note: an alternate behavior would be to synthesize a new name
		// or group all duplicately-named instruments of a certain type
		// together and use a tag for the original name, e.g.,
		//   name = 'invalid.counter.int64'
		//   label = 'original-name=duplicate-counter-name'
		instrument = sdkapi.NewNoopSyncInstrument()
	}
	return syncInstrument{
		instrument: instrument,
	}, err
}

// wrapInt64CounterInstrument converts a SyncImpl into Int64Counter.
func wrapInt64CounterInstrument(syncInst sdkapi.SyncImpl, err error) (Int64Counter, error) {
	common, err := checkNewSync(syncInst, err)
	return Int64Counter{syncInstrument: common}, err
}

// wrapFloat64CounterInstrument converts a SyncImpl into Float64Counter.
func wrapFloat64CounterInstrument(syncInst sdkapi.SyncImpl, err error) (Float64Counter, error) {
	common, err := checkNewSync(syncInst, err)
	return Float64Counter{syncInstrument: common}, err
}

// wrapInt64UpDownCounterInstrument converts a SyncImpl into Int64UpDownCounter.
func wrapInt64UpDownCounterInstrument(syncInst sdkapi.SyncImpl, err error) (Int64UpDownCounter, error) {
	common, err := checkNewSync(syncInst, err)
	return Int64UpDownCounter{syncInstrument: common}, err
}

// wrapFloat64UpDownCounterInstrument converts a SyncImpl into Float64UpDownCounter.
func wrapFloat64UpDownCounterInstrument(syncInst sdkapi.SyncImpl, err error) (Float64UpDownCounter, error) {
	common, err := checkNewSync(syncInst, err)
	return Float64UpDownCounter{syncInstrument: common}, err
}

// wrapInt64HistogramInstrument converts a SyncImpl into Int64Histogram.
func wrapInt64HistogramInstrument(syncInst sdkapi.SyncImpl, err error) (Int64Histogram, error) {
	common, err := checkNewSync(syncInst, err)
	return Int64Histogram{syncInstrument: common}, err
}

// wrapFloat64HistogramInstrument converts a SyncImpl into Float64Histogram.
func wrapFloat64HistogramInstrument(syncInst sdkapi.SyncImpl, err error) (Float64Histogram, error) {
	common, err := checkNewSync(syncInst, err)
	return Float64Histogram{syncInstrument: common}, err
}

// Float64Counter is a metric that accumulates float64 values.
type Float64Counter struct {
	syncInstrument
}

// Int64Counter is a metric that accumulates int64 values.
type Int64Counter struct {
	syncInstrument
}

// Measurement creates a Measurement object to use with batch
// recording.
func (c Float64Counter) Measurement(value float64) Measurement {
	return c.float64Measurement(value)
}

// Measurement creates a Measurement object to use with batch
// recording.
func (c Int64Counter) Measurement(value int64) Measurement {
	return c.int64Measurement(value)
}

// Add adds the value to the counter's sum. The labels should contain
// the keys and values to be associated with this value.
func (c Float64Counter) Add(ctx context.Context, value float64, labels ...attribute.KeyValue) {
	c.directRecord(ctx, number.NewFloat64Number(value), labels)
}

// Add adds the value to the counter's sum. The labels should contain
// the keys and values to be associated with this value.
func (c Int64Counter) Add(ctx context.Context, value int64, labels ...attribute.KeyValue) {
	c.directRecord(ctx, number.NewInt64Number(value), labels)
}

// Float64UpDownCounter is a metric instrument that sums floating
// point values.
type Float64UpDownCounter struct {
	syncInstrument
}

// Int64UpDownCounter is a metric instrument that sums integer values.
type Int64UpDownCounter struct {
	syncInstrument
}

// Measurement creates a Measurement object to use with batch
// recording.
func (c Float64UpDownCounter) Measurement(value float64) Measurement {
	return c.float64Measurement(value)
}

// Measurement creates a Measurement object to use with batch
// recording.
func (c Int64UpDownCounter) Measurement(value int64) Measurement {
	return c.int64Measurement(value)
}

// Add adds the value to the counter's sum. The labels should contain
// the keys and values to be associated with this value.
func (c Float64UpDownCounter) Add(ctx context.Context, value float64, labels ...attribute.KeyValue) {
	c.directRecord(ctx, number.NewFloat64Number(value), labels)
}

// Add adds the value to the counter's sum. The labels should contain
// the keys and values to be associated with this value.
func (c Int64UpDownCounter) Add(ctx context.Context, value int64, labels ...attribute.KeyValue) {
	c.directRecord(ctx, number.NewInt64Number(value), labels)
}

// Float64Histogram is a metric that records float64 values.
type Float64Histogram struct {
	syncInstrument
}

// Int64Histogram is a metric that records int64 values.
type Int64Histogram struct {
	syncInstrument
}

// Measurement creates a Measurement object to use with batch
// recording.
func (c Float64Histogram) Measurement(value float64) Measurement {
	return c.float64Measurement(value)
}

// Measurement creates a Measurement object to use with batch
// recording.
func (c Int64Histogram) Measurement(value int64) Measurement {
	return c.int64Measurement(value)
}

// Record adds a new value to the list of Histogram's records. The
// labels should contain the keys and values to be associated with
// this value.
func (c Float64Histogram) Record(ctx context.Context, value float64, labels ...attribute.KeyValue) {
	c.directRecord(ctx, number.NewFloat64Number(value), labels)
}

// Record adds a new value to the Histogram's distribution. The
// labels should contain the keys and values to be associated with
// this value.
func (c Int64Histogram) Record(ctx context.Context, value int64, labels ...attribute.KeyValue) {
	c.directRecord(ctx, number.NewInt64Number(value), labels)
}
