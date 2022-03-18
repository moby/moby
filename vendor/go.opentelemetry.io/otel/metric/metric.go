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

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/number"
	"go.opentelemetry.io/otel/metric/sdkapi"
)

// MeterProvider supports named Meter instances.
type MeterProvider interface {
	// Meter creates an implementation of the Meter interface.
	// The instrumentationName must be the name of the library providing
	// instrumentation. This name may be the same as the instrumented code
	// only if that code provides built-in instrumentation. If the
	// instrumentationName is empty, then a implementation defined default
	// name will be used instead.
	Meter(instrumentationName string, opts ...MeterOption) Meter
}

// Meter is the creator of metric instruments.
//
// An uninitialized Meter is a no-op implementation.
type Meter struct {
	impl sdkapi.MeterImpl
}

// WrapMeterImpl constructs a `Meter` implementation from a
// `MeterImpl` implementation.
func WrapMeterImpl(impl sdkapi.MeterImpl) Meter {
	return Meter{
		impl: impl,
	}
}

// Measurement is used for reporting a synchronous batch of metric
// values. Instances of this type should be created by synchronous
// instruments (e.g., Int64Counter.Measurement()).
//
// Note: This is an alias because it is a first-class member of the
// API but is also part of the lower-level sdkapi interface.
type Measurement = sdkapi.Measurement

// Observation is used for reporting an asynchronous  batch of metric
// values. Instances of this type should be created by asynchronous
// instruments (e.g., Int64GaugeObserver.Observation()).
//
// Note: This is an alias because it is a first-class member of the
// API but is also part of the lower-level sdkapi interface.
type Observation = sdkapi.Observation

// RecordBatch atomically records a batch of measurements.
func (m Meter) RecordBatch(ctx context.Context, ls []attribute.KeyValue, ms ...Measurement) {
	if m.impl == nil {
		return
	}
	m.impl.RecordBatch(ctx, ls, ms...)
}

// NewBatchObserver creates a new BatchObserver that supports
// making batches of observations for multiple instruments.
func (m Meter) NewBatchObserver(callback BatchObserverFunc) BatchObserver {
	return BatchObserver{
		meter:  m,
		runner: newBatchAsyncRunner(callback),
	}
}

// NewInt64Counter creates a new integer Counter instrument with the
// given name, customized with options.  May return an error if the
// name is invalid (e.g., empty) or improperly registered (e.g.,
// duplicate registration).
func (m Meter) NewInt64Counter(name string, options ...InstrumentOption) (Int64Counter, error) {
	return wrapInt64CounterInstrument(
		m.newSync(name, sdkapi.CounterInstrumentKind, number.Int64Kind, options))
}

// NewFloat64Counter creates a new floating point Counter with the
// given name, customized with options.  May return an error if the
// name is invalid (e.g., empty) or improperly registered (e.g.,
// duplicate registration).
func (m Meter) NewFloat64Counter(name string, options ...InstrumentOption) (Float64Counter, error) {
	return wrapFloat64CounterInstrument(
		m.newSync(name, sdkapi.CounterInstrumentKind, number.Float64Kind, options))
}

// NewInt64UpDownCounter creates a new integer UpDownCounter instrument with the
// given name, customized with options.  May return an error if the
// name is invalid (e.g., empty) or improperly registered (e.g.,
// duplicate registration).
func (m Meter) NewInt64UpDownCounter(name string, options ...InstrumentOption) (Int64UpDownCounter, error) {
	return wrapInt64UpDownCounterInstrument(
		m.newSync(name, sdkapi.UpDownCounterInstrumentKind, number.Int64Kind, options))
}

// NewFloat64UpDownCounter creates a new floating point UpDownCounter with the
// given name, customized with options.  May return an error if the
// name is invalid (e.g., empty) or improperly registered (e.g.,
// duplicate registration).
func (m Meter) NewFloat64UpDownCounter(name string, options ...InstrumentOption) (Float64UpDownCounter, error) {
	return wrapFloat64UpDownCounterInstrument(
		m.newSync(name, sdkapi.UpDownCounterInstrumentKind, number.Float64Kind, options))
}

// NewInt64Histogram creates a new integer Histogram instrument with the
// given name, customized with options.  May return an error if the
// name is invalid (e.g., empty) or improperly registered (e.g.,
// duplicate registration).
func (m Meter) NewInt64Histogram(name string, opts ...InstrumentOption) (Int64Histogram, error) {
	return wrapInt64HistogramInstrument(
		m.newSync(name, sdkapi.HistogramInstrumentKind, number.Int64Kind, opts))
}

