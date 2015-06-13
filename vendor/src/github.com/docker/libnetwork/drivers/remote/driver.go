package remote

import (
	"fmt"
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/types"
)

type driver struct {
	endpoint    *plugins.Client
	networkType string
}

func newDriver(name string, client *plugins.Client) driverapi.Driver {
	return &driver{networkType: name, endpoint: client}
}

// Init makes sure a remote driver is registered when a network driver
// plugin is activated.
func Init(dc driverapi.DriverCallback) error {
	plugins.Handle(driverapi.NetworkPluginEndpointType, func(name string, client *plugins.Client) {
		c := driverapi.Capability{
			Scope: driverapi.GlobalScope,
		}
		if err := dc.RegisterDriver(name, newDriver(name, client), c); err != nil {
			log.Errorf("error registering driver for %s due to %v", name, err)
		}
	})
	return nil
}

// Config is not implemented for remote drivers, since it is assumed
// to be supplied to the remote process out-of-band (e.g., as command
// line arguments).
func (d *driver) Config(option map[string]interface{}) error {
	return &driverapi.ErrNotImplemented{}
}

func (d *driver) call(methodName string, arg interface{}, retVal maybeError) error {
	method := driverapi.NetworkPluginEndpointType + "." + methodName
	err := d.endpoint.Call(method, arg, retVal)
	if err != nil {
		return err
	}
	if e := retVal.getError(); e != "" {
		return fmt.Errorf("remote: %s", e)
	}
	return nil
}

func (d *driver) CreateNetwork(id types.UUID, options map[string]interface{}) error {
	create := &createNetworkRequest{
		NetworkID: string(id),
		Options:   options,
	}
	return d.call("CreateNetwork", create, &createNetworkResponse{})
}

func (d *driver) DeleteNetwork(nid types.UUID) error {
	delete := &deleteNetworkRequest{NetworkID: string(nid)}
	return d.call("DeleteNetwork", delete, &deleteNetworkResponse{})
}

func (d *driver) CreateEndpoint(nid, eid types.UUID, epInfo driverapi.EndpointInfo, epOptions map[string]interface{}) error {
	if epInfo == nil {
		return fmt.Errorf("must not be called with nil EndpointInfo")
	}

	reqIfaces := make([]*endpointInterface, len(epInfo.Interfaces()))
	for i, iface := range epInfo.Interfaces() {
		addr4 := iface.Address()
		addr6 := iface.AddressIPv6()
		reqIfaces[i] = &endpointInterface{
			ID:          iface.ID(),
			Address:     addr4.String(),
			AddressIPv6: addr6.String(),
			MacAddress:  iface.MacAddress().String(),
		}
	}
	create := &createEndpointRequest{
		NetworkID:  string(nid),
		EndpointID: string(eid),
		Interfaces: reqIfaces,
		Options:    epOptions,
	}
	var res createEndpointResponse
	if err := d.call("CreateEndpoint", create, &res); err != nil {
		return err
	}

	ifaces, err := res.parseInterfaces()
	if err != nil {
		return err
	}
	if len(reqIfaces) > 0 && len(ifaces) > 0 {
		// We're not supposed to add interfaces if there already are
		// some. Attempt to roll back
		return errorWithRollback("driver attempted to add more interfaces", d.DeleteEndpoint(nid, eid))
	}
	for _, iface := range ifaces {
		var addr4, addr6 net.IPNet
		if iface.Address != nil {
			addr4 = *(iface.Address)
		}
		if iface.AddressIPv6 != nil {
			addr6 = *(iface.AddressIPv6)
		}
		if err := epInfo.AddInterface(iface.ID, iface.MacAddress, addr4, addr6); err != nil {
			return errorWithRollback(fmt.Sprintf("failed to AddInterface %v: %s", iface, err), d.DeleteEndpoint(nid, eid))
		}
	}
	return nil
}

