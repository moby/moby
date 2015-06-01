package bridge

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/ipallocator"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/portmapper"
	"github.com/docker/libnetwork/sandbox"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
)

const (
	networkType             = "bridge"
	vethPrefix              = "veth"
	vethLen                 = 7
	containerVethPrefix     = "eth"
	maxAllocatePortAttempts = 10
	ifaceID                 = 1
)

var (
	ipAllocator *ipallocator.IPAllocator
)

// configuration info for the "bridge" driver.
type configuration struct {
	EnableIPForwarding bool
}

// networkConfiguration for network specific configuration
type networkConfiguration struct {
	BridgeName            string
	AddressIPv4           *net.IPNet
	FixedCIDR             *net.IPNet
	FixedCIDRv6           *net.IPNet
	EnableIPv6            bool
	EnableIPTables        bool
	EnableIPMasquerade    bool
	EnableICC             bool
	Mtu                   int
	DefaultGatewayIPv4    net.IP
	DefaultGatewayIPv6    net.IP
	DefaultBindingIP      net.IP
	AllowNonDefaultBridge bool
	EnableUserlandProxy   bool
}

// endpointConfiguration represents the user specified configuration for the sandbox endpoint
type endpointConfiguration struct {
	MacAddress   net.HardwareAddr
	PortBindings []types.PortBinding
	ExposedPorts []types.TransportPort
}

// containerConfiguration represents the user specified configuration for a container
type containerConfiguration struct {
	ParentEndpoints []string
	ChildEndpoints  []string
}

type bridgeEndpoint struct {
	id              types.UUID
	intf            *sandbox.Interface
	macAddress      net.HardwareAddr
	config          *endpointConfiguration // User specified parameters
	containerConfig *containerConfiguration
	portMapping     []types.PortBinding // Operation port bindings
}

type bridgeNetwork struct {
	id         types.UUID
	bridge     *bridgeInterface // The bridge's L3 interface
	config     *networkConfiguration
	endpoints  map[types.UUID]*bridgeEndpoint // key: endpoint id
	portMapper *portmapper.PortMapper
	sync.Mutex
}

type driver struct {
	config   *configuration
	network  *bridgeNetwork
	networks map[types.UUID]*bridgeNetwork
	sync.Mutex
}

func init() {
	ipAllocator = ipallocator.New()
}

// New constructs a new bridge driver
func newDriver() driverapi.Driver {
	return &driver{networks: map[types.UUID]*bridgeNetwork{}}
}

// Init registers a new instance of bridge driver
func Init(dc driverapi.DriverCallback) error {
	return dc.RegisterDriver(networkType, newDriver())
}

// Validate performs a static validation on the network configuration parameters.
// Whatever can be assessed a priori before attempting any programming.
func (c *networkConfiguration) Validate() error {
	if c.Mtu < 0 {
		return ErrInvalidMtu(c.Mtu)
	}

	// If bridge v4 subnet is specified
	if c.AddressIPv4 != nil {
		// If Container restricted subnet is specified, it must be a subset of bridge subnet
		if c.FixedCIDR != nil {
			// Check Network address
			if !c.AddressIPv4.Contains(c.FixedCIDR.IP) {
				return &ErrInvalidContainerSubnet{}
			}
			// Check it is effectively a subset
			brNetLen, _ := c.AddressIPv4.Mask.Size()
			cnNetLen, _ := c.FixedCIDR.Mask.Size()
			if brNetLen > cnNetLen {
				return &ErrInvalidContainerSubnet{}
			}
		}
		// If default gw is specified, it must be part of bridge subnet
		if c.DefaultGatewayIPv4 != nil {
			if !c.AddressIPv4.Contains(c.DefaultGatewayIPv4) {
				return &ErrInvalidGateway{}
			}
		}
	}

	// If default v6 gw is specified, FixedCIDRv6 must be specified and gw must belong to FixedCIDRv6 subnet
	if c.EnableIPv6 && c.DefaultGatewayIPv6 != nil {
		if c.FixedCIDRv6 == nil || !c.FixedCIDRv6.Contains(c.DefaultGatewayIPv6) {
			return &ErrInvalidGateway{}
		}
	}

	return nil
}

