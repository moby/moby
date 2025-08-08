package bridge

import (
	"context"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/v2/daemon/libnetwork/drvregistry"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"github.com/moby/moby/v2/daemon/libnetwork/nlwrap"
	"github.com/moby/moby/v2/internal/testutils/netnsutils"
	"github.com/moby/moby/v2/internal/testutils/storeutils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestLinkCreate(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	d := newDriver(storeutils.NewTempStore(t), &drvregistry.PortMappers{})
	err := d.configure(nil)
	assert.NilError(t, err)

	mtu := 1490
	option := map[string]any{
		netlabel.GenericData: &networkConfiguration{
			BridgeName: DefaultBridgeName,
			EnableIPv4: true,
			EnableIPv6: true,
			Mtu:        mtu,
		},
	}

	ipdList := getIPv4Data(t)
	ipd6List := getIPv6Data(t)
	err = d.CreateNetwork(context.Background(), "dummy", option, nil, ipdList, ipd6List)
	assert.NilError(t, err, "Failed to create bridge")

	te := newTestEndpoint46(ipdList[0].Pool, ipd6List[0].Pool, 10)
	err = d.CreateEndpoint(context.Background(), "dummy", "", te.Interface(), nil)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.Error(err, "invalid endpoint id: "))

	// Good endpoint creation
	err = d.CreateEndpoint(context.Background(), "dummy", "ep", te.Interface(), nil)
	assert.NilError(t, err)

	err = d.Join(context.Background(), "dummy", "ep", "sbox", te, nil, nil)
	assert.NilError(t, err)
	assert.Assert(t, te.iface.dstPrefix != "", "got: %q, want: %q", te.iface.dstPrefix, "")

	// Verify sbox endpoint interface inherited MTU value from bridge config
	sboxLnk, err := nlwrap.LinkByName(te.iface.srcName)
	assert.NilError(t, err)
	assert.Assert(t, is.Equal(sboxLnk.Attrs().MTU, mtu), "Sandbox endpoint interface did not inherit bridge interface MTU config")

	// TODO: if we could get peer name from (sboxLnk.(*netlink.Veth)).PeerName
	// then we could check the MTU on hostLnk as well.

	te1 := newTestEndpoint(ipdList[0].Pool, 11)
	err = d.CreateEndpoint(context.Background(), "dummy", "ep", te1.Interface(), nil)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsPermissionDenied))
	assert.Assert(t, is.Error(err, "Endpoint (ep) already exists (Only one endpoint allowed)"), "Failed to detect duplicate endpoint id on same network")

	_, err = nlwrap.LinkByName(te.iface.srcName)
	assert.Check(t, err, "Could not find source link %s", te.iface.srcName)

	n, ok := d.networks["dummy"]
	assert.Check(t, ok, "Failed to find dummy network inside driveer")

	ip := te.iface.addr.IP
	assert.Check(t, n.bridge.bridgeIPv4.Contains(ip), "IP %s should be a valid ip in the subnet %s", ip.String(), n.bridge.bridgeIPv4.String())

	ip6 := te.iface.addrv6.IP
	assert.Check(t, n.bridge.bridgeIPv6.Contains(ip6), "IP %s should be a valid ip in the subnet %s", ip6.String(), n.bridge.bridgeIPv6.String())

	assert.Check(t, te.gw.Equal(n.bridge.bridgeIPv4.IP), "Invalid default gateway. Expected %s. Got %s", n.bridge.bridgeIPv4.IP.String(), te.gw.String())
	assert.Check(t, te.gw6.Equal(n.bridge.bridgeIPv6.IP), "Invalid default gateway for IPv6. Expected %s. Got %s", n.bridge.bridgeIPv6.IP.String(), te.gw6.String())
}

func TestLinkCreateTwo(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	d := newDriver(storeutils.NewTempStore(t), &drvregistry.PortMappers{})
	err := d.configure(nil)
	assert.NilError(t, err)

	option := map[string]any{
		netlabel.GenericData: &networkConfiguration{
			BridgeName: DefaultBridgeName,
			EnableIPv4: true,
			EnableIPv6: true,
		},
	}

	ipdList := getIPv4Data(t)
	err = d.CreateNetwork(context.Background(), "dummy", option, nil, ipdList, getIPv6Data(t))
	assert.NilError(t, err, "Failed to create bridge")

	te1 := newTestEndpoint(ipdList[0].Pool, 11)
	err = d.CreateEndpoint(context.Background(), "dummy", "ep", te1.Interface(), nil)
	assert.NilError(t, err)

	te2 := newTestEndpoint(ipdList[0].Pool, 12)
	err = d.CreateEndpoint(context.Background(), "dummy", "ep", te2.Interface(), nil)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsPermissionDenied))
	assert.Assert(t, is.Error(err, "Endpoint (ep) already exists (Only one endpoint allowed)"), "Failed to detect duplicate endpoint id on same network")
}

func TestLinkCreateNoEnableIPv6(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	d := newDriver(storeutils.NewTempStore(t), &drvregistry.PortMappers{})
	err := d.configure(nil)
	assert.NilError(t, err)

	option := map[string]any{
		netlabel.GenericData: &networkConfiguration{
			BridgeName: DefaultBridgeName,
			EnableIPv4: true,
		},
	}

	ipdList := getIPv4Data(t)
	err = d.CreateNetwork(context.Background(), "dummy", option, nil, ipdList, getIPv6Data(t))
	assert.NilError(t, err, "Failed to create bridge")

	te := newTestEndpoint(ipdList[0].Pool, 30)
	err = d.CreateEndpoint(context.Background(), "dummy", "ep", te.Interface(), nil)
	assert.NilError(t, err)

	assert.Check(t, is.Nil(te.iface.addrv6), "Expected IPv6 address to be nil when IPv6 is not enabled, got %s", te.iface.addrv6)
	assert.Check(t, is.Nil(te.gw6), "Expected GatewayIPv6 to be nil when IPv6 is not enabled, got %s", te.gw6)
}

func TestLinkDelete(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	d := newDriver(storeutils.NewTempStore(t), &drvregistry.PortMappers{})
	err := d.configure(nil)
	assert.NilError(t, err)

	option := map[string]any{
		netlabel.GenericData: &networkConfiguration{
			BridgeName: DefaultBridgeName,
			EnableIPv4: true,
			EnableIPv6: true,
		},
	}

	ipdList := getIPv4Data(t)
	err = d.CreateNetwork(context.Background(), "dummy", option, nil, ipdList, getIPv6Data(t))
	assert.NilError(t, err, "Failed to create bridge")

	te := newTestEndpoint(ipdList[0].Pool, 30)
	err = d.CreateEndpoint(context.Background(), "dummy", "ep1", te.Interface(), nil)
	assert.NilError(t, err)

	err = d.DeleteEndpoint("dummy", "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Assert(t, is.Error(err, "invalid endpoint id: "))

	err = d.DeleteEndpoint("dummy", "ep1")
	assert.NilError(t, err)
}
