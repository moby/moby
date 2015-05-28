package bridge

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"testing"

	"github.com/docker/libnetwork/netutils"
	"github.com/vishvananda/netlink"
)

func TestSetupIPv6(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	config, br := setupTestInterface(t)
	if err := setupBridgeIPv6(config, br); err != nil {
		t.Fatalf("Failed to setup bridge IPv6: %v", err)
	}

	procSetting, err := ioutil.ReadFile(fmt.Sprintf("/proc/sys/net/ipv6/conf/%s/disable_ipv6", config.BridgeName))
	if err != nil {
		t.Fatalf("Failed to read disable_ipv6 kernel setting: %v", err)
	}

	if expected := []byte("0\n"); bytes.Compare(expected, procSetting) != 0 {
		t.Fatalf("Invalid kernel setting disable_ipv6: expected %q, got %q", string(expected), string(procSetting))
	}

	addrsv6, err := netlink.AddrList(br.Link, netlink.FAMILY_V6)
	if err != nil {
		t.Fatalf("Failed to list device IPv6 addresses: %v", err)
	}

	var found bool
	for _, addr := range addrsv6 {
		if bridgeIPv6Str == addr.IPNet.String() {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Bridge device does not have requested IPv6 address %v", bridgeIPv6Str)
	}

}

func TestSetupGatewayIPv6(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	_, nw, _ := net.ParseCIDR("2001:db8:ea9:9abc:ffff::/80")
	gw := net.ParseIP("2001:db8:ea9:9abc:ffff::254")

	config := &NetworkConfiguration{
		BridgeName:         DefaultBridgeName,
		FixedCIDRv6:        nw,
		DefaultGatewayIPv6: gw}

	br := &bridgeInterface{}

	if err := setupGatewayIPv6(config, br); err != nil {
		t.Fatalf("Set Default Gateway failed: %v", err)
	}

	if !gw.Equal(br.gatewayIPv6) {
		t.Fatalf("Set Default Gateway failed. Expected %v, Found %v", gw, br.gatewayIPv6)
	}
}
