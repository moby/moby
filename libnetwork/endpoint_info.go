package libnetwork

import (
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

type endpointInterface struct {
	id        int
	mac       net.HardwareAddr
	addr      net.IPNet
	addrv6    net.IPNet
	srcName   string
	dstPrefix string
}

type endpointJoinInfo struct {
	gw             net.IP
	gw6            net.IP
	hostsPath      string
	resolvConfPath string
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

func (i *endpointInterface) ID() int {
	return i.id
}

func (i *endpointInterface) MacAddress() net.HardwareAddr {
	return types.GetMacCopy(i.mac)
}

func (i *endpointInterface) Address() net.IPNet {
	return (*types.GetIPNetCopy(&i.addr))
}

func (i *endpointInterface) AddressIPv6() net.IPNet {
	return (*types.GetIPNetCopy(&i.addrv6))
}

func (i *endpointInterface) SetNames(srcName string, dstPrefix string) error {
	i.srcName = srcName
	i.dstPrefix = dstPrefix
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
