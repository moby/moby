// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attrnorm

import (
	"cmp"
	"slices"

	"go.opentelemetry.io/otel/attribute"
)

// stackAttributes is the maximum number of unordered attributes copied to a
// stack buffer for canonicalization. Larger inputs use an allocated copy to
// keep sorting time bounded without modifying caller-owned attributes.
const stackAttributes = 10

// NewDistinct returns the identity of kvs after normalization and filtering.
// It does not retain or modify kvs. Duplicate keys use the last value supplied.
// The filter is called once for each unique normalized attribute in ascending
// key order.
func NewDistinct(kvs []attribute.KeyValue, filter attribute.Filter) attribute.Distinct {
	if filter == nil {
		if distinct, ok := tryUnfilteredDistinct(kvs); ok {
			return distinct
		}
	} else if ordered(kvs) {
		return orderedDistinct(kvs, filter)
	}
	if len(kvs) <= stackAttributes {
		return sortedDistinct(kvs, filter)
	}
	return allocatedDistinct(kvs, filter)
}

// tryUnfilteredDistinct hashes kvs while verifying their keys are ordered. The
// partial hash is discarded if an out-of-order key is found.
func tryUnfilteredDistinct(kvs []attribute.KeyValue) (attribute.Distinct, bool) {
	hasher := attribute.NewHasher()
	if len(kvs) == 0 {
		return hasher.Distinct(), true
	}

	// pending identifies the last value seen for the current key.
	pending := 0
	for i := 1; i < len(kvs); i++ {
		if kvs[i].Key < kvs[pending].Key {
			return attribute.Distinct{}, false
		}
		if kvs[i].Key == kvs[pending].Key {
			pending = i
			continue
		}

		normalized, _ := KeyValueDedup(kvs[pending])
		hasher.Write(normalized)
		pending = i
	}

	normalized, _ := KeyValueDedup(kvs[pending])
	hasher.Write(normalized)
	return hasher.Distinct(), true
}

// Keep this direct loop instead of slices.IsSortedFunc. It only needs a
// greater-than comparison and benchmarks faster for typical attribute counts.
func ordered(kvs []attribute.KeyValue) bool {
	for i := 1; i < len(kvs); i++ {
		if kvs[i-1].Key > kvs[i].Key {
			return false
		}
	}
	return true
}

// orderedDistinct hashes key-values whose keys are in ascending order. Equal
// keys remain in input order so selecting the last retains last-value-wins.
func orderedDistinct(kvs []attribute.KeyValue, filter attribute.Filter) attribute.Distinct {
	hasher := attribute.NewHasher()
	for i := 0; i < len(kvs); {
		j := i + 1
		for j < len(kvs) && kvs[j].Key == kvs[i].Key {
			j++
		}
		kv, _ := KeyValueDedup(kvs[j-1])
		if filter == nil || filter(kv) {
			hasher.Write(kv)
		}
		i = j
	}
	return hasher.Distinct()
}

func sortedDistinct(kvs []attribute.KeyValue, filter attribute.Filter) attribute.Distinct {
	var scratch [stackAttributes]attribute.KeyValue
	cpy := scratch[:len(kvs)]
	copy(cpy, kvs)
	return sortDistinct(cpy, filter)
}

func allocatedDistinct(kvs []attribute.KeyValue, filter attribute.Filter) attribute.Distinct {
	return sortDistinct(slices.Clone(kvs), filter)
}

func sortDistinct(kvs []attribute.KeyValue, filter attribute.Filter) attribute.Distinct {
	slices.SortStableFunc(kvs, func(a, b attribute.KeyValue) int {
		return cmp.Compare(a.Key, b.Key)
	})
	return orderedDistinct(kvs, filter)
}
