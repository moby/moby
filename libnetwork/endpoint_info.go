package libnetwork

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/types"
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

	// StaticRoutes returns the list of static routes configured by the network
	// driver when the container joins a network
	StaticRoutes() []*types.StaticRoute

	// Sandbox returns the attached sandbox if there, nil otherwise.
	Sandbox() *Sandbox

	// LoadBalancer returns whether the endpoint is the load balancer endpoint for the network.
	LoadBalancer() bool
}

// InterfaceInfo provides an interface to retrieve interface addresses bound to the endpoint.
type InterfaceInfo interface {
	// MacAddress returns the MAC address assigned to the endpoint.
	MacAddress() net.HardwareAddr

	// Address returns the IPv4 address assigned to the endpoint.
	Address() *net.IPNet

	// AddressIPv6 returns the IPv6 address assigned to the endpoint.
	AddressIPv6() *net.IPNet

	// LinkLocalAddresses returns the list of link-local (IPv4/IPv6) addresses assigned to the endpoint.
	LinkLocalAddresses() []*net.IPNet

	// SrcName returns the name of the interface w/in the container
	SrcName() string
}

type endpointInterface struct {
	mac       net.HardwareAddr
	addr      *net.IPNet
	addrv6    *net.IPNet
	llAddrs   []*net.IPNet
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
	if len(epi.llAddrs) != 0 {
		list := make([]string, 0, len(epi.llAddrs))
		for _, ll := range epi.llAddrs {
			list = append(list, ll.String())
		}
		epMap["llAddrs"] = list
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
	if v, ok := epMap["llAddrs"]; ok {
		list := v.([]interface{})
		epi.llAddrs = make([]*net.IPNet, 0, len(list))
		for _, llS := range list {
			ll, err := types.ParseCIDR(llS.(string))
			if err != nil {
				return types.InternalErrorf("failed to decode endpoint interface link-local address (%v) after json unmarshal: %v", llS, err)
			}
			epi.llAddrs = append(epi.llAddrs, ll)
		}
	}
	epi.srcName = epMap["srcName"].(string)
	epi.dstPrefix = epMap["dstPrefix"].(string)

	rb, _ := json.Marshal(epMap["routes"])
	var routes []string

	// TODO(cpuguy83): linter noticed we don't check the error here... no idea why but it seems like it could introduce problems if we start checking
	json.Unmarshal(rb, &routes) //nolint:errcheck

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
	if len(epi.llAddrs) != 0 {
		dstEpi.llAddrs = make([]*net.IPNet, 0, len(epi.llAddrs))
		dstEpi.llAddrs = append(dstEpi.llAddrs, epi.llAddrs...)
	}

	for _, route := range epi.routes {
		dstEpi.routes = append(dstEpi.routes, types.GetIPNetCopy(route))
	}

	return nil
}

type endpointJoinInfo struct {
	gw                    net.IP
	gw6                   net.IP
	StaticRoutes          []*types.StaticRoute
	driverTableEntries    []*tableEntry
	disableGatewayService bool
}

type tableEntry struct {
	tableName string
	key       string
	value     []byte
}

func (ep *endpoint) Info() EndpointInfo {
	if ep.sandboxID != "" {
		return ep
	}
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

	return sb.getEndpoint(ep.ID())
}

func (ep *endpoint) Iface() InterfaceInfo {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if ep.iface != nil {
		return ep.iface
	}

	return nil
}

func (ep *endpoint) Interface() driverapi.InterfaceInfo {
	ep.mu.Lock()
	defer ep.mu.Unlock()

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

func (epi *endpointInterface) LinkLocalAddresses() []*net.IPNet {
	return epi.llAddrs
}

func (epi *endpointInterface) SrcName() string {
	return epi.srcName
}

func (epi *endpointInterface) SetNames(srcName string, dstPrefix string) error {
	epi.srcName = srcName
	epi.dstPrefix = dstPrefix
	return nil
}

func (ep *endpoint) InterfaceName() driverapi.InterfaceNameInfo {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if ep.iface != nil {
		return ep.iface
	}

	return nil
}

func (ep *endpoint) AddStaticRoute(destination *net.IPNet, routeType int, nextHop net.IP) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

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

func (ep *endpoint) AddTableEntry(tableName, key string, value []byte) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	ep.joinInfo.driverTableEntries = append(ep.joinInfo.driverTableEntries, &tableEntry{
		tableName: tableName,
		key:       key,
		value:     value,
	})

	return nil
}

func (ep *endpoint) Sandbox() *Sandbox {
	cnt, ok := ep.getSandbox()
	if !ok {
		return nil
	}
	return cnt
}

func (ep *endpoint) LoadBalancer() bool {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return ep.loadBalancer
}

func (ep *endpoint) StaticRoutes() []*types.StaticRoute {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if ep.joinInfo == nil {
		return nil
	}

	return ep.joinInfo.StaticRoutes
}

func (ep *endpoint) Gateway() net.IP {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if ep.joinInfo == nil {
		return net.IP{}
	}

	return types.GetIPCopy(ep.joinInfo.gw)
}

func (ep *endpoint) GatewayIPv6() net.IP {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if ep.joinInfo == nil {
		return net.IP{}
	}

	return types.GetIPCopy(ep.joinInfo.gw6)
}

func (ep *endpoint) SetGateway(gw net.IP) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	ep.joinInfo.gw = types.GetIPCopy(gw)
	return nil
}

