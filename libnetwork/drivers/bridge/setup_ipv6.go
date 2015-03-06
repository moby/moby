package bridge

import (
	"fmt"
	"io/ioutil"
	"net"

	"github.com/vishvananda/netlink"
)

var bridgeIPv6 *net.IPNet

const bridgeIPv6Str = "fe80::1/64"

func init() {
	// We allow ourselves to panic in this special case because we indicate a
	// failure to parse a compile-time define constant.
	if ip, netw, err := net.ParseCIDR(bridgeIPv6Str); err == nil {
		bridgeIPv6 = &net.IPNet{IP: ip, Mask: netw.Mask}
	} else {
		panic(fmt.Sprintf("Cannot parse default bridge IPv6 address %q: %v", bridgeIPv6Str, err))
	}
}

func setupBridgeIPv6(i *bridgeInterface) error {
	// Enable IPv6 on the bridge
	procFile := "/proc/sys/net/ipv6/conf/" + i.Config.BridgeName + "/disable_ipv6"
	if err := ioutil.WriteFile(procFile, []byte{'0', '\n'}, 0644); err != nil {
		return fmt.Errorf("Unable to enable IPv6 addresses on bridge: %v", err)
	}

	if err := netlink.AddrAdd(i.Link, &netlink.Addr{IPNet: bridgeIPv6}); err != nil {
		return fmt.Errorf("Failed to add IPv6 address %s to bridge: %v", bridgeIPv6, err)
	}

	return nil
}
