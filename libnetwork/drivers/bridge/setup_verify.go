package bridge

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

func setupVerifyConfiguredAddresses(config *Configuration, i *bridgeInterface) error {
	// Fetch a single IPv4 and a slice of IPv6 addresses from the bridge.
	addrv4, addrsv6, err := i.addresses()
	if err != nil {
		return err
	}

	// Verify that the bridge does have an IPv4 address.
	if addrv4.IPNet == nil {
		return fmt.Errorf("Bridge has no IPv4 address configured")
	}

	// Verify that the bridge IPv4 address matches the requested configuration.
	if config.AddressIPv4 != nil && !addrv4.IP.Equal(config.AddressIPv4.IP) {
		return fmt.Errorf("Bridge IPv4 (%s) does not match requested configuration %s", addrv4.IP, config.AddressIPv4.IP)
	}

	// Verify that one of the bridge IPv6 addresses matches the requested
	// configuration.
	if config.EnableIPv6 && !findIPv6Address(netlink.Addr{IPNet: bridgeIPv6}, addrsv6) {
		return fmt.Errorf("Bridge IPv6 addresses do not match the expected bridge configuration %s", bridgeIPv6)
	}

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
