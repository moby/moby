// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregate

import (
	"math/bits"

	"go.opentelemetry.io/otel/attribute"
)

type lazyFilteredAttributes struct {
	orig       attribute.Set
	distinct   attribute.Distinct
	mask       uint64 // bitmask of kept attributes (bit i == 1 if kept) for len <= 64
	bigMask    []bool // fallback decision slice for len > 64
	hasDropped bool
}

// newLazyFilteredAttributes filters orig using filter and computes the distinct
// hash of the resulting attributes. The filter function is evaluated once per
// attribute during initialization, and kept attribute indices are recorded in a
// bitmask so subsequent Set and Dropped calls do not re-evaluate filter.
func newLazyFilteredAttributes(orig attribute.Set, filter attribute.Filter) lazyFilteredAttributes {
	if filter == nil {
		return lazyFilteredAttributes{
			orig:     orig,
			distinct: orig.Equivalent(),
		}
	}
	l := lazyFilteredAttributes{orig: orig, hasDropped: true}

	n := orig.Len()
	hasher := attribute.NewHasher()
	keptCount := 0
	for i := range n {
		kv, _ := orig.Get(i)
		if filter(kv) {
			hasher.Write(kv)
			l.recordKept(i, keptCount, n)
			keptCount++
		}
	}
	if keptCount == n {
		l.hasDropped = false
		l.mask = 0
		l.bigMask = nil
		l.distinct = orig.Equivalent()
		return l
	}
	keptBefore64 := bits.OnesCount64(l.mask)
	if keptCount-keptBefore64 > 0 {
		l.ensureBigMask(n, keptCount)
	}
	l.distinct = hasher.Distinct()
	return l
}

// recordKept marks index i as kept in the bitmask or fallback slice.
func (l *lazyFilteredAttributes) recordKept(i, keptCount, total int) {
	if i < 64 {
		l.mask |= uint64(1) << i
		return
	}
	if l.bigMask == nil && keptCount < i {
		l.ensureBigMask(total, keptCount)
	}
	if l.bigMask != nil {
		l.bigMask[i] = true
	}
}

// ensureBigMask initializes the bigMask slice and backfills any attributes past
// index 63 that were kept before attributes were dropped.
func (l *lazyFilteredAttributes) ensureBigMask(total, keptCount int) {
	if l.bigMask != nil {
		return
	}
	l.bigMask = make([]bool, total)
	keptBefore64 := bits.OnesCount64(l.mask)
	for j := 64; j < 64+(keptCount-keptBefore64); j++ {
		l.bigMask[j] = true
	}
}

// isKept reports whether the attribute at index i was kept by the filter.
func (l lazyFilteredAttributes) isKept(i int) bool {
	if i < 64 {
		return (l.mask & (uint64(1) << i)) != 0
	}
	return i < len(l.bigMask) && l.bigMask[i]
}

// Distinct returns the precomputed Distinct hash of the filtered attributes.
func (l lazyFilteredAttributes) Distinct() attribute.Distinct {
	return l.distinct
}

// HasDroppedAttributes reports whether the filter dropped any attributes.
func (l lazyFilteredAttributes) HasDroppedAttributes() bool {
	return l.hasDropped
}

// Set constructs the filtered attribute Set using the recorded bitmask.
func (l lazyFilteredAttributes) Set() attribute.Set {
	if !l.hasDropped {
		return l.orig
	}
	if l.mask == 0 && l.bigMask == nil {
		return attribute.NewSet()
	}
	n := l.orig.Len()
	var kept []attribute.KeyValue
	for i := range n {
		if l.isKept(i) {
			if kept == nil {
				kept = make([]attribute.KeyValue, 0, n)
			}
			kv, _ := l.orig.Get(i)
			kept = append(kept, kv)
		}
	}
	return attribute.NewSet(kept...)
}

// Dropped constructs the dropped attribute slice using the recorded bitmask.
func (l lazyFilteredAttributes) Dropped() []attribute.KeyValue {
	if !l.hasDropped {
		return nil
	}
	if l.mask == 0 && l.bigMask == nil {
		return l.orig.ToSlice()
	}
	n := l.orig.Len()
	var dropped []attribute.KeyValue
	for i := range n {
		if !l.isKept(i) {
			if dropped == nil {
				dropped = make([]attribute.KeyValue, 0, n)
			}
			kv, _ := l.orig.Get(i)
			dropped = append(dropped, kv)
		}
	}
	return dropped
}
