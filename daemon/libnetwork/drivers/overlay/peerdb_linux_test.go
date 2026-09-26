package overlay

import (
	"errors"
	"fmt"
	"net/netip"
	"syscall"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/internal/countmap"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/hashable"
	"github.com/moby/moby/v2/daemon/libnetwork/nlwrap"
	"github.com/moby/moby/v2/daemon/libnetwork/osl"
	"github.com/moby/moby/v2/internal/testutil/netnsutils"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// The remote peer used by the tests below, and the VTEPs of two nodes it can
// be placed on. testPeerMAC is the MAC the overlay derives from testPeerIP.
var (
	testPeerIP  = netip.MustParsePrefix("10.0.1.17/24")
	testPeerMAC = hashable.MACAddrFrom6([6]byte{0x02, 0x42, 0x0a, 0x00, 0x01, 0x11})
	testVTEPA   = netip.MustParseAddr("192.0.2.10")
	testVTEPB   = netip.MustParseAddr("192.0.2.20")
)

// newTestPeerNetwork returns an overlay network with one subnet and its
// sandbox, as it is once a local endpoint has joined, and a function that
// removes the sandbox. The caller must hold n.mu while calling peerAdd and
// peerDelete, and must call the returned function before the test OS context
// is torn down, as the sandbox is removed through that context.
func newTestPeerNetwork(t *testing.T, secure bool) (*network, *subnet, func()) {
	t.Helper()
	d := &driver{
		advertiseAddress: netip.MustParseAddr("192.0.2.1"),
		networks:         networkTable{},
		secMap:           encrMap{},
	}
	if secure {
		d.keys = []*key{{value: make([]byte, 16), tag: 1}}
	}
	s := &subnet{
		subnetIP: netip.MustParsePrefix("10.0.1.0/24"),
		gwIP:     netip.MustParsePrefix("10.0.1.1/24"),
		vni:      4242,
	}
	n := &network{
		id:        "peerdbtestnetwork",
		driver:    d,
		endpoints: endpointTable{},
		subnets:   []*subnet{s},
		fdbCnt:    countmap.Map[hashable.IPMAC]{},
		secure:    secure,
	}
	d.networks[n.id] = n
	n.mu.Lock()
	defer n.mu.Unlock()
	assert.NilError(t, n.joinSandbox(s, true))
	return n, s, func() {
		n.mu.Lock()
		defer n.mu.Unlock()
		n.leaveSandbox()
	}
}

// peerKernelState reads what the kernel holds for testPeerIP and testPeerMAC
// on the subnet's VXLAN device.
type peerKernelState struct {
	nlh  nlwrap.Handle
	link int
}

// newPeerKernelState opens a netlink handle in the sandbox of n and looks up
// the VXLAN device of subnet s in it. The handle is closed when the test ends.
func newPeerKernelState(t *testing.T, n *network, s *subnet) peerKernelState {
	t.Helper()
	nsh, err := netns.GetFromPath(n.sbox.Key())
	assert.NilError(t, err)
	t.Cleanup(func() { nsh.Close() })
	nlh, err := nlwrap.NewHandleAt(nsh)
	assert.NilError(t, err)
	t.Cleanup(nlh.Close)
	var vxlanName string
	for _, iface := range n.sbox.Interfaces() {
		if iface.SrcName() == s.vxlanName {
			vxlanName = iface.DstName()
		}
	}
	link, err := nlh.LinkByName(vxlanName)
	assert.NilError(t, err)
	return peerKernelState{nlh: nlh, link: link.Attrs().Index}
}

// neighMAC returns the MAC of the IP neighbor entry for testPeerIP, or "" if
// there is none.
func (k peerKernelState) neighMAC(t *testing.T) string {
	t.Helper()
	neighs, err := k.nlh.NeighList(k.link, syscall.AF_INET)
	assert.NilError(t, err)
	for _, nh := range neighs {
		if nh.IP.Equal(testPeerIP.Addr().AsSlice()) {
			return nh.HardwareAddr.String()
		}
	}
	return ""
}

// fdbVTEPs returns the VTEPs of the permanent FDB entries for testPeerMAC.
func (k peerKernelState) fdbVTEPs(t *testing.T) []string {
	t.Helper()
	neighs, err := k.nlh.NeighList(k.link, syscall.AF_BRIDGE)
	assert.NilError(t, err)
	var vteps []string
	for _, nh := range neighs {
		if nh.IP == nil || nh.HardwareAddr.String() != testPeerMAC.String() || nh.State != netlink.NUD_PERMANENT {
			continue
		}
		vteps = append(vteps, nh.IP.String())
	}
	return vteps
}

// TestPeerAddAfterIncompletePeerDelete reproduces an IP being reassigned to a
// peer on another node after the previous peer with that IP was removed only
// partially, leaving its IP neighbor entry in the kernel.
func TestPeerAddAfterIncompletePeerDelete(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(fmt.Sprintf("secure=%t", secure), func(t *testing.T) {
			testPeerAddAfterIncompletePeerDelete(t, secure)
		})
	}
}

