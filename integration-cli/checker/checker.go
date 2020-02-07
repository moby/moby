// Package checker provides helpers for gotest.tools/assert.
// Please remove this package whenever possible.
package checker // import "github.com/docker/docker/integration-cli/checker"

import (
	"fmt"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// Compare defines the interface to compare values
type Compare func(x interface{}) assert.BoolOrComparison

// False checks if the value is false
func False() Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return !x.(bool)
	}
}

// True checks if the value is true
func True() Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return x
	}
}

// Equals checks if the value is equal to the given value
func Equals(y interface{}) Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return cmp.Equal(x, y)
	}
}

// Contains checks if the value contains the given value
func Contains(y interface{}) Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return cmp.Contains(x, y)
	}
}

// Not checks if two values are not
func Not(c Compare) Compare {
	return func(x interface{}) assert.BoolOrComparison {
		r := c(x)
		switch r := r.(type) {
		case bool:
			return !r
		case cmp.Comparison:
			return !r().Success()
		default:
			panic(fmt.Sprintf("unexpected type %T", r))
		}
	}
}

// DeepEquals checks if two values are equal
func DeepEquals(y interface{}) Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return cmp.DeepEqual(x, y)
	}
}

// HasLen checks if the value has the expected number of elements
func HasLen(y int) Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return cmp.Len(x, y)
	}
}

// IsNil checks if the value is nil
func IsNil() Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return cmp.Nil(x)
	}
}

// GreaterThan checks if the value is greater than the given value
func GreaterThan(y int) Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return x.(int) > y
	}
}
