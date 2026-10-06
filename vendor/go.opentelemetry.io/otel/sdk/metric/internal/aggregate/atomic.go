// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregate

import (
	"math"
	"runtime"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
)

// atomicCounter is an efficient way of adding to a number which is either an
// int64 or float64. It is designed to be efficient when adding whole
// numbers, regardless of whether N is an int64 or float64.
//
// Inspired by the Prometheus counter implementation:
// https://github.com/prometheus/client_golang/blob/14ccb93091c00f86b85af7753100aa372d63602b/prometheus/counter.go#L108
type atomicCounter[N int64 | float64] struct {
	// nFloatBits contains only the non-integer portion of the counter.
	nFloatBits atomic.Uint64
	// nInt contains only the integer portion of the counter.
	nInt atomic.Int64
}

// load returns the current value. The caller must ensure all calls to add have
// returned prior to calling load.
func (n *atomicCounter[N]) load() (value N) {
	switch any(value).(type) {
	case int64:
		value = N(n.nInt.Load())
	case float64:
		fval := math.Float64frombits(n.nFloatBits.Load())
		ival := n.nInt.Load()
		value = N(fval + float64(ival))
	default:
		panic("unsupported type")
	}
	return value
}

func (n *atomicCounter[N]) add(value N) {
	ival := int64(value)
	// This case is where the value is an int, or if it is a whole-numbered float.
	if float64(ival) == float64(value) {
		n.nInt.Add(ival)
		return
	}

	// Value must be a float below.
	for {
		oldBits := n.nFloatBits.Load()
		newBits := math.Float64bits(math.Float64frombits(oldBits) + float64(value))
		if n.nFloatBits.CompareAndSwap(oldBits, newBits) {
			return
		}
	}
}

// reset resets the internal state, and is not safe to call concurrently.
func (n *atomicCounter[N]) reset() {
	n.nFloatBits.Store(0)
	n.nInt.Store(0)
}

// atomicN is a generic atomic number value.
type atomicN[N int64 | float64] struct {
	val atomic.Uint64
}

func (a *atomicN[N]) Load() (value N) {
	v := a.val.Load()
	switch any(value).(type) {
	case int64:
		value = N(v)
	case float64:
		value = N(math.Float64frombits(v))
	default:
		panic("unsupported type")
	}
	return value
}

func (a *atomicN[N]) Store(v N) {
	var val uint64
	switch any(v).(type) {
	case int64:
		val = uint64(v)
	case float64:
		val = math.Float64bits(float64(v))
	default:
		panic("unsupported type")
	}
	a.val.Store(val)
}

func (a *atomicN[N]) CompareAndSwap(oldN, newN N) bool {
	var o, n uint64
	switch any(oldN).(type) {
	case int64:
		o, n = uint64(oldN), uint64(newN)
	case float64:
		o, n = math.Float64bits(float64(oldN)), math.Float64bits(float64(newN))
	default:
		panic("unsupported type")
	}
	return a.val.CompareAndSwap(o, n)
}

type atomicMinMax[N int64 | float64] struct {
	minimum, maximum atomicN[N]
	set              atomic.Bool
	mu               sync.Mutex
}

// init returns true if the value was used to initialize min and max.
func (s *atomicMinMax[N]) init(val N) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.set.Load() {
		defer s.set.Store(true)
		s.minimum.Store(val)
		s.maximum.Store(val)
		return true
	}
	return false
}

func (s *atomicMinMax[N]) Update(val N) {
	if !s.set.Load() && s.init(val) {
		return
	}

	old := s.minimum.Load()
	for val < old {
		if s.minimum.CompareAndSwap(old, val) {
			return
		}
		old = s.minimum.Load()
	}

	old = s.maximum.Load()
	for old < val {
		if s.maximum.CompareAndSwap(old, val) {
			return
		}
		old = s.maximum.Load()
	}
}

