package bridge

import (
	"net"
	"testing"

	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/osl"
	"github.com/vishvananda/netlink"
)

func setupTestInterface(t *testing.T) (*networkConfiguration, *bridgeInterface) {
	config := &networkConfiguration{
		BridgeName: DefaultBridgeName}
	br := &bridgeInterface{}

	if err := setupDevice(config, br); err != nil {
		t.Fatalf("Bridge creation failed: %v", err)
	}
	return config, br
}

func TestSetupBridgeIPv4Fixed(t *testing.T) {
	defer osl.SetupTestOSContext(t)()

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
	defer osl.SetupTestOSContext(t)()

	var toBeChosen *net.IPNet
	for _, n := range bridgeNetworks {
		if err := netutils.CheckRouteOverlaps(n); err == nil {
			toBeChosen = n
			break
		}
	}
	if toBeChosen == nil {
		t.Skipf("Skip as no more automatic networks available")
	}

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
		if toBeChosen.String() == addr.IPNet.String() {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Bridge device does not have the automatic IPv4 address %s", toBeChosen.String())
	}
}

func TestSetupGatewayIPv4(t *testing.T) {
	defer osl.SetupTestOSContext(t)()

	ip, nw, _ := net.ParseCIDR("192.168.0.24/16")
	nw.IP = ip
	gw := net.ParseIP("192.168.2.254")

	config := &networkConfiguration{
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

func TestCheckPreallocatedBridgeNetworks(t *testing.T) {
	// Just make sure the bridge networks are created the way we want (172.17.x.x/16)
	for i := 0; i < len(bridgeNetworks); i++ {
		fb := bridgeNetworks[i].IP[0]
		ones, _ := bridgeNetworks[i].Mask.Size()
		if ((fb == 172 || fb == 10) && ones != 16) || (fb == 192 && ones != 24) {
			t.Fatalf("Wrong mask for preallocated bridge network: %s", bridgeNetworks[i].String())
		}
	}
}
