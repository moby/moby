package network

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
	ctr "github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestCreateWithMultiNetworks(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.44"), "requires API v1.44")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	network.CreateNoError(ctx, t, apiClient, "testnet1")
	defer network.RemoveNoError(ctx, t, apiClient, "testnet1")

	network.CreateNoError(ctx, t, apiClient, "testnet2")
	defer network.RemoveNoError(ctx, t, apiClient, "testnet2")

	attachCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	res := ctr.RunAttach(attachCtx, t, apiClient,
		ctr.WithCmd("ip", "-o", "-4", "addr", "show"),
		ctr.WithNetworkMode("testnet1"),
		ctr.WithEndpointSettings("testnet1", &networktypes.EndpointSettings{}),
		ctr.WithEndpointSettings("testnet2", &networktypes.EndpointSettings{}))

	assert.Equal(t, res.ExitCode, 0)
	assert.Equal(t, res.Stderr.String(), "")

	// Only interfaces with an IPv4 address are printed by iproute2 when flag -4 is specified. Here, we should have two
	// interfaces for testnet1 and testnet2, plus lo.
	ifacesWithAddress := strings.Count(res.Stdout.String(), "\n")
	assert.Equal(t, ifacesWithAddress, 3)
}

func TestCreateWithIPv6DefaultsToULAPrefix(t *testing.T) {
	// On Windows, network creation fails with this error message: Error response from daemon: this request is not supported by the 'windows' ipam driver
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	const nwName = "testnetula"
	network.CreateNoError(ctx, t, apiClient, nwName, network.WithIPv6())
	defer network.RemoveNoError(ctx, t, apiClient, nwName)

	nw, err := apiClient.NetworkInspect(ctx, "testnetula", networktypes.InspectOptions{})
	assert.NilError(t, err)

	for _, ipam := range nw.IPAM.Config {
		ipr := netip.MustParsePrefix(ipam.Subnet)
		if netip.MustParsePrefix("fd00::/8").Overlaps(ipr) {
			return
		}
	}

	t.Fatalf("Network %s has no ULA prefix, expected one.", nwName)
}

func TestCreateWithIPv6WithoutEnableIPv6Flag(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows") // d.Start fails on Windows with `protocol not available`
	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "-D", "--default-network-opt=bridge=com.docker.network.enable_ipv6=true")
	defer d.Stop(t)

	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	const nwName = "testnetula"
	network.CreateNoError(ctx, t, apiClient, nwName)
	defer network.RemoveNoError(ctx, t, apiClient, nwName)

	nw, err := apiClient.NetworkInspect(ctx, "testnetula", networktypes.InspectOptions{})
	assert.NilError(t, err)

	for _, ipam := range nw.IPAM.Config {
		ipr := netip.MustParsePrefix(ipam.Subnet)
		if netip.MustParsePrefix("fd00::/8").Overlaps(ipr) {
			return
		}
	}

	t.Fatalf("Network %s has no ULA prefix, expected one.", nwName)
}

// Check that it's possible to create IPv6 networks with a 64-bit ip-range,
// in 64-bit and bigger subnets, with and without a gateway.
func Test64BitIPRange(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "no bridge or IPv6 on Windows")
	ctx := setupTest(t)
	c := testEnv.APIClient()

	type kv struct{ k, v string }
	subnets := []kv{
		{"64-bit-subnet", "fd2e:b68c:ce26::/64"},
		{"56-bit-subnet", "fd2e:b68c:ce26::/56"},
	}
	ipRanges := []kv{
		{"no-range", ""},
		{"64-bit-range", "fd2e:b68c:ce26::/64"},
	}
	gateways := []kv{
		{"no-gateway", ""},
		{"with-gateway", "fd2e:b68c:ce26::1"},
	}

	for _, sn := range subnets {
		for _, ipr := range ipRanges {
			for _, gw := range gateways {
				ipamSetter := network.WithIPAMRange(sn.v, ipr.v, gw.v)
				t.Run(sn.k+"/"+ipr.k+"/"+gw.k, func(t *testing.T) {
					ctx := testutil.StartSpan(ctx, t)
					const netName = "test64br"
					network.CreateNoError(ctx, t, c, netName, network.WithIPv6(), ipamSetter)
					defer network.RemoveNoError(ctx, t, c, netName)
				})
			}
		}
	}
}

// Demonstrate a limitation of the IP address allocator, it can't
// allocate the last address in range that ends on a 64-bit boundary.
func TestIPRangeAt64BitLimit(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "no bridge or IPv6 on Windows")
	ctx := setupTest(t)
	c := testEnv.APIClient()

	testcases := []struct {
		name       string
		subnet     string
		ipRange    string
		expCtrFail bool
	}{
		{
			name:    "ipRange before end of 64-bit subnet",
			subnet:  "fda9:8d04:086e::/64",
			ipRange: "fda9:8d04:086e::ffff:ffff:ffff:ff0e/127",
		},
		{
			name:    "ipRange at end of 64-bit subnet",
			subnet:  "fda9:8d04:086e::/64",
			ipRange: "fda9:8d04:086e::ffff:ffff:ffff:fffe/127",
			// FIXME(robmry) - there should be two addresses available for
			//  allocation, just like the previous test. One for the gateway
			//  and one for the container. But, because the Bitmap in the
			//  allocator can't cope with a range that includes MaxUint64,
			//  only one address is currently available - so the container
			//  will not start.
			expCtrFail: true,
		},
		{
			name:    "ipRange at 64-bit boundary inside 56-bit subnet",
			subnet:  "fda9:8d04:086e::/56",
			ipRange: "fda9:8d04:086e:aa:ffff:ffff:ffff:fffe/127",
			// FIXME(robmry) - same issue as above, but this time the ip-range
			//  is in the middle of the subnet (on a 64-bit boundary) rather
			//  than at the top end.
			expCtrFail: true,
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			const netName = "test64bl"
			network.CreateNoError(ctx, t, c, netName,
				network.WithIPv6(),
				network.WithIPAMRange(tc.subnet, tc.ipRange, ""),
			)
			defer network.RemoveNoError(ctx, t, c, netName)

			id := ctr.Create(ctx, t, c, ctr.WithNetworkMode(netName))
			defer c.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})
			err := c.ContainerStart(ctx, id, containertypes.StartOptions{})
			if tc.expCtrFail {
				assert.Assert(t, err != nil)
				t.Skipf("XFAIL Container startup failed with error: %v", err)
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
