package libnetwork

import (
	"encoding/json"
	"net"

	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/types"
)

// EndpointInfo provides an interface to retrieve network resources bound to the endpoint.
type EndpointInfo interface {
	// InterfaceList returns an interface list which were assigned to the endpoint
	// by the driver. This can be used after the endpoint has been created.
	InterfaceList() []InterfaceInfo

	// Gateway returns the IPv4 gateway assigned by the driver.
	// This will only return a valid value if a container has joined the endpoint.
	Gateway() net.IP

	// GatewayIPv6 returns the IPv6 gateway assigned by the driver.
	// This will only return a valid value if a container has joined the endpoint.
	GatewayIPv6() net.IP

	// SandboxKey returns the sanbox key for the container which has joined
	// the endpoint. If there is no container joined then this will return an
	// empty string.
	SandboxKey() string
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

// ContainerInfo provides an interface to retrieve the info about the container attached to the endpoint
type ContainerInfo interface {
	// ID returns the ID of the container
	ID() string
	// Labels returns the container's labels
	Labels() map[string]interface{}
}

type endpointInterface struct {
	id        int
	mac       net.HardwareAddr
	addr      net.IPNet
	addrv6    net.IPNet
	srcName   string
	dstPrefix string
	routes    []*net.IPNet
}

func (epi *endpointInterface) MarshalJSON() ([]byte, error) {
	epMap := make(map[string]interface{})
	epMap["id"] = epi.id
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
	epi.id = int(epMap["id"].(float64))

	mac, _ := net.ParseMAC(epMap["mac"].(string))
	epi.mac = mac

	_, ipnet, _ := net.ParseCIDR(epMap["addr"].(string))
	if ipnet != nil {
		epi.addr = *ipnet
	}

	_, ipnet, _ = net.ParseCIDR(epMap["addrv6"].(string))
	if ipnet != nil {
		epi.addrv6 = *ipnet
	}

	epi.srcName = epMap["srcName"].(string)
	epi.dstPrefix = epMap["dstPrefix"].(string)

	rb, _ := json.Marshal(epMap["routes"])
	var routes []string
	json.Unmarshal(rb, &routes)
	epi.routes = make([]*net.IPNet, 0)
	for _, route := range routes {
		_, ipr, err := net.ParseCIDR(route)
		if err == nil {
			epi.routes = append(epi.routes, ipr)
		}
	}

	return nil
}

type endpointJoinInfo struct {
	gw             net.IP
	gw6            net.IP
	hostsPath      string
	resolvConfPath string
	StaticRoutes   []*types.StaticRoute
}

func (ep *endpoint) ContainerInfo() ContainerInfo {
	ep.Lock()
	ci := ep.container
	defer ep.Unlock()

	// Need this since we return the interface
	if ci == nil {
		return nil
	}
	return ci
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

func (ep *endpoint) InterfaceList() []InterfaceInfo {
	ep.Lock()
	defer ep.Unlock()

	iList := make([]InterfaceInfo, len(ep.iFaces))

	for i, iface := range ep.iFaces {
		iList[i] = iface
	}

	return iList
}

func (ep *endpoint) Interfaces() []driverapi.InterfaceInfo {
	ep.Lock()
	defer ep.Unlock()

	iList := make([]driverapi.InterfaceInfo, len(ep.iFaces))

	for i, iface := range ep.iFaces {
		iList[i] = iface
	}

	return iList
}

func (ep *endpoint) AddInterface(id int, mac net.HardwareAddr, ipv4 net.IPNet, ipv6 net.IPNet) error {
	ep.Lock()
	defer ep.Unlock()

	iface := &endpointInterface{
		id:     id,
		addr:   *types.GetIPNetCopy(&ipv4),
		addrv6: *types.GetIPNetCopy(&ipv6),
	}
	iface.mac = types.GetMacCopy(mac)

	ep.iFaces = append(ep.iFaces, iface)
	return nil
}

func (epi *endpointInterface) ID() int {
	return epi.id
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

func (ep *endpoint) InterfaceNames() []driverapi.InterfaceNameInfo {
	ep.Lock()
	defer ep.Unlock()

	iList := make([]driverapi.InterfaceNameInfo, len(ep.iFaces))

	for i, iface := range ep.iFaces {
		iList[i] = iface
	}

	return iList
}

func (ep *endpoint) AddStaticRoute(destination *net.IPNet, routeType int, nextHop net.IP, interfaceID int) error {
	ep.Lock()
	defer ep.Unlock()

	r := types.StaticRoute{Destination: destination, RouteType: routeType, NextHop: nextHop, InterfaceID: interfaceID}

	if routeType == types.NEXTHOP {
		// If the route specifies a next-hop, then it's loosely routed (i.e. not bound to a particular interface).
		ep.joinInfo.StaticRoutes = append(ep.joinInfo.StaticRoutes, &r)
	} else {
		// If the route doesn't specify a next-hop, it must be a connected route, bound to an interface.
		if err := ep.addInterfaceRoute(&r); err != nil {
			return err
		}
	}
	return nil
}

func (ep *endpoint) addInterfaceRoute(route *types.StaticRoute) error {
	for _, iface := range ep.iFaces {
		if iface.id == route.InterfaceID {
			iface.routes = append(iface.routes, route.Destination)
			return nil
		}
	}
	return types.BadRequestErrorf("Interface with ID %d doesn't exist.",
		route.InterfaceID)
}

func (ep *endpoint) SandboxKey() string {
	ep.Lock()
	defer ep.Unlock()

	if ep.container == nil {
		return ""
	}

	return ep.container.data.SandboxKey
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

func (ep *endpoint) SetHostsPath(path string) error {
	ep.Lock()
	defer ep.Unlock()

	ep.joinInfo.hostsPath = path
	return nil
}

func (ep *endpoint) SetResolvConfPath(path string) error {
	ep.Lock()
	defer ep.Unlock()

	ep.joinInfo.resolvConfPath = path
	return nil
}