// Conflict check if two NetworkConfiguration objects overlap in the multinetwork
func (c *networkConfiguration) Conflict(o *networkConfiguration) bool {
	if o == nil {
		return false
	}

	// Also empty, becasue only one network with empty name is allowed
	if c.BridgeName == o.BridgeName {
		return true
	}

	// They must be in different subnets

	if (c.AddressIPv4 != nil && o.AddressIPv4.IP != nil) &&
		(c.AddressIPv4.Contains(o.AddressIPv4.IP) || o.AddressIPv4.Contains(c.AddressIPv4.IP)) {
		return true
	}

	return false
}

// FromMap retrieve the configuration data from the map form.
func (c *networkConfiguration) FromMap(data map[string]interface{}) error {
	var err error

	if i, ok := data["BridgeName"]; ok && i != nil {
		if c.BridgeName, ok = i.(string); !ok {
			return types.BadRequestErrorf("invalid type for BridgeName value")
		}
	}

	if i, ok := data["Mtu"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.Mtu, err = strconv.Atoi(s); err != nil {
				return types.BadRequestErrorf("failed to parse Mtu value: %s", err.Error())
			}
		} else {
			return types.BadRequestErrorf("invalid type for Mtu value")
		}
	}

	if i, ok := data["EnableIPv6"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.EnableIPv6, err = strconv.ParseBool(s); err != nil {
				return types.BadRequestErrorf("failed to parse EnableIPv6 value: %s", err.Error())
			}
		} else {
			return types.BadRequestErrorf("invalid type for EnableIPv6 value")
		}
	}

	if i, ok := data["EnableIPTables"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.EnableIPTables, err = strconv.ParseBool(s); err != nil {
				return types.BadRequestErrorf("failed to parse EnableIPTables value: %s", err.Error())
			}
		} else {
			return types.BadRequestErrorf("invalid type for EnableIPTables value")
		}
	}

	if i, ok := data["EnableIPMasquerade"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.EnableIPMasquerade, err = strconv.ParseBool(s); err != nil {
				return types.BadRequestErrorf("failed to parse EnableIPMasquerade value: %s", err.Error())
			}
		} else {
			return types.BadRequestErrorf("invalid type for EnableIPMasquerade value")
		}
	}

	if i, ok := data["EnableICC"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.EnableICC, err = strconv.ParseBool(s); err != nil {
				return types.BadRequestErrorf("failed to parse EnableICC value: %s", err.Error())
			}
		} else {
			return types.BadRequestErrorf("invalid type for EnableICC value")
		}
	}

	if i, ok := data["AllowNonDefaultBridge"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.AllowNonDefaultBridge, err = strconv.ParseBool(s); err != nil {
				return types.BadRequestErrorf("failed to parse AllowNonDefaultBridge value: %s", err.Error())
			}
		} else {
			return types.BadRequestErrorf("invalid type for AllowNonDefaultBridge value")
		}
	}

	if i, ok := data["AddressIPv4"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if ip, nw, e := net.ParseCIDR(s); e == nil {
				nw.IP = ip
				c.AddressIPv4 = nw
			} else {
				return types.BadRequestErrorf("failed to parse AddressIPv4 value")
			}
		} else {
			return types.BadRequestErrorf("invalid type for AddressIPv4 value")
		}
	}

	if i, ok := data["FixedCIDR"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if ip, nw, e := net.ParseCIDR(s); e == nil {
				nw.IP = ip
				c.FixedCIDR = nw
			} else {
				return types.BadRequestErrorf("failed to parse FixedCIDR value")
			}
		} else {
			return types.BadRequestErrorf("invalid type for FixedCIDR value")
		}
	}

	if i, ok := data["FixedCIDRv6"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if ip, nw, e := net.ParseCIDR(s); e == nil {
				nw.IP = ip
				c.FixedCIDRv6 = nw
			} else {
				return types.BadRequestErrorf("failed to parse FixedCIDRv6 value")
			}
		} else {
			return types.BadRequestErrorf("invalid type for FixedCIDRv6 value")
		}
	}

	if i, ok := data["DefaultGatewayIPv4"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.DefaultGatewayIPv4 = net.ParseIP(s); c.DefaultGatewayIPv4 == nil {
				return types.BadRequestErrorf("failed to parse DefaultGatewayIPv4 value")
			}
		} else {
			return types.BadRequestErrorf("invalid type for DefaultGatewayIPv4 value")
		}
	}

	if i, ok := data["DefaultGatewayIPv6"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.DefaultGatewayIPv6 = net.ParseIP(s); c.DefaultGatewayIPv6 == nil {
				return types.BadRequestErrorf("failed to parse DefaultGatewayIPv6 value")
			}
		} else {
			return types.BadRequestErrorf("invalid type for DefaultGatewayIPv6 value")
		}
	}

	if i, ok := data["DefaultBindingIP"]; ok && i != nil {
		if s, ok := i.(string); ok {
			if c.DefaultBindingIP = net.ParseIP(s); c.DefaultBindingIP == nil {
				return types.BadRequestErrorf("failed to parse DefaultBindingIP value")
			}
		} else {
			return types.BadRequestErrorf("invalid type for DefaultBindingIP value")
		}
	}
	return nil
}