// NewFloat64Histogram creates a new floating point Histogram with the
// given name, customized with options.  May return an error if the
// name is invalid (e.g., empty) or improperly registered (e.g.,
// duplicate registration).
func (m Meter) NewFloat64Histogram(name string, opts ...InstrumentOption) (Float64Histogram, error) {
	return wrapFloat64HistogramInstrument(
		m.newSync(name, sdkapi.HistogramInstrumentKind, number.Float64Kind, opts))
}

// NewInt64GaugeObserver creates a new integer GaugeObserver instrument
// with the given name, running a given callback, and customized with
// options.  May return an error if the name is invalid (e.g., empty)
// or improperly registered (e.g., duplicate registration).
func (m Meter) NewInt64GaugeObserver(name string, callback Int64ObserverFunc, opts ...InstrumentOption) (Int64GaugeObserver, error) {
	if callback == nil {
		return wrapInt64GaugeObserverInstrument(sdkapi.NewNoopAsyncInstrument(), nil)
	}
	return wrapInt64GaugeObserverInstrument(
		m.newAsync(name, sdkapi.GaugeObserverInstrumentKind, number.Int64Kind, opts,
			newInt64AsyncRunner(callback)))
}

// NewFloat64GaugeObserver creates a new floating point GaugeObserver with
// the given name, running a given callback, and customized with
// options.  May return an error if the name is invalid (e.g., empty)
// or improperly registered (e.g., duplicate registration).
func (m Meter) NewFloat64GaugeObserver(name string, callback Float64ObserverFunc, opts ...InstrumentOption) (Float64GaugeObserver, error) {
	if callback == nil {
		return wrapFloat64GaugeObserverInstrument(sdkapi.NewNoopAsyncInstrument(), nil)
	}
	return wrapFloat64GaugeObserverInstrument(
		m.newAsync(name, sdkapi.GaugeObserverInstrumentKind, number.Float64Kind, opts,
			newFloat64AsyncRunner(callback)))
}

// NewInt64CounterObserver creates a new integer CounterObserver instrument
// with the given name, running a given callback, and customized with
// options.  May return an error if the name is invalid (e.g., empty)
// or improperly registered (e.g., duplicate registration).
func (m Meter) NewInt64CounterObserver(name string, callback Int64ObserverFunc, opts ...InstrumentOption) (Int64CounterObserver, error) {
	if callback == nil {
		return wrapInt64CounterObserverInstrument(sdkapi.NewNoopAsyncInstrument(), nil)
	}
	return wrapInt64CounterObserverInstrument(
		m.newAsync(name, sdkapi.CounterObserverInstrumentKind, number.Int64Kind, opts,
			newInt64AsyncRunner(callback)))
}

// NewFloat64CounterObserver creates a new floating point CounterObserver with
// the given name, running a given callback, and customized with
// options.  May return an error if the name is invalid (e.g., empty)
// or improperly registered (e.g., duplicate registration).
func (m Meter) NewFloat64CounterObserver(name string, callback Float64ObserverFunc, opts ...InstrumentOption) (Float64CounterObserver, error) {
	if callback == nil {
		return wrapFloat64CounterObserverInstrument(sdkapi.NewNoopAsyncInstrument(), nil)
	}
	return wrapFloat64CounterObserverInstrument(
		m.newAsync(name, sdkapi.CounterObserverInstrumentKind, number.Float64Kind, opts,
			newFloat64AsyncRunner(callback)))
}

// NewInt64UpDownCounterObserver creates a new integer UpDownCounterObserver instrument
// with the given name, running a given callback, and customized with
// options.  May return an error if the name is invalid (e.g., empty)
// or improperly registered (e.g., duplicate registration).
func (m Meter) NewInt64UpDownCounterObserver(name string, callback Int64ObserverFunc, opts ...InstrumentOption) (Int64UpDownCounterObserver, error) {
	if callback == nil {
		return wrapInt64UpDownCounterObserverInstrument(sdkapi.NewNoopAsyncInstrument(), nil)
	}
	return wrapInt64UpDownCounterObserverInstrument(
		m.newAsync(name, sdkapi.UpDownCounterObserverInstrumentKind, number.Int64Kind, opts,
			newInt64AsyncRunner(callback)))
}

