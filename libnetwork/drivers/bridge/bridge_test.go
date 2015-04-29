package bridge

import (
	"bytes"
	"net"
	"testing"

	"github.com/docker/libnetwork/netutils"
	"github.com/vishvananda/netlink"
)

func TestCreate(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New()

	config := &Configuration{BridgeName: DefaultBridgeName}
	if err := d.Config(config); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	if err := d.CreateNetwork("dummy", ""); err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}
}

func TestCreateFail(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New()

	config := &Configuration{BridgeName: "dummy0"}
	if err := d.Config(config); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	if err := d.CreateNetwork("dummy", ""); err == nil {
		t.Fatal("Bridge creation was expected to fail")
	}
}

func TestCreateFullOptions(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New()

	config := &Configuration{
		BridgeName:         DefaultBridgeName,
		EnableIPv6:         true,
		FixedCIDR:          bridgeNetworks[0],
		EnableIPTables:     true,
		EnableIPForwarding: true,
	}
	_, config.FixedCIDRv6, _ = net.ParseCIDR("2001:db8::/48")
	if err := d.Config(config); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	err := d.CreateNetwork("dummy", "")
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}
}
func TestCreateLinkWithOptions(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	_, d := New()

	config := &Configuration{BridgeName: DefaultBridgeName}
	if err := d.Config(config); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	err := d.CreateNetwork("net1", "")
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	mac := net.HardwareAddr([]byte{0x1e, 0x67, 0x66, 0x44, 0x55, 0x66})
	epConf := &EndpointConfiguration{MacAddress: mac}

	sinfo, err := d.CreateEndpoint("net1", "ep", epConf)
	if err != nil {
		t.Fatalf("Failed to create a link: %s", err.Error())
	}

	ifaceName := sinfo.Interfaces[0].SrcName
	veth, err := netlink.LinkByName(ifaceName)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(mac, veth.Attrs().HardwareAddr) {
		t.Fatalf("Failed to parse and program endpoint configuration")
	}
}

func TestValidateConfig(t *testing.T) {

	// Test mtu
	c := Configuration{Mtu: -2}
	err := c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid MTU number")
	}

	c.Mtu = 9000
	err = c.Validate()
	if err != nil {
		t.Fatalf("unexpected validation error on MTU number")
	}

	// Bridge network
	_, network, _ := net.ParseCIDR("172.28.0.0/16")

	// Test FixedCIDR
	_, containerSubnet, _ := net.ParseCIDR("172.27.0.0/16")
	c = Configuration{
		AddressIPv4: network,
		FixedCIDR:   containerSubnet,
	}

	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid FixedCIDR network")
	}

	_, containerSubnet, _ = net.ParseCIDR("172.28.0.0/16")
	c.FixedCIDR = containerSubnet
	err = c.Validate()
	if err != nil {
		t.Fatalf("Unexpected validation error on FixedCIDR network")
	}

	_, containerSubnet, _ = net.ParseCIDR("172.28.0.0/15")
	c.FixedCIDR = containerSubnet
	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid FixedCIDR network")
	}

	_, containerSubnet, _ = net.ParseCIDR("172.28.0.0/17")
	c.FixedCIDR = containerSubnet
	err = c.Validate()
	if err != nil {
		t.Fatalf("Unexpected validation error on FixedCIDR network")
	}

	// Test v4 gw
	c.DefaultGatewayIPv4 = net.ParseIP("172.27.30.234")
	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid default gateway")
	}

	c.DefaultGatewayIPv4 = net.ParseIP("172.28.30.234")
	err = c.Validate()
	if err != nil {
		t.Fatalf("Unexpected validation error on default gateway")
	}

	// Test v6 gw
	_, containerSubnet, _ = net.ParseCIDR("2001:1234:ae:b004::/64")
	c = Configuration{
		EnableIPv6:         true,
		FixedCIDRv6:        containerSubnet,
		DefaultGatewayIPv6: net.ParseIP("2001:1234:ac:b004::bad:a55"),
	}
	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid v6 default gateway")
	}

	c.DefaultGatewayIPv6 = net.ParseIP("2001:1234:ae:b004::bad:a55")
	err = c.Validate()
	if err != nil {
		t.Fatalf("Unexpected validation error on v6 default gateway")
	}

	c.FixedCIDRv6 = nil
	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid v6 default gateway")
	}
}

func TestSetDefaultGw(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New()

	_, subnetv6, _ := net.ParseCIDR("2001:db8:ea9:9abc:b0c4::/80")
	gw4 := bridgeNetworks[0].IP.To4()
	gw4[3] = 254
	gw6 := net.ParseIP("2001:db8:ea9:9abc:b0c4::254")

	config := &Configuration{
		BridgeName:         DefaultBridgeName,
		EnableIPv6:         true,
		FixedCIDRv6:        subnetv6,
		DefaultGatewayIPv4: gw4,
		DefaultGatewayIPv6: gw6,
	}

	if err := d.Config(config); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	err := d.CreateNetwork("dummy", nil)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	sinfo, err := d.CreateEndpoint("dummy", "ep", nil)
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	if !gw4.Equal(sinfo.Gateway) {
		t.Fatalf("Failed to configure default gateway. Expected %v. Found %v", gw4, sinfo.Gateway)
	}

	if !gw6.Equal(sinfo.GatewayIPv6) {
		t.Fatalf("Failed to configure default gateway. Expected %v. Found %v", gw6, sinfo.GatewayIPv6)
	}
}
