package bridge

import (
	"github.com/vishvananda/netlink"
)

func setupVerifyAndReconcile(config *networkConfiguration, i *bridgeInterface) error {
	// Fetch a single IPv4 and a slice of IPv6 addresses from the bridge.
	addrv4, addrsv6, err := i.addresses()
	if err != nil {
		return err
	}

	// Verify that the bridge does have an IPv4 address.
	if addrv4.IPNet == nil {
		return &ErrNoIPAddr{}
	}

	// Verify that the bridge IPv4 address matches the requested configuration.
	if config.AddressIPv4 != nil && !addrv4.IP.Equal(config.AddressIPv4.IP) {
		return &IPv4AddrNoMatchError{IP: addrv4.IP, CfgIP: config.AddressIPv4.IP}
	}

	// Verify that one of the bridge IPv6 addresses matches the requested
	// configuration.
	if config.EnableIPv6 && !findIPv6Address(netlink.Addr{IPNet: bridgeIPv6}, addrsv6) {
		return (*IPv6AddrNoMatchError)(bridgeIPv6)
	}

	// By this time we have either configured a new bridge with an IP address
	// or made sure an existing bridge's IP matches the configuration
	// Now is the time to cache these states in the bridgeInterface.
	i.bridgeIPv4 = addrv4.IPNet
	i.bridgeIPv6 = bridgeIPv6

	return nil
}

func findIPv6Address(addr netlink.Addr, addresses []netlink.Addr) bool {
	for _, addrv6 := range addresses {
		if addrv6.String() == addr.String() {
			return true
		}
	}
	return false
}
