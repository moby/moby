package bridge

import (
	"testing"

	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netutils"
	"github.com/vishvananda/netlink"
)

func TestLinkCreate(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New()
	dr := d.(*driver)

	config := &Configuration{
		BridgeName: DefaultBridgeName,
		EnableIPv6: true}
	err := d.CreateNetwork("dummy", config)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	sinfo, err := d.CreateEndpoint("dummy", "ep", "", "")
	if err != nil {
		t.Fatalf("Failed to create a link: %v", err)
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

	if sinfo.Gateway.String() != n.bridge.bridgeIPv4.IP.String() {
		t.Fatalf("Invalid default gateway. Expected %s. Got %s", n.bridge.bridgeIPv4.IP.String(),
			sinfo.Gateway.String())
	}

	if sinfo.GatewayIPv6.String() != n.bridge.bridgeIPv6.IP.String() {
		t.Fatalf("Invalid default gateway for IPv6. Expected %s. Got %s", n.bridge.bridgeIPv6.IP.String(),
			sinfo.GatewayIPv6.String())
	}
}

func TestLinkCreateTwo(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New()

	config := &Configuration{
		BridgeName: DefaultBridgeName,
		EnableIPv6: true}
	err := d.CreateNetwork("dummy", config)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	_, err = d.CreateEndpoint("dummy", "ep", "", "")
	if err != nil {
		t.Fatalf("Failed to create a link: %v", err)
	}

	_, err = d.CreateEndpoint("dummy", "ep1", "", "")
	if err != nil {
		if err != driverapi.ErrEndpointExists {
			t.Fatalf("Failed with a wrong error :%v", err)
		}
	} else {
		t.Fatalf("Expected to fail while trying to add more than one endpoint")
	}
}

func TestLinkCreateNoEnableIPv6(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New()

	config := &Configuration{
		BridgeName: DefaultBridgeName}
	err := d.CreateNetwork("dummy", config)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	sinfo, err := d.CreateEndpoint("dummy", "ep", "", "")
	if err != nil {
		t.Fatalf("Failed to create a link: %v", err)
	}

	interfaces := sinfo.Interfaces
	if len(interfaces[0].AddressIPv6.IP) != 0 {
		t.Fatalf("Expectd IPv6 address to be nil when IPv6 is not enabled. Got IPv6 = %s", interfaces[0].AddressIPv6.String())
	}

	if sinfo.GatewayIPv6 != nil {
		t.Fatalf("Expected GatewayIPv6 to be nil when IPv6 is not enabled. Got GatewayIPv6 = %s", sinfo.GatewayIPv6.String())
	}
}
