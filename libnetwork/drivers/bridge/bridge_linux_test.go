package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"net/netip"
	"os/exec"
	"slices"
	"strconv"
	"testing"

	"github.com/docker/docker/internal/nlwrap"
	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/internal/testutils/storeutils"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/docker/docker/libnetwork/internal/netiputil"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipams/defaultipam"
	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/netutils"
	"github.com/docker/docker/libnetwork/options"
	"github.com/docker/docker/libnetwork/portallocator"
	"github.com/docker/docker/libnetwork/types"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
)

func TestEndpointMarshalling(t *testing.T) {
	ip1, _ := types.ParseCIDR("172.22.0.9/16")
	ip2, _ := types.ParseCIDR("2001:db8::9")
	mac, _ := net.ParseMAC("ac:bd:24:57:66:77")
	e := &bridgeEndpoint{
		id:         "d2c015a1fe5930650cbcd50493efba0500bcebd8ee1f4401a16319f8a567de33",
		nid:        "ee33fbb43c323f1920b6b35a0101552ac22ede960d0e5245e9738bccc68b2415",
		addr:       ip1,
		addrv6:     ip2,
		macAddress: mac,
		srcName:    "veth123456",
		containerConfig: &containerConfiguration{
			ParentEndpoints: []string{"one", "due", "three"},
			ChildEndpoints:  []string{"four", "five", "six"},
		},
		extConnConfig: &connectivityConfiguration{
			ExposedPorts: []types.TransportPort{
				{
					Proto: 6,
					Port:  18,
				},
			},
			PortBindings: []types.PortBinding{
				{
					Proto:       6,
					IP:          net.ParseIP("17210.33.9.56"),
					Port:        18,
					HostPort:    3000,
					HostPortEnd: 14000,
				},
			},
		},
		portMapping: []portBinding{
			{
				PortBinding: types.PortBinding{
					Proto:       17,
					IP:          net.ParseIP("172.33.9.56"),
					Port:        99,
					HostIP:      net.ParseIP("10.10.100.2"),
					HostPort:    9900,
					HostPortEnd: 10000,
				},
			},
			{
				PortBinding: types.PortBinding{
					Proto:       6,
					IP:          net.ParseIP("171.33.9.56"),
					Port:        55,
					HostIP:      net.ParseIP("10.11.100.2"),
					HostPort:    5500,
					HostPortEnd: 55000,
				},
			},
		},
	}

	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}

	ee := &bridgeEndpoint{}
	err = json.Unmarshal(b, ee)
	if err != nil {
		t.Fatal(err)
	}

	if e.id != ee.id || e.nid != ee.nid || e.srcName != ee.srcName || !bytes.Equal(e.macAddress, ee.macAddress) ||
		!types.CompareIPNet(e.addr, ee.addr) || !types.CompareIPNet(e.addrv6, ee.addrv6) ||
		!compareContainerConfig(e.containerConfig, ee.containerConfig) ||
		!compareConnConfig(e.extConnConfig, ee.extConnConfig) {
		t.Fatalf("JSON marsh/unmarsh failed.\nOriginal:\n%#v\nDecoded:\n%#v", e, ee)
	}

	// On restore, the HostPortEnd in portMapping is set to HostPort (so that
	// a different port cannot be selected on live-restore if the original is
	// already in-use). So, fix up portMapping in the original before running
	// the comparison.
	epms := make([]portBinding, len(e.portMapping))
	for i, p := range e.portMapping {
		epms[i] = p
		epms[i].HostPortEnd = epms[i].HostPort
	}
	if !compareBindings(epms, ee.portMapping) {
		t.Fatalf("JSON marsh/unmarsh failed.\nOriginal portMapping:\n%#v\nDecoded portMapping:\n%#v", e, ee)
	}
}

func compareContainerConfig(a, b *containerConfiguration) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a.ParentEndpoints) != len(b.ParentEndpoints) ||
		len(a.ChildEndpoints) != len(b.ChildEndpoints) {
		return false
	}
	for i := 0; i < len(a.ParentEndpoints); i++ {
		if a.ParentEndpoints[i] != b.ParentEndpoints[i] {
			return false
		}
	}
	for i := 0; i < len(a.ChildEndpoints); i++ {
		if a.ChildEndpoints[i] != b.ChildEndpoints[i] {
			return false
		}
	}
	return true
}

func compareConnConfig(a, b *connectivityConfiguration) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a.ExposedPorts) != len(b.ExposedPorts) ||
		len(a.PortBindings) != len(b.PortBindings) {
		return false
	}
	for i := 0; i < len(a.ExposedPorts); i++ {
		if !a.ExposedPorts[i].Equal(&b.ExposedPorts[i]) {
			return false
		}
	}
	for i := 0; i < len(a.PortBindings); i++ {
		if !comparePortBinding(&a.PortBindings[i], &b.PortBindings[i]) {
			return false
		}
	}
	return true
}

// comparePortBinding returns whether the given PortBindings are equal.
func comparePortBinding(p *types.PortBinding, o *types.PortBinding) bool {
	if p == o {
		return true
	}

	if o == nil {
		return false
	}

	if p.Proto != o.Proto || p.Port != o.Port ||
		p.HostPort != o.HostPort || p.HostPortEnd != o.HostPortEnd {
		return false
	}

	if p.IP != nil {
		if !p.IP.Equal(o.IP) {
			return false
		}
	} else {
		if o.IP != nil {
			return false
		}
	}

	if p.HostIP != nil {
		if !p.HostIP.Equal(o.HostIP) {
			return false
		}
	} else {
		if o.HostIP != nil {
			return false
		}
	}

	return true
}