func errorWithRollback(msg string, err error) error {
	rollback := "rolled back"
	if err != nil {
		rollback = "failed to roll back: " + err.Error()
	}
	return fmt.Errorf("%s; %s", msg, rollback)
}

func (d *driver) DeleteEndpoint(nid, eid types.UUID) error {
	delete := &deleteEndpointRequest{
		NetworkID:  string(nid),
		EndpointID: string(eid),
	}
	return d.call("DeleteEndpoint", delete, &deleteEndpointResponse{})
}

func (d *driver) EndpointOperInfo(nid, eid types.UUID) (map[string]interface{}, error) {
	info := &endpointInfoRequest{
		NetworkID:  string(nid),
		EndpointID: string(eid),
	}
	var res endpointInfoResponse
	if err := d.call("EndpointOperInfo", info, &res); err != nil {
		return nil, err
	}
	return res.Value, nil
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(nid, eid types.UUID, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
	join := &joinRequest{
		NetworkID:  string(nid),
		EndpointID: string(eid),
		SandboxKey: sboxKey,
		Options:    options,
	}
	var (
		res joinResponse
		err error
	)
	if err = d.call("Join", join, &res); err != nil {
		return err
	}

	// Expect each interface ID given by CreateEndpoint to have an
	// entry at that index in the names supplied here. In other words,
	// if you supply 0..n interfaces with IDs 0..n above, you should
	// supply the names in the same order.
	ifaceNames := res.InterfaceNames
	for _, iface := range jinfo.InterfaceNames() {
		i := iface.ID()
		if i >= len(ifaceNames) || i < 0 {
			return fmt.Errorf("no correlating interface %d in supplied interface names", i)
		}
		supplied := ifaceNames[i]
		if err := iface.SetNames(supplied.SrcName, supplied.DstPrefix); err != nil {
			return errorWithRollback(fmt.Sprintf("failed to set interface name: %s", err), d.Leave(nid, eid))
		}
	}

	var addr net.IP
	if res.Gateway != "" {
		if addr = net.ParseIP(res.Gateway); addr == nil {
			return fmt.Errorf(`unable to parse Gateway "%s"`, res.Gateway)
		}
		if jinfo.SetGateway(addr) != nil {
			return errorWithRollback(fmt.Sprintf("failed to set gateway: %v", addr), d.Leave(nid, eid))
		}
	}
	if res.GatewayIPv6 != "" {
		if addr = net.ParseIP(res.GatewayIPv6); addr == nil {
			return fmt.Errorf(`unable to parse GatewayIPv6 "%s"`, res.GatewayIPv6)
		}
		if jinfo.SetGatewayIPv6(addr) != nil {
			return errorWithRollback(fmt.Sprintf("failed to set gateway IPv6: %v", addr), d.Leave(nid, eid))
		}
	}
	if len(res.StaticRoutes) > 0 {
		routes, err := res.parseStaticRoutes()
		if err != nil {
			return err
		}
		for _, route := range routes {
			if jinfo.AddStaticRoute(route.Destination, route.RouteType, route.NextHop, route.InterfaceID) != nil {
				return errorWithRollback(fmt.Sprintf("failed to set static route: %v", route), d.Leave(nid, eid))
			}
		}
	}
	if jinfo.SetHostsPath(res.HostsPath) != nil {
		return errorWithRollback(fmt.Sprintf("failed to set hosts path: %s", res.HostsPath), d.Leave(nid, eid))
	}
	if jinfo.SetResolvConfPath(res.ResolvConfPath) != nil {
		return errorWithRollback(fmt.Sprintf("failed to set resolv.conf path: %s", res.ResolvConfPath), d.Leave(nid, eid))
	}
	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid types.UUID) error {
	leave := &leaveRequest{
		NetworkID:  string(nid),
		EndpointID: string(eid),
	}
	return d.call("Leave", leave, &leaveResponse{})
}

func (d *driver) Type() string {
	return d.networkType
}
