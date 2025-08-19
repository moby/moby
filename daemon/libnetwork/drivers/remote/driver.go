package remote

import (
	"context"
	"fmt"
	"maps"
	"net"
	"sync"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/discoverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/remote/api"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"github.com/moby/moby/v2/daemon/libnetwork/options"
	"github.com/moby/moby/v2/daemon/libnetwork/scope"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"github.com/moby/moby/v2/pkg/plugingetter"
	"github.com/moby/moby/v2/pkg/plugins"
	"github.com/pkg/errors"
)

// remote driver must implement the discover-API.
var _ discoverapi.Discover = (*driver)(nil)

type driver struct {
	endpoint       *plugins.Client
	networkType    string
	gwAllocChecker bool
	nwEndpoints    map[string]*nwEndpoint // Set of endpoint ids that are currently acting as container gateways.
	nwEndpointsMu  sync.Mutex
}

// State info for an endpoint.
type nwEndpoint struct {
	sbOptions  map[string]any // Sandbox (container) options, from Join.
	isGateway4 bool           // Whether ProgramExternalConnectivity reported that this ep is a gateway.
	isGateway6 bool
}

type maybeError interface {
	GetError() string
}

func newDriver(name string, client *plugins.Client) *driver {
	return &driver{
		networkType: name,
		endpoint:    client,
		nwEndpoints: make(map[string]*nwEndpoint),
	}
}

// Register makes sure a remote driver is registered with r when a network
// driver plugin is activated.
func Register(r driverapi.Registerer, pg plugingetter.PluginGetter) error {
	newPluginHandler := func(name string, client *plugins.Client) {
		// negotiate driver capability with client
		d := newDriver(name, client)
		c, err := d.getCapabilities()
		if err != nil {
			log.G(context.TODO()).Errorf("error getting capability for %s due to %v", name, err)
			return
		}
		if err = r.RegisterDriver(name, d, *c); err != nil {
			log.G(context.TODO()).Errorf("error registering driver for %s due to %v", name, err)
		}
	}

	// Unit test code is unaware of a true PluginStore. So we fall back to v1 plugins.
	handleFunc := plugins.Handle
	if pg != nil {
		handleFunc = pg.Handle
		activePlugins := pg.GetAllManagedPluginsByCap(driverapi.NetworkPluginEndpointType)
		for _, ap := range activePlugins {
			client, err := getPluginClient(ap)
			if err != nil {
				return err
			}
			newPluginHandler(ap.Name(), client)
		}
	}
	handleFunc(driverapi.NetworkPluginEndpointType, newPluginHandler)

	return nil
}

func getPluginClient(p plugingetter.CompatPlugin) (*plugins.Client, error) {
	if v1, ok := p.(plugingetter.PluginWithV1Client); ok {
		return v1.Client(), nil
	}

	pa, ok := p.(plugingetter.PluginAddr)
	if !ok {
		return nil, errors.Errorf("unknown plugin type %T", p)
	}

	if pa.Protocol() != plugins.ProtocolSchemeHTTPV1 {
		return nil, errors.Errorf("unsupported plugin protocol %s", pa.Protocol())
	}

	addr := pa.Addr()
	client, err := plugins.NewClientWithTimeout(addr.Network()+"://"+addr.String(), nil, pa.Timeout())
	if err != nil {
		return nil, errors.Wrap(err, "error creating plugin client")
	}
	return client, nil
}

// Get capability from client
func (d *driver) getCapabilities() (*driverapi.Capability, error) {
	var capResp api.GetCapabilityResponse
	if err := d.call("GetCapabilities", nil, &capResp); err != nil {
		return nil, err
	}

	c := &driverapi.Capability{}
	switch capResp.Scope {
	case scope.Global, scope.Local:
		c.DataScope = capResp.Scope
	default:
		return nil, fmt.Errorf("invalid capability: expecting 'local' or 'global', got %s", capResp.Scope)
	}

	switch capResp.ConnectivityScope {
	case scope.Global, scope.Local:
		c.ConnectivityScope = capResp.ConnectivityScope
	case "":
		c.ConnectivityScope = c.DataScope
	default:
		return nil, fmt.Errorf("invalid capability: expecting 'local' or 'global', got %s", capResp.Scope)
	}

	d.gwAllocChecker = capResp.GwAllocChecker

	return c, nil
}

