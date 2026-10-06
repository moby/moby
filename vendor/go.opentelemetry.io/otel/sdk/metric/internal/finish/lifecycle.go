// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package finish

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type lifecycleState uint32

const (
	lifecycleActive lifecycleState = iota
	lifecycleShared
	lifecycleFinishPending
	lifecycleRetired
	lifecycleCollecting
)

// Lifecycle coordinates measurements, explicit finishes, collections, and
// retirement for one metric series. Its zero value is ready for use.
//
// Lifecycle methods are safe to call concurrently. A successful measurement
// acquisition and an emitted collection must each be completed exactly once.
// A Lifecycle must not be copied after first use.
//
// The valid transitions are active to finish-pending on Finish, active or
// finish-pending to shared on a shared measurement, finish-pending to active on
// an ordinary measurement, any non-retired state to collecting on collection,
// and collecting to its prior state or retired when collection completes.
// Shared lifetimes ignore Finish. Retired is terminal.
type Lifecycle struct {
	control sync.Mutex
	state   atomic.Uint32
	writers atomic.Uint64
	// finished is meaningful only while a pending finish is being collected.
	// Reactivation leaves the old value in place so measurements remain lock-free.
	finished time.Time
}

// Measurement represents admission to update one metric-series lifetime. A
// successful call to Lifecycle.AcquireMeasurement returns a Measurement, which
// must be released exactly once after the aggregate and exemplars are updated.
type Measurement struct {
	lifecycle *Lifecycle
}

// Release completes the admitted measurement.
func (m Measurement) Release() {
	m.lifecycle.releaseMeasurement()
}

// Collection describes an exclusive collection phase for a metric-series
// lifetime. A Collection returned with true by a Lifecycle collection method
// must be completed exactly once after the aggregate and exemplars are read.
type Collection struct {
	lifecycle *Lifecycle
	time      time.Time
	restore   lifecycleState
	retire    bool
}

// Time returns the timestamp to use for the collected point.
func (c Collection) Time() time.Time {
	return c.time
}

// ShouldRetire reports whether Complete will retire the series lifetime.
func (c Collection) ShouldRetire() bool {
	return c.retire
}

// Complete ends the exclusive collection phase. It must be called exactly once.
func (c Collection) Complete() {
	state := c.restore
	if c.retire {
		state = lifecycleRetired
	}
	c.lifecycle.finished = time.Time{}
	c.lifecycle.state.Store(uint32(state))
	c.lifecycle.control.Unlock()
}

// AcquireMeasurement reserves this series lifetime for a measurement. It waits
// for an in-progress collection and returns false only if the lifetime is
// retired. A successful acquisition must be paired with exactly one call to
// Measurement.Release after the aggregate and exemplars are updated.
func (l *Lifecycle) AcquireMeasurement() (Measurement, bool) {
	for {
		// Increment before checking state so a collection either waits for this
		// attempt or closes admission before the attempt can update the aggregate.
		l.writers.Add(1)
		switch lifecycleState(l.state.Load()) {
		case lifecycleActive, lifecycleShared:
			return Measurement{lifecycle: l}, true
		case lifecycleFinishPending:
			if l.state.CompareAndSwap(
				uint32(lifecycleFinishPending),
				uint32(lifecycleActive),
			) {
				return Measurement{lifecycle: l}, true
			}
			l.releaseMeasurement()
		case lifecycleCollecting:
			l.releaseMeasurement()
			l.waitForCollection()
		case lifecycleRetired:
			l.releaseMeasurement()
			return Measurement{}, false
		}
	}
}

