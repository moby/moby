package libnetwork_test

import (
	"flag"
	"net"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork"
	_ "github.com/docker/libnetwork/drivers/bridge"
	"github.com/vishvananda/netlink"
)

var bridgeName = "docker0"
var enableBridgeTest = flag.Bool("enable-bridge-test", false, "")

func TestSimplebridge(t *testing.T) {
	if *enableBridgeTest == false {
		t.Skip()
	}

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
	options := libnetwork.DriverParams{
		"BridgeName":         bridgeName,
		"AddressIPv4":        subnet,
		"FixedCIDR":          cidr,
		"FixedCIDRv6":        cidrv6,
		"EnableIPv6":         true,
		"EnableIPTables":     true,
		"EnableIPMasquerade": true,
		"EnableICC":          true,
		"EnableIPForwarding": true}

	network, err := libnetwork.NewNetwork("simplebridge", "dummy", options)
	if err != nil {
		t.Fatal(err)
	}

	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}
