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
