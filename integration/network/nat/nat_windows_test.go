package nat // import "github.com/docker/docker/integration/network/nat"

import (
	"net/netip"
	"testing"

	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/integration/internal/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestWindowsNoDisableIPv4(t *testing.T) {
	ctx := setupTest(t)
	c := testEnv.APIClient()

	_, err := network.Create(ctx, c, "ipv6only",
		network.WithDriver("nat"),
		network.WithIPv4(false),
	)
	// This error message should change to "IPv4 cannot be disabled on Windows"
	// when "--experimental" is no longer required to disable IPv4. But, there's
	// no way to start a second daemon with "--experimental" in Windows CI.
	assert.Check(t, is.ErrorContains(err,
		"IPv4 can only be disabled if experimental features are enabled"))
}

func TestCreateIPv6NATNetwork(t *testing.T) {
	ctx := setupTest(t)
	c := testEnv.APIClient()

	nid, err := network.Create(ctx, c, "ipv6only",
		network.WithDriver("nat"),
		network.WithIPv6(),
	)
	assert.NilError(t, err)

	nw, err := c.NetworkInspect(ctx, nid, networktypes.InspectOptions{})
	assert.NilError(t, err)

	// HNS automatically adds an IPv4 subnet if none is provided, so we should
	// have one IPv4 and one IPv6 here.
	assert.Equal(t, len(nw.IPAM.Config), 2, "nat network should have two subnets")

	var hasIPv6Subnet bool
	for _, subnet := range nw.IPAM.Config {
		if netip.MustParseAddr(subnet.Subnet).Is6() {
			hasIPv6Subnet = true
			break
		}
	}

	assert.Equal(t, hasIPv6Subnet, true)
}
