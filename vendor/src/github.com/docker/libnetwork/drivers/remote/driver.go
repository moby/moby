package remote

import (
	"fmt"
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/drivers/remote/api"
	"github.com/docker/libnetwork/types"
)

type driver struct {
	endpoint    *plugins.Client
	networkType string
}

type maybeError interface {
	GetError() string
}

func newDriver(name string, client *plugins.Client) driverapi.Driver {
	return &driver{networkType: name, endpoint: client}
}

// Init makes sure a remote driver is registered when a network driver
// plugin is activated.
func Init(dc driverapi.DriverCallback, config map[string]interface{}) error {
	plugins.Handle(driverapi.NetworkPluginEndpointType, func(name string, client *plugins.Client) {
		// negotiate driver capability with client
		d := newDriver(name, client)
		c, err := d.(*driver).getCapabilities()
		if err != nil {
			log.Errorf("error getting capability for %s due to %v", name, err)
			return
		}
		if err = dc.RegisterDriver(name, d, *c); err != nil {
			log.Errorf("error registering driver for %s due to %v", name, err)
		}
	})
	return nil
}

// Get capability from client
func (d *driver) getCapabilities() (*driverapi.Capability, error) {
	var capResp api.GetCapabilityResponse
	if err := d.call("GetCapabilities", nil, &capResp); err != nil {
		return nil, err
	}

	c := &driverapi.Capability{}
	switch capResp.Scope {
	case "global":
		c.DataScope = datastore.GlobalScope
	case "local":
		c.DataScope = datastore.LocalScope
	default:
		return nil, fmt.Errorf("invalid capability: expecting 'local' or 'global', got %s", capResp.Scope)
	}

	return c, nil
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
	if e := retVal.GetError(); e != "" {
		return fmt.Errorf("remote: %s", e)
	}
	return nil
}

func (d *driver) CreateNetwork(id string, options map[string]interface{}) error {
	create := &api.CreateNetworkRequest{
		NetworkID: id,
		Options:   options,
	}
	return d.call("CreateNetwork", create, &api.CreateNetworkResponse{})
}

func (d *driver) DeleteNetwork(nid string) error {
	delete := &api.DeleteNetworkRequest{NetworkID: nid}
	return d.call("DeleteNetwork", delete, &api.DeleteNetworkResponse{})
}

