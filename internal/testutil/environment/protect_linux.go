package environment

import (
	"context"
	"errors"
	"maps"
	"net"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge"
	"github.com/moby/moby/v2/daemon/libnetwork/nlwrap"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
)

type defaultBridgeInfo struct {
	bridge netlink.Link
	addrs  map[string]*netlink.Addr
}

var _, llSubnet, _ = net.ParseCIDR("fe80::/64")

// ProtectDefaultBridge remembers default bridge settings so that, when a test
// runs its own daemon and tramples settings of the bridge belonging to the
// CI-started bridge, the bridge is restored to its old state before the next
// test.
//
// For example, a test may enable IPv6 with a link-local fixed-cidr-v6. That's
// likely to break later tests, even if they also start their own daemon
// (because, in the absence of any specific settings, the daemon learns default
// bridge config from addresses on an existing bridge device).
func ProtectDefaultBridge(_ context.Context, t testing.TB, testEnv *Execution) {
	t.Helper()
	// Find the bridge - there should always be one, belonging to the daemon started by CI.
	br, err := nlwrap.LinkByName(bridge.DefaultBridgeName)
	if err != nil {
		var lnf netlink.LinkNotFoundError
		if !errors.As(err, &lnf) {
			t.Fatal("Getting default bridge before test:", err)
		}
		return
	}
	testEnv.ProtectDefaultBridge(t, &defaultBridgeInfo{
		bridge: br,
		addrs:  getAddrs(t, br),
	})
}

func getAddrs(t testing.TB, br netlink.Link) map[string]*netlink.Addr {
	t.Helper()
	addrs, err := nlwrap.AddrList(br, netlink.FAMILY_ALL)
	assert.NilError(t, err, "Getting default bridge addresses before test")
	addrMap := map[string]*netlink.Addr{}
	for _, addr := range addrs {
		addrMap[addr.IPNet.String()] = &addr
	}
	return addrMap
}

// ProtectDefaultBridge stores default bridge info, to be restored on clean.
func (e *Execution) ProtectDefaultBridge(t testing.TB, info *defaultBridgeInfo) {
	e.protectedElements.defaultBridgeInfo = info
}

func restoreDefaultBridge(t testing.TB, info *defaultBridgeInfo) {
	t.Helper()
	if info == nil {
		return
	}
	// Re-create the bridge if the test was antisocial enough to delete it.
	// Yes, I'm looking at you TestDockerDaemonSuite/TestBuildOnDisabledBridgeNetworkDaemon.
	br, err := nlwrap.LinkByName(bridge.DefaultBridgeName)
	if err != nil {
		var lnf netlink.LinkNotFoundError
		if !errors.As(err, &lnf) {
			t.Fatal("Failed to find default bridge after test:", err)
		}
		err := netlink.LinkAdd(info.bridge)
		assert.NilError(t, err, "Failed to re-create default bridge after test")
		br, err = nlwrap.LinkByName(bridge.DefaultBridgeName)
		assert.NilError(t, err, "Failed to find re-created default bridge after test")
	}
	addrs, err := nlwrap.AddrList(br, netlink.FAMILY_ALL)
	assert.NilError(t, err, "Failed get default bridge addresses after test")
	// Delete addresses the bridge didn't have before the test, apart from IPv6 LL
	// addresses - because the bridge doesn't get a kernel-assigned LL address until
	// the first veth is hooked up and, once that address is deleted, it's not
	// re-added.
	wantAddrs := maps.Clone(info.addrs)
	for _, addr := range addrs {
		if _, ok := wantAddrs[addr.IPNet.String()]; ok {
			delete(wantAddrs, addr.IPNet.String())
		} else if !llSubnet.Contains(addr.IP) {
			err := netlink.AddrDel(br, &netlink.Addr{IPNet: addr.IPNet})
			assert.NilError(t, err, "Failed to remove default bridge address '%s' after test", addr.IPNet.String())
		}
	}
	// Add missing addresses.
	for _, wantAddr := range wantAddrs {
		err = netlink.AddrAdd(br, wantAddr)
		assert.NilError(t, err, "Failed to add default bridge address '%s' after test", wantAddr.IPNet.String())
	}
}