// AcquireSharedMeasurement reserves this series lifetime for a measurement and
// atomically makes the lifetime shared. A shared lifetime cannot be finished.
// It waits for an in-progress collection and returns false only if the lifetime
// is retired. A successful acquisition must be paired with exactly one call to
// Measurement.Release after the aggregate and exemplars are updated.
func (l *Lifecycle) AcquireSharedMeasurement() (Measurement, bool) {
	for {
		// Increment before checking state so a collection either waits for this
		// attempt or closes admission before the attempt can update the aggregate.
		l.writers.Add(1)
		switch lifecycleState(l.state.Load()) {
		case lifecycleShared:
			return Measurement{lifecycle: l}, true
		case lifecycleActive:
			if l.state.CompareAndSwap(
				uint32(lifecycleActive),
				uint32(lifecycleShared),
			) {
				return Measurement{lifecycle: l}, true
			}
			l.releaseMeasurement()
		case lifecycleFinishPending:
			if l.state.CompareAndSwap(
				uint32(lifecycleFinishPending),
				uint32(lifecycleShared),
			) {
				return Measurement{lifecycle: l}, true
			}
			l.releaseMeasurement()
		case lifecycleCollecting:
			l.releaseMeasurement()
			l.waitForCollection()
		case lifecycleRetired:
			l.releaseMeasurement()
			return Measurement{}, false
		}
	}
}

func (l *Lifecycle) waitForCollection() {
	l.control.Lock()
	l.control.Unlock() //nolint:gocritic,staticcheck // Lock acquisition parks until collection completes.
}

func (l *Lifecycle) releaseMeasurement() {
	// atomic.Uint64 has no subtraction operation. Adding MaxUint64 decrements
	// the writer count modulo 2^64. A matching acquisition guarantees the count
	// is nonzero.
	l.writers.Add(^uint64(0))
}

// Finish marks an active series lifetime to be retired by its next collection
// at the supplied time. It returns false if the lifetime is already pending,
// shared, collecting, or retired. A measurement admitted after Finish
// reactivates the lifetime.
func (l *Lifecycle) Finish(at time.Time) bool {
	l.control.Lock()
	defer l.control.Unlock()
	if !l.state.CompareAndSwap(
		uint32(lifecycleActive),
		uint32(lifecycleFinishPending),
	) {
		return false
	}
	// Collection also holds control, so it cannot observe finish-pending before
	// its timestamp is available. A concurrent measurement may reactivate or
	// share the lifetime; in that case the timestamp is intentionally ignored.
	l.finished = at
	return true
}

// BeginCumulativeCollection closes measurement admission and waits for admitted
// measurements to complete. An active or shared lifetime returns to its prior
// state when the returned Collection is completed; a finish-pending lifetime is
// retired. It returns false without a Collection if the lifetime is retired. A
// returned Collection must be completed exactly once.
func (l *Lifecycle) BeginCumulativeCollection(at time.Time) (Collection, bool) {
	l.control.Lock()
	collection, ok := l.beginCollection(at)
	if !ok {
		l.control.Unlock()
	}
	return collection, ok
}

// BeginDeltaCollection closes measurement admission and waits for admitted
// measurements to complete. Every emitted lifetime is retired when the returned
// Collection is completed. It returns false without a Collection if the
// lifetime is retired. A returned Collection must be completed exactly once.
func (l *Lifecycle) BeginDeltaCollection(at time.Time) (Collection, bool) {
	l.control.Lock()
	collection, ok := l.beginCollection(at)
	if !ok {
		l.control.Unlock()
		return Collection{}, false
	}
	collection.retire = true
	return collection, true
}

func (l *Lifecycle) beginCollection(at time.Time) (Collection, bool) {
	state := lifecycleState(l.state.Load())
	if state == lifecycleRetired {
		return Collection{}, false
	}
	state = lifecycleState(l.state.Swap(uint32(lifecycleCollecting)))
	for l.writers.Load() != 0 {
		runtime.Gosched()
	}

	if state == lifecycleFinishPending {
		return Collection{
			lifecycle: l,
			time:      l.finished,
			restore:   state,
			retire:    true,
		}, true
	}
	return Collection{lifecycle: l, time: at, restore: state}, true
}

// Retire permanently closes the series lifetime. It waits for admitted
// measurements to complete and has no effect if the lifetime is already
// retired.
func (l *Lifecycle) Retire() {
	l.control.Lock()
	defer l.control.Unlock()
	if lifecycleState(l.state.Load()) == lifecycleRetired {
		return
	}
	l.state.Store(uint32(lifecycleCollecting))
	for l.writers.Load() != 0 {
		runtime.Gosched()
	}
	l.finished = time.Time{}
	l.state.Store(uint32(lifecycleRetired))
}
