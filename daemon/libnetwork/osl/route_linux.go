//go:build linux || freebsd

package osl

import (
	"errors"
	"fmt"
	"net"
	"slices"

	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"github.com/vishvananda/netlink"
)

// Gateway returns the IPv4 gateway for the sandbox.
func (n *Namespace) Gateway() net.IP {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.gw
}

// GatewayIPv6 returns the IPv6 gateway for the sandbox.
func (n *Namespace) GatewayIPv6() net.IP {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.gwv6
}

// StaticRoutes returns additional static routes for the sandbox. Note that
// directly connected routes are stored on the particular interface they
// refer to.
func (n *Namespace) StaticRoutes() []*types.StaticRoute {
	n.mu.Lock()
	defer n.mu.Unlock()

	routes := make([]*types.StaticRoute, len(n.staticRoutes))
	for i, route := range n.staticRoutes {
		routes[i] = route.Copy()
	}

	return routes
}

// SetGateway sets the default IPv4 gateway for the sandbox. It is a no-op
// if the given gateway is empty.
func (n *Namespace) SetGateway(gw net.IP) error {
	if len(gw) == 0 {
		return nil
	}

	if err := n.programGateway(gw, true); err != nil {
		return err
	}
	n.mu.Lock()
	n.gw = gw
	n.mu.Unlock()
	return nil
}

// UnsetGateway the previously set default IPv4 gateway in the sandbox.
// It is a no-op if no gateway was set.
func (n *Namespace) UnsetGateway() error {
	gw := n.Gateway()
	if len(gw) == 0 {
		return nil
	}

	if err := n.programGateway(gw, false); err != nil {
		return err
	}
	n.mu.Lock()
	n.gw = net.IP{}
	n.mu.Unlock()
	return nil
}

func (n *Namespace) programGateway(gw net.IP, isAdd bool) error {
	gwRoutes, err := n.nlHandle.RouteGet(gw)
	if err != nil {
		return fmt.Errorf("route for the gateway %s could not be found: %v", gw, err)
	}

	var linkIndex int
	for _, gwRoute := range gwRoutes {
		if gwRoute.Gw == nil {
			linkIndex = gwRoute.LinkIndex
			break
		}
	}

	if linkIndex == 0 {
		return fmt.Errorf("direct route for the gateway %s could not be found", gw)
	}

	if isAdd {
		return n.nlHandle.RouteAdd(&netlink.Route{
			Scope:     netlink.SCOPE_UNIVERSE,
			LinkIndex: linkIndex,
			Gw:        gw,
		})
	}

	return n.nlHandle.RouteDel(&netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: linkIndex,
		Gw:        gw,
	})
}

// Program a route in to the namespace routing table.
func (n *Namespace) programRoute(dest *net.IPNet, nh net.IP) error {
	gwRoutes, err := n.nlHandle.RouteGet(nh)
	if err != nil {
		return fmt.Errorf("route for the next hop %s could not be found: %v", nh, err)
	}

	return n.nlHandle.RouteAdd(&netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: gwRoutes[0].LinkIndex,
		Gw:        nh,
		Dst:       dest,
	})
}

// Delete a route from the namespace routing table.
func (n *Namespace) removeRoute(dest *net.IPNet, nh net.IP) error {
	gwRoutes, err := n.nlHandle.RouteGet(nh)
	if err != nil {
		return fmt.Errorf("route for the next hop could not be found: %v", err)
	}

	return n.nlHandle.RouteDel(&netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: gwRoutes[0].LinkIndex,
		Gw:        nh,
		Dst:       dest,
	})
}

// SetGatewayIPv6 sets the default IPv6 gateway for the sandbox. It is a no-op
// if the given gateway is empty.
func (n *Namespace) SetGatewayIPv6(gwv6 net.IP) error {
	if len(gwv6) == 0 {
		return nil
	}

	if err := n.programGateway(gwv6, true); err != nil {
		return err
	}

	n.mu.Lock()
	n.gwv6 = gwv6
	n.mu.Unlock()
	return nil
}

// UnsetGatewayIPv6 unsets the previously set default IPv6 gateway in the sandbox.
// It is a no-op if no gateway was set.
func (n *Namespace) UnsetGatewayIPv6() error {
	gwv6 := n.GatewayIPv6()
	if len(gwv6) == 0 {
		return nil
	}

	if err := n.programGateway(gwv6, false); err != nil {
		return err
	}

	n.mu.Lock()
	n.gwv6 = net.IP{}
	n.mu.Unlock()
	return nil
}

// AddStaticRoute adds a static route to the sandbox.
func (n *Namespace) AddStaticRoute(r *types.StaticRoute) error {
	if err := n.programRoute(r.Destination, r.NextHop); err != nil {
		return err
	}

	n.mu.Lock()
	n.staticRoutes = append(n.staticRoutes, r)
	n.mu.Unlock()
	return nil
}

