package libnetwork

import (
	"net/netip"
	"slices"
	"testing"

	"gotest.tools/v3/assert"
)

func TestEndpointEvent_EquivalentTo(t *testing.T) {
	assert.Check(t, (&endpointEvent{}).EquivalentTo(&endpointEvent{}))

	a := endpointEvent{
		EndpointRecord: EndpointRecord{
			Name:        "foo",
			ServiceName: "bar",
			ServiceID:   "baz",
			IngressPorts: []*PortConfig{
				{
					Protocol:   ProtocolTCP,
					TargetPort: 80,
				},
				{
					Name:          "dns",
					Protocol:      ProtocolUDP,
					TargetPort:    5353,
					PublishedPort: 53,
				},
			},
		},
		VirtualIP:  netip.MustParseAddr("10.0.0.42"),
		EndpointIP: netip.MustParseAddr("192.168.69.42"),
	}
	assert.Check(t, a.EquivalentTo(&a))

	reflexiveEquiv := func(a, b *endpointEvent) bool {
		t.Helper()
		assert.Check(t, a.EquivalentTo(b) == b.EquivalentTo(a), "reflexive equivalence")
		return a.EquivalentTo(b)
	}

	b := a
	b.ServiceDisabled = true
	assert.Check(t, !reflexiveEquiv(&a, &b), "differing by ServiceDisabled")

	c := a
	c.IngressPorts = slices.Clone(a.IngressPorts)
	slices.Reverse(c.IngressPorts)
	assert.Check(t, reflexiveEquiv(&a, &c), "IngressPorts order should not matter")

	d := a
	d.IngressPorts = append(d.IngressPorts, a.IngressPorts[0])
	assert.Check(t, !reflexiveEquiv(&a, &d), "Differing number of copies of IngressPort entries should not be equivalent")
	d.IngressPorts = a.IngressPorts[:1]
	assert.Check(t, !reflexiveEquiv(&a, &d), "Removing an IngressPort entry should not be equivalent")

	e := a
	e.Aliases = []string{"alias1", "alias2"}
	assert.Check(t, !reflexiveEquiv(&a, &e), "Differing Aliases should not be equivalent")

	f := a
	f.TaskAliases = []string{"taskalias1", "taskalias2"}
	assert.Check(t, !reflexiveEquiv(&a, &f), "Adding TaskAliases should not be equivalent")
	g := a
	g.TaskAliases = []string{"taskalias2", "taskalias1"}
	assert.Check(t, reflexiveEquiv(&f, &g), "TaskAliases order should not matter")
	g.TaskAliases = g.TaskAliases[:1]
	assert.Check(t, !reflexiveEquiv(&f, &g), "Differing number of TaskAliases should not be equivalent")

	h := a
	h.EndpointIP = netip.MustParseAddr("192.168.69.43")
	assert.Check(t, !reflexiveEquiv(&a, &h), "Differing EndpointIP should not be equivalent")

	i := a
	i.VirtualIP = netip.MustParseAddr("10.0.0.69")
	assert.Check(t, !reflexiveEquiv(&a, &i), "Differing VirtualIP should not be equivalent")

	j := a
	j.ServiceID = "qux"
	assert.Check(t, !reflexiveEquiv(&a, &j), "Differing ServiceID should not be equivalent")

	k := a
	k.ServiceName = "quux"
	assert.Check(t, !reflexiveEquiv(&a, &k), "Differing ServiceName should not be equivalent")

	l := a
	l.Name = "aaaaa"
	assert.Check(t, !reflexiveEquiv(&a, &l), "Differing Name should not be equivalent")
}
