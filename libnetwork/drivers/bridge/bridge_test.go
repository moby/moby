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

	sinfo, err := d.CreateEndpoint("net1", "ep", "s1", epConf)
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