// Config is not implemented for remote drivers, since it is assumed
// to be supplied to the remote process out-of-band (e.g., as command
// line arguments).
func (d *driver) Config(option map[string]any) error {
	return &driverapi.ErrNotImplemented{}
}

func (d *driver) call(methodName string, arg any, retVal maybeError) error {
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

func (d *driver) NetworkAllocate(id string, options map[string]string, ipV4Data, ipV6Data []driverapi.IPAMData) (map[string]string, error) {
	create := &api.AllocateNetworkRequest{
		NetworkID: id,
		Options:   options,
		IPv4Data:  ipV4Data,
		IPv6Data:  ipV6Data,
	}
	retVal := api.AllocateNetworkResponse{}
	err := d.call("AllocateNetwork", create, &retVal)
	return retVal.Options, err
}

func (d *driver) NetworkFree(id string) error {
	fr := &api.FreeNetworkRequest{NetworkID: id}
	return d.call("FreeNetwork", fr, &api.FreeNetworkResponse{})
}

func (d *driver) CreateNetwork(ctx context.Context, id string, options map[string]any, nInfo driverapi.NetworkInfo, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	create := &api.CreateNetworkRequest{
		NetworkID: id,
		Options:   options,
		IPv4Data:  ipV4Data,
		IPv6Data:  ipV6Data,
	}
	return d.call("CreateNetwork", create, &api.CreateNetworkResponse{})
}

func (d *driver) GetSkipGwAlloc(opts options.Generic) (ipv4, ipv6 bool, _ error) {
	if !d.gwAllocChecker {
		return false, false, nil
	}
	resp := &api.GwAllocCheckerResponse{}
	if err := d.call("GwAllocCheck", &api.GwAllocCheckerRequest{Options: opts}, resp); err != nil {
		return false, false, err
	}
	return resp.SkipIPv4, resp.SkipIPv6, nil
}

func (d *driver) DeleteNetwork(nid string) error {
	return d.call("DeleteNetwork", &api.DeleteNetworkRequest{NetworkID: nid}, &api.DeleteNetworkResponse{})
}

func (d *driver) CreateEndpoint(_ context.Context, nid, eid string, ifInfo driverapi.InterfaceInfo, epOptions map[string]any) (retErr error) {
	if ifInfo == nil {
		return errors.New("must not be called with nil InterfaceInfo")
	}

	reqIface := &api.EndpointInterface{}
	if ifInfo.Address() != nil {
		reqIface.Address = ifInfo.Address().String()
	}
	if ifInfo.AddressIPv6() != nil {
		reqIface.AddressIPv6 = ifInfo.AddressIPv6().String()
	}
	if ifInfo.MacAddress() != nil {
		reqIface.MacAddress = ifInfo.MacAddress().String()
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

	defer func() {
		if retErr != nil {
			if err := d.DeleteEndpoint(nid, eid); err != nil {
				retErr = fmt.Errorf("%w; failed to roll back: %w", err, retErr)
			} else {
				retErr = fmt.Errorf("%w; rolled back", retErr)
			}
		}
	}()

	inIface, err := parseInterface(res)
	if err != nil {
		return err
	}
	if inIface == nil {
		// Remote driver did not set any field
		return nil
	}

	if inIface.MacAddress != nil {
		if err := ifInfo.SetMacAddress(inIface.MacAddress); err != nil {
			return fmt.Errorf("driver modified interface MAC address: %v", err)
		}
	}
	if inIface.Address != nil {
		if err := ifInfo.SetIPAddress(inIface.Address); err != nil {
			return fmt.Errorf("driver modified interface address: %v", err)
		}
	}
	if inIface.AddressIPv6 != nil {
		if err := ifInfo.SetIPAddress(inIface.AddressIPv6); err != nil {
			return fmt.Errorf("driver modified interface address: %v", err)
		}
	}

	return nil
}

func (d *driver) DeleteEndpoint(nid, eid string) error {
	deleteRequest := &api.DeleteEndpointRequest{
		NetworkID:  nid,
		EndpointID: eid,
	}
	return d.call("DeleteEndpoint", deleteRequest, &api.DeleteEndpointResponse{})
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]any, error) {
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
func (d *driver) Join(_ context.Context, nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, _, options map[string]any) (retErr error) {
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

	defer func() {
		if retErr != nil {
			if err := d.Leave(nid, eid); err != nil {
				retErr = fmt.Errorf("%w; failed to roll back: %w", err, retErr)
			} else {
				retErr = fmt.Errorf("%w; rolled back", retErr)
			}
		}
	}()

	ifaceName := res.InterfaceName
	if iface := jinfo.InterfaceName(); iface != nil && ifaceName != nil {
		if err := iface.SetNames(ifaceName.SrcName, ifaceName.DstPrefix, ""); err != nil {
			return fmt.Errorf("failed to set interface name: %s", err)
		}
	}

	var addr net.IP
	if res.Gateway != "" {
		if addr = net.ParseIP(res.Gateway); addr == nil {
			return fmt.Errorf(`unable to parse Gateway "%s"`, res.Gateway)
		}
		if jinfo.SetGateway(addr) != nil {
			return fmt.Errorf("failed to set gateway: %v", addr)
		}
	}
	if res.GatewayIPv6 != "" {
		if addr = net.ParseIP(res.GatewayIPv6); addr == nil {
			return fmt.Errorf(`unable to parse GatewayIPv6 "%s"`, res.GatewayIPv6)
		}
		if jinfo.SetGatewayIPv6(addr) != nil {
			return fmt.Errorf("failed to set gateway IPv6: %v", addr)
		}
	}
	if len(res.StaticRoutes) > 0 {
		routes, err := parseStaticRoutes(res)
		if err != nil {
			return err
		}
		for _, route := range routes {
			if jinfo.AddStaticRoute(route.Destination, route.RouteType, route.NextHop) != nil {
				return fmt.Errorf("failed to set static route: %v", route)
			}
		}
	}
	if res.DisableGatewayService {
		jinfo.DisableGatewayService()
	}

	d.nwEndpointsMu.Lock()
	defer d.nwEndpointsMu.Unlock()
	d.nwEndpoints[eid] = &nwEndpoint{sbOptions: options}
	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid string) error {
	leave := &api.LeaveRequest{
		NetworkID:  nid,
		EndpointID: eid,
	}
	if err := d.call("Leave", leave, &api.LeaveResponse{}); err != nil {
		return err
	}
	d.nwEndpointsMu.Lock()
	defer d.nwEndpointsMu.Unlock()
	delete(d.nwEndpoints, eid)
	return nil
}

// ProgramExternalConnectivity is invoked to program the rules to allow external connectivity for the endpoint.
func (d *driver) ProgramExternalConnectivity(_ context.Context, nid, eid string, gw4Id, gw6Id string) error {
	d.nwEndpointsMu.Lock()
	ep, ok := d.nwEndpoints[eid]
	d.nwEndpointsMu.Unlock()
	if !ok {
		return fmt.Errorf("remote network driver: endpoint %s not found", eid)
	}
	isGw4, isGw6 := gw4Id == eid, gw6Id == eid
	if ep.isGateway4 == isGw4 && ep.isGateway6 == isGw6 {
		return nil
	}
	if !isGw4 && !isGw6 {
		return d.revokeExternalConnectivity(nid, eid)
	}
	ep.isGateway4, ep.isGateway6 = isGw4, isGw6
	options := ep.sbOptions
	if !isGw6 && gw6Id != "" {
		// If there is an IPv6 gateway, but it's not eid, set NoProxy6To4. This label was
		// used to tell the bridge driver not to try to use the userland proxy for dual
		// stack port mappings between host IPv6 and container IPv4 (because a different
		// endpoint may be dealing with IPv6 host addresses). It was undocumented for the
		// remote driver, marked as being for internal use and subject to later removal.
		// But, preserve it here for now as there's no other way for a remote driver to
		// know it shouldn't try to deal with IPv6 in this case.
		options = maps.Clone(ep.sbOptions)
		options[netlabel.NoProxy6To4] = true
	}
	data := &api.ProgramExternalConnectivityRequest{
		NetworkID:  nid,
		EndpointID: eid,
		Options:    options,
	}
	err := d.call("ProgramExternalConnectivity", data, &api.ProgramExternalConnectivityResponse{})
	if err != nil && plugins.IsNotFound(err) {
		// It is not mandatory yet to support this method
		return nil
	}
	return err
}

// revokeExternalConnectivity method is invoked to remove any external connectivity programming related to the endpoint.
func (d *driver) revokeExternalConnectivity(nid, eid string) error {
	ep, ok := d.nwEndpoints[eid]
	d.nwEndpointsMu.Unlock()
	if !ok {
		return fmt.Errorf("remote network driver: endpoint %s not found", eid)
	}
	data := &api.RevokeExternalConnectivityRequest{
		NetworkID:  nid,
		EndpointID: eid,
	}
	ep.isGateway4, ep.isGateway6 = false, false
	err := d.call("RevokeExternalConnectivity", data, &api.RevokeExternalConnectivityResponse{})
	if err != nil && plugins.IsNotFound(err) {
		// It is not mandatory yet to support this method
		return nil
	}
	return err
}

func (d *driver) Type() string {
	return d.networkType
}

func (d *driver) IsBuiltIn() bool {
	return false
}

// DiscoverNew is a notification for a new discovery event, such as a new node joining a cluster
func (d *driver) DiscoverNew(dType discoverapi.DiscoveryType, data any) error {
	if dType != discoverapi.NodeDiscovery {
		return nil
	}
	notif := &api.DiscoveryNotification{
		DiscoveryType: dType,
		DiscoveryData: data,
	}
	return d.call("DiscoverNew", notif, &api.DiscoveryResponse{})
}

// DiscoverDelete is a notification for a discovery delete event, such as a node leaving a cluster
func (d *driver) DiscoverDelete(dType discoverapi.DiscoveryType, data any) error {
	if dType != discoverapi.NodeDiscovery {
		return nil
	}
	notif := &api.DiscoveryNotification{
		DiscoveryType: dType,
		DiscoveryData: data,
	}
	return d.call("DiscoverDelete", notif, &api.DiscoveryResponse{})
}

func parseStaticRoutes(r api.JoinResponse) ([]*types.StaticRoute, error) {
	routes := make([]*types.StaticRoute, len(r.StaticRoutes))
	for i, inRoute := range r.StaticRoutes {
		var err error
		outRoute := &types.StaticRoute{RouteType: inRoute.RouteType}

		if inRoute.Destination != "" {
			if outRoute.Destination, err = types.ParseCIDR(inRoute.Destination); err != nil {
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

// parseInterface validates all the parameters of an Interface and returns them.
func parseInterface(r api.CreateEndpointResponse) (*api.Interface, error) {
	var outIf *api.Interface

	inIf := r.Interface
	if inIf != nil {
		var err error
		outIf = &api.Interface{}
		if inIf.Address != "" {
			if outIf.Address, err = types.ParseCIDR(inIf.Address); err != nil {
				return nil, err
			}
		}
		if inIf.AddressIPv6 != "" {
			if outIf.AddressIPv6, err = types.ParseCIDR(inIf.AddressIPv6); err != nil {
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