func (n *bridgeNetwork) getEndpoint(eid types.UUID) (*bridgeEndpoint, error) {
	n.Lock()
	defer n.Unlock()

	if eid == "" {
		return nil, InvalidEndpointIDError(eid)
	}

	if ep, ok := n.endpoints[eid]; ok {
		return ep, nil
	}

	return nil, nil
}

func (d *driver) Config(option map[string]interface{}) error {
	var config *configuration

	d.Lock()
	defer d.Unlock()

	if d.config != nil {
		return &ErrConfigExists{}
	}

	genericData, ok := option[netlabel.GenericData]
	if ok && genericData != nil {
		switch opt := genericData.(type) {
		case options.Generic:
			opaqueConfig, err := options.GenerateFromModel(opt, &configuration{})
			if err != nil {
				return err
			}
			config = opaqueConfig.(*configuration)
		case *configuration:
			config = opt
		default:
			return &ErrInvalidDriverConfig{}
		}

		d.config = config
	} else {
		config = &configuration{}
	}

	if config.EnableIPForwarding {
		return setupIPForwarding(config)
	}

	return nil
}

func (d *driver) getNetwork(id types.UUID) (*bridgeNetwork, error) {
	d.Lock()
	defer d.Unlock()

	if id == "" {
		return nil, types.BadRequestErrorf("invalid network id: %s", id)
	}

	if nw, ok := d.networks[id]; ok {
		return nw, nil
	}

	return nil, nil
}

func parseNetworkGenericOptions(data interface{}) (*networkConfiguration, error) {
	var (
		err    error
		config *networkConfiguration
	)

	switch opt := data.(type) {
	case *networkConfiguration:
		config = opt
	case map[string]interface{}:
		config = &networkConfiguration{}
		err = config.FromMap(opt)
	case options.Generic:
		var opaqueConfig interface{}
		if opaqueConfig, err = options.GenerateFromModel(opt, config); err == nil {
			config = opaqueConfig.(*networkConfiguration)
		}
	default:
		err = types.BadRequestErrorf("do not recognize network configuration format: %T", opt)
	}

	return config, err
}

