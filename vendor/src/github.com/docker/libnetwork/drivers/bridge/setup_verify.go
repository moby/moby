package bridge

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
)

func setupVerifyAndReconcile(config *networkConfiguration, i *bridgeInterface) error {
	// Fetch a single IPv4 and a slice of IPv6 addresses from the bridge.
	addrv4, addrsv6, err := i.addresses()
	if err != nil {
		return fmt.Errorf("Failed to verify ip addresses: %v", err)
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

	// Release any residual IPv6 address that might be there because of older daemon instances
	for _, addrv6 := range addrsv6 {
		if addrv6.IP.IsGlobalUnicast() && !types.CompareIPNet(addrv6.IPNet, i.bridgeIPv6) {
			if err := i.nlh.AddrDel(i.Link, &addrv6); err != nil {
				log.Warnf("Failed to remove residual IPv6 address %s from bridge: %v", addrv6.IPNet, err)
			}
		}
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
