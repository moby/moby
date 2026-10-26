package osl

import (
	"net"
	"syscall"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/ns"
	"github.com/moby/moby/v2/internal/testutil/netnsutils"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
)

// TestAddNeighborReplacesLearnedFDBEntry checks that a permanent FDB entry can
// be programmed while the VXLAN device already holds a dynamic entry for the
// same MAC, learned from inbound traffic, and that the permanent entry wins.
func TestAddNeighborReplacesLearnedFDBEntry(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	key, err := newKey(t)
	assert.NilError(t, err)
	n, err := NewSandbox(key, true, false)
	assert.NilError(t, err)
	defer destroyTest(t, n)

	const srcName = "vxtest0"
	err = ns.NlHandle().LinkAdd(&netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{Name: srcName},
		VxlanId:   42,
		Learning:  true,
	})
	assert.NilError(t, err)
	err = n.AddInterface(t.Context(), srcName, "vxlan", "")
	assert.NilError(t, err)

	link, err := n.nlHandle.LinkByName(n.findDst(srcName, false))
	assert.NilError(t, err)

	vtep := net.ParseIP("192.0.2.10")
	mac, err := net.ParseMAC("02:42:0a:00:01:11")
	assert.NilError(t, err)

	fdbEntry := func() netlink.Neigh {
		t.Helper()
		neighs, err := n.nlHandle.NeighList(link.Attrs().Index, syscall.AF_BRIDGE)
		assert.NilError(t, err)
		for _, nh := range neighs {
			if nh.HardwareAddr.String() == mac.String() && nh.IP.Equal(vtep) {
				return nh
			}
		}
		t.Fatalf("no fdb entry for %s via %s", mac, vtep)
		return netlink.Neigh{}
	}

	// The entry the kernel writes when a frame from mac arrives via vtep.
	err = n.nlHandle.NeighAdd(&netlink.Neigh{
		LinkIndex:    link.Attrs().Index,
		Family:       syscall.AF_BRIDGE,
		Flags:        netlink.NTF_SELF,
		State:        netlink.NUD_REACHABLE,
		IP:           vtep,
		HardwareAddr: mac,
	})
	assert.NilError(t, err)
	assert.Equal(t, fdbEntry().State, netlink.NUD_REACHABLE)

	err = n.AddNeighbor(vtep, mac, WithLinkName(srcName), WithFamily(syscall.AF_BRIDGE))
	assert.NilError(t, err)
	assert.Equal(t, fdbEntry().State, netlink.NUD_PERMANENT)
}
