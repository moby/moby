package libnetwork

import (
	"sort"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSortByNetworkType(t *testing.T) {
	nws := []*Network{
		{name: "local2"},
		{name: "ovl2", dynamic: true},
		{name: "local3"},
		{name: "ingress", ingress: true},
		{name: "ovl3", dynamic: true},
		{name: "local1"},
		{name: "ovl1", dynamic: true},
	}
	eps := make([]*Endpoint, 0, len(nws))
	for _, nw := range nws {
		eps = append(eps, &Endpoint{
			name:    "ep-" + nw.name,
			network: nw,
		})
	}
	sort.Sort(ByNetworkType(eps))
	actual := make([]string, 0, len(eps))
	for _, ep := range eps {
		actual = append(actual, ep.name)
	}
	expected := []string{
		"ep-ovl2",
		"ep-ovl3",
		"ep-ovl1",
		"ep-ingress",
		"ep-local2",
		"ep-local3",
		"ep-local1",
	}
	assert.Check(t, is.DeepEqual(actual, expected))
}

func TestUnmarshalJSONField(t *testing.T) {
	type sample struct {
		Name string `json:"name"`
	}

	t.Run("happy path", func(t *testing.T) {
		m := map[string]any{"field": map[string]any{"name": "foo"}}
		var dst sample
		err := unmarshalJSONField(m, "field", &dst)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(dst, sample{Name: "foo"}))
	})

	t.Run("slice destination", func(t *testing.T) {
		m := map[string]any{"field": []any{"a", "b", "c"}}
		var dst []string
		err := unmarshalJSONField(m, "field", &dst)
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(dst, []string{"a", "b", "c"}))
	})

	t.Run("missing key unmarshals into zero value", func(t *testing.T) {
		m := map[string]any{}
		var dst sample
		err := unmarshalJSONField(m, "field", &dst)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(dst, sample{}))
	})

	t.Run("array where struct is expected returns an error", func(t *testing.T) {
		m := map[string]any{"field": []any{"unexpected", "array"}}
		var dst sample
		err := unmarshalJSONField(m, "field", &dst)
		assert.Check(t, err != nil)
	})
}
