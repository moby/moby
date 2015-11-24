package libnetwork

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/types"
)

// EndpointInfo provides an interface to retrieve network resources bound to the endpoint.
type EndpointInfo interface {
	// Iface returns InterfaceInfo, go interface that can be used
	// to get more information on the interface which was assigned to
	// the endpoint by the driver. This can be used after the
	// endpoint has been created.
	Iface() InterfaceInfo

	// Gateway returns the IPv4 gateway assigned by the driver.
	// This will only return a valid value if a container has joined the endpoint.
	Gateway() net.IP

	// GatewayIPv6 returns the IPv6 gateway assigned by the driver.
	// This will only return a valid value if a container has joined the endpoint.
	GatewayIPv6() net.IP

	// Sandbox returns the attached sandbox if there, nil otherwise.
	Sandbox() Sandbox
}

// InterfaceInfo provides an interface to retrieve interface addresses bound to the endpoint.
type InterfaceInfo interface {
	// MacAddress returns the MAC address assigned to the endpoint.
	MacAddress() net.HardwareAddr

	// Address returns the IPv4 address assigned to the endpoint.
	Address() *net.IPNet

	// AddressIPv6 returns the IPv6 address assigned to the endpoint.
	AddressIPv6() *net.IPNet
}

type endpointInterface struct {
	mac       net.HardwareAddr
	addr      *net.IPNet
	addrv6    *net.IPNet
	srcName   string
	dstPrefix string
	routes    []*net.IPNet
	v4PoolID  string
	v6PoolID  string
}

func (epi *endpointInterface) MarshalJSON() ([]byte, error) {
	epMap := make(map[string]interface{})
	if epi.mac != nil {
		epMap["mac"] = epi.mac.String()
	}
	if epi.addr != nil {
		epMap["addr"] = epi.addr.String()
	}
	if epi.addrv6 != nil {
		epMap["addrv6"] = epi.addrv6.String()
	}
	epMap["srcName"] = epi.srcName
	epMap["dstPrefix"] = epi.dstPrefix
	var routes []string
	for _, route := range epi.routes {
		routes = append(routes, route.String())
	}
	epMap["routes"] = routes
	epMap["v4PoolID"] = epi.v4PoolID
	epMap["v6PoolID"] = epi.v6PoolID
	return json.Marshal(epMap)
}

func (epi *endpointInterface) UnmarshalJSON(b []byte) error {
	var (
		err   error
		epMap map[string]interface{}
	)
	if err = json.Unmarshal(b, &epMap); err != nil {
		return err
	}
	if v, ok := epMap["mac"]; ok {
		if epi.mac, err = net.ParseMAC(v.(string)); err != nil {
			return types.InternalErrorf("failed to decode endpoint interface mac address after json unmarshal: %s", v.(string))
		}
	}
	if v, ok := epMap["addr"]; ok {
		if epi.addr, err = types.ParseCIDR(v.(string)); err != nil {
			return types.InternalErrorf("failed to decode endpoint interface ipv4 address after json unmarshal: %v", err)
		}
	}
	if v, ok := epMap["addrv6"]; ok {
		if epi.addrv6, err = types.ParseCIDR(v.(string)); err != nil {
			return types.InternalErrorf("failed to decode endpoint interface ipv6 address after json unmarshal: %v", err)
		}
	}

	epi.srcName = epMap["srcName"].(string)
	epi.dstPrefix = epMap["dstPrefix"].(string)

	rb, _ := json.Marshal(epMap["routes"])
	var routes []string
	json.Unmarshal(rb, &routes)
	epi.routes = make([]*net.IPNet, 0)
	for _, route := range routes {
		ip, ipr, err := net.ParseCIDR(route)
		if err == nil {
			ipr.IP = ip
			epi.routes = append(epi.routes, ipr)
		}
	}
	epi.v4PoolID = epMap["v4PoolID"].(string)
	epi.v6PoolID = epMap["v6PoolID"].(string)

	return nil
}

func (epi *endpointInterface) CopyTo(dstEpi *endpointInterface) error {
	dstEpi.mac = types.GetMacCopy(epi.mac)
	dstEpi.addr = types.GetIPNetCopy(epi.addr)
	dstEpi.addrv6 = types.GetIPNetCopy(epi.addrv6)
	dstEpi.srcName = epi.srcName
	dstEpi.dstPrefix = epi.dstPrefix
	dstEpi.v4PoolID = epi.v4PoolID
	dstEpi.v6PoolID = epi.v6PoolID

	for _, route := range epi.routes {
		dstEpi.routes = append(dstEpi.routes, types.GetIPNetCopy(route))
	}

	return nil
}

type endpointJoinInfo struct {
	gw           net.IP
	gw6          net.IP
	StaticRoutes []*types.StaticRoute
}

func (ep *endpoint) Info() EndpointInfo {
	n, err := ep.getNetworkFromStore()
	if err != nil {
		return nil
	}

	ep, err = n.getEndpointFromStore(ep.ID())
	if err != nil {
		return nil
	}

	sb, ok := ep.getSandbox()
	if !ok {
		// endpoint hasn't joined any sandbox.
		// Just return the endpoint
		return ep
	}

	if epi := sb.getEndpoint(ep.ID()); epi != nil {
		return epi
	}

	return nil
}

