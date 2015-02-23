package bridge

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

func SetupVerifyConfiguredAddresses(i *Interface) error {
	// Fetch a single IPv4 and a slice of IPv6 addresses from the bridge.
	addrv4, addrsv6, err := getInterfaceAddresses(i.Link)
	if err != nil {
		return err
	}

	// Verify that the bridge IPv4 address matches the requested configuration.
	if i.Config.AddressIPv4 != nil && !addrv4.IP.Equal(i.Config.AddressIPv4.IP) {
		return fmt.Errorf("Bridge IPv4 (%s) does not match requested configuration %s", addrv4.IP, i.Config.AddressIPv4.IP)
	}

	// Verify that one of the bridge IPv6 addresses matches the requested
	// configuration.
	for _, addrv6 := range addrsv6 {
		if addrv6.String() == BridgeIPv6.String() {
			return nil
		}
	}

	return fmt.Errorf("Bridge IPv6 addresses do not match the expected bridge configuration %s", BridgeIPv6)
}

func getInterfaceAddresses(iface netlink.Link) (netlink.Addr, []netlink.Addr, error) {
	v4addr, err := netlink.AddrList(iface, netlink.FAMILY_V4)
	if err != nil {
		return netlink.Addr{}, nil, err
	}

	v6addr, err := netlink.AddrList(iface, netlink.FAMILY_V6)
	if err != nil {
		return netlink.Addr{}, nil, err
	}

	// We only return the first IPv4 address, and the complete slice of IPv6
	// addresses.
	return v4addr[0], v6addr, nil
}