// NewFloat64UpDownCounterObserver creates a new floating point UpDownCounterObserver with
// the given name, running a given callback, and customized with
// options.  May return an error if the name is invalid (e.g., empty)
// or improperly registered (e.g., duplicate registration).
func (m Meter) NewFloat64UpDownCounterObserver(name string, callback Float64ObserverFunc, opts ...InstrumentOption) (Float64UpDownCounterObserver, error) {
	if callback == nil {
		return wrapFloat64UpDownCounterObserverInstrument(sdkapi.NewNoopAsyncInstrument(), nil)
	}
	return wrapFloat64UpDownCounterObserverInstrument(
		m.newAsync(name, sdkapi.UpDownCounterObserverInstrumentKind, number.Float64Kind, opts,
			newFloat64AsyncRunner(callback)))
}

// NewInt64GaugeObserver creates a new integer GaugeObserver instrument
// with the given name, running in a batch callback, and customized with
// options.  May return an error if the name is invalid (e.g., empty)
// or improperly registered (e.g., duplicate registration).
func (b BatchObserver) NewInt64GaugeObserver(name string, opts ...InstrumentOption) (Int64GaugeObserver, error) {
	if b.runner == nil {
		return wrapInt64GaugeObserverInstrument(sdkapi.NewNoopAsyncInstrument(), nil)
	}
	return wrapInt64GaugeObserverInstrument(
		b.meter.newAsync(name, sdkapi.GaugeObserverInstrumentKind, number.Int64Kind, opts, b.runner))
}

// NewFloat64GaugeObserver creates a new floating point GaugeObserver with
// the given name, running in a batch callback, and customized with
// options.  May return an error if the name is invalid (e.g., empty)
// or improperly registered (e.g., duplicate registration).
func (b BatchObserver) NewFloat64GaugeObserver(name string, opts ...InstrumentOption) (Float64GaugeObserver, error) {
	if b.runner == nil {
		return wrapFloat64GaugeObserverInstrument(sdkapi.NewNoopAsyncInstrument(), nil)
	}
	return wrapFloat64GaugeObserverInstrument(
		b.meter.newAsync(name, sdkapi.GaugeObserverInstrumentKind, number.Float64Kind, opts,
			b.runner))
}

// NewInt64CounterObserver creates a new integer CounterObserver instrument
// with the given name, running in a batch callback, and customized with
// options.  May return an error if the name is invalid (e.g., empty)
// or improperly registered (e.g., duplicate registration).
func (b BatchObserver) NewInt64CounterObserver(name string, opts ...InstrumentOption) (Int64CounterObserver, error) {
	if b.runner == nil {
		return wrapInt64CounterObserverInstrument(sdkapi.NewNoopAsyncInstrument(), nil)
	}
	return wrapInt64CounterObserverInstrument(
		b.meter.newAsync(name, sdkapi.CounterObserverInstrumentKind, number.Int64Kind, opts, b.runner))
}

// NewFloat64CounterObserver creates a new floating point CounterObserver with
// the given name, running in a batch callback, and customized with
// options.  May return an error if the name is invalid (e.g., empty)
// or improperly registered (e.g., duplicate registration).
func (b BatchObserver) NewFloat64CounterObserver(name string, opts ...InstrumentOption) (Float64CounterObserver, error) {
	if b.runner == nil {
		return wrapFloat64CounterObserverInstrument(sdkapi.NewNoopAsyncInstrument(), nil)
	}
	return wrapFloat64CounterObserverInstrument(
		b.meter.newAsync(name, sdkapi.CounterObserverInstrumentKind, number.Float64Kind, opts,
			b.runner))
}

// NewInt64UpDownCounterObserver creates a new integer UpDownCounterObserver instrument
// with the given name, running in a batch callback, and customized with
// options.  May return an error if the name is invalid (e.g., empty)
// or improperly registered (e.g., duplicate registration).
func (b BatchObserver) NewInt64UpDownCounterObserver(name string, opts ...InstrumentOption) (Int64UpDownCounterObserver, error) {
	if b.runner == nil {
		return wrapInt64UpDownCounterObserverInstrument(sdkapi.NewNoopAsyncInstrument(), nil)
	}
	return wrapInt64UpDownCounterObserverInstrument(
		b.meter.newAsync(name, sdkapi.UpDownCounterObserverInstrumentKind, number.Int64Kind, opts, b.runner))
}

// NewFloat64UpDownCounterObserver creates a new floating point UpDownCounterObserver with
// the given name, running in a batch callback, and customized with
// options.  May return an error if the name is invalid (e.g., empty)
// or improperly registered (e.g., duplicate registration).
func (b BatchObserver) NewFloat64UpDownCounterObserver(name string, opts ...InstrumentOption) (Float64UpDownCounterObserver, error) {
	if b.runner == nil {
		return wrapFloat64UpDownCounterObserverInstrument(sdkapi.NewNoopAsyncInstrument(), nil)
	}
	return wrapFloat64UpDownCounterObserverInstrument(
		b.meter.newAsync(name, sdkapi.UpDownCounterObserverInstrumentKind, number.Float64Kind, opts,
			b.runner))
}