func (ep *endpoint) DriverInfo() (map[string]interface{}, error) {
	ep, err := ep.retrieveFromStore()
	if err != nil {
		return nil, err
	}

	if sb, ok := ep.getSandbox(); ok {
		if gwep := sb.getEndpointInGWNetwork(); gwep != nil && gwep.ID() != ep.ID() {
			return gwep.DriverInfo()
		}
	}

	n, err := ep.getNetworkFromStore()
	if err != nil {
		return nil, fmt.Errorf("could not find network in store for driver info: %v", err)
	}

	driver, err := n.driver()
	if err != nil {
		return nil, fmt.Errorf("failed to get driver info: %v", err)
	}

	return driver.EndpointOperInfo(n.ID(), ep.ID())
}

func (ep *endpoint) Iface() InterfaceInfo {
	ep.Lock()
	defer ep.Unlock()

	if ep.iface != nil {
		return ep.iface
	}

	return nil
}

func (ep *endpoint) Interface() driverapi.InterfaceInfo {
	ep.Lock()
	defer ep.Unlock()

	if ep.iface != nil {
		return ep.iface
	}

	return nil
}

func (epi *endpointInterface) SetMacAddress(mac net.HardwareAddr) error {
	if epi.mac != nil {
		return types.ForbiddenErrorf("endpoint interface MAC address present (%s). Cannot be modified with %s.", epi.mac, mac)
	}
	if mac == nil {
		return types.BadRequestErrorf("tried to set nil MAC address to endpoint interface")
	}
	epi.mac = types.GetMacCopy(mac)
	return nil
}

func (epi *endpointInterface) SetIPAddress(address *net.IPNet) error {
	if address.IP == nil {
		return types.BadRequestErrorf("tried to set nil IP address to endpoint interface")
	}
	if address.IP.To4() == nil {
		return setAddress(&epi.addrv6, address)
	}
	return setAddress(&epi.addr, address)
}

func setAddress(ifaceAddr **net.IPNet, address *net.IPNet) error {
	if *ifaceAddr != nil {
		return types.ForbiddenErrorf("endpoint interface IP present (%s). Cannot be modified with (%s).", *ifaceAddr, address)
	}
	*ifaceAddr = types.GetIPNetCopy(address)
	return nil
}

func (epi *endpointInterface) MacAddress() net.HardwareAddr {
	return types.GetMacCopy(epi.mac)
}

func (epi *endpointInterface) Address() *net.IPNet {
	return types.GetIPNetCopy(epi.addr)
}

func (epi *endpointInterface) AddressIPv6() *net.IPNet {
	return types.GetIPNetCopy(epi.addrv6)
}

func (epi *endpointInterface) SetNames(srcName string, dstPrefix string) error {
	epi.srcName = srcName
	epi.dstPrefix = dstPrefix
	return nil
}

func (ep *endpoint) InterfaceName() driverapi.InterfaceNameInfo {
	ep.Lock()
	defer ep.Unlock()

	if ep.iface != nil {
		return ep.iface
	}

	return nil
}

func (ep *endpoint) AddStaticRoute(destination *net.IPNet, routeType int, nextHop net.IP) error {
	ep.Lock()
	defer ep.Unlock()

	r := types.StaticRoute{Destination: destination, RouteType: routeType, NextHop: nextHop}

	if routeType == types.NEXTHOP {
		// If the route specifies a next-hop, then it's loosely routed (i.e. not bound to a particular interface).
		ep.joinInfo.StaticRoutes = append(ep.joinInfo.StaticRoutes, &r)
	} else {
		// If the route doesn't specify a next-hop, it must be a connected route, bound to an interface.
		ep.iface.routes = append(ep.iface.routes, r.Destination)
	}
	return nil
}

func (ep *endpoint) Sandbox() Sandbox {
	cnt, ok := ep.getSandbox()
	if !ok {
		return nil
	}
	return cnt
}

func (ep *endpoint) Gateway() net.IP {
	ep.Lock()
	defer ep.Unlock()

	if ep.joinInfo == nil {
		return net.IP{}
	}

	return types.GetIPCopy(ep.joinInfo.gw)
}

func (ep *endpoint) GatewayIPv6() net.IP {
	ep.Lock()
	defer ep.Unlock()

	if ep.joinInfo == nil {
		return net.IP{}
	}

	return types.GetIPCopy(ep.joinInfo.gw6)
}

func (ep *endpoint) SetGateway(gw net.IP) error {
	ep.Lock()
	defer ep.Unlock()

	ep.joinInfo.gw = types.GetIPCopy(gw)
	return nil
}

func (ep *endpoint) SetGatewayIPv6(gw6 net.IP) error {
	ep.Lock()
	defer ep.Unlock()

	ep.joinInfo.gw6 = types.GetIPCopy(gw6)
	return nil
}

func (ep *endpoint) retrieveFromStore() (*endpoint, error) {
	n, err := ep.getNetworkFromStore()
	if err != nil {
		return nil, fmt.Errorf("could not find network in store to get latest endpoint %s: %v", ep.Name(), err)
	}
	return n.getEndpointFromStore(ep.ID())
}