func parseNetworkOptions(option options.Generic) (*networkConfiguration, error) {
	var err error
	config := &networkConfiguration{}

	// Parse generic label first, config will be re-assigned
	if genData, ok := option[netlabel.GenericData]; ok && genData != nil {
		if config, err = parseNetworkGenericOptions(genData); err != nil {
			return nil, err
		}
	}

	// Process well-known labels next
	if _, ok := option[netlabel.EnableIPv6]; ok {
		config.EnableIPv6 = option[netlabel.EnableIPv6].(bool)
	}

	// Finally validate the configuration
	if err = config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// Create a new network using bridge plugin
func (d *driver) CreateNetwork(id types.UUID, option map[string]interface{}) error {
	var err error

	// Driver must be configured
	d.Lock()

	// Sanity checks
	if _, ok := d.networks[id]; ok {
		d.Unlock()
		return types.ForbiddenErrorf("network %s exists", id)
	}

	// Parse and validate the config. It should not conflict with existing networks' config
	config, err := parseNetworkOptions(option)
	if err != nil {
		d.Unlock()
		return err
	}
	for _, nw := range d.networks {
		if nw.config.Conflict(config) {
			d.Unlock()
			return types.ForbiddenErrorf("conflicts with network %s (%s)", nw.id, nw.config.BridgeName)
		}
	}

	// Create and set network handler in driver
	network := &bridgeNetwork{
		id:         id,
		endpoints:  make(map[types.UUID]*bridgeEndpoint),
		config:     config,
		portMapper: portmapper.New(),
	}
	d.networks[id] = network
	d.Unlock()

	// On failure make sure to reset driver network handler to nil
	defer func() {
		if err != nil {
			d.Lock()
			delete(d.networks, id)
			d.Unlock()
		}
	}()

	// Create or retrieve the bridge L3 interface
	bridgeIface := newInterface(config)
	network.bridge = bridgeIface

	// Verify network does not conflict with previously configured networks
	// on parameters that were chosen by the driver.
	d.Lock()
	for _, nw := range d.networks {
		if nw.id == id {
			continue
		}
		// Verify the name (which may have been set by newInterface()) does not conflict with
		// existing bridge interfaces. Ironically the system chosen name gets stored in the config...
		// Basically we are checking if the two original configs were both empty.
		if nw.config.BridgeName == config.BridgeName {
			d.Unlock()
			return types.ForbiddenErrorf("conflicts with network %s (%s)", nw.id, nw.config.BridgeName)
		}
		// If this network config specifies the AddressIPv4, we need
		// to make sure it does not conflict with any previously allocated
		// bridges. This could not be completely caught by the config conflict
		// check, because networks which config does not specify the AddressIPv4
		// get their address and subnet selected by the driver (see electBridgeIPv4())
		if config.AddressIPv4 != nil {
			if nw.bridge.bridgeIPv4.Contains(config.AddressIPv4.IP) ||
				config.AddressIPv4.Contains(nw.bridge.bridgeIPv4.IP) {
				d.Unlock()
				return types.ForbiddenErrorf("conflicts with network %s (%s)", nw.id, nw.config.BridgeName)
			}
		}
	}
	d.Unlock()

	// Prepare the bridge setup configuration
	bridgeSetup := newBridgeSetup(config, bridgeIface)

	// If the bridge interface doesn't exist, we need to start the setup steps
	// by creating a new device and assigning it an IPv4 address.
	bridgeAlreadyExists := bridgeIface.exists()
	if !bridgeAlreadyExists {
		bridgeSetup.queueStep(setupDevice)
	}

	// Even if a bridge exists try to setup IPv4.
	bridgeSetup.queueStep(setupBridgeIPv4)

	// Conditionally queue setup steps depending on configuration values.
	for _, step := range []struct {
		Condition bool
		Fn        setupStep
	}{
		// Enable IPv6 on the bridge if required. We do this even for a
		// previously  existing bridge, as it may be here from a previous
		// installation where IPv6 wasn't supported yet and needs to be
		// assigned an IPv6 link-local address.
		{config.EnableIPv6, setupBridgeIPv6},

		// We ensure that the bridge has the expectedIPv4 and IPv6 addresses in
		// the case of a previously existing device.
		{bridgeAlreadyExists, setupVerifyAndReconcile},

		// Setup the bridge to allocate containers IPv4 addresses in the
		// specified subnet.
		{config.FixedCIDR != nil, setupFixedCIDRv4},

		// Setup the bridge to allocate containers global IPv6 addresses in the
		// specified subnet.
		{config.FixedCIDRv6 != nil, setupFixedCIDRv6},

		// Setup Loopback Adresses Routing
		{!config.EnableUserlandProxy, setupLoopbackAdressesRouting},

		// Setup IPTables.
		{config.EnableIPTables, network.setupIPTables},

		// Setup DefaultGatewayIPv4
		{config.DefaultGatewayIPv4 != nil, setupGatewayIPv4},

		// Setup DefaultGatewayIPv6
		{config.DefaultGatewayIPv6 != nil, setupGatewayIPv6},
	} {
		if step.Condition {
			bridgeSetup.queueStep(step.Fn)
		}
	}

	// Block bridge IP from being allocated.
	bridgeSetup.queueStep(allocateBridgeIP)
	// Apply the prepared list of steps, and abort at the first error.
	bridgeSetup.queueStep(setupDeviceUp)
	if err = bridgeSetup.apply(); err != nil {
		return err
	}

	return nil
}

func (d *driver) DeleteNetwork(nid types.UUID) error {
	var err error

	// Get network handler and remove it from driver
	d.Lock()
	n, ok := d.networks[nid]
	if !ok {
		d.Unlock()
		return types.InternalMaskableErrorf("network %s does not exist", nid)
	}
	delete(d.networks, nid)
	d.Unlock()

	// On failure set network handler back in driver, but
	// only if is not already taken over by some other thread
	defer func() {
		if err != nil {
			d.Lock()
			if _, ok := d.networks[nid]; !ok {
				d.networks[nid] = n
			}
			d.Unlock()
		}
	}()

	// Sanity check
	if n == nil {
		err = driverapi.ErrNoNetwork(nid)
		return err
	}

	// Cannot remove network if endpoints are still present
	if len(n.endpoints) != 0 {
		err = ActiveEndpointsError(n.id)
		return err
	}

	// Programming
	err = netlink.LinkDel(n.bridge.Link)

	return err
}

func (d *driver) CreateEndpoint(nid, eid types.UUID, epInfo driverapi.EndpointInfo, epOptions map[string]interface{}) error {
	var (
		ipv6Addr *net.IPNet
		err      error
	)

	if epInfo == nil {
		return errors.New("invalid endpoint info passed")
	}

	if len(epInfo.Interfaces()) != 0 {
		return errors.New("non empty interface list passed to bridge(local) driver")
	}

	// Get the network handler and make sure it exists
	d.Lock()
	n, ok := d.networks[nid]
	if !ok {
		d.Unlock()
		return types.NotFoundErrorf("network %s does not exist", nid)
	}
	config := n.config
	d.Unlock()
	if n == nil {
		return driverapi.ErrNoNetwork(nid)
	}

	// Sanity check
	n.Lock()
	if n.id != nid {
		n.Unlock()
		return InvalidNetworkIDError(nid)
	}
	n.Unlock()

	// Check if endpoint id is good and retrieve correspondent endpoint
	ep, err := n.getEndpoint(eid)
	if err != nil {
		return err
	}

	// Endpoint with that id exists either on desired or other sandbox
	if ep != nil {
		return driverapi.ErrEndpointExists(eid)
	}

	// Try to convert the options to endpoint configuration
	epConfig, err := parseEndpointOptions(epOptions)
	if err != nil {
		return err
	}

	// Create and add the endpoint
	n.Lock()
	endpoint := &bridgeEndpoint{id: eid, config: epConfig}
	n.endpoints[eid] = endpoint
	n.Unlock()

	// On failure make sure to remove the endpoint
	defer func() {
		if err != nil {
			n.Lock()
			delete(n.endpoints, eid)
			n.Unlock()
		}
	}()

	// Generate a name for what will be the host side pipe interface
	name1, err := generateIfaceName()
	if err != nil {
		return err
	}

	// Generate a name for what will be the sandbox side pipe interface
	name2, err := generateIfaceName()
	if err != nil {
		return err
	}

	// Generate and add the interface pipe host <-> sandbox
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: name1, TxQLen: 0},
		PeerName:  name2}
	if err = netlink.LinkAdd(veth); err != nil {
		return err
	}

	// Get the host side pipe interface handler
	host, err := netlink.LinkByName(name1)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			netlink.LinkDel(host)
		}
	}()

	// Get the sandbox side pipe interface handler
	sbox, err := netlink.LinkByName(name2)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			netlink.LinkDel(sbox)
		}
	}()

	// Set the sbox's MAC. If specified, use the one configured by user, otherwise use a random one
	mac := electMacAddress(epConfig)
	err = netlink.LinkSetHardwareAddr(sbox, mac)
	if err != nil {
		return err
	}
	endpoint.macAddress = mac

	// Add bridge inherited attributes to pipe interfaces
	if config.Mtu != 0 {
		err = netlink.LinkSetMTU(host, config.Mtu)
		if err != nil {
			return err
		}
		err = netlink.LinkSetMTU(sbox, config.Mtu)
		if err != nil {
			return err
		}
	}

	// Attach host side pipe interface into the bridge
	if err = netlink.LinkSetMaster(host,
		&netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: config.BridgeName}}); err != nil {
		return err
	}

	// v4 address for the sandbox side pipe interface
	ip4, err := ipAllocator.RequestIP(n.bridge.bridgeIPv4, nil)
	if err != nil {
		return err
	}
	ipv4Addr := &net.IPNet{IP: ip4, Mask: n.bridge.bridgeIPv4.Mask}

	// v6 address for the sandbox side pipe interface
	ipv6Addr = &net.IPNet{}
	if config.EnableIPv6 {
		var ip6 net.IP

		network := n.bridge.bridgeIPv6
		if config.FixedCIDRv6 != nil {
			network = config.FixedCIDRv6
		}

		ones, _ := network.Mask.Size()
		if ones <= 80 {
			ip6 = make(net.IP, len(network.IP))
			copy(ip6, network.IP)
			for i, h := range mac {
				ip6[i+10] = h
			}
		}

		ip6, err := ipAllocator.RequestIP(network, ip6)
		if err != nil {
			return err
		}

		ipv6Addr = &net.IPNet{IP: ip6, Mask: network.Mask}
	}

	// Create the sandbox side pipe interface
	intf := &sandbox.Interface{}
	intf.SrcName = name2
	intf.DstName = containerVethPrefix
	intf.Address = ipv4Addr

	if config.EnableIPv6 {
		intf.AddressIPv6 = ipv6Addr
	}

	// Store the interface in endpoint, this is needed for cleanup on DeleteEndpoint()
	endpoint.intf = intf

	err = epInfo.AddInterface(ifaceID, endpoint.macAddress, *ipv4Addr, *ipv6Addr)
	if err != nil {
		return err
	}

	// Program any required port mapping and store them in the endpoint
	endpoint.portMapping, err = n.allocatePorts(epConfig, intf, config.DefaultBindingIP, config.EnableUserlandProxy)
	if err != nil {
		return err
	}

	return nil
}