// MeterImpl returns the underlying MeterImpl of this Meter.
func (m Meter) MeterImpl() sdkapi.MeterImpl {
	return m.impl
}

// newAsync constructs one new asynchronous instrument.
func (m Meter) newAsync(
	name string,
	mkind sdkapi.InstrumentKind,
	nkind number.Kind,
	opts []InstrumentOption,
	runner sdkapi.AsyncRunner,
) (
	sdkapi.AsyncImpl,
	error,
) {
	if m.impl == nil {
		return sdkapi.NewNoopAsyncInstrument(), nil
	}
	cfg := NewInstrumentConfig(opts...)
	desc := sdkapi.NewDescriptor(name, mkind, nkind, cfg.description, cfg.unit)
	return m.impl.NewAsyncInstrument(desc, runner)
}

// newSync constructs one new synchronous instrument.
func (m Meter) newSync(
	name string,
	metricKind sdkapi.InstrumentKind,
	numberKind number.Kind,
	opts []InstrumentOption,
) (
	sdkapi.SyncImpl,
	error,
) {
	if m.impl == nil {
		return sdkapi.NewNoopSyncInstrument(), nil
	}
	cfg := NewInstrumentConfig(opts...)
	desc := sdkapi.NewDescriptor(name, metricKind, numberKind, cfg.description, cfg.unit)
	return m.impl.NewSyncInstrument(desc)
}

// MeterMust is a wrapper for Meter interfaces that panics when any
// instrument constructor encounters an error.
type MeterMust struct {
	meter Meter
}

// BatchObserverMust is a wrapper for BatchObserver that panics when
// any instrument constructor encounters an error.
type BatchObserverMust struct {
	batch BatchObserver
}

// Must constructs a MeterMust implementation from a Meter, allowing
// the application to panic when any instrument constructor yields an
// error.
func Must(meter Meter) MeterMust {
	return MeterMust{meter: meter}
}