func compareBindings(a, b []portBinding) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if !comparePortBinding(&a[i].PortBinding, &b[i].PortBinding) {
			return false
		}
	}
	return true
}

func TestNetworkConfigurationMarshalling(t *testing.T) {
	nc := &networkConfiguration{
		ID:                    "nid",
		BridgeName:            "bridgename",
		EnableIPv4:            true,
		EnableIPv6:            true,
		EnableIPMasquerade:    true,
		GwModeIPv4:            gwModeRouted,
		GwModeIPv6:            gwModeIsolated,
		EnableICC:             true,
		TrustedHostInterfaces: []string{"foo0", "bar1"},
		InhibitIPv4:           true,
		Mtu:                   1234,
		DefaultBindingIP:      net.ParseIP("192.0.2.1"),
		DefaultBridge:         true,
		HostIPv4:              net.ParseIP("192.0.2.2"),
		HostIPv6:              net.ParseIP("2001:db8::1"),
		ContainerIfacePrefix:  "baz",
	}

	b, err := json.Marshal(nc)
	assert.Assert(t, err)

	nnc := &networkConfiguration{}
	err = json.Unmarshal(b, nnc)
	assert.Assert(t, err)
	assert.Check(t, is.DeepEqual(nnc, nc, cmpopts.IgnoreUnexported(networkConfiguration{})))
}

func getIPv4Data(t *testing.T) []driverapi.IPAMData {
	t.Helper()

	a, _ := defaultipam.NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), nil)
	alloc, err := a.RequestPool(ipamapi.PoolRequest{
		AddressSpace: "LocalDefault",
		Exclude:      netutils.InferReservedNetworks(false),
	})
	assert.NilError(t, err)

	gw, _, err := a.RequestAddress(alloc.PoolID, nil, nil)
	assert.NilError(t, err)

	return []driverapi.IPAMData{{AddressSpace: "LocalDefault", Pool: netiputil.ToIPNet(alloc.Pool), Gateway: gw}}
}

func getIPv6Data(t *testing.T) []driverapi.IPAMData {
	ipd := driverapi.IPAMData{AddressSpace: "full"}
	// There's no default IPv6 address pool, so use an arbitrary unique-local prefix.
	addr, nw, _ := net.ParseCIDR("fdcd:d1b1:99d2:abcd::1/64")
	ipd.Pool = nw
	ipd.Gateway = &net.IPNet{IP: addr, Mask: nw.Mask}
	return []driverapi.IPAMData{ipd}
}

func TestCreateFullOptions(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	d := newDriver(storeutils.NewTempStore(t))

	config := &configuration{
		EnableIPForwarding: true,
		EnableIPTables:     true,
	}

	// Test this scenario: Default gw address does not belong to
	// container network and it's greater than bridge address
	cnw, _ := types.ParseCIDR("172.16.122.0/24")
	bnw, _ := types.ParseCIDR("172.16.0.0/24")
	br, _ := types.ParseCIDR("172.16.0.1/16")
	defgw, _ := types.ParseCIDR("172.16.0.100/16")

	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	if err := d.configure(genericOption); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	netOption := make(map[string]interface{})
	netOption[netlabel.EnableIPv4] = true
	netOption[netlabel.EnableIPv6] = true
	netOption[netlabel.GenericData] = &networkConfiguration{
		BridgeName: DefaultBridgeName,
	}

	ipdList := []driverapi.IPAMData{
		{
			Pool:         bnw,
			Gateway:      br,
			AuxAddresses: map[string]*net.IPNet{DefaultGatewayV4AuxKey: defgw},
		},
	}
	err := d.CreateNetwork(context.Background(), "dummy", netOption, nil, ipdList, getIPv6Data(t))
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	// Verify the IP address allocated for the endpoint belongs to the container network
	epOptions := make(map[string]interface{})
	te := newTestEndpoint(cnw, 10)
	err = d.CreateEndpoint(context.Background(), "dummy", "ep1", te.Interface(), epOptions)
	if err != nil {
		t.Fatalf("Failed to create an endpoint : %s", err.Error())
	}

	if !cnw.Contains(te.Interface().Address().IP) {
		t.Fatalf("endpoint got assigned address outside of container network(%s): %s", cnw.String(), te.Interface().Address())
	}
}

func TestCreateNoConfig(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	d := newDriver(storeutils.NewTempStore(t))
	err := d.configure(nil)
	assert.NilError(t, err)

	netconfig := &networkConfiguration{BridgeName: DefaultBridgeName, EnableIPv4: true}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = netconfig

	if err := d.CreateNetwork(context.Background(), "dummy", genericOption, nil, getIPv4Data(t), nil); err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}
}

