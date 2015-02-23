package bridge

import "github.com/vishvananda/netlink"

/*
func electBridgeNetwork(config *Configuration) (*net.IPNet, error) {
	// Is a bridge IP is provided as part of the configuration, we only check
	// its validity.
	if config.AddressIPv4 != "" {
		ip, network, err := net.ParseCIDR(config.AddressIPv4)
		if err != nil {
			return nil, err
		}
		network.IP = ip
		return network, nil
	}

	// No bridge IP was specified: we have to elect one ourselves from a set of
	// predetermined networks.
	for _, n := range bridgeNetworks {
		// TODO CheckNameserverOverlaps
		// TODO CheckRouteOverlaps
		return n, nil
	}

	return nil, fmt.Errorf("Couldn't find an address range for interface %q", config.BridgeName)
}

func createBridgeInterface(name string) (netlink.Link, error) {
	link := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: name,
		},
	}

	// Only set the bridge's MAC address if the kernel version is > 3.3, as it
	// was not supported before that.
	kv, err := kernel.GetKernelVersion()
	if err == nil && (kv.Kernel >= 3 && kv.Major >= 3) {
		link.Attrs().HardwareAddr = generateRandomMAC()
		log.Debugf("Setting bridge mac address to %s", link.Attrs().HardwareAddr)
	}

	if err := netlink.LinkAdd(link); err != nil {
		return nil, err
	}
	return netlink.LinkByName(name)
}
*/

func getInterfaceAddr(iface netlink.Link) (netlink.Addr, []netlink.Addr, error) {
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

func getInterfaceAddrByName(ifaceName string) (netlink.Addr, []netlink.Addr, error) {
	iface, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return netlink.Addr{}, nil, err
	}
	return getInterfaceAddr(iface)
}
