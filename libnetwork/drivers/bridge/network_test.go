package bridge

import (
	"testing"

	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/pkg/netlabel"
	"github.com/vishvananda/netlink"
)

func TestLinkCreate(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New(nil)
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

	sinfo, err := d.CreateEndpoint("dummy", "", nil)
	if err != nil {
		if _, ok := err.(InvalidEndpointIDError); !ok {
			t.Fatalf("Failed with a wrong error :%s", err.Error())
		}
	} else {
		t.Fatalf("Failed to detect invalid config")
	}

	// Good endpoint creation
	sinfo, err = d.CreateEndpoint("dummy", "ep", nil)
	if err != nil {
		t.Fatalf("Failed to create a link: %s", err.Error())
	}

	// Verify sbox endoint interface inherited MTU value from bridge config
	sboxLnk, err := netlink.LinkByName(sinfo.Interfaces[0].SrcName)
	if err != nil {
		t.Fatal(err)
	}
	if mtu != sboxLnk.Attrs().MTU {
		t.Fatalf("Sandbox endpoint interface did not inherit bridge interface MTU config")
	}
	// TODO: if we could get peer name from (sboxLnk.(*netlink.Veth)).PeerName
	// then we could check the MTU on hostLnk as well.

	_, err = d.CreateEndpoint("dummy", "ep", nil)
	if err == nil {
		t.Fatalf("Failed to detect duplicate endpoint id on same network")
	}

	interfaces := sinfo.Interfaces
	if len(interfaces) != 1 {
		t.Fatalf("Expected exactly one interface. Instead got %d interface(s)", len(interfaces))
	}

	if interfaces[0].DstName == "" {
		t.Fatal("Invalid Dstname returned")
	}

	_, err = netlink.LinkByName(interfaces[0].SrcName)
	if err != nil {
		t.Fatalf("Could not find source link %s: %v", interfaces[0].SrcName, err)
	}

	n := dr.network
	ip := interfaces[0].Address.IP
	if !n.bridge.bridgeIPv4.Contains(ip) {
		t.Fatalf("IP %s is not a valid ip in the subnet %s", ip.String(), n.bridge.bridgeIPv4.String())
	}

	ip6 := interfaces[0].AddressIPv6.IP
	if !n.bridge.bridgeIPv6.Contains(ip6) {
		t.Fatalf("IP %s is not a valid ip in the subnet %s", ip6.String(), bridgeIPv6.String())
	}

	if !sinfo.Gateway.Equal(n.bridge.bridgeIPv4.IP) {
		t.Fatalf("Invalid default gateway. Expected %s. Got %s", n.bridge.bridgeIPv4.IP.String(),
			sinfo.Gateway.String())
	}

	if !sinfo.GatewayIPv6.Equal(n.bridge.bridgeIPv6.IP) {
		t.Fatalf("Invalid default gateway for IPv6. Expected %s. Got %s", n.bridge.bridgeIPv6.IP.String(),
			sinfo.GatewayIPv6.String())
	}
}

func TestLinkCreateTwo(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New(nil)

	config := &NetworkConfiguration{
		BridgeName: DefaultBridgeName,
		EnableIPv6: true}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	err := d.CreateNetwork("dummy", genericOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	_, err = d.CreateEndpoint("dummy", "ep", nil)
	if err != nil {
		t.Fatalf("Failed to create a link: %s", err.Error())
	}

	_, err = d.CreateEndpoint("dummy", "ep", nil)
	if err != nil {
		if err != driverapi.ErrEndpointExists {
			t.Fatalf("Failed with a wrong error :%s", err.Error())
		}
	} else {
		t.Fatalf("Expected to fail while trying to add same endpoint twice")
	}
}

func TestLinkCreateNoEnableIPv6(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New(nil)

	config := &NetworkConfiguration{
		BridgeName: DefaultBridgeName}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	err := d.CreateNetwork("dummy", genericOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	sinfo, err := d.CreateEndpoint("dummy", "ep", nil)
	if err != nil {
		t.Fatalf("Failed to create a link: %s", err.Error())
	}

	interfaces := sinfo.Interfaces
	if interfaces[0].AddressIPv6 != nil {
		t.Fatalf("Expectd IPv6 address to be nil when IPv6 is not enabled. Got IPv6 = %s", interfaces[0].AddressIPv6.String())
	}

	if sinfo.GatewayIPv6 != nil {
		t.Fatalf("Expected GatewayIPv6 to be nil when IPv6 is not enabled. Got GatewayIPv6 = %s", sinfo.GatewayIPv6.String())
	}
}

func TestLinkDelete(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New(nil)

	config := &NetworkConfiguration{
		BridgeName: DefaultBridgeName,
		EnableIPv6: true}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	err := d.CreateNetwork("dummy", genericOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	_, err = d.CreateEndpoint("dummy", "ep1", nil)
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

	err = d.DeleteEndpoint("dummy", "ep1")
	if err == nil {
		t.Fatal(err)
	}
}