func TestCreateFullOptionsLabels(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	d := newDriver(storeutils.NewTempStore(t))

	config := &configuration{
		EnableIPForwarding: true,
	}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	if err := d.configure(genericOption); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	bndIPs := "127.0.0.1"
	testHostIPv4 := "1.2.3.4"
	nwV6s := "2001:db8:2600:2700:2800::/80"
	gwV6s := "2001:db8:2600:2700:2800::25/80"
	nwV6, _ := types.ParseCIDR(nwV6s)
	gwV6, _ := types.ParseCIDR(gwV6s)

	labels := map[string]string{
		BridgeName:         DefaultBridgeName,
		DefaultBridge:      "true",
		EnableICC:          "true",
		EnableIPMasquerade: "true",
		DefaultBindingIP:   bndIPs,
		netlabel.HostIPv4:  testHostIPv4,
	}

	netOption := make(map[string]interface{})
	netOption[netlabel.EnableIPv4] = true
	netOption[netlabel.EnableIPv6] = true
	netOption[netlabel.GenericData] = labels

	ipdList := getIPv4Data(t)
	ipd6List := []driverapi.IPAMData{
		{
			Pool: nwV6,
			AuxAddresses: map[string]*net.IPNet{
				DefaultGatewayV6AuxKey: gwV6,
			},
		},
	}

	err := d.CreateNetwork(context.Background(), "dummy", netOption, nil, ipdList, ipd6List)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	nw, ok := d.networks["dummy"]
	if !ok {
		t.Fatal("Cannot find dummy network in bridge driver")
	}

	if nw.config.BridgeName != DefaultBridgeName {
		t.Fatal("incongruent name in bridge network")
	}

	if !nw.config.EnableIPv4 {
		t.Fatal("incongruent EnableIPv4 in bridge network")
	}

	if !nw.config.EnableIPv6 {
		t.Fatal("incongruent EnableIPv6 in bridge network")
	}

	if !nw.config.EnableICC {
		t.Fatal("incongruent EnableICC in bridge network")
	}

	if !nw.config.EnableIPMasquerade {
		t.Fatal("incongruent EnableIPMasquerade in bridge network")
	}

	bndIP := net.ParseIP(bndIPs)
	if !bndIP.Equal(nw.config.DefaultBindingIP) {
		t.Fatalf("Unexpected: %v", nw.config.DefaultBindingIP)
	}

	hostIP := net.ParseIP(testHostIPv4)
	if !hostIP.Equal(nw.config.HostIPv4) {
		t.Fatalf("Unexpected: %v", nw.config.HostIPv4)
	}

	if !types.CompareIPNet(nw.config.AddressIPv6, nwV6) {
		t.Fatalf("Unexpected: %v", nw.config.AddressIPv6)
	}

	if !gwV6.IP.Equal(nw.config.DefaultGatewayIPv6) {
		t.Fatalf("Unexpected: %v", nw.config.DefaultGatewayIPv6)
	}

	// Check that a MAC address is generated if not already configured.
	te1 := newTestEndpoint(ipdList[0].Pool, 20)
	err = d.CreateEndpoint(context.Background(), "dummy", "ep1", te1.Interface(), map[string]interface{}{})
	assert.NilError(t, err)
	assert.Check(t, is.Len(te1.iface.mac, 6))

	// Check that a configured --mac-address isn't overwritten by a generated address.
	te2 := newTestEndpoint(ipdList[0].Pool, 20)
	const macAddr = "aa:bb:cc:dd:ee:ff"
	te2.iface.mac = netutils.MustParseMAC(macAddr)
	err = d.CreateEndpoint(context.Background(), "dummy", "ep2", te2.Interface(), map[string]interface{}{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(te2.iface.mac.String(), macAddr))
}

func TestCreateVeth(t *testing.T) {
	tests := []struct {
		name                  string
		netnsName             string
		createNetns           bool
		expCreatedInContainer bool
	}{
		{
			name: "host netns",
		},
		{
			name:                  "container netns",
			netnsName:             "testnsctr",
			createNetns:           true,
			expCreatedInContainer: true,
		},
		{
			name:      "netns not created",
			netnsName: "testnsctr",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a "host" network namespace with a netlink handle.
			const hostNsName = "testnshost"
			res := icmd.RunCommand("ip", "netns", "add", hostNsName)
			assert.Assert(t, is.Equal(res.ExitCode, 0))
			defer icmd.RunCommand("ip", "netns", "del", hostNsName)
			nsh, err := netns.GetFromPath("/var/run/netns/" + hostNsName)
			assert.NilError(t, err)
			defer nsh.Close()
			nlh, err := nlwrap.NewHandleAt(nsh)
			assert.NilError(t, err)
			defer nlh.Close()

			netnsPath := ""
			if tc.netnsName != "" {
				netnsPath = "/var/run/netns/" + tc.netnsName
			}
			if tc.createNetns {
				res := icmd.RunCommand("ip", "netns", "add", tc.netnsName)
				assert.Assert(t, is.Equal(res.ExitCode, 0))
				defer icmd.RunCommand("ip", "netns", "del", tc.netnsName)
			}

			const hostIfName = "vethtesth"
			const containerIfName = "vethtestc"
			defer func() {
				// Just in case anything ends up in the host's netns, make sure it doesn't hang around ...
				icmd.RunCommand("ip", "link", "del", hostIfName)
				icmd.RunCommand("ip", "link", "del", containerIfName)
			}()

			iface := &testInterface{netnsPath: netnsPath}
			nlhCtr, err := createVeth(context.Background(), hostIfName, containerIfName, iface, nlh)
			assert.Check(t, err)

			assert.Check(t, is.Equal(iface.createdInContainer, tc.expCreatedInContainer))
			if tc.expCreatedInContainer {
				assert.Check(t, nlhCtr != nil)
				res := icmd.RunCommand("ip", "netns", "exec", hostNsName, "ip", "link", "show", hostIfName)
				assert.Check(t, is.Equal(res.ExitCode, 0))
				res = icmd.RunCommand("ip", "netns", "exec", hostNsName, "ip", "link", "show", containerIfName)
				assert.Check(t, is.Equal(res.ExitCode, 1))
				res = icmd.RunCommand("ip", "netns", "exec", tc.netnsName, "ip", "link", "show", containerIfName)
				assert.Check(t, is.Equal(res.ExitCode, 0))
			} else {
				assert.Check(t, nlhCtr == nil)
				res := icmd.RunCommand("ip", "netns", "exec", hostNsName, "ip", "link", "show", hostIfName)
				assert.Check(t, is.Equal(res.ExitCode, 0))
				res = icmd.RunCommand("ip", "netns", "exec", hostNsName, "ip", "link", "show", containerIfName)
				assert.Check(t, is.Equal(res.ExitCode, 0))
			}
		})
	}
}

