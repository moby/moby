//go:build linux

package libnetwork

import (
	"math"
	"net"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge"
	"github.com/moby/moby/v2/daemon/libnetwork/iptables"
	"github.com/moby/moby/v2/internal/testutil/netnsutils"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// TestAddIngressPortsRollsBackEveryReference checks that a failed publish drops a
// reference for every port addIngressPorts took one for, not just the
// newly-referenced subset it was told to plumb. Rolling back only the subset leaks
// a reference on every port another service already held, leaving its config in
// portConfigTbl where nothing will ever clean it up.
//
// Only an iptables failure can get the add this far - the userland-proxy plumbing
// that follows can't fail the call - so the failure is injected with a published
// port iptables won't accept.
func TestAddIngressPortsRollsBackEveryReference(t *testing.T) {
	skip.If(t, iptables.UsingFirewalld(), "firewalld is running in the host netns, it can't modify rules in the test's netns")
	defer netnsutils.SetupTestOSContext(t)()

	portConfigMu.Lock()
	orig := portConfigTbl
	portConfigTbl = map[PortConfig]int{}
	portConfigMu.Unlock()
	t.Cleanup(func() {
		portConfigMu.Lock()
		defer portConfigMu.Unlock()
		portConfigTbl = orig
	})

	// initIngressConfiguration expects the bridge driver to have created
	// DOCKER-FORWARD already, and resolves an output interface for the gateway IP to
	// enable route_localnet on. Nothing here needs a real gateway bridge, so point it
	// at a loopback address and let it find "lo".
	iptable := iptables.GetIptable(iptables.IPv4)
	_, err := iptable.NewChain(bridge.DockerForwardChain, iptables.Filter)
	assert.NilError(t, err)
	assert.NilError(t, netlink.LinkSetUp(&netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: "lo"}}))

	shared := &PortConfig{Protocol: ProtocolTCP, PublishedPort: 8080, TargetPort: 80}
	// iptables rejects a --dport above 65535, so this port can't be programmed.
	unprogrammable := &PortConfig{Protocol: ProtocolTCP, PublishedPort: math.MaxUint16 + 1, TargetPort: 90}

	// Another service already holds 8080, so it isn't among the ports the failing
	// publish is asked to plumb.
	assert.Assert(t, is.Len(filterPortConfigs([]*PortConfig{shared}, false), 1))

	err = addIngressPorts(net.ParseIP("127.0.0.1"), []*PortConfig{shared, unprogrammable})
	assert.Check(t, is.ErrorContains(err, "failed to program ingress ports"))

	// The other service's reference must survive, and the failed port's must be gone.
	assert.Check(t, is.DeepEqual(portConfigTbl, map[PortConfig]int{*shared: 1}))
}
