// +build ignore

package main

import (
	"testing"

	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

type fn func(re func(cmp.RegexOrPattern, string) cmp.Comparison, r interface{}, v string, extra ...interface{}) bool
type assertfn func(t assert.TestingT, comparison assert.BoolOrComparison, msgAndArgs ...interface{})

func before(
	t *testing.T,
	a assertfn,
	eg_matches fn,
	re func(cmp.RegexOrPattern, string) cmp.Comparison,
	r string,
	v string,
	extra ...interface{}) {

	a(t, eg_matches(re, v, r, extra...))
}

func after(
	t *testing.T,
	a assertfn,
	eg_matches fn,
	re func(cmp.RegexOrPattern, string) cmp.Comparison,
	r string,
	v string,
	extra ...interface{}) {

	a(t, re("^"+r+"$", v), extra...)
}
