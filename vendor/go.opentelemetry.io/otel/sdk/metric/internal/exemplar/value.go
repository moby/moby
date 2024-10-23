// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exemplar // import "go.opentelemetry.io/otel/sdk/metric/internal/exemplar"

import "math"

// ValueType identifies the type of value used in exemplar data.
type ValueType uint8

const (
	// UnknownValueType should not be used. It represents a misconfigured
	// Value.
	UnknownValueType ValueType = 0
	// Int64ValueType represents a Value with int64 data.
	Int64ValueType ValueType = 1
	// Float64ValueType represents a Value with float64 data.
	Float64ValueType ValueType = 2
)

// Value is the value of data held by an exemplar.
type Value struct {
	t   ValueType
	val uint64
}

// NewValue returns a new [Value] for the provided value.
func NewValue[N int64 | float64](value N) Value {
	switch v := any(value).(type) {
	case int64:
		return Value{t: Int64ValueType, val: uint64(v)}
	case float64:
		return Value{t: Float64ValueType, val: math.Float64bits(v)}
	}
	return Value{}
}

// Type returns the [ValueType] of data held by v.
func (v Value) Type() ValueType { return v.t }

// Int64 returns the value of v as an int64. If the ValueType of v is not an
// Int64ValueType, 0 is returned.
func (v Value) Int64() int64 {
	if v.t == Int64ValueType {
		return int64(v.val)
	}
	return 0
}

// Float64 returns the value of v as an float64. If the ValueType of v is not
// an Float64ValueType, 0 is returned.
func (v Value) Float64() float64 {
	if v.t == Float64ValueType {
		return math.Float64frombits(v.val)
	}
	return 0
}
