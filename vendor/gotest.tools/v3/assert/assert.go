/*Package assert provides assertions for comparing expected values to actual
values. When an assertion fails a helpful error message is printed.

Assert and Check

Assert() and Check() both accept a Comparison, and fail the test when the
comparison fails. The one difference is that Assert() will end the test execution
immediately (using t.FailNow()) whereas Check() will fail the test (using t.Fail()),
return the value of the comparison, then proceed with the rest of the test case.

Example usage

The example below shows assert used with some common types.


	import (
	    "testing"

	    "gotest.tools/assert"
	    is "gotest.tools/assert/cmp"
	)

	func TestEverything(t *testing.T) {
	    // booleans
	    assert.Assert(t, ok)
	    assert.Assert(t, !missing)

	    // primitives
	    assert.Equal(t, count, 1)
	    assert.Equal(t, msg, "the message")
	    assert.Assert(t, total != 10) // NotEqual

	    // errors
	    assert.NilError(t, closer.Close())
	    assert.Error(t, err, "the exact error message")
	    assert.ErrorContains(t, err, "includes this")
	    assert.ErrorType(t, err, os.IsNotExist)

	    // complex types
	    assert.DeepEqual(t, result, myStruct{Name: "title"})
	    assert.Assert(t, is.Len(items, 3))
	    assert.Assert(t, len(sequence) != 0) // NotEmpty
	    assert.Assert(t, is.Contains(mapping, "key"))

	    // pointers and interface
	    assert.Assert(t, is.Nil(ref))
	    assert.Assert(t, ref != nil) // NotNil
	}

Comparisons

Package http://pkg.go.dev/gotest.tools/v3/assert/cmp provides
many common comparisons. Additional comparisons can be written to compare
values in other ways. See the example Assert (CustomComparison).

Automated migration from testify

gty-migrate-from-testify is a command which translates Go source code from
testify assertions to the assertions provided by this package.

See http://pkg.go.dev/gotest.tools/v3/assert/cmd/gty-migrate-from-testify.


*/
package assert // import "gotest.tools/v3/assert"

import (
	gocmp "github.com/google/go-cmp/cmp"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/internal/assert"
)

// BoolOrComparison can be a bool, or cmp.Comparison. See Assert() for usage.
type BoolOrComparison interface{}

// TestingT is the subset of testing.T used by the assert package.
type TestingT interface {
	FailNow()
	Fail()
	Log(args ...interface{})
}

type helperT interface {
	Helper()
}

// Assert performs a comparison. If the comparison fails, the test is marked as
// failed, a failure message is logged, and execution is stopped immediately.
//
// The comparison argument may be one of three types:
//   bool
// True is success. False is a failure.
// The failure message will contain the literal source code of the expression.
//   cmp.Comparison
// Uses cmp.Result.Success() to check for success of failure.
// The comparison is responsible for producing a helpful failure message.
// http://pkg.go.dev/gotest.tools/v3/assert/cmp provides many common comparisons.
//   error
// A nil value is considered success.
// A non-nil error is a failure, err.Error() is used as the failure message.
func Assert(t TestingT, comparison BoolOrComparison, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	if !assert.Eval(t, assert.ArgsFromComparisonCall, comparison, msgAndArgs...) {
		t.FailNow()
	}
}

// Check performs a comparison. If the comparison fails the test is marked as
// failed, a failure message is logged, and Check returns false. Otherwise returns
// true.
//
// See Assert for details about the comparison arg and failure messages.
func Check(t TestingT, comparison BoolOrComparison, msgAndArgs ...interface{}) bool {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	if !assert.Eval(t, assert.ArgsFromComparisonCall, comparison, msgAndArgs...) {
		t.Fail()
		return false
	}
	return true
}

// NilError fails the test immediately if err is not nil.
// This is equivalent to Assert(t, err)
func NilError(t TestingT, err error, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	if !assert.Eval(t, assert.ArgsAfterT, err, msgAndArgs...) {
		t.FailNow()
	}
}

// Equal uses the == operator to assert two values are equal and fails the test
// if they are not equal.
//
// If the comparison fails Equal will use the variable names for x and y as part
// of the failure message to identify the actual and expected values.
//
// If either x or y are a multi-line string the failure message will include a
// unified diff of the two values. If the values only differ by whitespace
// the unified diff will be augmented by replacing whitespace characters with
// visible characters to identify the whitespace difference.
//
// This is equivalent to Assert(t, cmp.Equal(x, y)).
func Equal(t TestingT, x, y interface{}, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	if !assert.Eval(t, assert.ArgsAfterT, cmp.Equal(x, y), msgAndArgs...) {
		t.FailNow()
	}
}

// DeepEqual uses google/go-cmp (https://godoc.org/github.com/google/go-cmp/cmp)
// to assert two values are equal and fails the test if they are not equal.
//
// Package http://pkg.go.dev/gotest.tools/v3/assert/opt provides some additional
// commonly used Options.
//
// This is equivalent to Assert(t, cmp.DeepEqual(x, y)).
func DeepEqual(t TestingT, x, y interface{}, opts ...gocmp.Option) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	if !assert.Eval(t, assert.ArgsAfterT, cmp.DeepEqual(x, y, opts...)) {
		t.FailNow()
	}
}

// Error fails the test if err is nil, or the error message is not the expected
// message.
// Equivalent to Assert(t, cmp.Error(err, message)).
func Error(t TestingT, err error, message string, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	if !assert.Eval(t, assert.ArgsAfterT, cmp.Error(err, message), msgAndArgs...) {
		t.FailNow()
	}
}

// ErrorContains fails the test if err is nil, or the error message does not
// contain the expected substring.
// Equivalent to Assert(t, cmp.ErrorContains(err, substring)).
func ErrorContains(t TestingT, err error, substring string, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	if !assert.Eval(t, assert.ArgsAfterT, cmp.ErrorContains(err, substring), msgAndArgs...) {
		t.FailNow()
	}
}

// ErrorType fails the test if err is nil, or err is not the expected type.
// Equivalent to Assert(t, cmp.ErrorType(err, expected)).
//
// Expected can be one of:
//   func(error) bool
// Function should return true if the error is the expected type.
//   type struct{}, type &struct{}
// A struct or a pointer to a struct.
// Fails if the error is not of the same type as expected.
//   type &interface{}
// A pointer to an interface type.
// Fails if err does not implement the interface.
//   reflect.Type
// Fails if err does not implement the reflect.Type
func ErrorType(t TestingT, err error, expected interface{}, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	if !assert.Eval(t, assert.ArgsAfterT, cmp.ErrorType(err, expected), msgAndArgs...) {
		t.FailNow()
	}
}