func TestCreate(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	d := newDriver(storeutils.NewTempStore(t))

	if err := d.configure(nil); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	netconfig := &networkConfiguration{BridgeName: DefaultBridgeName, EnableIPv4: true}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = netconfig

	if err := d.CreateNetwork(context.Background(), "dummy", genericOption, nil, getIPv4Data(t), nil); err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	err := d.CreateNetwork(context.Background(), "dummy", genericOption, nil, getIPv4Data(t), nil)
	if err == nil {
		t.Fatal("Expected bridge driver to refuse creation of second network with default name")
	}
	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatal("Creation of second network with default name failed with unexpected error type")
	}
}

func TestCreateFail(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	d := newDriver(storeutils.NewTempStore(t))

	if err := d.configure(nil); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	netconfig := &networkConfiguration{BridgeName: "dummy0", DefaultBridge: true}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = netconfig

	if err := d.CreateNetwork(context.Background(), "dummy", genericOption, nil, getIPv4Data(t), nil); err == nil {
		t.Fatal("Bridge creation was expected to fail")
	}
}

func TestCreateMultipleNetworks(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	useStubFirewaller(t)

	d := newDriver(storeutils.NewTempStore(t))

	checkFirewallerNetworks := func() {
		t.Helper()
		fw := d.firewaller.(*firewaller.StubFirewaller)
		got := maps.Clone(fw.Networks)
		for _, wantNw := range d.networks {
			_, ok := got[wantNw.config.BridgeName]
			assert.Check(t, ok, "Rules for bridge %s (nid:%s) have been deleted", wantNw.config.BridgeName, wantNw.id)
			delete(got, wantNw.config.BridgeName)
		}
		assert.Check(t, is.Len(slices.Collect(maps.Keys(got)), 0), "Rules for bridges have not been deleted")
	}

	config := &configuration{
		EnableIPTables: true,
	}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	if err := d.configure(genericOption); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	config1 := &networkConfiguration{BridgeName: "net_test_1", EnableIPv4: true}
	genericOption = make(map[string]interface{})
	genericOption[netlabel.GenericData] = config1
	if err := d.CreateNetwork(context.Background(), "1", genericOption, nil, getIPv4Data(t), nil); err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}
	checkFirewallerNetworks()

	config2 := &networkConfiguration{BridgeName: "net_test_2", EnableIPv4: true}
	genericOption[netlabel.GenericData] = config2
	if err := d.CreateNetwork(context.Background(), "2", genericOption, nil, getIPv4Data(t), nil); err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}
	checkFirewallerNetworks()

	config3 := &networkConfiguration{BridgeName: "net_test_3", EnableIPv4: true}
	genericOption[netlabel.GenericData] = config3
	if err := d.CreateNetwork(context.Background(), "3", genericOption, nil, getIPv4Data(t), nil); err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}
	checkFirewallerNetworks()

	config4 := &networkConfiguration{BridgeName: "net_test_4", EnableIPv4: true}
	genericOption[netlabel.GenericData] = config4
	if err := d.CreateNetwork(context.Background(), "4", genericOption, nil, getIPv4Data(t), nil); err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}
	checkFirewallerNetworks()

	if err := d.DeleteNetwork("1"); err != nil {
		t.Log(err)
	}
	checkFirewallerNetworks()

	if err := d.DeleteNetwork("2"); err != nil {
		t.Log(err)
	}
	checkFirewallerNetworks()

	if err := d.DeleteNetwork("3"); err != nil {
		t.Log(err)
	}
	checkFirewallerNetworks()

	if err := d.DeleteNetwork("4"); err != nil {
		t.Log(err)
	}
	checkFirewallerNetworks()
}

type testInterface struct {
	mac                net.HardwareAddr
	addr               *net.IPNet
	addrv6             *net.IPNet
	srcName            string
	dstPrefix          string
	dstName            string
	createdInContainer bool
	netnsPath          string
}

type testEndpoint struct {
	iface  *testInterface
	gw     net.IP
	gw6    net.IP
	routes []types.StaticRoute
}

func newTestEndpoint(nw *net.IPNet, ordinal byte) *testEndpoint {
	addr := types.GetIPNetCopy(nw)
	addr.IP[len(addr.IP)-1] = ordinal
	return &testEndpoint{iface: &testInterface{addr: addr}}
}

// newTestEndpoint46 is like newTestEndpoint, but assigns an IPv6 address as well as IPv4.
func newTestEndpoint46(nw4, nw6 *net.IPNet, ordinal byte) *testEndpoint {
	addr4 := types.GetIPNetCopy(nw4)
	addr4.IP[len(addr4.IP)-1] = ordinal
	addr6 := types.GetIPNetCopy(nw6)
	addr6.IP[len(addr6.IP)-1] = ordinal
	return &testEndpoint{
		iface: &testInterface{
			addr:   addr4,
			addrv6: addr6,
		},
	}
}

func (te *testEndpoint) Interface() *testInterface {
	return te.iface
}

func (i *testInterface) MacAddress() net.HardwareAddr {
	return i.mac
}

func (i *testInterface) Address() *net.IPNet {
	return i.addr
}

func (i *testInterface) AddressIPv6() *net.IPNet {
	return i.addrv6
}

