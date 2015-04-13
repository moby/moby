package bridge

import (
	"net"
	"testing"

	"github.com/docker/libnetwork/netutils"
)

func TestCreate(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New()

	config := &Configuration{BridgeName: DefaultBridgeName}
	err := d.CreateNetwork("dummy", config)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}
}

func TestCreateFail(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New()

	config := &Configuration{BridgeName: "dummy0"}
	if err := d.CreateNetwork("dummy", config); err == nil {
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

	err := d.CreateNetwork("dummy", config)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}
}
