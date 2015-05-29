package sandbox

import (
	"fmt"
	"net"
	"os"
	"runtime"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
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
		{setInterfaceRoutes, fmt.Sprintf("error setting interface %q routes to %q", ifaceName, settings.Routes)},
	}

	for _, config := range ifaceConfigurators {
		if err := config.Fn(iface, settings); err != nil {
			return fmt.Errorf("%s: %v", config.ErrMessage, err)
		}
	}
	return nil
}

func programGateway(path string, gw net.IP) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origns, err := netns.Get()
	if err != nil {
		return err
	}
	defer origns.Close()

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed get network namespace %q: %v", path, err)
	}
	defer f.Close()

	nsFD := f.Fd()
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		return err
	}
	defer netns.Set(origns)

	gwRoutes, err := netlink.RouteGet(gw)
	if err != nil {
		return fmt.Errorf("route for the gateway could not be found: %v", err)
	}

	return netlink.RouteAdd(&netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: gwRoutes[0].LinkIndex,
		Gw:        gw,
	})
}

// Program a route in to the namespace routing table.
func programRoute(path string, dest *net.IPNet, nh net.IP) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origns, err := netns.Get()
	if err != nil {
		return err
	}
	defer origns.Close()

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed get network namespace %q: %v", path, err)
	}
	defer f.Close()

	nsFD := f.Fd()
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		return err
	}
	defer netns.Set(origns)

	gwRoutes, err := netlink.RouteGet(nh)
	if err != nil {
		return fmt.Errorf("route for the next hop could not be found: %v", err)
	}

	return netlink.RouteAdd(&netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: gwRoutes[0].LinkIndex,
		Gw:        gwRoutes[0].Gw,
		Dst:       dest,
	})
}

// Delete a route from the namespace routing table.
func removeRoute(path string, dest *net.IPNet, nh net.IP) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origns, err := netns.Get()
	if err != nil {
		return err
	}
	defer origns.Close()

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed get network namespace %q: %v", path, err)
	}
	defer f.Close()

	nsFD := f.Fd()
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		return err
	}
	defer netns.Set(origns)

	gwRoutes, err := netlink.RouteGet(nh)
	if err != nil {
		return fmt.Errorf("route for the next hop could not be found: %v", err)
	}

	return netlink.RouteDel(&netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: gwRoutes[0].LinkIndex,
		Gw:        gwRoutes[0].Gw,
		Dst:       dest,
	})
}

func setInterfaceIP(iface netlink.Link, settings *Interface) error {
	ipAddr := &netlink.Addr{IPNet: settings.Address, Label: ""}
	return netlink.AddrAdd(iface, ipAddr)
}

func setInterfaceIPv6(iface netlink.Link, settings *Interface) error {
	if settings.AddressIPv6 == nil {
		return nil
	}
	ipAddr := &netlink.Addr{IPNet: settings.AddressIPv6, Label: ""}
	return netlink.AddrAdd(iface, ipAddr)
}

func setInterfaceName(iface netlink.Link, settings *Interface) error {
	return netlink.LinkSetName(iface, settings.DstName)
}

func setInterfaceRoutes(iface netlink.Link, settings *Interface) error {
	for _, route := range settings.Routes {
		err := netlink.RouteAdd(&netlink.Route{
			Scope:     netlink.SCOPE_LINK,
			LinkIndex: iface.Attrs().Index,
			Dst:       route,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