func (i *testInterface) SetMacAddress(mac net.HardwareAddr) error {
	if i.mac != nil {
		return types.ForbiddenErrorf("endpoint interface MAC address present (%s). Cannot be modified with %s.", i.mac, mac)
	}
	if mac == nil {
		return types.InvalidParameterErrorf("tried to set nil MAC address to endpoint interface")
	}
	i.mac = types.GetMacCopy(mac)
	return nil
}

func (i *testInterface) SetIPAddress(address *net.IPNet) error {
	if address.IP == nil {
		return types.InvalidParameterErrorf("tried to set nil IP address to endpoint interface")
	}
	if address.IP.To4() == nil {
		return setAddress(&i.addrv6, address)
	}
	return setAddress(&i.addr, address)
}

func setAddress(ifaceAddr **net.IPNet, address *net.IPNet) error {
	if *ifaceAddr != nil {
		return types.ForbiddenErrorf("endpoint interface IP present (%s). Cannot be modified with (%s).", *ifaceAddr, address)
	}
	*ifaceAddr = types.GetIPNetCopy(address)
	return nil
}

func (i *testInterface) NetnsPath() string {
	return i.netnsPath
}

func (i *testInterface) SetCreatedInContainer(cic bool) {
	i.createdInContainer = cic
}

func (i *testInterface) SetNames(srcName, dstPrefix, dstName string) error {
	i.srcName = srcName
	i.dstPrefix = dstPrefix
	i.dstName = dstName
	return nil
}

func (te *testEndpoint) InterfaceName() driverapi.InterfaceNameInfo {
	if te.iface != nil {
		return te.iface
	}

	return nil
}

func (te *testEndpoint) SetGateway(gw net.IP) error {
	te.gw = gw
	return nil
}

func (te *testEndpoint) SetGatewayIPv6(gw6 net.IP) error {
	te.gw6 = gw6
	return nil
}

func (te *testEndpoint) AddStaticRoute(destination *net.IPNet, routeType int, nextHop net.IP) error {
	te.routes = append(te.routes, types.StaticRoute{Destination: destination, RouteType: routeType, NextHop: nextHop})
	return nil
}

func (te *testEndpoint) AddTableEntry(tableName string, key string, value []byte) error {
	return nil
}

func (te *testEndpoint) DisableGatewayService() {}

func TestQueryEndpointInfo(t *testing.T) {
	testQueryEndpointInfo(t, true)
}

func TestQueryEndpointInfoHairpin(t *testing.T) {
	testQueryEndpointInfo(t, false)
}

func testQueryEndpointInfo(t *testing.T, ulPxyEnabled bool) {
	defer netnsutils.SetupTestOSContext(t)()
	useStubFirewaller(t)

	d := newDriver(storeutils.NewTempStore(t))
	portallocator.Get().ReleaseAll()

	var proxyBinary string
	var err error
	if ulPxyEnabled {
		proxyBinary, err = exec.LookPath("docker-proxy")
		if err != nil {
			t.Fatalf("failed to lookup userland-proxy binary: %v", err)
		}
	}
	config := &configuration{
		EnableIPTables:      true,
		EnableUserlandProxy: ulPxyEnabled,
		UserlandProxyPath:   proxyBinary,
	}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	if err := d.configure(genericOption); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	netconfig := &networkConfiguration{
		BridgeName: DefaultBridgeName,
		EnableIPv4: true,
		EnableICC:  false,
	}
	genericOption = make(map[string]interface{})
	genericOption[netlabel.GenericData] = netconfig

	ipdList := getIPv4Data(t)
	err = d.CreateNetwork(context.Background(), "net1", genericOption, nil, ipdList, nil)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	sbOptions := make(map[string]interface{})
	sbOptions[netlabel.PortMap] = getPortMapping()

	te := newTestEndpoint(ipdList[0].Pool, 11)
	err = d.CreateEndpoint(context.Background(), "net1", "ep1", te.Interface(), nil)
	if err != nil {
		t.Fatalf("Failed to create an endpoint : %s", err.Error())
	}

	err = d.Join(context.Background(), "net1", "ep1", "sbox", te, nil, sbOptions)
	if err != nil {
		t.Fatalf("Failed to join the endpoint: %v", err)
	}

	err = d.ProgramExternalConnectivity(context.Background(), "net1", "ep1", sbOptions)
	if err != nil {
		t.Fatalf("Failed to program external connectivity: %v", err)
	}

	network, ok := d.networks["net1"]
	if !ok {
		t.Fatalf("Cannot find network %s inside driver", "net1")
	}
	ep := network.endpoints["ep1"]
	data, err := d.EndpointOperInfo(network.id, ep.id)
	if err != nil {
		t.Fatalf("Failed to ask for endpoint operational data:  %v", err)
	}
	pmd, ok := data[netlabel.PortMap]
	if !ok {
		t.Fatal("Endpoint operational data does not contain port mapping data")
	}
	pm, ok := pmd.([]types.PortBinding)
	if !ok {
		t.Fatal("Unexpected format for port mapping in endpoint operational data")
	}
	if len(ep.portMapping) != len(pm) {
		t.Fatal("Incomplete data for port mapping in endpoint operational data")
	}
	for i, pb := range ep.portMapping {
		if !comparePortBinding(&pb.PortBinding, &pm[i]) {
			t.Fatal("Unexpected data for port mapping in endpoint operational data")
		}
	}

	err = d.RevokeExternalConnectivity("net1", "ep1")
	if err != nil {
		t.Fatal(err)
	}

	// release host mapped ports
	err = d.Leave("net1", "ep1")
	if err != nil {
		t.Fatal(err)
	}
}