func (d *driver) DeleteEndpoint(nid, eid types.UUID) error {
	var err error

	// Get the network handler and make sure it exists
	d.Lock()
	n, ok := d.networks[nid]
	if !ok {
		d.Unlock()
		return types.NotFoundErrorf("network %s does not exist", nid)
	}
	config := n.config
	d.Unlock()
	if n == nil {
		return driverapi.ErrNoNetwork(nid)
	}

	// Sanity Check
	n.Lock()
	if n.id != nid {
		n.Unlock()
		return InvalidNetworkIDError(nid)
	}
	n.Unlock()

	// Check endpoint id and if an endpoint is actually there
	ep, err := n.getEndpoint(eid)
	if err != nil {
		return err
	}
	if ep == nil {
		return EndpointNotFoundError(eid)
	}

	// Remove it
	n.Lock()
	delete(n.endpoints, eid)
	n.Unlock()

	// On failure make sure to set back ep in n.endpoints, but only
	// if it hasn't been taken over already by some other thread.
	defer func() {
		if err != nil {
			n.Lock()
			if _, ok := n.endpoints[eid]; !ok {
				n.endpoints[eid] = ep
			}
			n.Unlock()
		}
	}()

	// Remove port mappings. Do not stop endpoint delete on unmap failure
	n.releasePorts(ep)

	// Release the v4 address allocated to this endpoint's sandbox interface
	err = ipAllocator.ReleaseIP(n.bridge.bridgeIPv4, ep.intf.Address.IP)
	if err != nil {
		return err
	}

	// Release the v6 address allocated to this endpoint's sandbox interface
	if config.EnableIPv6 {
		err := ipAllocator.ReleaseIP(n.bridge.bridgeIPv6, ep.intf.AddressIPv6.IP)
		if err != nil {
			return err
		}
	}

	// Try removal of link. Discard error: link pair might have
	// already been deleted by sandbox delete.
	link, err := netlink.LinkByName(ep.intf.SrcName)
	if err == nil {
		netlink.LinkDel(link)
	}

	return nil
}

