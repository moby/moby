package sandbox

import (
	"fmt"
	"net"

	"github.com/docker/libnetwork/driverapi"
	"github.com/vishvananda/netlink"
)

func configureInterface(iface netlink.Link, settings *driverapi.Interface) error {
	ifaceName := iface.Attrs().Name
	ifaceConfigurators := []struct {
		Fn         func(netlink.Link, *driverapi.Interface) error
		ErrMessage string
	}{
		{setInterfaceName, fmt.Sprintf("error renaming interface %q to %q", ifaceName, settings.DstName)},
		{setInterfaceIP, fmt.Sprintf("error setting interface %q IP to %q", ifaceName, settings.Address)},
		{setInterfaceIPv6, fmt.Sprintf("error setting interface %q IPv6 to %q", ifaceName, settings.AddressIPv6)},
	}

	for _, config := range ifaceConfigurators {
		if err := config.Fn(iface, settings); err != nil {
			return fmt.Errorf("%s: %v", config.ErrMessage, err)
		}
	}
	return nil
}

func setGatewayIP(gw net.IP) error {
	return netlink.RouteAdd(&netlink.Route{
		Scope: netlink.SCOPE_UNIVERSE,
		Gw:    gw,
	})
}

func setInterfaceIP(iface netlink.Link, settings *driverapi.Interface) error {
	ipAddr := &netlink.Addr{IPNet: settings.Address, Label: ""}
	return netlink.AddrAdd(iface, ipAddr)
}

func setInterfaceIPv6(iface netlink.Link, settings *driverapi.Interface) error {
	ipAddr := &netlink.Addr{IPNet: settings.Address, Label: ""}
	return netlink.AddrAdd(iface, ipAddr)
}

func setInterfaceName(iface netlink.Link, settings *driverapi.Interface) error {
	return netlink.LinkSetName(iface, settings.DstName)
}