func getExposedPorts() []types.TransportPort {
	return []types.TransportPort{
		{Proto: types.TCP, Port: 5000},
		{Proto: types.UDP, Port: 400},
		{Proto: types.TCP, Port: 600},
	}
}

func getPortMapping() []types.PortBinding {
	return []types.PortBinding{
		{Proto: types.TCP, Port: 230, HostPort: 23000},
		{Proto: types.UDP, Port: 200, HostPort: 22000},
		{Proto: types.TCP, Port: 120, HostPort: 12000},
	}
}

func TestLinkContainers(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	useStubFirewaller(t)

	d := newDriver(storeutils.NewTempStore(t))

	config := &configuration{
		EnableIPTables: true,
	}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	if err := d.configure(genericOption); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	netconfig := &networkConfiguration{
		BridgeName: DefaultBridgeName,
		EnableIPv4: true,
		EnableICC:  false,
	}
	genericOption = make(map[string]interface{})
	genericOption[netlabel.GenericData] = netconfig

	ipdList := getIPv4Data(t)
	err := d.CreateNetwork(context.Background(), "net1", genericOption, nil, ipdList, nil)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	te1 := newTestEndpoint(ipdList[0].Pool, 11)
	err = d.CreateEndpoint(context.Background(), "net1", "ep1", te1.Interface(), nil)
	if err != nil {
		t.Fatalf("Failed to create an endpoint : %s", err.Error())
	}

	exposedPorts := getExposedPorts()
	sbOptions := make(map[string]interface{})
	sbOptions[netlabel.ExposedPorts] = exposedPorts

	err = d.Join(context.Background(), "net1", "ep1", "sbox", te1, nil, sbOptions)
	if err != nil {
		t.Fatalf("Failed to join the endpoint: %v", err)
	}

	err = d.ProgramExternalConnectivity(context.Background(), "net1", "ep1", sbOptions)
	if err != nil {
		t.Fatalf("Failed to program external connectivity: %v", err)
	}

	addr1 := te1.iface.addr
	if addr1.IP.To4() == nil {
		t.Fatal("No Ipv4 address assigned to the endpoint:  ep1")
	}

	te2 := newTestEndpoint(ipdList[0].Pool, 22)
	err = d.CreateEndpoint(context.Background(), "net1", "ep2", te2.Interface(), nil)
	if err != nil {
		t.Fatalf("Failed to create an endpoint : %s", err.Error())
	}

	addr2 := te2.iface.addr
	if addr2.IP.To4() == nil {
		t.Fatal("No Ipv4 address assigned to the endpoint:  ep2")
	}

	sbOptions = make(map[string]interface{})
	sbOptions[netlabel.GenericData] = options.Generic{
		"ChildEndpoints": []string{"ep1"},
	}

	err = d.Join(context.Background(), "net1", "ep2", "", te2, nil, sbOptions)
	if err != nil {
		t.Fatal("Failed to link ep1 and ep2")
	}

	err = d.ProgramExternalConnectivity(context.Background(), "net1", "ep2", sbOptions)
	if err != nil {
		t.Fatalf("Failed to program external connectivity: %v", err)
	}

	checkLink := func(expExists bool) {
		t.Helper()
		dnw, ok := d.networks["net1"]
		assert.Assert(t, ok)
		fnw := dnw.firewallerNetwork.(*firewaller.StubFirewallerNetwork)
		parentAddr, ok := netip.AddrFromSlice(te2.iface.addr.IP)
		assert.Assert(t, ok)
		childAddr, ok := netip.AddrFromSlice(te1.iface.addr.IP)
		assert.Assert(t, ok)
		exists := fnw.LinkExists(parentAddr, childAddr, exposedPorts)
		assert.Check(t, is.Equal(exists, expExists))
	}
	checkLink(true)

	err = d.RevokeExternalConnectivity("net1", "ep2")
	if err != nil {
		t.Fatalf("Failed to revoke external connectivity: %v", err)
	}
	err = d.Leave("net1", "ep2")
	if err != nil {
		t.Fatal("Failed to unlink ep1 and ep2")
	}
	checkLink(false)

	// Error condition test with an invalid endpoint-id "ep4"
	sbOptions = make(map[string]interface{})
	sbOptions[netlabel.GenericData] = options.Generic{
		"ChildEndpoints": []string{"ep1", "ep4"},
	}

	err = d.Join(context.Background(), "net1", "ep2", "", te2, nil, sbOptions)
	if err != nil {
		t.Fatal(err)
	}
	err = d.ProgramExternalConnectivity(context.Background(), "net1", "ep2", sbOptions)
	assert.Check(t, err != nil, "Expected Join to fail given link conditions are not satisfied")
	checkLink(false)
}