func (d *driver) EndpointOperInfo(nid, eid types.UUID) (map[string]interface{}, error) {
	// Get the network handler and make sure it exists
	d.Lock()
	n, ok := d.networks[nid]
	if !ok {
		d.Unlock()
		return nil, types.NotFoundErrorf("network %s does not exist", nid)
	}
	d.Unlock()
	if n == nil {
		return nil, driverapi.ErrNoNetwork(nid)
	}

	// Sanity check
	n.Lock()
	if n.id != nid {
		n.Unlock()
		return nil, InvalidNetworkIDError(nid)
	}
	n.Unlock()

	// Check if endpoint id is good and retrieve correspondent endpoint
	ep, err := n.getEndpoint(eid)
	if err != nil {
		return nil, err
	}
	if ep == nil {
		return nil, driverapi.ErrNoEndpoint(eid)
	}

	m := make(map[string]interface{})

	if ep.portMapping != nil {
		// Return a copy of the operational data
		pmc := make([]types.PortBinding, 0, len(ep.portMapping))
		for _, pm := range ep.portMapping {
			pmc = append(pmc, pm.GetCopy())
		}
		m[netlabel.PortMap] = pmc
	}

	if len(ep.macAddress) != 0 {
		m[netlabel.MacAddress] = ep.macAddress
	}

	return m, nil
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(nid, eid types.UUID, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
	network, err := d.getNetwork(nid)
	if err != nil {
		return err
	}

	endpoint, err := network.getEndpoint(eid)
	if err != nil {
		return err
	}

	if endpoint == nil {
		return EndpointNotFoundError(eid)
	}

	for _, iNames := range jinfo.InterfaceNames() {
		// Make sure to set names on the correct interface ID.
		if iNames.ID() == ifaceID {
			err = iNames.SetNames(endpoint.intf.SrcName, endpoint.intf.DstName)
			if err != nil {
				return err
			}
		}
	}

	err = jinfo.SetGateway(network.bridge.gatewayIPv4)
	if err != nil {
		return err
	}

	err = jinfo.SetGatewayIPv6(network.bridge.gatewayIPv6)
	if err != nil {
		return err
	}

	if !network.config.EnableICC {
		return d.link(network, endpoint, options, true)
	}

	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid types.UUID) error {
	network, err := d.getNetwork(nid)
	if err != nil {
		return err
	}

	endpoint, err := network.getEndpoint(eid)
	if err != nil {
		return err
	}

	if endpoint == nil {
		return EndpointNotFoundError(eid)
	}

	if !network.config.EnableICC {
		return d.link(network, endpoint, nil, false)
	}

	return nil
}

func (d *driver) link(network *bridgeNetwork, endpoint *bridgeEndpoint, options map[string]interface{}, enable bool) error {
	var (
		cc  *containerConfiguration
		err error
	)

	if enable {
		cc, err = parseContainerOptions(options)
		if err != nil {
			return err
		}
	} else {
		cc = endpoint.containerConfig
	}

	if cc == nil {
		return nil
	}

	if endpoint.config != nil && endpoint.config.ExposedPorts != nil {
		for _, p := range cc.ParentEndpoints {
			var parentEndpoint *bridgeEndpoint
			parentEndpoint, err = network.getEndpoint(types.UUID(p))
			if err != nil {
				return err
			}
			if parentEndpoint == nil {
				err = InvalidEndpointIDError(p)
				return err
			}

			l := newLink(parentEndpoint.intf.Address.IP.String(),
				endpoint.intf.Address.IP.String(),
				endpoint.config.ExposedPorts, network.config.BridgeName)
			if enable {
				err = l.Enable()
				if err != nil {
					return err
				}
				defer func() {
					if err != nil {
						l.Disable()
					}
				}()
			} else {
				l.Disable()
			}
		}
	}

	for _, c := range cc.ChildEndpoints {
		var childEndpoint *bridgeEndpoint
		childEndpoint, err = network.getEndpoint(types.UUID(c))
		if err != nil {
			return err
		}
		if childEndpoint == nil {
			err = InvalidEndpointIDError(c)
			return err
		}
		if childEndpoint.config == nil || childEndpoint.config.ExposedPorts == nil {
			continue
		}

		l := newLink(endpoint.intf.Address.IP.String(),
			childEndpoint.intf.Address.IP.String(),
			childEndpoint.config.ExposedPorts, network.config.BridgeName)
		if enable {
			err = l.Enable()
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					l.Disable()
				}
			}()
		} else {
			l.Disable()
		}
	}

	if enable {
		endpoint.containerConfig = cc
	}

	return nil
}