// RemoveStaticRoute removes a static route from the sandbox.
func (n *Namespace) RemoveStaticRoute(r *types.StaticRoute) error {
	if err := n.removeRoute(r.Destination, r.NextHop); err != nil {
		return err
	}

	n.mu.Lock()
	lastIndex := len(n.staticRoutes) - 1
	for i, v := range n.staticRoutes {
		if v == r {
			// Overwrite the route we're removing with the last element
			n.staticRoutes[i] = n.staticRoutes[lastIndex]
			// Shorten the slice to trim the extra element
			n.staticRoutes = n.staticRoutes[:lastIndex]
			break
		}
	}
	n.mu.Unlock()
	return nil
}

// SetDefaultRouteIPv4 sets up a connected route to 0.0.0.0 via the Interface
// with srcName, if that Interface has a route to 0.0.0.0. Otherwise, it
// returns an error.
func (n *Namespace) SetDefaultRouteIPv4(srcName string) error {
	if err := n.setDefaultRoute(srcName, func(ipNet *net.IPNet) bool {
		return ipNet.IP.IsUnspecified() && ipNet.IP.To4() != nil
	}); err != nil {
		return fmt.Errorf("setting IPv4 default route to interface with srcName '%s': %w", srcName, err)
	}

	n.mu.Lock()
	n.defRoute4SrcName = srcName
	n.mu.Unlock()
	return nil
}

// SetDefaultRouteIPv6 sets up a connected route to [::] via the Interface
// with srcName, if that Interface has a route to [::]. Otherwise, it
// returns an error.
func (n *Namespace) SetDefaultRouteIPv6(srcName string) error {
	if err := n.setDefaultRoute(srcName, func(ipNet *net.IPNet) bool {
		return ipNet.IP.IsUnspecified() && ipNet.IP.To4() == nil
	}); err != nil {
		return fmt.Errorf("setting IPv6 default route to interface with srcName '%s': %w", srcName, err)
	}

	n.mu.Lock()
	n.defRoute6SrcName = srcName
	n.mu.Unlock()
	return nil
}

func (n *Namespace) setDefaultRoute(srcName string, routeMatcher func(*net.IPNet) bool) error {
	iface := n.ifaceBySrcName(srcName)
	if iface == nil {
		return errors.New("no interface")
	}

	ridx := slices.IndexFunc(iface.routes, routeMatcher)
	if ridx == -1 {
		return errors.New("no default route")
	}

	link, err := n.nlHandle.LinkByName(iface.dstName)
	if err != nil {
		return fmt.Errorf("no link src:%s dst:%s", srcName, iface.dstName)
	}

	if err := n.nlHandle.RouteAdd(&netlink.Route{
		Scope:     netlink.SCOPE_LINK,
		LinkIndex: link.Attrs().Index,
		Dst:       iface.routes[ridx],
	}); err != nil {
		return err
	}
	return nil
}

// UnsetDefaultRouteIPv4 unsets the previously set default IPv4 default route
// in the sandbox. It is a no-op if no gateway was set.
func (n *Namespace) UnsetDefaultRouteIPv4() error {
	n.mu.Lock()
	srcName := n.defRoute4SrcName
	n.mu.Unlock()

	if err := n.unsetDefaultRoute(srcName, func(ipNet *net.IPNet) bool {
		return ipNet.IP.IsUnspecified() && ipNet.IP.To4() != nil
	}); err != nil {
		return fmt.Errorf("removing IPv4 default route to interface with srcName '%s': %w", srcName, err)
	}

	n.mu.Lock()
	n.defRoute4SrcName = ""
	n.mu.Unlock()
	return nil
}

// UnsetDefaultRouteIPv6 unsets the previously set default IPv6 default route
// in the sandbox. It is a no-op if no gateway was set.
func (n *Namespace) UnsetDefaultRouteIPv6() error {
	n.mu.Lock()
	srcName := n.defRoute6SrcName
	n.mu.Unlock()

	if err := n.unsetDefaultRoute(srcName, func(ipNet *net.IPNet) bool {
		return ipNet.IP.IsUnspecified() && ipNet.IP.To4() == nil
	}); err != nil {
		return fmt.Errorf("removing IPv6 default route to interface with srcName '%s': %w", srcName, err)
	}

	n.mu.Lock()
	n.defRoute6SrcName = ""
	n.mu.Unlock()
	return nil
}

func (n *Namespace) unsetDefaultRoute(srcName string, routeMatcher func(*net.IPNet) bool) error {
	if srcName == "" {
		return nil
	}

	iface := n.ifaceBySrcName(srcName)
	if iface == nil {
		return nil
	}

	ridx := slices.IndexFunc(iface.routes, routeMatcher)
	if ridx == -1 {
		return errors.New("no default route")
	}

	link, err := n.nlHandle.LinkByName(iface.dstName)
	if err != nil {
		return errors.New("no link")
	}

	return n.nlHandle.RouteDel(&netlink.Route{
		Scope:     netlink.SCOPE_LINK,
		LinkIndex: link.Attrs().Index,
		Dst:       iface.routes[ridx],
	})
}
