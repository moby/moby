//go:build linux

package overlay

import (
	"net/netip"
	"os"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
	"github.com/moby/moby/v2/internal/testutil/netnsutils"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/icmd"
)

// nftTestSetup enables nftables and joins the test to a fresh network
// namespace, mirroring the setup used by the nftables package's own tests.
func nftTestSetup(t *testing.T) {
	t.Helper()
	if err := nftables.Enable(); err != nil {
		// If this is not CI, skip. In CI, nft should always be installed.
		if _, ok := os.LookupEnv("CI"); !ok {
			t.Skip("Cannot enable nftables, no 'nft' command in $PATH ?")
		}
		t.Fatalf("Failed to enable nftables: %s", err)
	}
	cleanupContext := netnsutils.SetupTestOSContext(t)
	t.Cleanup(func() {
		cleanupContext()
		nftables.Disable()
	})
}

// TestOverlayEncNftTable pins the nftables ruleset programmed for encrypted
// overlay networks.
//
// The statement order within the enc-out chain is load-bearing. The
// socket-exists predicate has to be evaluated, and the encryption mark cleared,
// before the mark-setting statement is reached: a datagram forged by a local
// process carries a socket association, so it must be diverted before anything
// can mark it for encryption, and it must lose any mark it arrived with.
func TestOverlayEncNftTable(t *testing.T) {
	nftTestSetup(t)

	d := &driver{advertiseAddress: netip.MustParseAddr("192.0.2.1")}
	defer func() {
		if d.overlayEncNftTable.IsValid() {
			assert.Check(t, d.overlayEncNftTable.Close())
		}
	}()

	const vni = 4096
	assert.NilError(t, d.programOverlayEncVNINft(t.Context(), vni, true))

	res := icmd.RunCommand("nft", "list", "table", "ip", nftOverlayTable)
	res.Assert(t, icmd.Success)
	golden.Assert(t, res.Combined(), t.Name()+"/encrypted.golden")

	// Marking the network as unencrypted removes its VNI from the set, but
	// leaves the ruleset in place for any other encrypted network.
	assert.NilError(t, d.programOverlayEncVNINft(t.Context(), vni, false))

	res = icmd.RunCommand("nft", "list", "table", "ip", nftOverlayTable)
	res.Assert(t, icmd.Success)
	golden.Assert(t, res.Combined(), t.Name()+"/cleared.golden")
}
