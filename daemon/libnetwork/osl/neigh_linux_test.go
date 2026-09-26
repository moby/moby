package osl

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/ns"
	"github.com/moby/moby/v2/internal/testutil/netnsutils"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
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
	err = n.AddInterface(context.Background(), srcName, "vxlan", "")
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

// TestAddNeighborWithReplace checks that an IP neighbor entry already present
// is refused by default and replaced when the entry is added WithReplace.
func TestAddNeighborWithReplace(t *testing.T) {
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
	})
	assert.NilError(t, err)
	err = n.AddInterface(context.Background(), srcName, "vxlan", "")
	assert.NilError(t, err)

	link, err := n.nlHandle.LinkByName(n.findDst(srcName, false))
	assert.NilError(t, err)

	ip := net.ParseIP("10.0.1.17")
	oldMAC, err := net.ParseMAC("02:42:0a:00:01:99")
	assert.NilError(t, err)
	newMAC, err := net.ParseMAC("02:42:0a:00:01:11")
	assert.NilError(t, err)

	neighMAC := func() string {
		t.Helper()
		neighs, err := n.nlHandle.NeighList(link.Attrs().Index, syscall.AF_INET)
		assert.NilError(t, err)
		for _, nh := range neighs {
			if nh.IP.Equal(ip) {
				return nh.HardwareAddr.String()
			}
		}
		return ""
	}

	assert.NilError(t, n.AddNeighbor(ip, oldMAC, WithLinkName(srcName)))

	err = n.AddNeighbor(ip, newMAC, WithLinkName(srcName))
	assert.Check(t, errors.As(err, &NeighborSearchError{}), "unexpected error: %v", err)
	assert.Check(t, is.Equal(neighMAC(), oldMAC.String()))

	assert.NilError(t, n.AddNeighbor(ip, newMAC, WithLinkName(srcName), WithReplace()))
	assert.Check(t, is.Equal(neighMAC(), newMAC.String()))
}
