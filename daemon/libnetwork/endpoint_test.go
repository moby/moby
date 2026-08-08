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

func TestEndpointUnmarshalJSONMalformedDNSNamesTriggersMigration(t *testing.T) {
	data, err := json.Marshal(map[string]any{
		"name":      "ep1",
		"id":        "id1",
		"dnsNames":  123, // malformed: not a []string
		"myAliases": []string{"alias1"},
	})
	assert.NilError(t, err)

	var ep Endpoint
	assert.NilError(t, ep.UnmarshalJSON(data))
	// A malformed dnsNames value is treated the same as a missing one, so
	// the pre-v25.0 migration path repopulates it from myAliases instead of
	// leaving it empty.
	assert.Check(t, is.DeepEqual(ep.dnsNames, []string{"ep1", "alias1"}))
}
