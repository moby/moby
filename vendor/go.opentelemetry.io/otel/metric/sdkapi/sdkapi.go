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

package sdkapi // import "go.opentelemetry.io/otel/metric/sdkapi"

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/number"
)

// MeterImpl is the interface an SDK must implement to supply a Meter
// implementation.
type MeterImpl interface {
	// RecordBatch atomically records a batch of measurements.
	RecordBatch(ctx context.Context, labels []attribute.KeyValue, measurement ...Measurement)

	// NewSyncInstrument returns a newly constructed
	// synchronous instrument implementation or an error, should
	// one occur.
	NewSyncInstrument(descriptor Descriptor) (SyncImpl, error)

	// NewAsyncInstrument returns a newly constructed
	// asynchronous instrument implementation or an error, should
	// one occur.
	NewAsyncInstrument(
		descriptor Descriptor,
		runner AsyncRunner,
	) (AsyncImpl, error)
}

// InstrumentImpl is a common interface for synchronous and
// asynchronous instruments.
type InstrumentImpl interface {
	// Implementation returns the underlying implementation of the
	// instrument, which allows the implementation to gain access
	// to its own representation especially from a `Measurement`.
	Implementation() interface{}

	// Descriptor returns a copy of the instrument's Descriptor.
	Descriptor() Descriptor
}

// SyncImpl is the implementation-level interface to a generic
// synchronous instrument (e.g., Histogram and Counter instruments).
type SyncImpl interface {
	InstrumentImpl

	// RecordOne captures a single synchronous metric event.
	RecordOne(ctx context.Context, number number.Number, labels []attribute.KeyValue)
}

// AsyncImpl is an implementation-level interface to an
// asynchronous instrument (e.g., Observer instruments).
type AsyncImpl interface {
	InstrumentImpl
}

// AsyncRunner is expected to convert into an AsyncSingleRunner or an
// AsyncBatchRunner.  SDKs will encounter an error if the AsyncRunner
// does not satisfy one of these interfaces.
type AsyncRunner interface {
	// AnyRunner is a non-exported method with no functional use
	// other than to make this a non-empty interface.
	AnyRunner()
}

// AsyncSingleRunner is an interface implemented by single-observer
// callbacks.
type AsyncSingleRunner interface {
	// Run accepts a single instrument and function for capturing
	// observations of that instrument.  Each call to the function
	// receives one captured observation.  (The function accepts
	// multiple observations so the same implementation can be
	// used for batch runners.)
	Run(ctx context.Context, single AsyncImpl, capture func([]attribute.KeyValue, ...Observation))

	AsyncRunner
}

// AsyncBatchRunner is an interface implemented by batch-observer
// callbacks.
type AsyncBatchRunner interface {
	// Run accepts a function for capturing observations of
	// multiple instruments.
	Run(ctx context.Context, capture func([]attribute.KeyValue, ...Observation))

	AsyncRunner
}

// NewMeasurement constructs a single observation, a binding between
// an asynchronous instrument and a number.
func NewMeasurement(instrument SyncImpl, number number.Number) Measurement {
	return Measurement{
		instrument: instrument,
		number:     number,
	}
}

// Measurement is a low-level type used with synchronous instruments
// as a direct interface to the SDK via `RecordBatch`.
type Measurement struct {
	// number needs to be aligned for 64-bit atomic operations.
	number     number.Number
	instrument SyncImpl
}

// SyncImpl returns the instrument that created this measurement.
// This returns an implementation-level object for use by the SDK,
// users should not refer to this.
func (m Measurement) SyncImpl() SyncImpl {
	return m.instrument
}

// Number returns a number recorded in this measurement.
func (m Measurement) Number() number.Number {
	return m.number
}

// NewObservation constructs a single observation, a binding between
// an asynchronous instrument and a number.
func NewObservation(instrument AsyncImpl, number number.Number) Observation {
	return Observation{
		instrument: instrument,
		number:     number,
	}
}

// Observation is a low-level type used with asynchronous instruments
// as a direct interface to the SDK via `BatchObserver`.
type Observation struct {
	// number needs to be aligned for 64-bit atomic operations.
	number     number.Number
	instrument AsyncImpl
}

// AsyncImpl returns the instrument that created this observation.
// This returns an implementation-level object for use by the SDK,
// users should not refer to this.
func (m Observation) AsyncImpl() AsyncImpl {
	return m.instrument
}

// Number returns a number recorded in this observation.
func (m Observation) Number() number.Number {
	return m.number
}
