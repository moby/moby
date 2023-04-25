package service

import (
	"testing"

	"github.com/docker/docker/api/types/filters"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestFilterWithPrune(t *testing.T) {
	f := filters.NewArgs()
	assert.NilError(t, withPrune(f))
	assert.Check(t, cmp.Len(f.Get("label"), 1))
	assert.Check(t, f.Match("label", AnonymousLabel))

	f = filters.NewArgs(
		filters.Arg("label", "foo=bar"),
		filters.Arg("label", "bar=baz"),
	)
	assert.NilError(t, withPrune(f))

	assert.Check(t, cmp.Len(f.Get("label"), 3))
	assert.Check(t, f.Match("label", AnonymousLabel))
	assert.Check(t, f.Match("label", "foo=bar"))
	assert.Check(t, f.Match("label", "bar=baz"))

	f = filters.NewArgs(
		filters.Arg("label", "foo=bar"),
		filters.Arg("all", "1"),
	)
	assert.NilError(t, withPrune(f))

	assert.Check(t, cmp.Len(f.Get("label"), 1))
	assert.Check(t, f.Match("label", "foo=bar"))

	f = filters.NewArgs(
		filters.Arg("label", "foo=bar"),
		filters.Arg("all", "true"),
	)
	assert.NilError(t, withPrune(f))

	assert.Check(t, cmp.Len(f.Get("label"), 1))
	assert.Check(t, f.Match("label", "foo=bar"))

	f = filters.NewArgs(filters.Arg("all", "0"))
	assert.NilError(t, withPrune(f))
	assert.Check(t, cmp.Len(f.Get("label"), 1))
	assert.Check(t, f.Match("label", AnonymousLabel))

	f = filters.NewArgs(filters.Arg("all", "false"))
	assert.NilError(t, withPrune(f))
	assert.Check(t, cmp.Len(f.Get("label"), 1))
	assert.Check(t, f.Match("label", AnonymousLabel))

	f = filters.NewArgs(filters.Arg("all", ""))
	assert.ErrorContains(t, withPrune(f), "invalid filter 'all'")

	f = filters.NewArgs(
		filters.Arg("all", "1"),
		filters.Arg("all", "0"),
	)
	assert.ErrorContains(t, withPrune(f), "invalid filter 'all")
}
