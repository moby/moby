package bridge

import (
	"net"
	"testing"

	"github.com/docker/libnetwork/netutils"
	"github.com/vishvananda/netlink"
)

func setupTestInterface(t *testing.T) *bridgeInterface {
	br := &bridgeInterface{
		Config: &Configuration{
			BridgeName: DefaultBridgeName,
		},
	}
	if err := setupDevice(br); err != nil {
		t.Fatalf("Bridge creation failed: %v", err)
	}
	return br
}

func TestSetupBridgeIPv4Fixed(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	ip, netw, err := net.ParseCIDR("192.168.1.1/24")
	if err != nil {
		t.Fatalf("Failed to parse bridge IPv4: %v", err)
	}

	br := setupTestInterface(t)
	br.Config.AddressIPv4 = &net.IPNet{IP: ip, Mask: netw.Mask}
	if err := setupBridgeIPv4(br); err != nil {
		t.Fatalf("Failed to setup bridge IPv4: %v", err)
	}

	addrsv4, err := netlink.AddrList(br.Link, netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("Failed to list device IPv4 addresses: %v", err)
	}

	var found bool
	for _, addr := range addrsv4 {
		if br.Config.AddressIPv4.String() == addr.IPNet.String() {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Bridge device does not have requested IPv4 address %v", br.Config.AddressIPv4)
	}
}

func TestSetupBridgeIPv4Auto(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	br := setupTestInterface(t)
	if err := setupBridgeIPv4(br); err != nil {
		t.Fatalf("Failed to setup bridge IPv4: %v", err)
	}

	addrsv4, err := netlink.AddrList(br.Link, netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("Failed to list device IPv4 addresses: %v", err)
	}

	var found bool
	for _, addr := range addrsv4 {
		if bridgeNetworks[0].String() == addr.IPNet.String() {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Bridge device does not have the automatic IPv4 address %v", bridgeNetworks[0].String())
	}
}
