//go:build linux

package libnetwork

import (
	"math"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// withEmptyPortConfigTbl swaps in a fresh ingress reference table for the
// duration of the test, since it's process-global state.
func withEmptyPortConfigTbl(t *testing.T) {
	t.Helper()

	portConfigMu.Lock()
	orig := portConfigTbl
	portConfigTbl = map[types.PublishedPort]int{}
	portConfigMu.Unlock()

	t.Cleanup(func() {
		portConfigMu.Lock()
		defer portConfigMu.Unlock()
		portConfigTbl = orig
	})
}

// TestPublishedPortIgnoresTargetPort pins down what the reference table counts:
// the host-port reservation, which says nothing about TargetPort. Two configs
// differing only in TargetPort therefore describe one reservation - if they
// didn't, each would take a first reference and both would try to reserve host
// port 8080.
func TestPublishedPortIgnoresTargetPort(t *testing.T) {
	a, err := publishedPort(&PortConfig{Protocol: ProtocolTCP, PublishedPort: 8080, TargetPort: 80})
	assert.NilError(t, err)
	b, err := publishedPort(&PortConfig{Protocol: ProtocolTCP, PublishedPort: 8080, TargetPort: 8080})
	assert.NilError(t, err)

	assert.Check(t, is.Equal(a, b))
	assert.Check(t, is.Equal(a, types.PublishedPort{Proto: types.TCP, Port: 8080, HostPort: 8080}))

	// A differing protocol is a different reservation.
	udp, err := publishedPort(&PortConfig{Protocol: ProtocolUDP, PublishedPort: 8080, TargetPort: 80})
	assert.NilError(t, err)
	assert.Check(t, a != udp)
}

func TestPublishedPortsRejectsOutOfRange(t *testing.T) {
	_, err := publishedPort(&PortConfig{Protocol: ProtocolTCP, PublishedPort: math.MaxUint16 + 1})
	assert.Check(t, is.ErrorContains(err, "out of range"))

	// The whole set is narrowed before any of it is returned, so a caller is never
	// left with a partial conversion to undo.
	pps, err := publishedPorts([]*PortConfig{
		{Protocol: ProtocolTCP, PublishedPort: 8080, TargetPort: 80},
		{Protocol: ProtocolTCP, PublishedPort: math.MaxUint16 + 1},
	})
	assert.Check(t, is.ErrorContains(err, "out of range"))
	assert.Check(t, is.Len(pps, 0))
}

// TestIngressPortRefsAcrossGenerations covers the case that made the reference
// table's old key wrong. A service updated from `-p 8080:80` to `-p 8080:8080`
// has two bindings alive at once whose port configs differ only in TargetPort;
// they share one host-port reservation, so only the first may publish it and only
// the last release may unpublish it.
func TestIngressPortRefsAcrossGenerations(t *testing.T) {
	withEmptyPortConfigTbl(t)

	ppsA, err := publishedPorts([]*PortConfig{{Protocol: ProtocolTCP, PublishedPort: 8080, TargetPort: 80}})
	assert.NilError(t, err)
	ppsB, err := publishedPorts([]*PortConfig{{Protocol: ProtocolTCP, PublishedPort: 8080, TargetPort: 8080}})
	assert.NilError(t, err)

	want := types.PublishedPort{Proto: types.TCP, Port: 8080, HostPort: 8080}

	// The first generation publishes the reservation.
	assert.Check(t, is.DeepEqual(refIngressPorts(ppsA), []types.PublishedPort{want}))

	// The second finds it already held, so has nothing to plumb - rather than
	// trying to reserve a host port that is already taken and failing.
	assert.Check(t, is.Len(refIngressPorts(ppsB), 0))

	// The old generation draining must not unpublish a port the new one serves.
	assert.Check(t, is.Len(unrefIngressPorts(ppsA), 0))

	// Only the last reference going releases it.
	assert.Check(t, is.DeepEqual(unrefIngressPorts(ppsB), []types.PublishedPort{want}))
	assert.Check(t, is.Len(portConfigTbl, 0))
}

// TestIngressPortRefsRollbackBalanced covers the add path's rollback: it drops a
// reference for every port it took one for, not just the newly-referenced ones,
// so a reservation another service still holds keeps its count.
func TestIngressPortRefsRollbackBalanced(t *testing.T) {
	withEmptyPortConfigTbl(t)

	shared := types.PublishedPort{Proto: types.TCP, Port: 8080, HostPort: 8080}
	fresh := types.PublishedPort{Proto: types.TCP, Port: 9090, HostPort: 9090}

	// Another service already holds 8080.
	assert.Check(t, is.DeepEqual(refIngressPorts([]types.PublishedPort{shared}), []types.PublishedPort{shared}))

	pps := []types.PublishedPort{shared, fresh}
	assert.Check(t, is.DeepEqual(refIngressPorts(pps), []types.PublishedPort{fresh}))

	// Publishing fails, so the caller undoes every reference it took.
	unrefIngressPorts(pps)

	// The other service's reference must survive, and the failed port's must be gone.
	assert.Check(t, is.DeepEqual(portConfigTbl, map[types.PublishedPort]int{shared: 1}))
}

// TestAddIngressPortsRollsBackEveryReference checks that addIngressPorts hands
// the whole set back to unrefIngressPorts on a publish failure, not just the
// subset it was told to plumb. Rolling back only the subset leaks a reference
// on every port some other service already held, pinning a reservation that
// nothing will ever release.
func TestAddIngressPortsRollsBackEveryReference(t *testing.T) {
	withEmptyPortConfigTbl(t)

	shared := &PortConfig{Protocol: ProtocolTCP, PublishedPort: 8080, TargetPort: 80}
	fresh := &PortConfig{Protocol: ProtocolTCP, PublishedPort: 9090, TargetPort: 90}
	sharedPP, err := publishedPort(shared)
	assert.NilError(t, err)

	// Another service already holds 8080, so it isn't among the ports the failing
	// publish is asked to plumb.
	assert.Assert(t, is.DeepEqual(refIngressPorts([]types.PublishedPort{sharedPP}), []types.PublishedPort{sharedPP}))

	// An endpoint that isn't joined to a sandbox can't publish anything, so
	// AddEphemeralPorts fails before it reaches a driver - no root, no live
	// network, no host state.
	gwEP := &Endpoint{network: &Network{ctrlr: &Controller{}}}
	err = addIngressPorts(gwEP, []*PortConfig{shared, fresh})
	assert.Check(t, is.ErrorContains(err, "failed to program ingress ports"))

	// The other service's reference must survive, and the failed port's must be gone.
	assert.Check(t, is.DeepEqual(portConfigTbl, map[types.PublishedPort]int{sharedPP: 1}))
}

// TestUnrefIngressPortsIgnoresUnknown checks that dropping a reference nobody
// holds is a no-op rather than driving the count negative - which would make a
// later release of the same port look like it still had holders.
func TestUnrefIngressPortsIgnoresUnknown(t *testing.T) {
	withEmptyPortConfigTbl(t)

	pp := types.PublishedPort{Proto: types.TCP, Port: 8080, HostPort: 8080}
	assert.Check(t, is.Len(unrefIngressPorts([]types.PublishedPort{pp}), 0))
	assert.Check(t, is.Len(portConfigTbl, 0))
}
