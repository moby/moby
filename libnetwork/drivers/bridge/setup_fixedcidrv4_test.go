package bridge

import (
	"net"
	"testing"

	"github.com/docker/libnetwork/netutils"
)

func TestSetupFixedCIDRv4(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	config := &NetworkConfiguration{
		BridgeName:  DefaultBridgeName,
		AddressIPv4: &net.IPNet{IP: net.ParseIP("192.168.1.1"), Mask: net.CIDRMask(16, 32)},
		FixedCIDR:   &net.IPNet{IP: net.ParseIP("192.168.2.0"), Mask: net.CIDRMask(24, 32)}}
	br := &bridgeInterface{}

	if err := setupDevice(config, br); err != nil {
		t.Fatalf("Bridge creation failed: %v", err)
	}
	if err := setupBridgeIPv4(config, br); err != nil {
		t.Fatalf("Assign IPv4 to bridge failed: %v", err)
	}

	if err := setupFixedCIDRv4(config, br); err != nil {
		t.Fatalf("Failed to setup bridge FixedCIDRv4: %v", err)
	}

	if ip, err := ipAllocator.RequestIP(config.FixedCIDR, nil); err != nil {
		t.Fatalf("Failed to request IP to allocator: %v", err)
	} else if expected := "192.168.2.1"; ip.String() != expected {
		t.Fatalf("Expected allocated IP %s, got %s", expected, ip)
	}
}

func TestSetupBadFixedCIDRv4(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	config := &NetworkConfiguration{
		BridgeName:  DefaultBridgeName,
		AddressIPv4: &net.IPNet{IP: net.ParseIP("192.168.1.1"), Mask: net.CIDRMask(24, 32)},
		FixedCIDR:   &net.IPNet{IP: net.ParseIP("192.168.2.0"), Mask: net.CIDRMask(24, 32)}}
	br := &bridgeInterface{}

	if err := setupDevice(config, br); err != nil {
		t.Fatalf("Bridge creation failed: %v", err)
	}
	if err := setupBridgeIPv4(config, br); err != nil {
		t.Fatalf("Assign IPv4 to bridge failed: %v", err)
	}

	err := setupFixedCIDRv4(config, br)
	if err == nil {
		t.Fatal("Setup bridge FixedCIDRv4 should have failed")
	}

	if _, ok := err.(*FixedCIDRv4Error); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}

}
