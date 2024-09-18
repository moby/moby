package netutils

import (
	"bytes"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/libnetwork/internal/netiputil"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// Test veth name generation "veth"+rand (e.g.veth0f60e2c)
func TestGenerateRandomName(t *testing.T) {
	const vethPrefix = "veth"
	const vethLen = len(vethPrefix) + 7

	testCases := []struct {
		prefix string
		length int
		error  bool
	}{
		{vethPrefix, -1, true},
		{vethPrefix, 0, true},
		{vethPrefix, len(vethPrefix) - 1, true},
		{vethPrefix, len(vethPrefix), true},
		{vethPrefix, len(vethPrefix) + 1, false},
		{vethPrefix, 255, false},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("prefix=%s/length=%d", tc.prefix, tc.length), func(t *testing.T) {
			name, err := GenerateRandomName(tc.prefix, tc.length)
			if tc.error {
				assert.Check(t, is.ErrorContains(err, "invalid length"))
			} else {
				assert.NilError(t, err)
				assert.Check(t, strings.HasPrefix(name, tc.prefix), "Expected name to start with %s", tc.prefix)
				assert.Check(t, is.Equal(len(name), tc.length), "Expected %d characters, instead received %d characters", tc.length, len(name))
			}
		})
	}

	var randomNames [16]string
	for i := range randomNames {
		randomName, err := GenerateRandomName(vethPrefix, vethLen)
		assert.NilError(t, err)

		for _, oldName := range randomNames {
			if randomName == oldName {
				t.Fatalf("Duplicate random name generated: %s", randomName)
			}
		}

		randomNames[i] = randomName
	}
}

// Test mac generation.
func TestUtilGenerateRandomMAC(t *testing.T) {
	mac1 := GenerateRandomMAC()
	mac2 := GenerateRandomMAC()
	// ensure bytes are unique
	if bytes.Equal(mac1, mac2) {
		t.Fatalf("mac1 %s should not equal mac2 %s", mac1, mac2)
	}
	// existing tests check string functionality so keeping the pattern
	if mac1.String() == mac2.String() {
		t.Fatalf("mac1 %s should not equal mac2 %s", mac1, mac2)
	}
}

func TestInferReservedNetworksV4(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	ifaceID := createInterface(t, "foobar")
	addRoute(t, ifaceID, netlink.SCOPE_LINK, netip.MustParsePrefix("100.0.0.0/24"))
	addRoute(t, ifaceID, netlink.SCOPE_LINK, netip.MustParsePrefix("10.0.0.0/8"))
	addRoute(t, ifaceID, netlink.SCOPE_UNIVERSE, netip.MustParsePrefix("20.0.0.0/8"))

	reserved := InferReservedNetworks(false)
	t.Logf("reserved: %+v", reserved)

	// We don't check the size of 'reserved' here because it also includes
	// nameservers set in /etc/resolv.conf. This file might change from one test
	// env to another, and it'd be unnecessarily complex to set up a mount
	// namespace just to check that. Current implementation uses a function
	// which is properly tested, so everything should be good.
	assert.Check(t, slices.Contains(reserved, netip.MustParsePrefix("100.0.0.0/24")))
	assert.Check(t, slices.Contains(reserved, netip.MustParsePrefix("10.0.0.0/8")))
	assert.Check(t, !slices.Contains(reserved, netip.MustParsePrefix("20.0.0.0/8")))
}

func createInterface(t *testing.T, name string) int {
	t.Helper()

	link := &netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{
			Name: name,
		},
	}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatalf("failed to create interface %s: %v", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	return link.Attrs().Index
}

func addRoute(t *testing.T, linkID int, scope netlink.Scope, prefix netip.Prefix) {
	t.Helper()

	if err := netlink.RouteAdd(&netlink.Route{
		Scope:     scope,
		LinkIndex: linkID,
		Dst:       netiputil.ToIPNet(prefix),
	}); err != nil {
		t.Fatalf("failed to add on-link route %s: %v", prefix, err)
	}
}
