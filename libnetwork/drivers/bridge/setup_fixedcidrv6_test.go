package bridge

import (
	"net"
	"testing"

	"github.com/docker/docker/daemon/networkdriver/ipallocator"
	"github.com/docker/libnetwork"
)

func TestSetupFixedCIDRv6(t *testing.T) {
	defer libnetwork.SetupTestNetNS(t)()

	br := newInterface(&Configuration{})

	_, br.Config.FixedCIDRv6, _ = net.ParseCIDR("2002:db8::/48")
	if err := setupDevice(br); err != nil {
		t.Fatalf("Bridge creation failed: %v", err)
	}
	if err := setupBridgeIPv4(br); err != nil {
		t.Fatalf("Assign IPv4 to bridge failed: %v", err)
	}

	if err := setupBridgeIPv6(br); err != nil {
		t.Fatalf("Assign IPv4 to bridge failed: %v", err)
	}

	if err := setupFixedCIDRv6(br); err != nil {
		t.Fatalf("Failed to setup bridge FixedCIDRv6: %v", err)
	}

	if ip, err := ipallocator.RequestIP(br.Config.FixedCIDRv6, nil); err != nil {
		t.Fatalf("Failed to request IP to allocator: %v", err)
	} else if expected := "2002:db8::1"; ip.String() != expected {
		t.Fatalf("Expected allocated IP %s, got %s", expected, ip)
	}
}
