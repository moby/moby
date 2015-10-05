package libnetwork

import (
	"encoding/json"
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
	Address() net.IPNet

	// AddressIPv6 returns the IPv6 address assigned to the endpoint.
	AddressIPv6() net.IPNet
}

type endpointInterface struct {
	mac       net.HardwareAddr
	addr      net.IPNet
	addrv6    net.IPNet
	srcName   string
	dstPrefix string
	routes    []*net.IPNet
}

func (epi *endpointInterface) MarshalJSON() ([]byte, error) {
	epMap := make(map[string]interface{})
	epMap["mac"] = epi.mac.String()
	epMap["addr"] = epi.addr.String()
	epMap["addrv6"] = epi.addrv6.String()
	epMap["srcName"] = epi.srcName
	epMap["dstPrefix"] = epi.dstPrefix
	var routes []string
	for _, route := range epi.routes {
		routes = append(routes, route.String())
	}
	epMap["routes"] = routes
	return json.Marshal(epMap)
}

func (epi *endpointInterface) UnmarshalJSON(b []byte) (err error) {
	var epMap map[string]interface{}
	if err := json.Unmarshal(b, &epMap); err != nil {
		return err
	}

	mac, _ := net.ParseMAC(epMap["mac"].(string))
	epi.mac = mac

	ip, ipnet, _ := net.ParseCIDR(epMap["addr"].(string))
	if ipnet != nil {
		ipnet.IP = ip
		epi.addr = *ipnet
	}

	ip, ipnet, _ = net.ParseCIDR(epMap["addrv6"].(string))
	if ipnet != nil {
		ipnet.IP = ip
		epi.addrv6 = *ipnet
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

	return nil
}

type endpointJoinInfo struct {
	gw           net.IP
	gw6          net.IP
	StaticRoutes []*types.StaticRoute
}

func (ep *endpoint) Info() EndpointInfo {
	return ep
}

func (ep *endpoint) DriverInfo() (map[string]interface{}, error) {
	ep.Lock()
	network := ep.network
	epid := ep.id
	ep.Unlock()

	network.Lock()
	driver := network.driver
	nid := network.id
	network.Unlock()

	return driver.EndpointOperInfo(nid, epid)
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

func (ep *endpoint) AddInterface(mac net.HardwareAddr, ipv4 net.IPNet, ipv6 net.IPNet) error {
	ep.Lock()
	defer ep.Unlock()

	iface := &endpointInterface{
		addr:   *types.GetIPNetCopy(&ipv4),
		addrv6: *types.GetIPNetCopy(&ipv6),
	}
	iface.mac = types.GetMacCopy(mac)

	ep.iface = iface
	return nil
}

func (epi *endpointInterface) MacAddress() net.HardwareAddr {
	return types.GetMacCopy(epi.mac)
}

func (epi *endpointInterface) Address() net.IPNet {
	return (*types.GetIPNetCopy(&epi.addr))
}

func (epi *endpointInterface) AddressIPv6() net.IPNet {
	return (*types.GetIPNetCopy(&epi.addrv6))
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
