package bridge

import (
	"net"
	"testing"

	"github.com/docker/libnetwork/netutils"
)

func TestSetupFixedCIDRv6(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	config := &NetworkConfiguration{}
	br := newInterface(config)

	_, config.FixedCIDRv6, _ = net.ParseCIDR("2002:db8::/48")
	if err := setupDevice(config, br); err != nil {
		t.Fatalf("Bridge creation failed: %v", err)
	}
	if err := setupBridgeIPv4(config, br); err != nil {
		t.Fatalf("Assign IPv4 to bridge failed: %v", err)
	}

	if err := setupBridgeIPv6(config, br); err != nil {
		t.Fatalf("Assign IPv4 to bridge failed: %v", err)
	}

	if err := setupFixedCIDRv6(config, br); err != nil {
		t.Fatalf("Failed to setup bridge FixedCIDRv6: %v", err)
	}

	if ip, err := ipAllocator.RequestIP(config.FixedCIDRv6, nil); err != nil {
		t.Fatalf("Failed to request IP to allocator: %v", err)
	} else if expected := "2002:db8::1"; ip.String() != expected {
		t.Fatalf("Expected allocated IP %s, got %s", expected, ip)
	}
}
