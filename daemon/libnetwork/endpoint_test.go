package libnetwork

import (
	"encoding/json"
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

func TestEndpointUnmarshalJSONTypeMismatch(t *testing.T) {
	t.Run("non-string name returns an error", func(t *testing.T) {
		data, err := json.Marshal(map[string]any{
			"name": 123,
			"id":   "id1",
		})
		assert.NilError(t, err)

		var ep Endpoint
		assert.Check(t, ep.UnmarshalJSON(data) != nil)
	})

	t.Run("non-string id returns an error", func(t *testing.T) {
		data, err := json.Marshal(map[string]any{
			"name": "ep1",
			"id":   123,
		})
		assert.NilError(t, err)

		var ep Endpoint
		assert.Check(t, ep.UnmarshalJSON(data) != nil)
	})

	t.Run("non-bool disableIPv6 is logged and ignored, not an error", func(t *testing.T) {
		data, err := json.Marshal(map[string]any{
			"name":        "ep1",
			"id":          "id1",
			"disableIPv6": "not-a-bool",
		})
		assert.NilError(t, err)

		var ep Endpoint
		assert.NilError(t, ep.UnmarshalJSON(data))
		assert.Check(t, is.Equal(ep.disableIPv6, false))
	})

	t.Run("non-map generic is logged and ignored, not an error", func(t *testing.T) {
		data, err := json.Marshal(map[string]any{
			"name":    "ep1",
			"id":      "id1",
			"generic": "not-a-map",
		})
		assert.NilError(t, err)

		var ep Endpoint
		assert.NilError(t, ep.UnmarshalJSON(data))
		assert.Check(t, is.Nil(ep.generic))
	})
}

func TestDecodeGenericList(t *testing.T) {
	type sample struct {
		Name string `json:"name"`
	}

	t.Run("happy path", func(t *testing.T) {
		opt := []any{
			map[string]any{"name": "a"},
			map[string]any{"name": "b"},
		}
		list := decodeGenericList[sample]("key", opt)
		assert.Check(t, is.DeepEqual(list, []sample{{Name: "a"}, {Name: "b"}}))
	})

	t.Run("non-slice returns nil", func(t *testing.T) {
		list := decodeGenericList[sample]("key", "not-a-slice")
		assert.Check(t, is.Nil(list))
	})

	t.Run("element not a map stops and returns what was collected so far", func(t *testing.T) {
		opt := []any{
			map[string]any{"name": "a"},
			"not-a-map",
			map[string]any{"name": "c"},
		}
		list := decodeGenericList[sample]("key", opt)
		assert.Check(t, is.DeepEqual(list, []sample{{Name: "a"}}))
	})
}