// hotColdWaitGroup is a synchronization primitive which enables lockless
// writes for concurrent writers and enables a reader to acquire exclusive
// access to a snapshot of state including only completed operations.
// Conceptually, it can be thought of as a "hot" wait group,
// and a "cold" wait group, with the ability for the reader to atomically swap
// the hot and cold wait groups, and wait for the now-cold wait group to
// complete.
//
// Inspired by the prometheus/client_golang histogram implementation:
// https://github.com/prometheus/client_golang/blob/a974e0d45e0aa54c65492559114894314d8a2447/prometheus/histogram.go#L725
//
// Usage:
//
//	var hcwg hotColdWaitGroup
//	var data [2]any
//
//	func write() {
//	  hotIdx := hcwg.start()
//	  defer hcwg.done(hotIdx)
//	  // modify data without locking
//	  data[hotIdx].update()
//	}
//
//	func read() {
//	  coldIdx := hcwg.swapHotAndWait()
//	  // read data now that all writes to the cold data have completed.
//	  data[coldIdx].read()
//	}
type hotColdWaitGroup struct {
	// startedCountAndHotIdx contains a 63-bit counter in the lower bits,
	// and a 1 bit hot index to denote which of the two data-points new
	// measurements to write to. These are contained together so that read()
	// can atomically swap the hot bit, reset the started writes to zero, and
	// read the number writes that were started prior to the hot bit being
	// swapped.
	startedCountAndHotIdx atomic.Uint64
	// endedCounts is the number of writes that have completed to each
	// dataPoint.
	endedCounts [2]atomic.Uint64
}

// hotIdx represents an index (0 or 1) to the currently active "hot" buffer
// where concurrent writes are accepted.
//
// coldIdx represents an index (0 or 1) to the "cold" buffer where collection
// reads and resets occur after a swap.
//
// These distinct types enforce at compile time that hot (write) and cold (read)
// indices are not accidentally interchanged across call boundaries.
type (
	hotIdx  uint64
	coldIdx uint64
)

// start returns the hot index that the writer should write to. The returned
// hot index is 0 or 1. The caller must call done(hot index) after it finishes
// its operation. start() is safe to call concurrently with other methods.
func (l *hotColdWaitGroup) start() hotIdx {
	// We increment h.startedCountAndHotIdx so that the counter in the lower
	// 63 bits gets incremented. At the same time, we get the new value
	// back, which we can use to return the currently-hot index.
	return hotIdx(l.startedCountAndHotIdx.Add(1) >> 63)
}

// done signals to the reader that an operation has fully completed.
// done is safe to call concurrently.
func (l *hotColdWaitGroup) done(hotIdx hotIdx) {
	l.endedCounts[hotIdx].Add(1)
}

// swapHotAndWait swaps the hot bit, waits for all start() calls to be done(),
// and then returns the now-cold index for the reader to read from. The
// returned index is 0 or 1. swapHotAndWait must not be called concurrently.
func (l *hotColdWaitGroup) swapHotAndWait() coldIdx {
	n := l.startedCountAndHotIdx.Load()
	coldIdxVal := (^n) >> 63
	// Swap the hot and cold index while resetting the started measurements
	// count to zero.
	n = l.startedCountAndHotIdx.Swap((coldIdxVal << 63))
	hotIdxVal := n >> 63
	startedCount := n & ((1 << 63) - 1)
	// Wait for all measurements to the previously-hot map to finish.
	for startedCount != l.endedCounts[hotIdxVal].Load() {
		runtime.Gosched() // Let measurements complete.
	}
	// reset the number of ended operations
	l.endedCounts[hotIdxVal].Store(0)
	return coldIdx(hotIdxVal)
}

// limitedSyncMap is a sync.Map which enforces the aggregation limit on
// attribute sets and provides a Len() function.
type limitedSyncMap[V any] struct {
	sync.Map
	aggLimit int
	len      int
	lenMux   sync.Mutex
}

// LoadOrStoreAttr performs lookup using lazy.Distinct() on the hot path without
// constructing a Set. If the entry is not found and the aggregation limit has not
// been exceeded, lazy.Set() is called to construct the attribute.Set for storage.
// If the aggregation limit is exceeded, overflowSet is used instead without calling lazy.Set().
func (m *limitedSyncMap[V]) LoadOrStoreAttr(lazy lazyFilteredAttributes, newValue func(attribute.Set) V) V {
	distinct := lazy.Distinct()
	actual, loaded := m.Load(distinct)
	if loaded {
		return actual.(V)
	}
	// If the overflow set exists, assume we have already overflowed and don't
	// bother with the slow path below.
	actual, loaded = m.Load(overflowSet.Equivalent())
	if loaded {
		return actual.(V)
	}
	// Slow path: add a new attribute set.
	m.lenMux.Lock()
	defer m.lenMux.Unlock()

	// re-fetch now that we hold the lock to ensure we don't use the overflow
	// set unless we are sure the attribute set isn't being written
	// concurrently.
	actual, loaded = m.Load(distinct)
	if loaded {
		return actual.(V)
	}

	var fltrAttr attribute.Set
	if m.aggLimit > 0 && m.len >= m.aggLimit-1 {
		fltrAttr = overflowSet
		distinct = overflowSet.Equivalent()
	} else {
		fltrAttr = lazy.Set()
	}
	actual, loaded = m.LoadOrStore(distinct, newValue(fltrAttr))
	if !loaded {
		m.len++
	}
	return actual.(V)
}

