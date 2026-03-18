package client

import (
	"net/url"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestFilters(t *testing.T) {
	f := make(Filters).Add("foo", "bar", "baz", "bar").
		Add("quux", "xyzzy").
		Add("quux", "plugh")
	f["lol"] = map[string]bool{"abc": true}
	f.Add("lol", "def")
	assert.Check(t, is.DeepEqual(f, Filters{
		"foo":  {"bar": true, "baz": true},
		"quux": {"xyzzy": true, "plugh": true},
		"lol":  {"abc": true, "def": true},
	}))
}

func TestFilters_UpdateURLValues(t *testing.T) {
	v := url.Values{}
	Filters(nil).updateURLValues(v)
	assert.Check(t, is.DeepEqual(v, url.Values{}))

	v = url.Values{"filters": []string{"bogus"}}
	Filters(nil).updateURLValues(v)
	assert.Check(t, is.DeepEqual(v, url.Values{}))

	v = url.Values{}
	Filters{}.updateURLValues(v)
	assert.Check(t, is.DeepEqual(v, url.Values{}))

	v = url.Values{"filters": []string{"bogus"}}
	Filters{}.updateURLValues(v)
	assert.Check(t, is.DeepEqual(v, url.Values{}))

	v = url.Values{}
	Filters{"foo": map[string]bool{"bar": true}}.updateURLValues(v)
	assert.Check(t, is.DeepEqual(v, url.Values{"filters": []string{`{"foo":{"bar":true}}`}}))

	v = url.Values{"filters": []string{"bogus"}}
	Filters{"foo": map[string]bool{"bar": true}}.updateURLValues(v)
	assert.Check(t, is.DeepEqual(v, url.Values{"filters": []string{`{"foo":{"bar":true}}`}}))
}

func TestFilters_Clone(t *testing.T) {
	f1 := make(Filters).Add("foo", "one")
	f2 := f1.Clone()

	f2.Add("foo", "f2-extra").Add("f2", "f2-value")
	assert.Check(t, is.DeepEqual(f1, Filters{"foo": {"one": true}}))
	assert.Check(t, is.DeepEqual(f2, Filters{
		"foo": {"one": true, "f2-extra": true},
		"f2":  {"f2-value": true},
	}))

	f1.Add("foo", "f1-extra").Add("f1", "f1-value")
	assert.Check(t, is.DeepEqual(f1, Filters{
		"foo": {"one": true, "f1-extra": true},
		"f1":  {"f1-value": true},
	}))

	assert.Check(t, is.DeepEqual(f2, Filters{
		"foo": {"one": true, "f2-extra": true},
		"f2":  {"f2-value": true},
	}))
}