func (d *driver) Type() string {
	return networkType
}

func parseEndpointOptions(epOptions map[string]interface{}) (*endpointConfiguration, error) {
	if epOptions == nil {
		return nil, nil
	}

	ec := &endpointConfiguration{}

	if opt, ok := epOptions[netlabel.MacAddress]; ok {
		if mac, ok := opt.(net.HardwareAddr); ok {
			ec.MacAddress = mac
		} else {
			return nil, &ErrInvalidEndpointConfig{}
		}
	}

	if opt, ok := epOptions[netlabel.PortMap]; ok {
		if bs, ok := opt.([]types.PortBinding); ok {
			ec.PortBindings = bs
		} else {
			return nil, &ErrInvalidEndpointConfig{}
		}
	}

	if opt, ok := epOptions[netlabel.ExposedPorts]; ok {
		if ports, ok := opt.([]types.TransportPort); ok {
			ec.ExposedPorts = ports
		} else {
			return nil, &ErrInvalidEndpointConfig{}
		}
	}

	return ec, nil
}

func parseContainerOptions(cOptions map[string]interface{}) (*containerConfiguration, error) {
	if cOptions == nil {
		return nil, nil
	}
	genericData := cOptions[netlabel.GenericData]
	if genericData == nil {
		return nil, nil
	}
	switch opt := genericData.(type) {
	case options.Generic:
		opaqueConfig, err := options.GenerateFromModel(opt, &containerConfiguration{})
		if err != nil {
			return nil, err
		}
		return opaqueConfig.(*containerConfiguration), nil
	case *containerConfiguration:
		return opt, nil
	default:
		return nil, nil
	}
}

func electMacAddress(epConfig *endpointConfiguration) net.HardwareAddr {
	if epConfig != nil && epConfig.MacAddress != nil {
		return epConfig.MacAddress
	}
	return netutils.GenerateRandomMAC()
}

// Generates a name to be used for a virtual ethernet
// interface. The name is constructed by 'veth' appended
// by a randomly generated hex value. (example: veth0f60e2c)
func generateIfaceName() (string, error) {
	for i := 0; i < 3; i++ {
		name, err := netutils.GenerateRandomName(vethPrefix, vethLen)
		if err != nil {
			continue
		}
		if _, err := net.InterfaceByName(name); err != nil {
			if strings.Contains(err.Error(), "no such") {
				return name, nil
			}
			return "", err
		}
	}
	return "", &ErrIfaceName{}
}
