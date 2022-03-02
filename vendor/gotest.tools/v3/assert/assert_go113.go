// +build go1.13

package assert

import (
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/internal/assert"
)

// ErrorIs fails the test if err is nil, or the error does not match expected
// when compared using errors.Is. See https://golang.org/pkg/errors/#Is for
// accepted argument values.
func ErrorIs(t TestingT, err error, expected error, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	if !assert.Eval(t, assert.ArgsAfterT, cmp.ErrorIs(err, expected), msgAndArgs...) {
		t.FailNow()
	}
}