func (d *driver) CreateEndpoint(nid, eid string, epInfo driverapi.EndpointInfo, epOptions map[string]interface{}) error {
	var reqIface *api.EndpointInterface

	if epInfo == nil {
		return fmt.Errorf("must not be called with nil EndpointInfo")
	}

	iface := epInfo.Interface()
	if iface != nil {
		addr4 := iface.Address()
		addr6 := iface.AddressIPv6()
		reqIface = &api.EndpointInterface{
			Address:     addr4.String(),
			AddressIPv6: addr6.String(),
			MacAddress:  iface.MacAddress().String(),
		}
	}
	create := &api.CreateEndpointRequest{
		NetworkID:  nid,
		EndpointID: eid,
		Interface:  reqIface,
		Options:    epOptions,
	}
	var res api.CreateEndpointResponse
	if err := d.call("CreateEndpoint", create, &res); err != nil {
		return err
	}

	inIface, err := parseInterface(res)
	if err != nil {
		return err
	}
	if reqIface != nil && inIface != nil {
		// We're not supposed to add interface if there is already
		// one. Attempt to roll back
		return errorWithRollback("driver attempted to add interface ignoring the one provided", d.DeleteEndpoint(nid, eid))
	}

	if inIface != nil {
		var addr4, addr6 net.IPNet
		if inIface.Address != nil {
			addr4 = *(inIface.Address)
		}
		if inIface.AddressIPv6 != nil {
			addr6 = *(inIface.AddressIPv6)
		}
		if err := epInfo.AddInterface(inIface.MacAddress, addr4, addr6); err != nil {
			return errorWithRollback(fmt.Sprintf("failed to AddInterface %v: %s", inIface, err), d.DeleteEndpoint(nid, eid))
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

func (d *driver) DeleteEndpoint(nid, eid string) error {
	delete := &api.DeleteEndpointRequest{
		NetworkID:  nid,
		EndpointID: eid,
	}
	return d.call("DeleteEndpoint", delete, &api.DeleteEndpointResponse{})
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	info := &api.EndpointInfoRequest{
		NetworkID:  nid,
		EndpointID: eid,
	}
	var res api.EndpointInfoResponse
	if err := d.call("EndpointOperInfo", info, &res); err != nil {
		return nil, err
	}
	return res.Value, nil
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
	join := &api.JoinRequest{
		NetworkID:  nid,
		EndpointID: eid,
		SandboxKey: sboxKey,
		Options:    options,
	}
	var (
		res api.JoinResponse
		err error
	)
	if err = d.call("Join", join, &res); err != nil {
		return err
	}

	ifaceName := res.InterfaceName
	if jinfo.InterfaceName() != nil && ifaceName == nil {
		return fmt.Errorf("no interface name information received while one is expected")
	}

	if iface := jinfo.InterfaceName(); iface != nil {
		if err := iface.SetNames(ifaceName.SrcName, ifaceName.DstPrefix); err != nil {
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
		routes, err := parseStaticRoutes(res)
		if err != nil {
			return err
		}
		for _, route := range routes {
			if jinfo.AddStaticRoute(route.Destination, route.RouteType, route.NextHop) != nil {
				return errorWithRollback(fmt.Sprintf("failed to set static route: %v", route), d.Leave(nid, eid))
			}
		}
	}
	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid string) error {
	leave := &api.LeaveRequest{
		NetworkID:  nid,
		EndpointID: eid,
	}
	return d.call("Leave", leave, &api.LeaveResponse{})
}

func (d *driver) Type() string {
	return d.networkType
}

// DiscoverNew is a notification for a new discovery event, such as a new node joining a cluster
func (d *driver) DiscoverNew(dType driverapi.DiscoveryType, data interface{}) error {
	if dType != driverapi.NodeDiscovery {
		return fmt.Errorf("Unknown discovery type : %v", dType)
	}
	notif := &api.DiscoveryNotification{
		DiscoveryType: dType,
		DiscoveryData: data,
	}
	return d.call("DiscoverNew", notif, &api.DiscoveryResponse{})
}

// DiscoverDelete is a notification for a discovery delete event, such as a node leaving a cluster
func (d *driver) DiscoverDelete(dType driverapi.DiscoveryType, data interface{}) error {
	if dType != driverapi.NodeDiscovery {
		return fmt.Errorf("Unknown discovery type : %v", dType)
	}
	notif := &api.DiscoveryNotification{
		DiscoveryType: dType,
		DiscoveryData: data,
	}
	return d.call("DiscoverDelete", notif, &api.DiscoveryResponse{})
}

func parseStaticRoutes(r api.JoinResponse) ([]*types.StaticRoute, error) {
	var routes = make([]*types.StaticRoute, len(r.StaticRoutes))
	for i, inRoute := range r.StaticRoutes {
		var err error
		outRoute := &types.StaticRoute{RouteType: inRoute.RouteType}

		if inRoute.Destination != "" {
			if outRoute.Destination, err = toAddr(inRoute.Destination); err != nil {
				return nil, err
			}
		}

		if inRoute.NextHop != "" {
			outRoute.NextHop = net.ParseIP(inRoute.NextHop)
			if outRoute.NextHop == nil {
				return nil, fmt.Errorf("failed to parse nexthop IP %s", inRoute.NextHop)
			}
		}

		routes[i] = outRoute
	}
	return routes, nil
}

// parseInterfaces validates all the parameters of an Interface and returns them.
func parseInterface(r api.CreateEndpointResponse) (*api.Interface, error) {
	var outIf *api.Interface

	inIf := r.Interface
	if inIf != nil {
		var err error
		outIf = &api.Interface{}
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
	}

	return outIf, nil
}

func toAddr(ipAddr string) (*net.IPNet, error) {
	ip, ipnet, err := net.ParseCIDR(ipAddr)
	if err != nil {
		return nil, err
	}
	ipnet.IP = ip
	return ipnet, nil
}
