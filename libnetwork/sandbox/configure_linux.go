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
		/*		{setInterfaceGateway, fmt.Sprintf("error setting interface %q gateway to %q", ifaceName, settings.Gateway)},
				{setInterfaceGatewayIPv6, fmt.Sprintf("error setting interface %q IPv6 gateway to %q", ifaceName, settings.GatewayIPv6)}, */
	}

	for _, config := range ifaceConfigurators {
		if err := config.Fn(iface, settings); err != nil {
			return fmt.Errorf("%s: %v", config.ErrMessage, err)
		}
	}
	return nil
}

func setGatewayIP(gw string) error {
	ip := net.ParseIP(gw)
	if ip == nil {
		return fmt.Errorf("bad address format %q", gw)
	}

	return netlink.RouteAdd(&netlink.Route{
		Scope: netlink.SCOPE_UNIVERSE,
		Gw:    ip,
	})
}

func setInterfaceIP(iface netlink.Link, settings *driverapi.Interface) error {
	ipAddr, err := netlink.ParseAddr(settings.Address)
	if err == nil {
		err = netlink.AddrAdd(iface, ipAddr)
	}
	return err
}

func setInterfaceIPv6(iface netlink.Link, settings *driverapi.Interface) error {
	if settings.AddressIPv6 == "" {
		return nil
	}

	ipAddr, err := netlink.ParseAddr(settings.AddressIPv6)
	if err == nil {
		err = netlink.AddrAdd(iface, ipAddr)
	}
	return err
}

func setInterfaceName(iface netlink.Link, settings *driverapi.Interface) error {
	return netlink.LinkSetName(iface, settings.DstName)
}