// NewInt64Counter calls `Meter.NewInt64Counter` and returns the
// instrument, panicking if it encounters an error.
func (mm MeterMust) NewInt64Counter(name string, cos ...InstrumentOption) Int64Counter {
	if inst, err := mm.meter.NewInt64Counter(name, cos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewFloat64Counter calls `Meter.NewFloat64Counter` and returns the
// instrument, panicking if it encounters an error.
func (mm MeterMust) NewFloat64Counter(name string, cos ...InstrumentOption) Float64Counter {
	if inst, err := mm.meter.NewFloat64Counter(name, cos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewInt64UpDownCounter calls `Meter.NewInt64UpDownCounter` and returns the
// instrument, panicking if it encounters an error.
func (mm MeterMust) NewInt64UpDownCounter(name string, cos ...InstrumentOption) Int64UpDownCounter {
	if inst, err := mm.meter.NewInt64UpDownCounter(name, cos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewFloat64UpDownCounter calls `Meter.NewFloat64UpDownCounter` and returns the
// instrument, panicking if it encounters an error.
func (mm MeterMust) NewFloat64UpDownCounter(name string, cos ...InstrumentOption) Float64UpDownCounter {
	if inst, err := mm.meter.NewFloat64UpDownCounter(name, cos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewInt64Histogram calls `Meter.NewInt64Histogram` and returns the
// instrument, panicking if it encounters an error.
func (mm MeterMust) NewInt64Histogram(name string, mos ...InstrumentOption) Int64Histogram {
	if inst, err := mm.meter.NewInt64Histogram(name, mos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewFloat64Histogram calls `Meter.NewFloat64Histogram` and returns the
// instrument, panicking if it encounters an error.
func (mm MeterMust) NewFloat64Histogram(name string, mos ...InstrumentOption) Float64Histogram {
	if inst, err := mm.meter.NewFloat64Histogram(name, mos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewInt64GaugeObserver calls `Meter.NewInt64GaugeObserver` and
// returns the instrument, panicking if it encounters an error.
func (mm MeterMust) NewInt64GaugeObserver(name string, callback Int64ObserverFunc, oos ...InstrumentOption) Int64GaugeObserver {
	if inst, err := mm.meter.NewInt64GaugeObserver(name, callback, oos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewFloat64GaugeObserver calls `Meter.NewFloat64GaugeObserver` and
// returns the instrument, panicking if it encounters an error.
func (mm MeterMust) NewFloat64GaugeObserver(name string, callback Float64ObserverFunc, oos ...InstrumentOption) Float64GaugeObserver {
	if inst, err := mm.meter.NewFloat64GaugeObserver(name, callback, oos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewInt64CounterObserver calls `Meter.NewInt64CounterObserver` and
// returns the instrument, panicking if it encounters an error.
func (mm MeterMust) NewInt64CounterObserver(name string, callback Int64ObserverFunc, oos ...InstrumentOption) Int64CounterObserver {
	if inst, err := mm.meter.NewInt64CounterObserver(name, callback, oos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewFloat64CounterObserver calls `Meter.NewFloat64CounterObserver` and
// returns the instrument, panicking if it encounters an error.
func (mm MeterMust) NewFloat64CounterObserver(name string, callback Float64ObserverFunc, oos ...InstrumentOption) Float64CounterObserver {
	if inst, err := mm.meter.NewFloat64CounterObserver(name, callback, oos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewInt64UpDownCounterObserver calls `Meter.NewInt64UpDownCounterObserver` and
// returns the instrument, panicking if it encounters an error.
func (mm MeterMust) NewInt64UpDownCounterObserver(name string, callback Int64ObserverFunc, oos ...InstrumentOption) Int64UpDownCounterObserver {
	if inst, err := mm.meter.NewInt64UpDownCounterObserver(name, callback, oos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewFloat64UpDownCounterObserver calls `Meter.NewFloat64UpDownCounterObserver` and
// returns the instrument, panicking if it encounters an error.
func (mm MeterMust) NewFloat64UpDownCounterObserver(name string, callback Float64ObserverFunc, oos ...InstrumentOption) Float64UpDownCounterObserver {
	if inst, err := mm.meter.NewFloat64UpDownCounterObserver(name, callback, oos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewBatchObserver returns a wrapper around BatchObserver that panics
// when any instrument constructor returns an error.
func (mm MeterMust) NewBatchObserver(callback BatchObserverFunc) BatchObserverMust {
	return BatchObserverMust{
		batch: mm.meter.NewBatchObserver(callback),
	}
}

// NewInt64GaugeObserver calls `BatchObserver.NewInt64GaugeObserver` and
// returns the instrument, panicking if it encounters an error.
func (bm BatchObserverMust) NewInt64GaugeObserver(name string, oos ...InstrumentOption) Int64GaugeObserver {
	if inst, err := bm.batch.NewInt64GaugeObserver(name, oos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewFloat64GaugeObserver calls `BatchObserver.NewFloat64GaugeObserver` and
// returns the instrument, panicking if it encounters an error.
func (bm BatchObserverMust) NewFloat64GaugeObserver(name string, oos ...InstrumentOption) Float64GaugeObserver {
	if inst, err := bm.batch.NewFloat64GaugeObserver(name, oos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewInt64CounterObserver calls `BatchObserver.NewInt64CounterObserver` and
// returns the instrument, panicking if it encounters an error.
func (bm BatchObserverMust) NewInt64CounterObserver(name string, oos ...InstrumentOption) Int64CounterObserver {
	if inst, err := bm.batch.NewInt64CounterObserver(name, oos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewFloat64CounterObserver calls `BatchObserver.NewFloat64CounterObserver` and
// returns the instrument, panicking if it encounters an error.
func (bm BatchObserverMust) NewFloat64CounterObserver(name string, oos ...InstrumentOption) Float64CounterObserver {
	if inst, err := bm.batch.NewFloat64CounterObserver(name, oos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewInt64UpDownCounterObserver calls `BatchObserver.NewInt64UpDownCounterObserver` and
// returns the instrument, panicking if it encounters an error.
func (bm BatchObserverMust) NewInt64UpDownCounterObserver(name string, oos ...InstrumentOption) Int64UpDownCounterObserver {
	if inst, err := bm.batch.NewInt64UpDownCounterObserver(name, oos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}

// NewFloat64UpDownCounterObserver calls `BatchObserver.NewFloat64UpDownCounterObserver` and
// returns the instrument, panicking if it encounters an error.
func (bm BatchObserverMust) NewFloat64UpDownCounterObserver(name string, oos ...InstrumentOption) Float64UpDownCounterObserver {
	if inst, err := bm.batch.NewFloat64UpDownCounterObserver(name, oos...); err != nil {
		panic(err)
	} else {
		return inst
	}
}
