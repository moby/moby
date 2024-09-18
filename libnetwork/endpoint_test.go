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
