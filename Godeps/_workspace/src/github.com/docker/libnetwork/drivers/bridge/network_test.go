package bridge

import (
	"testing"

	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/netutils"
	"github.com/vishvananda/netlink"
)

func TestLinkCreate(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	d := newDriver()
	dr := d.(*driver)

	mtu := 1490
	config := &NetworkConfiguration{
		BridgeName: DefaultBridgeName,
		Mtu:        mtu,
		EnableIPv6: true,
	}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	err := d.CreateNetwork("dummy", genericOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	te := &testEndpoint{ifaces: []*testInterface{}}
	err = d.CreateEndpoint("dummy", "", te, nil)
	if err != nil {
		if _, ok := err.(InvalidEndpointIDError); !ok {
			t.Fatalf("Failed with a wrong error :%s", err.Error())
		}
	} else {
		t.Fatalf("Failed to detect invalid config")
	}

	// Good endpoint creation
	err = d.CreateEndpoint("dummy", "ep", te, nil)
	if err != nil {
		t.Fatalf("Failed to create a link: %s", err.Error())
	}

	err = d.Join("dummy", "ep", "sbox", te, nil)
	if err != nil {
		t.Fatalf("Failed to create a link: %s", err.Error())
	}

	// Verify sbox endoint interface inherited MTU value from bridge config
	sboxLnk, err := netlink.LinkByName(te.ifaces[0].srcName)
	if err != nil {
		t.Fatal(err)
	}
	if mtu != sboxLnk.Attrs().MTU {
		t.Fatalf("Sandbox endpoint interface did not inherit bridge interface MTU config")
	}
	// TODO: if we could get peer name from (sboxLnk.(*netlink.Veth)).PeerName
	// then we could check the MTU on hostLnk as well.

	te1 := &testEndpoint{ifaces: []*testInterface{}}
	err = d.CreateEndpoint("dummy", "ep", te1, nil)
	if err == nil {
		t.Fatalf("Failed to detect duplicate endpoint id on same network")
	}

	if len(te.ifaces) != 1 {
		t.Fatalf("Expected exactly one interface. Instead got %d interface(s)", len(te.ifaces))
	}

	if te.ifaces[0].dstName == "" {
		t.Fatal("Invalid Dstname returned")
	}

	_, err = netlink.LinkByName(te.ifaces[0].srcName)
	if err != nil {
		t.Fatalf("Could not find source link %s: %v", te.ifaces[0].srcName, err)
	}

	n := dr.network
	ip := te.ifaces[0].addr.IP
	if !n.bridge.bridgeIPv4.Contains(ip) {
		t.Fatalf("IP %s is not a valid ip in the subnet %s", ip.String(), n.bridge.bridgeIPv4.String())
	}

	ip6 := te.ifaces[0].addrv6.IP
	if !n.bridge.bridgeIPv6.Contains(ip6) {
		t.Fatalf("IP %s is not a valid ip in the subnet %s", ip6.String(), bridgeIPv6.String())
	}

	if !te.gw.Equal(n.bridge.bridgeIPv4.IP) {
		t.Fatalf("Invalid default gateway. Expected %s. Got %s", n.bridge.bridgeIPv4.IP.String(),
			te.gw.String())
	}

	if !te.gw6.Equal(n.bridge.bridgeIPv6.IP) {
		t.Fatalf("Invalid default gateway for IPv6. Expected %s. Got %s", n.bridge.bridgeIPv6.IP.String(),
			te.gw6.String())
	}
}

func TestLinkCreateTwo(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	d := newDriver()

	config := &NetworkConfiguration{
		BridgeName: DefaultBridgeName,
		EnableIPv6: true}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	err := d.CreateNetwork("dummy", genericOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	te1 := &testEndpoint{ifaces: []*testInterface{}}
	err = d.CreateEndpoint("dummy", "ep", te1, nil)
	if err != nil {
		t.Fatalf("Failed to create a link: %s", err.Error())
	}

	te2 := &testEndpoint{ifaces: []*testInterface{}}
	err = d.CreateEndpoint("dummy", "ep", te2, nil)
	if err != nil {
		if _, ok := err.(driverapi.ErrEndpointExists); !ok {
			t.Fatalf("Failed with a wrong error: %s", err.Error())
		}
	} else {
		t.Fatalf("Expected to fail while trying to add same endpoint twice")
	}
}

func TestLinkCreateNoEnableIPv6(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	d := newDriver()

	config := &NetworkConfiguration{
		BridgeName: DefaultBridgeName}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	err := d.CreateNetwork("dummy", genericOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	te := &testEndpoint{ifaces: []*testInterface{}}
	err = d.CreateEndpoint("dummy", "ep", te, nil)
	if err != nil {
		t.Fatalf("Failed to create a link: %s", err.Error())
	}

	interfaces := te.ifaces
	if interfaces[0].addrv6.IP.To16() != nil {
		t.Fatalf("Expectd IPv6 address to be nil when IPv6 is not enabled. Got IPv6 = %s", interfaces[0].addrv6.String())
	}

	if te.gw6.To16() != nil {
		t.Fatalf("Expected GatewayIPv6 to be nil when IPv6 is not enabled. Got GatewayIPv6 = %s", te.gw6.String())
	}
}

func TestLinkDelete(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	d := newDriver()

	config := &NetworkConfiguration{
		BridgeName: DefaultBridgeName,
		EnableIPv6: true}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	err := d.CreateNetwork("dummy", genericOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	te := &testEndpoint{ifaces: []*testInterface{}}
	err = d.CreateEndpoint("dummy", "ep1", te, nil)
	if err != nil {
		t.Fatalf("Failed to create a link: %s", err.Error())
	}

	err = d.DeleteEndpoint("dummy", "")
	if err != nil {
		if _, ok := err.(InvalidEndpointIDError); !ok {
			t.Fatalf("Failed with a wrong error :%s", err.Error())
		}
	} else {
		t.Fatalf("Failed to detect invalid config")
	}

	err = d.DeleteEndpoint("dummy", "ep1")
	if err != nil {
		t.Fatal(err)
	}
}
