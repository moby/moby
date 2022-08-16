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

package registry // import "go.opentelemetry.io/otel/internal/metric/registry"

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/sdkapi"
)

// UniqueInstrumentMeterImpl implements the metric.MeterImpl interface, adding
// uniqueness checking for instrument descriptors.
type UniqueInstrumentMeterImpl struct {
	lock  sync.Mutex
	impl  sdkapi.MeterImpl
	state map[string]sdkapi.InstrumentImpl
}

var _ sdkapi.MeterImpl = (*UniqueInstrumentMeterImpl)(nil)

// ErrMetricKindMismatch is the standard error for mismatched metric
// instrument definitions.
var ErrMetricKindMismatch = fmt.Errorf(
	"a metric was already registered by this name with another kind or number type")

// NewUniqueInstrumentMeterImpl returns a wrapped metric.MeterImpl
// with the addition of instrument name uniqueness checking.
func NewUniqueInstrumentMeterImpl(impl sdkapi.MeterImpl) *UniqueInstrumentMeterImpl {
	return &UniqueInstrumentMeterImpl{
		impl:  impl,
		state: map[string]sdkapi.InstrumentImpl{},
	}
}

// MeterImpl gives the caller access to the underlying MeterImpl
// used by this UniqueInstrumentMeterImpl.
func (u *UniqueInstrumentMeterImpl) MeterImpl() sdkapi.MeterImpl {
	return u.impl
}

// RecordBatch implements sdkapi.MeterImpl.
func (u *UniqueInstrumentMeterImpl) RecordBatch(ctx context.Context, labels []attribute.KeyValue, ms ...sdkapi.Measurement) {
	u.impl.RecordBatch(ctx, labels, ms...)
}

// NewMetricKindMismatchError formats an error that describes a
// mismatched metric instrument definition.
func NewMetricKindMismatchError(desc sdkapi.Descriptor) error {
	return fmt.Errorf("metric %s registered as %s %s: %w",
		desc.Name(),
		desc.NumberKind(),
		desc.InstrumentKind(),
		ErrMetricKindMismatch)
}

// Compatible determines whether two sdkapi.Descriptors are considered
// the same for the purpose of uniqueness checking.
func Compatible(candidate, existing sdkapi.Descriptor) bool {
	return candidate.InstrumentKind() == existing.InstrumentKind() &&
		candidate.NumberKind() == existing.NumberKind()
}

// checkUniqueness returns an ErrMetricKindMismatch error if there is
// a conflict between a descriptor that was already registered and the
// `descriptor` argument.  If there is an existing compatible
// registration, this returns the already-registered instrument.  If
// there is no conflict and no prior registration, returns (nil, nil).
func (u *UniqueInstrumentMeterImpl) checkUniqueness(descriptor sdkapi.Descriptor) (sdkapi.InstrumentImpl, error) {
	impl, ok := u.state[descriptor.Name()]
	if !ok {
		return nil, nil
	}

	if !Compatible(descriptor, impl.Descriptor()) {
		return nil, NewMetricKindMismatchError(impl.Descriptor())
	}

	return impl, nil
}

// NewSyncInstrument implements sdkapi.MeterImpl.
func (u *UniqueInstrumentMeterImpl) NewSyncInstrument(descriptor sdkapi.Descriptor) (sdkapi.SyncImpl, error) {
	u.lock.Lock()
	defer u.lock.Unlock()

	impl, err := u.checkUniqueness(descriptor)

	if err != nil {
		return nil, err
	} else if impl != nil {
		return impl.(sdkapi.SyncImpl), nil
	}

	syncInst, err := u.impl.NewSyncInstrument(descriptor)
	if err != nil {
		return nil, err
	}
	u.state[descriptor.Name()] = syncInst
	return syncInst, nil
}

// NewAsyncInstrument implements sdkapi.MeterImpl.
func (u *UniqueInstrumentMeterImpl) NewAsyncInstrument(
	descriptor sdkapi.Descriptor,
	runner sdkapi.AsyncRunner,
) (sdkapi.AsyncImpl, error) {
	u.lock.Lock()
	defer u.lock.Unlock()

	impl, err := u.checkUniqueness(descriptor)

	if err != nil {
		return nil, err
	} else if impl != nil {
		return impl.(sdkapi.AsyncImpl), nil
	}

	asyncInst, err := u.impl.NewAsyncInstrument(descriptor, runner)
	if err != nil {
		return nil, err
	}
	u.state[descriptor.Name()] = asyncInst
	return asyncInst, nil
}
