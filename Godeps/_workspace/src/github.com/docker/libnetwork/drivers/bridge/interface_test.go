package bridge

import (
	"testing"

	"github.com/docker/libnetwork/netutils"
	"github.com/vishvananda/netlink"
)

func TestInterfaceDefaultName(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	config := &NetworkConfiguration{}
	if _ = newInterface(config); config.BridgeName != DefaultBridgeName {
		t.Fatalf("Expected default interface name %q, got %q", DefaultBridgeName, config.BridgeName)
	}
}

func TestAddressesEmptyInterface(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	inf := newInterface(&NetworkConfiguration{})
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
