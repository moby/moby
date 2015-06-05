package bridge

import (
	"net"
	"testing"

	"github.com/docker/libnetwork/netutils"
	"github.com/vishvananda/netlink"
)

func setupTestInterface(t *testing.T) (*NetworkConfiguration, *bridgeInterface) {
	config := &NetworkConfiguration{
		BridgeName: DefaultBridgeName}
	br := &bridgeInterface{}

	if err := setupDevice(config, br); err != nil {
		t.Fatalf("Bridge creation failed: %v", err)
	}
	return config, br
}

func TestSetupBridgeIPv4Fixed(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	ip, netw, err := net.ParseCIDR("192.168.1.1/24")
	if err != nil {
		t.Fatalf("Failed to parse bridge IPv4: %v", err)
	}

	config, br := setupTestInterface(t)
	config.AddressIPv4 = &net.IPNet{IP: ip, Mask: netw.Mask}
	if err := setupBridgeIPv4(config, br); err != nil {
		t.Fatalf("Failed to setup bridge IPv4: %v", err)
	}

	addrsv4, err := netlink.AddrList(br.Link, netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("Failed to list device IPv4 addresses: %v", err)
	}

	var found bool
	for _, addr := range addrsv4 {
		if config.AddressIPv4.String() == addr.IPNet.String() {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Bridge device does not have requested IPv4 address %v", config.AddressIPv4)
	}
}

func TestSetupBridgeIPv4Auto(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	config, br := setupTestInterface(t)
	if err := setupBridgeIPv4(config, br); err != nil {
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

func TestSetupGatewayIPv4(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	ip, nw, _ := net.ParseCIDR("192.168.0.24/16")
	nw.IP = ip
	gw := net.ParseIP("192.168.0.254")

	config := &NetworkConfiguration{
		BridgeName:         DefaultBridgeName,
		DefaultGatewayIPv4: gw}

	br := &bridgeInterface{bridgeIPv4: nw}

	if err := setupGatewayIPv4(config, br); err != nil {
		t.Fatalf("Set Default Gateway failed: %v", err)
	}

	if !gw.Equal(br.gatewayIPv4) {
		t.Fatalf("Set Default Gateway failed. Expected %v, Found %v", gw, br.gatewayIPv4)
	}
}
