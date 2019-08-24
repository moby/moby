// +build ignore

package main

import (
	"strings"
	"testing"

	"gotest.tools/assert"
)

type fn func(arg1, arg2 string, extra ...interface{}) bool
type assertfn func(t assert.TestingT, comparison assert.BoolOrComparison, msgAndArgs ...interface{})

func before(
	t *testing.T,
	a assertfn,
	eg_contains fn,
	arg1 string,
	arg2 string,
	extra ...interface{}) {

	a(t, !eg_contains(arg1, arg2, extra...))
}

func after(
	t *testing.T,
	a assertfn,
	eg_contains fn,
	arg1 string,
	arg2 string,
	extra ...interface{}) {

	a(t, !strings.Contains(arg1, arg2), extra...)
}
