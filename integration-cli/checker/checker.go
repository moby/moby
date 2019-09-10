// Package checker provides helpers for gotest.tools/assert.
// Please remove this package whenever possible.
package checker // import "github.com/docker/docker/integration-cli/checker"

import (
	"fmt"

	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

type Compare func(x interface{}) assert.BoolOrComparison

func False() Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return !x.(bool)
	}
}

func True() Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return x
	}
}

func Equals(y interface{}) Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return cmp.Equal(x, y)
	}
}

func Contains(y interface{}) Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return cmp.Contains(x, y)
	}
}

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

func DeepEquals(y interface{}) Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return cmp.DeepEqual(x, y)
	}
}

func HasLen(y int) Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return cmp.Len(x, y)
	}
}

func IsNil() Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return cmp.Nil(x)
	}
}

func GreaterThan(y int) Compare {
	return func(x interface{}) assert.BoolOrComparison {
		return x.(int) > y
	}
}

func NotNil() Compare {
	return Not(IsNil())
}
