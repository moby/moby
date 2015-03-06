package bridge

import (
	"testing"

	"github.com/docker/libnetwork"
	"github.com/vishvananda/netlink"
)

func TestInterfaceDefaultName(t *testing.T) {
	defer libnetwork.SetupTestNetNS(t)()

	if inf := newInterface(&Configuration{}); inf.Config.BridgeName != DefaultBridgeName {
		t.Fatalf("Expected default interface name %q, got %q", DefaultBridgeName, inf.Config.BridgeName)
	}
}

func TestAddressesEmptyInterface(t *testing.T) {
	defer libnetwork.SetupTestNetNS(t)()

	inf := newInterface(&Configuration{})
	addrv4, addrsv6, err := inf.addresses()
	if err != nil {
		t.Fatalf("Failed to get addresses of default interface: %v", err)
	}
	if expected := (netlink.Addr{}); addrv4 != expected {
		t.Fatalf("Default interface has unexpected IPv4: %s", addrv4)
	}
	if len(addrsv6) != 0 {
		t.Fatalf("Default interface has unexpected IPv6: %v", addrsv6)
	}
}
