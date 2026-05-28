// Package checker provides helpers for gotest.tools/assert.
// Please remove this package whenever possible.
package checker

import (
	"fmt"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// Compare defines the interface to compare values
type Compare func(x any) assert.BoolOrComparison

// False checks if the value is false
func False() Compare {
	return func(x any) assert.BoolOrComparison {
		return !x.(bool)
	}
}

// True checks if the value is true
func True() Compare {
	return func(x any) assert.BoolOrComparison {
		return x
	}
}

// Equals checks if the value is equal to the given value
func Equals(y any) Compare {
	return func(x any) assert.BoolOrComparison {
		return is.Equal(x, y)
	}
}

// Contains checks if the value contains the given value
func Contains(y any) Compare {
	return func(x any) assert.BoolOrComparison {
		return is.Contains(x, y)
	}
}

// Not checks if two values are not
func Not(c Compare) Compare {
	return func(x any) assert.BoolOrComparison {
		r := c(x)
		switch r := r.(type) {
		case bool:
			return !r
		case is.Comparison:
			return !r().Success()
		default:
			panic(fmt.Sprintf("unexpected type %T", r))
		}
	}
}

// DeepEquals checks if two values are equal
func DeepEquals(y any) Compare {
	return func(x any) assert.BoolOrComparison {
		return is.DeepEqual(x, y)
	}
}

// HasLen checks if the value has the expected number of elements
func HasLen(y int) Compare {
	return func(x any) assert.BoolOrComparison {
		return is.Len(x, y)
	}
}

// IsNil checks if the value is nil
func IsNil() Compare {
	return func(x any) assert.BoolOrComparison {
		return is.Nil(x)
	}
}

// GreaterThan checks if the value is greater than the given value
func GreaterThan(y int) Compare {
	return func(x any) assert.BoolOrComparison {
		return x.(int) > y
	}
}