// LoadOrStoreAttrReclaiming is equivalent to LoadOrStoreAttr, except that a
// slot released after overflow was created can be reused for a new attribute
// set. The shared overflow entry remains available for measurements made while
// all normal slots are occupied. It reports whether the value was loaded and
// whether this lookup was routed to the shared overflow entry.
func (m *limitedSyncMap[V]) LoadOrStoreAttrReclaiming(
	lazy lazyFilteredAttributes,
	newValue func(attribute.Set, bool) V,
) (value V, loaded, overflowed bool) {
	distinct := lazy.Distinct()
	actual, loaded := m.Load(distinct)
	if loaded {
		return actual.(V), true, false
	}

	m.lenMux.Lock()
	defer m.lenMux.Unlock()

	actual, loaded = m.Load(distinct)
	if loaded {
		return actual.(V), true, false
	}

	overflow, hasOverflow := m.Load(overflowSet.Equivalent())
	if m.aggLimit > 0 && hasOverflow && m.len >= m.aggLimit {
		return overflow.(V), true, true
	}

	var attrs attribute.Set
	overflowed = m.aggLimit > 0 && !hasOverflow && m.len >= m.aggLimit-1
	if overflowed {
		attrs = overflowSet
		distinct = overflowSet.Equivalent()
	} else {
		attrs = lazy.Set()
	}
	actual, loaded = m.LoadOrStore(distinct, newValue(attrs, overflowed))
	if !loaded {
		m.len++
	}
	return actual.(V), loaded, overflowed
}

func (m *limitedSyncMap[V]) Clear() {
	m.lenMux.Lock()
	defer m.lenMux.Unlock()
	m.len = 0
	m.Map.Clear()
}

// CompareAndDelete deletes the entry for key if its value is equal to old.
// It reports whether the entry was deleted.
func (m *limitedSyncMap[V]) CompareAndDelete(key attribute.Distinct, old V) bool {
	m.lenMux.Lock()
	defer m.lenMux.Unlock()
	if !m.Map.CompareAndDelete(key, old) {
		return false
	}
	m.len--
	return true
}

func (m *limitedSyncMap[V]) Len() int {
	m.lenMux.Lock()
	defer m.lenMux.Unlock()
	return m.len
}

// hotColdMap manages two [limitedSyncMap] instances so that measurement and
// collection operate on separate maps and do not lock each other out.
//
// Measurements write to the currently hot map returned by [hotColdMap.hot]
// using the index from [hotColdWaitGroup.start]. Collection is performed by
// calling [hotColdWaitGroup.swapHotAndWait] to atomically swap the hot and cold
// maps and wait for in-flight writes to the cold map to complete. The caller
// can then read the cold map via [hotColdMap.Range] and must call
// [hotColdMap.Clear] on the cold index so that unused attribute sets do not
// report in subsequent collection cycles and the cardinality limit budget is
// reset.
//
// swapHotAndWait must not be called concurrently.
type hotColdMap[V any] struct {
	hotColdWaitGroup
	hotColdValMap [2]limitedSyncMap[V]
}

func (m *hotColdMap[V]) init(limit int) {
	m.hotColdValMap[0].aggLimit = limit
	m.hotColdValMap[1].aggLimit = limit
}

func (m *hotColdMap[V]) hot(i hotIdx) *limitedSyncMap[V] {
	return &m.hotColdValMap[i]
}

// Len returns the length of the specified map.
func (m *hotColdMap[V]) Len(readIdx coldIdx) int {
	return m.hotColdValMap[readIdx].Len()
}

// Range calls f sequentially for each key and value present in the map at readIdx.
func (m *hotColdMap[V]) Range(readIdx coldIdx, f func(key, value any) bool) {
	m.hotColdValMap[readIdx].Range(f)
}

// Clear clears the map at readIdx.
func (m *hotColdMap[V]) Clear(readIdx coldIdx) {
	m.hotColdValMap[readIdx].Clear()
}