func TestValidateConfig(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	// Bridge network
	_, network, _ := net.ParseCIDR("172.28.0.0/16")
	c := networkConfiguration{
		AddressIPv4: network,
		EnableIPv4:  true,
	}
	err := c.Validate()
	if err != nil {
		t.Fatal("unexpected validation error:", err)
	}

	// Test mtu
	c.Mtu = -2
	err = c.Validate()
	if err == nil {
		t.Fatal("Failed to detect invalid MTU number")
	}
	c.Mtu = 9000
	err = c.Validate()
	if err != nil {
		t.Fatal("unexpected validation error on MTU number:", err)
	}

	err = c.Validate()
	if err != nil {
		t.Fatal(err)
	}

	// Test v4 gw
	c.DefaultGatewayIPv4 = net.ParseIP("172.27.30.234")
	err = c.Validate()
	if err == nil {
		t.Fatal("Failed to detect invalid default gateway")
	}

	c.DefaultGatewayIPv4 = net.ParseIP("172.28.30.234")
	err = c.Validate()
	if err != nil {
		t.Fatal("Unexpected validation error on default gateway")
	}

	// Test v6 gw
	_, v6nw, _ := net.ParseCIDR("2001:db8:ae:b004::/64")
	c = networkConfiguration{
		EnableIPv6:         true,
		AddressIPv6:        v6nw,
		DefaultGatewayIPv6: net.ParseIP("2001:db8:ac:b004::bad:a55"),
	}
	err = c.Validate()
	if err == nil {
		t.Fatal("Failed to detect invalid v6 default gateway")
	}

	c.DefaultGatewayIPv6 = net.ParseIP("2001:db8:ae:b004::bad:a55")
	err = c.Validate()
	if err != nil {
		t.Fatal("Unexpected validation error on v6 default gateway")
	}

	c.AddressIPv6 = nil
	err = c.Validate()
	if err == nil {
		t.Fatal("Failed to detect invalid v6 default gateway")
	}

	c.AddressIPv6 = nil
	err = c.Validate()
	if err == nil {
		t.Fatal("Failed to detect invalid v6 default gateway")
	}
}

func TestValidateFixedCIDRV6(t *testing.T) {
	tests := []struct {
		doc, input, expectedErr string
	}{
		{
			doc:   "valid",
			input: "2001:db8::/32",
		},
		{
			// fixed-cidr-v6 doesn't have to be specified.
			doc: "empty",
		},
		{
			// Using the LL subnet prefix is ok.
			doc:   "Link-Local subnet prefix",
			input: "fe80::/64",
		},
		{
			// Using a nonstandard LL prefix that doesn't overlap with the standard LL prefix
			// is ok.
			doc:   "non-overlapping link-local prefix",
			input: "fe80:1234::/80",
		},
		{
			// Overlapping with the standard LL prefix isn't allowed.
			doc:         "overlapping link-local prefix fe80::/63",
			input:       "fe80::/63",
			expectedErr: "invalid fixed-cidr-v6: 'fe80::/63' clashes with the Link-Local prefix 'fe80::/64'",
		},
		{
			// Overlapping with the standard LL prefix isn't allowed.
			doc:         "overlapping link-local subnet fe80::/65",
			input:       "fe80::/65",
			expectedErr: "invalid fixed-cidr-v6: 'fe80::/65' clashes with the Link-Local prefix 'fe80::/64'",
		},
		{
			// The address has to be valid IPv6 subnet.
			doc:         "invalid IPv6 subnet",
			input:       "2000:db8::",
			expectedErr: "invalid fixed-cidr-v6: netip.ParsePrefix(\"2000:db8::\"): no '/'",
		},
		{
			doc:         "non-IPv6 subnet",
			input:       "10.3.4.5/24",
			expectedErr: "invalid fixed-cidr-v6: '10.3.4.5/24' is not a valid IPv6 subnet",
		},
		{
			doc:         "IPv4-mapped subnet 1",
			input:       "::ffff:10.2.4.0/24",
			expectedErr: "invalid fixed-cidr-v6: '::ffff:10.2.4.0/24' is not a valid IPv6 subnet",
		},
		{
			doc:         "IPv4-mapped subnet 2",
			input:       "::ffff:a01:203/24",
			expectedErr: "invalid fixed-cidr-v6: '::ffff:10.1.2.3/24' is not a valid IPv6 subnet",
		},
		{
			doc:         "invalid subnet",
			input:       "nonsense",
			expectedErr: "invalid fixed-cidr-v6: netip.ParsePrefix(\"nonsense\"): no '/'",
		},
		{
			doc:         "multicast IPv6 subnet",
			input:       "ff05::/64",
			expectedErr: "invalid fixed-cidr-v6: multicast subnet 'ff05::/64' is not allowed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			err := ValidateFixedCIDRV6(tc.input)
			if tc.expectedErr == "" {
				assert.Check(t, err)
			} else {
				assert.Check(t, is.Error(err, tc.expectedErr))
			}
		})
	}
}

func TestSetDefaultGw(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	d := newDriver(storeutils.NewTempStore(t))

	if err := d.configure(nil); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	ipam4 := getIPv4Data(t)
	gw4 := types.GetIPCopy(ipam4[0].Pool.IP).To4()
	gw4[3] = 254
	ipam6 := getIPv6Data(t)
	gw6 := types.GetIPCopy(ipam6[0].Pool.IP)
	gw6[15] = 0x42

	option := map[string]interface{}{
		netlabel.EnableIPv4: true,
		netlabel.EnableIPv6: true,
		netlabel.GenericData: &networkConfiguration{
			BridgeName:         DefaultBridgeName,
			DefaultGatewayIPv4: gw4,
			DefaultGatewayIPv6: gw6,
		},
	}

	err := d.CreateNetwork(context.Background(), "dummy", option, nil, ipam4, ipam6)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	te := newTestEndpoint(ipam4[0].Pool, 10)
	err = d.CreateEndpoint(context.Background(), "dummy", "ep", te.Interface(), nil)
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	err = d.Join(context.Background(), "dummy", "ep", "sbox", te, nil, nil)
	if err != nil {
		t.Fatalf("Failed to join endpoint: %v", err)
	}

	if !gw4.Equal(te.gw) {
		t.Fatalf("Failed to configure default gateway. Expected %v. Found %v", gw4, te.gw)
	}

	if !gw6.Equal(te.gw6) {
		t.Fatalf("Failed to configure default gateway. Expected %v. Found %v", gw6, te.gw6)
	}
}