// testPeerAddAfterIncompletePeerDelete runs the scenario of
// [TestPeerAddAfterIncompletePeerDelete] on an unencrypted or, if secure is
// set, an encrypted network.
func testPeerAddAfterIncompletePeerDelete(t *testing.T, secure bool) {
	defer netnsutils.SetupTestOSContext(t)()
	n, s, leave := newTestPeerNetwork(t, secure)
	defer leave()
	k := newPeerKernelState(t, n, s)
	n.mu.Lock()
	defer n.mu.Unlock()

	assert.NilError(t, n.peerAdd("ep-a", testPeerIP, testPeerMAC, testVTEPA))
	assert.Check(t, is.DeepEqual(k.fdbVTEPs(t), []string{testVTEPA.String()}))

	// The permanent FDB entry is gone by the time the peer is removed, as
	// when the entry learned by the VXLAN device took its place and then aged
	// out. peerDelete stops at the missing FDB entry.
	err := k.nlh.NeighDel(&netlink.Neigh{
		LinkIndex:    k.link,
		Family:       syscall.AF_BRIDGE,
		Flags:        netlink.NTF_SELF,
		State:        netlink.NUD_PERMANENT,
		IP:           testVTEPA.AsSlice(),
		HardwareAddr: testPeerMAC.AsSlice(),
	})
	assert.NilError(t, err)
	assert.NilError(t, n.peerDelete("ep-a", testPeerIP, testPeerMAC, testVTEPA))

	// The IP is assigned to a peer on another node.
	assert.NilError(t, n.peerAdd("ep-b", testPeerIP, testPeerMAC, testVTEPB))
	assert.Check(t, is.Equal(k.neighMAC(t), testPeerMAC.String()))
	assert.Check(t, is.DeepEqual(k.fdbVTEPs(t), []string{testVTEPB.String()}))
	if secure {
		// One peer per node, so one reference to each node's IPsec state.
		_, ok := n.driver.secMap[testVTEPA]
		assert.Check(t, !ok, "encryption state for %s left behind", testVTEPA)
		assert.Check(t, is.Equal(n.driver.secMap[testVTEPB].count, 1))
	}
}

// TestPeerSharedIPOlderDeletedFirst covers the usual transient case of a
// redeploy: the add of the new peer arrives before the delete of the old peer
// with the same IP.
func TestPeerSharedIPOlderDeletedFirst(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	n, s, leave := newTestPeerNetwork(t, false)
	defer leave()
	k := newPeerKernelState(t, n, s)
	n.mu.Lock()
	defer n.mu.Unlock()

	assert.NilError(t, n.peerAdd("ep-a", testPeerIP, testPeerMAC, testVTEPA))
	assert.NilError(t, n.peerAdd("ep-b", testPeerIP, testPeerMAC, testVTEPB))
	assert.Check(t, is.DeepEqual(k.fdbVTEPs(t), []string{testVTEPA.String()}))

	assert.NilError(t, n.peerDelete("ep-a", testPeerIP, testPeerMAC, testVTEPA))
	assert.Check(t, is.Equal(k.neighMAC(t), testPeerMAC.String()))
	assert.Check(t, is.DeepEqual(k.fdbVTEPs(t), []string{testVTEPB.String()}))

	assert.NilError(t, n.peerDelete("ep-b", testPeerIP, testPeerMAC, testVTEPB))
	assert.Check(t, is.Equal(k.neighMAC(t), ""))
	assert.Check(t, is.Len(k.fdbVTEPs(t), 0))
}

// TestPeerAddRepeated covers an add event delivered again for a peer that is
// already programmed: its own neighbor entry is not taken for a stale one, and
// a single delete removes everything the add programmed.
func TestPeerAddRepeated(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	n, s, leave := newTestPeerNetwork(t, false)
	defer leave()
	k := newPeerKernelState(t, n, s)
	n.mu.Lock()
	defer n.mu.Unlock()

	assert.NilError(t, n.peerAdd("ep-a", testPeerIP, testPeerMAC, testVTEPA))
	err := n.peerAdd("ep-a", testPeerIP, testPeerMAC, testVTEPA)
	assert.Check(t, errors.As(err, &osl.NeighborSearchError{}), "unexpected error: %v", err)
	assert.Check(t, is.Equal(n.fdbCnt[hashable.IPMACFrom(testVTEPA, testPeerMAC)], 1))

	assert.NilError(t, n.peerDelete("ep-a", testPeerIP, testPeerMAC, testVTEPA))
	assert.Check(t, is.Equal(k.neighMAC(t), ""))
	assert.Check(t, is.Len(k.fdbVTEPs(t), 0))
	assert.Check(t, is.Len(n.fdbCnt, 0))
}
