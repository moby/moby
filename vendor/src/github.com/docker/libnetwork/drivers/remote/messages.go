package remote

import "net"

type response struct {
	Err string
}

type maybeError interface {
	getError() string
}

func (r *response) getError() string {
	return r.Err
}

type createNetworkRequest struct {
	NetworkID string
	Options   map[string]interface{}
}

type createNetworkResponse struct {
	response
}

type deleteNetworkRequest struct {
	NetworkID string
}

type deleteNetworkResponse struct {
	response
}

type createEndpointRequest struct {
	NetworkID  string
	EndpointID string
	Interfaces []*endpointInterface
	Options    map[string]interface{}
}

type endpointInterface struct {
	ID          int
	Address     string
	AddressIPv6 string
	MacAddress  string
}

type createEndpointResponse struct {
	response
	Interfaces []*endpointInterface
}

func toAddr(ipAddr string) (*net.IPNet, error) {
	ip, ipnet, err := net.ParseCIDR(ipAddr)
	if err != nil {
		return nil, err
	}
	ipnet.IP = ip
	return ipnet, nil
}

type iface struct {
	ID          int
	Address     *net.IPNet
	AddressIPv6 *net.IPNet
	MacAddress  net.HardwareAddr
}

func (r *createEndpointResponse) parseInterfaces() ([]*iface, error) {
	var (
		ifaces = make([]*iface, len(r.Interfaces))
	)
	for i, inIf := range r.Interfaces {
		var err error
		outIf := &iface{ID: inIf.ID}
		if inIf.Address != "" {
			if outIf.Address, err = toAddr(inIf.Address); err != nil {
				return nil, err
			}
		}
		if inIf.AddressIPv6 != "" {
			if outIf.AddressIPv6, err = toAddr(inIf.AddressIPv6); err != nil {
				return nil, err
			}
		}
		if inIf.MacAddress != "" {
			if outIf.MacAddress, err = net.ParseMAC(inIf.MacAddress); err != nil {
				return nil, err
			}
		}
		ifaces[i] = outIf
	}
	return ifaces, nil
}

type deleteEndpointRequest struct {
	NetworkID  string
	EndpointID string
}

type deleteEndpointResponse struct {
	response
}

type endpointInfoRequest struct {
	NetworkID  string
	EndpointID string
}

type endpointInfoResponse struct {
	response
	Value map[string]interface{}
}

type joinRequest struct {
	NetworkID  string
	EndpointID string
	SandboxKey string
	Options    map[string]interface{}
}

type ifaceName struct {
	SrcName string
	DstName string
}

type joinResponse struct {
	response
	InterfaceNames []*ifaceName
	Gateway        string
	GatewayIPv6    string
	HostsPath      string
	ResolvConfPath string
}

type leaveRequest struct {
	NetworkID  string
	EndpointID string
}

type leaveResponse struct {
	response
}
