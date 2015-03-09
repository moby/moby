package bridge

import (
	"net"
	"testing"

	"github.com/docker/libnetwork"
)

func TestCreate(t *testing.T) {
	defer libnetwork.SetupTestNetNS(t)()
	d := &driver{}

	config := &Configuration{BridgeName: DefaultBridgeName}
	netw, err := d.CreateNetwork("dummy", config)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	if expected := networkType; netw.Type() != expected {
		t.Fatalf("Expected networkType %q, got %q", expected, netw.Type())
	}
}

func TestCreateFail(t *testing.T) {
	defer libnetwork.SetupTestNetNS(t)()
	d := &driver{}

	config := &Configuration{BridgeName: "dummy0"}
	if _, err := d.CreateNetwork("dummy", config); err == nil {
		t.Fatal("Bridge creation was expected to fail")
	}
}

func TestCreateFullOptions(t *testing.T) {
	defer libnetwork.SetupTestNetNS(t)()
	d := &driver{}

	config := &Configuration{
		BridgeName:         DefaultBridgeName,
		EnableIPv6:         true,
		FixedCIDR:          bridgeNetworks[0],
		EnableIPTables:     true,
		EnableIPForwarding: true,
	}
	_, config.FixedCIDRv6, _ = net.ParseCIDR("2001:db8::/48")

	netw, err := d.CreateNetwork("dummy", config)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	if expected := networkType; netw.Type() != expected {
		t.Fatalf("Expected networkType %q, got %q", expected, netw.Type())
	}
}