func (ep *endpoint) SetGatewayIPv6(gw6 net.IP) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

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

func (ep *endpoint) DisableGatewayService() {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	ep.joinInfo.disableGatewayService = true
}

func (epj *endpointJoinInfo) MarshalJSON() ([]byte, error) {
	epMap := make(map[string]interface{})
	if epj.gw != nil {
		epMap["gw"] = epj.gw.String()
	}
	if epj.gw6 != nil {
		epMap["gw6"] = epj.gw6.String()
	}
	epMap["disableGatewayService"] = epj.disableGatewayService
	epMap["StaticRoutes"] = epj.StaticRoutes
	return json.Marshal(epMap)
}

func (epj *endpointJoinInfo) UnmarshalJSON(b []byte) error {
	var (
		err   error
		epMap map[string]interface{}
	)
	if err = json.Unmarshal(b, &epMap); err != nil {
		return err
	}
	if v, ok := epMap["gw"]; ok {
		epj.gw = net.ParseIP(v.(string))
	}
	if v, ok := epMap["gw6"]; ok {
		epj.gw6 = net.ParseIP(v.(string))
	}
	epj.disableGatewayService = epMap["disableGatewayService"].(bool)

	var tStaticRoute []types.StaticRoute
	if v, ok := epMap["StaticRoutes"]; ok {
		tb, _ := json.Marshal(v)
		var tStaticRoute []types.StaticRoute
		// TODO(cpuguy83): Linter caught that we aren't checking errors here
		// I don't know why we aren't other than potentially the data is not always expected to be right?
		// This is why I'm not adding the error check.
		//
		// In any case for posterity please if you figure this out document it or check the error
		json.Unmarshal(tb, &tStaticRoute) //nolint:errcheck
	}
	var StaticRoutes []*types.StaticRoute
	for _, r := range tStaticRoute {
		r := r
		StaticRoutes = append(StaticRoutes, &r)
	}
	epj.StaticRoutes = StaticRoutes

	return nil
}

func (epj *endpointJoinInfo) CopyTo(dstEpj *endpointJoinInfo) error {
	dstEpj.disableGatewayService = epj.disableGatewayService
	dstEpj.StaticRoutes = make([]*types.StaticRoute, len(epj.StaticRoutes))
	copy(dstEpj.StaticRoutes, epj.StaticRoutes)
	dstEpj.driverTableEntries = make([]*tableEntry, len(epj.driverTableEntries))
	copy(dstEpj.driverTableEntries, epj.driverTableEntries)
	dstEpj.gw = types.GetIPCopy(epj.gw)
	dstEpj.gw6 = types.GetIPCopy(epj.gw6)
	return nil
}