func TestCreateWithExistingBridge(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	d := newDriver(storeutils.NewTempStore(t))

	if err := d.configure(nil); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	brName := "br111"
	br := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: brName,
		},
	}
	if err := netlink.LinkAdd(br); err != nil {
		t.Fatalf("Failed to create bridge interface: %v", err)
	}
	defer netlink.LinkDel(br)
	if err := netlink.LinkSetUp(br); err != nil {
		t.Fatalf("Failed to set bridge interface up: %v", err)
	}

	ip := net.IP{192, 168, 122, 1}
	addr := &netlink.Addr{IPNet: &net.IPNet{
		IP:   ip,
		Mask: net.IPv4Mask(255, 255, 255, 0),
	}}
	if err := netlink.AddrAdd(br, addr); err != nil {
		t.Fatalf("Failed to add IP address to bridge: %v", err)
	}

	netconfig := &networkConfiguration{BridgeName: brName, EnableIPv4: true}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = netconfig

	ipv4Data := []driverapi.IPAMData{{
		AddressSpace: "full",
		Pool:         types.GetIPNetCopy(addr.IPNet),
		Gateway:      types.GetIPNetCopy(addr.IPNet),
	}}
	// Set network gateway to X.X.X.1
	ipv4Data[0].Gateway.IP[len(ipv4Data[0].Gateway.IP)-1] = 1

	if err := d.CreateNetwork(context.Background(), brName, genericOption, nil, ipv4Data, nil); err != nil {
		t.Fatalf("Failed to create bridge network: %v", err)
	}

	nw, err := d.getNetwork(brName)
	if err != nil {
		t.Fatalf("Failed to getNetwork(%s): %v", brName, err)
	}

	addrs4, err := nw.bridge.addresses(netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("Failed to get the bridge network's address: %v", err)
	}

	if !addrs4[0].IP.Equal(ip) {
		t.Fatal("Creating bridge network with existing bridge interface unexpectedly modified the IP address of the bridge")
	}

	if err := d.DeleteNetwork(brName); err != nil {
		t.Fatalf("Failed to delete network %s: %v", brName, err)
	}

	if _, err := nlwrap.LinkByName(brName); err != nil {
		t.Fatal("Deleting bridge network that using existing bridge interface unexpectedly deleted the bridge interface")
	}
}

func TestCreateParallel(t *testing.T) {
	c := netnsutils.SetupTestOSContextEx(t)
	defer c.Cleanup(t)

	d := newDriver(storeutils.NewTempStore(t))
	portallocator.Get().ReleaseAll()

	if err := d.configure(nil); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	ipV4Data := getIPv4Data(t)

	ch := make(chan error, 100)
	for i := 0; i < 100; i++ {
		name := "net" + strconv.Itoa(i)
		c.Go(t, func() {
			config := &networkConfiguration{BridgeName: name, EnableIPv4: true}
			genericOption := make(map[string]interface{})
			genericOption[netlabel.GenericData] = config
			if err := d.CreateNetwork(context.Background(), name, genericOption, nil, ipV4Data, nil); err != nil {
				ch <- fmt.Errorf("failed to create %s", name)
				return
			}
			if err := d.CreateNetwork(context.Background(), name, genericOption, nil, ipV4Data, nil); err == nil {
				ch <- fmt.Errorf("failed was able to create overlap %s", name)
				return
			}
			ch <- nil
		})
	}
	// wait for the go routines
	var success int
	for i := 0; i < 100; i++ {
		val := <-ch
		if val == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("Success should be 1 instead: %d", success)
	}
}

func useStubFirewaller(t *testing.T) {
	origNewFirewaller := newFirewaller
	newFirewaller = func(_ context.Context, config firewaller.Config) (firewaller.Firewaller, error) {
		return firewaller.NewStubFirewaller(config), nil
	}
	t.Cleanup(func() { newFirewaller = origNewFirewaller })
}

// Regression test for https://github.com/moby/moby/issues/46445
func TestSetupIP6TablesWithHostIPv4(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	d := newDriver(storeutils.NewTempStore(t))
	dc := &configuration{
		EnableIPTables:  true,
		EnableIP6Tables: true,
	}
	if err := d.configure(map[string]interface{}{netlabel.GenericData: dc}); err != nil {
		t.Fatal(err)
	}
	nc := &networkConfiguration{
		BridgeName:         DefaultBridgeName,
		AddressIPv4:        &net.IPNet{IP: net.ParseIP("192.168.42.1"), Mask: net.CIDRMask(16, 32)},
		EnableIPMasquerade: true,
		EnableIPv4:         true,
		EnableIPv6:         true,
		AddressIPv6:        &net.IPNet{IP: net.ParseIP("2001:db8::1"), Mask: net.CIDRMask(64, 128)},
		HostIPv4:           net.ParseIP("192.0.2.2"),
	}

	// Create test bridge.
	nh, err := nlwrap.NewHandle()
	if err != nil {
		t.Fatal(err)
	}
	br := &bridgeInterface{nlh: nh}
	if err := setupDevice(nc, br); err != nil {
		t.Fatalf("Failed to create the testing Bridge: %s", err.Error())
	}
	if err := setupBridgeIPv4(nc, br); err != nil {
		t.Fatalf("Failed to bring up the testing Bridge: %s", err.Error())
	}
	if err := setupBridgeIPv6(nc, br); err != nil {
		t.Fatalf("Failed to bring up the testing Bridge: %s", err.Error())
	}

	// Check firewall configuration succeeds.
	nw := bridgeNetwork{
		config: nc,
		driver: d,
		bridge: br,
	}
	fwn, err := nw.newFirewallerNetwork(context.Background())
	assert.NilError(t, err)
	assert.Check(t, fwn != nil, "no firewaller network")
}
