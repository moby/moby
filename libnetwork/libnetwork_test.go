package libnetwork_test

import (
	"net"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork"
	_ "github.com/docker/libnetwork/drivers/bridge"
	"github.com/docker/libnetwork/pkg/options"
	"github.com/vishvananda/netlink"
)

var bridgeName = "dockertest0"

func TestSimplebridge(t *testing.T) {
	bridge := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: bridgeName}}
	netlink.LinkDel(bridge)

	ip, subnet, err := net.ParseCIDR("192.168.100.1/24")
	if err != nil {
		t.Fatal(err)
	}
	subnet.IP = ip

	ip, cidr, err := net.ParseCIDR("192.168.100.2/28")
	if err != nil {
		t.Fatal(err)
	}
	cidr.IP = ip

	ip, cidrv6, err := net.ParseCIDR("fe90::1/96")
	if err != nil {
		t.Fatal(err)
	}
	cidrv6.IP = ip

	log.Debug("Adding a simple bridge")
	options := options.Generic{
		"BridgeName":            bridgeName,
		"AddressIPv4":           subnet,
		"FixedCIDR":             cidr,
		"FixedCIDRv6":           cidrv6,
		"EnableIPv6":            true,
		"EnableIPTables":        true,
		"EnableIPMasquerade":    true,
		"EnableICC":             true,
		"EnableIPForwarding":    true,
		"AllowNonDefaultBridge": true}

	controller := libnetwork.New()

	driver, err := controller.NewNetworkDriver("simplebridge", options)
	if err != nil {
		t.Fatal(err)
	}

	network, err := controller.NewNetwork(driver, "testnetwork", "")
	if err != nil {
		t.Fatal(err)
	}

	ep, _, err := network.CreateEndpoint("testep", "", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := ep.Delete(); err != nil {
		t.Fatal(err)
	}

	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}
