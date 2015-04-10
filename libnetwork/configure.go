package libnetwork

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

func configureInterface(iface netlink.Link, settings *Interface) error {
	ifaceName := iface.Attrs().Name
	ifaceConfigurators := []struct {
		Fn         func(netlink.Link, *Interface) error
		ErrMessage string
	}{
		{setInterfaceName, fmt.Sprintf("error renaming interface %q to %q", ifaceName, settings.DstName)},
		{setInterfaceIP, fmt.Sprintf("error setting interface %q IP to %q", ifaceName, settings.Address)},
		{setInterfaceIPv6, fmt.Sprintf("error setting interface %q IPv6 to %q", ifaceName, settings.AddressIPv6)},
		{setInterfaceGateway, fmt.Sprintf("error setting interface %q gateway to %q", ifaceName, settings.Gateway)},
		{setInterfaceGatewayIPv6, fmt.Sprintf("error setting interface %q IPv6 gateway to %q", ifaceName, settings.GatewayIPv6)},
	}

	for _, config := range ifaceConfigurators {
		if err := config.Fn(iface, settings); err != nil {
			return fmt.Errorf("%s: %v", config.ErrMessage, err)
		}
	}
	return nil
}

func setGatewayIP(iface netlink.Link, ip net.IP) error {
	return netlink.RouteAdd(&netlink.Route{
		LinkIndex: iface.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Gw:        ip,
	})
}

func setInterfaceGateway(iface netlink.Link, settings *Interface) error {
	ip := net.ParseIP(settings.Gateway)
	if ip == nil {
		return fmt.Errorf("bad address format %q", settings.Gateway)
	}
	return setGatewayIP(iface, ip)
}

func setInterfaceGatewayIPv6(iface netlink.Link, settings *Interface) error {
	if settings.GatewayIPv6 != "" {
		return nil
	}

	ip := net.ParseIP(settings.GatewayIPv6)
	if ip == nil {
		return fmt.Errorf("bad address format %q", settings.GatewayIPv6)
	}
	return setGatewayIP(iface, ip)
}

func setInterfaceIP(iface netlink.Link, settings *Interface) (err error) {
	var ipAddr *netlink.Addr
	if ipAddr, err = netlink.ParseAddr(settings.Address); err == nil {
		err = netlink.AddrAdd(iface, ipAddr)
	}
	return err
}

func setInterfaceIPv6(iface netlink.Link, settings *Interface) (err error) {
	if settings.AddressIPv6 != "" {
		return nil
	}

	var ipAddr *netlink.Addr
	if ipAddr, err = netlink.ParseAddr(settings.AddressIPv6); err == nil {
		err = netlink.AddrAdd(iface, ipAddr)
	}
	return err
}

func setInterfaceName(iface netlink.Link, settings *Interface) error {
	return netlink.LinkSetName(iface, settings.DstName)
}
