package osl

import (
	"fmt"
	"net"

	"github.com/docker/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
)

// Gateway returns the IPv4 gateway for the sandbox.
func (n *Namespace) Gateway() net.IP {
	n.Lock()
	defer n.Unlock()

	return n.gw
}

// GatewayIPv6 returns the IPv6 gateway for the sandbox.
func (n *Namespace) GatewayIPv6() net.IP {
	n.Lock()
	defer n.Unlock()

	return n.gwv6
}

// StaticRoutes returns additional static routes for the sandbox. Note that
// directly connected routes are stored on the particular interface they
// refer to.
func (n *Namespace) StaticRoutes() []*types.StaticRoute {
	n.Lock()
	defer n.Unlock()

	routes := make([]*types.StaticRoute, len(n.staticRoutes))
	for i, route := range n.staticRoutes {
		r := route.GetCopy()
		routes[i] = r
	}

	return routes
}

func (n *Namespace) setGateway(gw net.IP) {
	n.Lock()
	n.gw = gw
	n.Unlock()
}

func (n *Namespace) setGatewayIPv6(gwv6 net.IP) {
	n.Lock()
	n.gwv6 = gwv6
	n.Unlock()
}

// SetGateway sets the default IPv4 gateway for the sandbox.
func (n *Namespace) SetGateway(gw net.IP) error {
	// Silently return if the gateway is empty
	if len(gw) == 0 {
		return nil
	}

	err := n.programGateway(gw, true)
	if err == nil {
		n.setGateway(gw)
	}

	return err
}

// UnsetGateway the previously set default IPv4 gateway in the sandbox.
func (n *Namespace) UnsetGateway() error {
	gw := n.Gateway()

	// Silently return if the gateway is empty
	if len(gw) == 0 {
		return nil
	}

	err := n.programGateway(gw, false)
	if err == nil {
		n.setGateway(net.IP{})
	}

	return err
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
		return fmt.Errorf("Direct route for the gateway %s could not be found", gw)
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
func (n *Namespace) programRoute(path string, dest *net.IPNet, nh net.IP) error {
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
func (n *Namespace) removeRoute(path string, dest *net.IPNet, nh net.IP) error {
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

// SetGatewayIPv6 sets the default IPv6 gateway for the sandbox.
func (n *Namespace) SetGatewayIPv6(gwv6 net.IP) error {
	// Silently return if the gateway is empty
	if len(gwv6) == 0 {
		return nil
	}

	err := n.programGateway(gwv6, true)
	if err == nil {
		n.setGatewayIPv6(gwv6)
	}

	return err
}

// UnsetGatewayIPv6 unsets the previously set default IPv6 gateway in the sandbox.
func (n *Namespace) UnsetGatewayIPv6() error {
	gwv6 := n.GatewayIPv6()

	// Silently return if the gateway is empty
	if len(gwv6) == 0 {
		return nil
	}

	err := n.programGateway(gwv6, false)
	if err == nil {
		n.Lock()
		n.gwv6 = net.IP{}
		n.Unlock()
	}

	return err
}

// AddStaticRoute adds a static route to the sandbox.
func (n *Namespace) AddStaticRoute(r *types.StaticRoute) error {
	err := n.programRoute(n.nsPath(), r.Destination, r.NextHop)
	if err == nil {
		n.Lock()
		n.staticRoutes = append(n.staticRoutes, r)
		n.Unlock()
	}
	return err
}

// RemoveStaticRoute removes a static route from the sandbox.
func (n *Namespace) RemoveStaticRoute(r *types.StaticRoute) error {
	err := n.removeRoute(n.nsPath(), r.Destination, r.NextHop)
	if err == nil {
		n.Lock()
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
		n.Unlock()
	}
	return err
}
