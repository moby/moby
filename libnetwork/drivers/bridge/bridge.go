package bridge

import (
	"net"
	"strings"
	"sync"

	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/ipallocator"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/pkg/options"
	"github.com/docker/libnetwork/portmapper"
	"github.com/docker/libnetwork/sandbox"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
)

const (
	networkType   = "bridge"
	vethPrefix    = "veth"
	vethLen       = 7
	containerVeth = "eth0"
)

var (
	ipAllocator *ipallocator.IPAllocator
	portMapper  *portmapper.PortMapper
)

// Configuration info for the "bridge" driver.
type Configuration struct {
	BridgeName            string
	AddressIPv4           *net.IPNet
	FixedCIDR             *net.IPNet
	FixedCIDRv6           *net.IPNet
	EnableIPv6            bool
	EnableIPTables        bool
	EnableIPMasquerade    bool
	EnableICC             bool
	EnableIPForwarding    bool
	AllowNonDefaultBridge bool
	Mtu                   int
	DefaultGatewayIPv4    net.IP
	DefaultGatewayIPv6    net.IP
}

// EndpointConfiguration represents the user specified configuration for the sandbox endpoint
type EndpointConfiguration struct {
	MacAddress net.HardwareAddr
}

type bridgeEndpoint struct {
	id     types.UUID
	port   *sandbox.Interface
	config *EndpointConfiguration // User specified parameters
}

type bridgeNetwork struct {
	id        types.UUID
	bridge    *bridgeInterface           // The bridge's L3 interface
	endpoints map[string]*bridgeEndpoint // key: sandbox id
	sync.Mutex
}

type driver struct {
	config  *Configuration
	network *bridgeNetwork
	sync.Mutex
}

func init() {
	ipAllocator = ipallocator.New()
	portMapper = portmapper.New()
}

// New provides a new instance of bridge driver
func New() (string, driverapi.Driver) {
	return networkType, &driver{}
}

// Validate performs a static validation on the configuration parameters.
// Whatever can be assessed a priori before attempting any programming.
func (c *Configuration) Validate() error {
	if c.Mtu < 0 {
		return ErrInvalidMtu
	}

	// If bridge v4 subnet is specified
	if c.AddressIPv4 != nil {
		// If Container restricted subnet is specified, it must be a subset of bridge subnet
		if c.FixedCIDR != nil {
			// Check Network address
			if !c.AddressIPv4.Contains(c.FixedCIDR.IP) {
				return ErrInvalidContainerSubnet
			}
			// Check it is effectively a subset
			brNetLen, _ := c.AddressIPv4.Mask.Size()
			cnNetLen, _ := c.FixedCIDR.Mask.Size()
			if brNetLen > cnNetLen {
				return ErrInvalidContainerSubnet
			}
		}
		// If default gw is specified, it must be part of bridge subnet
		if c.DefaultGatewayIPv4 != nil {
			if !c.AddressIPv4.Contains(c.DefaultGatewayIPv4) {
				return ErrInvalidGateway
			}
		}
	}

	// If default v6 gw is specified, FixedCIDRv6 must be specified and gw must belong to FixedCIDRv6 subnet
	if c.EnableIPv6 && c.DefaultGatewayIPv6 != nil {
		if c.FixedCIDRv6 == nil || !c.FixedCIDRv6.Contains(c.DefaultGatewayIPv6) {
			return ErrInvalidGateway
		}
	}

	return nil
}

func (n *bridgeNetwork) getEndpoint(eid types.UUID) (string, *bridgeEndpoint, error) {
	n.Lock()
	defer n.Unlock()

	if eid == "" {
		return "", nil, InvalidEndpointIDError(eid)
	}

	for sk, ep := range n.endpoints {
		if ep.id == eid {
			return sk, ep, nil
		}
	}

	return "", nil, nil
}

func (d *driver) Config(option interface{}) error {
	var config *Configuration

	d.Lock()
	defer d.Unlock()

	if d.config != nil {
		return ErrConfigExists
	}

	switch opt := option.(type) {
	case options.Generic:
		opaqueConfig, err := options.GenerateFromModel(opt, &Configuration{})
		if err != nil {
			return err
		}
		config = opaqueConfig.(*Configuration)
	case *Configuration:
		config = opt
	}

	if err := config.Validate(); err != nil {
		return err
	}

	d.config = config

	return nil
}

// Create a new network using bridge plugin
func (d *driver) CreateNetwork(id types.UUID, option interface{}) error {
	var err error

	// Driver must be configured
	d.Lock()
	if d.config == nil {
		d.Unlock()
		return ErrInvalidConfig
	}
	config := d.config

	// Sanity checks
	if d.network != nil {
		d.Unlock()
		return ErrNetworkExists
	}

	// Create and set network handler in driver
	d.network = &bridgeNetwork{id: id, endpoints: make(map[string]*bridgeEndpoint)}
	d.Unlock()

	// On failure make sure to reset driver network handler to nil
	defer func() {
		if err != nil {
			d.Lock()
			d.network = nil
			d.Unlock()
		}
	}()

	// Create or retrieve the bridge L3 interface
	bridgeIface := newInterface(config)
	d.network.bridge = bridgeIface

	// Prepare the bridge setup configuration
	bridgeSetup := newBridgeSetup(config, bridgeIface)

	// If the bridge interface doesn't exist, we need to start the setup steps
	// by creating a new device and assigning it an IPv4 address.
	bridgeAlreadyExists := bridgeIface.exists()
	if !bridgeAlreadyExists {
		bridgeSetup.queueStep(setupDevice)
		bridgeSetup.queueStep(setupBridgeIPv4)
	}

	// Conditionnally queue setup steps depending on configuration values.
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

		// Setup IPTables.
		{config.EnableIPTables, setupIPTables},

		// Setup IP forwarding.
		{config.EnableIPForwarding, setupIPForwarding},

		// Setup DefaultGatewayIPv4
		{config.DefaultGatewayIPv4 != nil, setupGatewayIPv4},

		// Setup DefaultGatewayIPv6
		{config.DefaultGatewayIPv6 != nil, setupGatewayIPv6},
	} {
		if step.Condition {
			bridgeSetup.queueStep(step.Fn)
		}
	}

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
	n := d.network
	d.network = nil
	d.Unlock()

	// On failure set network handler back in driver, but
	// only if is not already taken over by some other thread
	defer func() {
		if err != nil {
			d.Lock()
			if d.network == nil {
				d.network = n
			}
			d.Unlock()
		}
	}()

	// Sanity check
	if n == nil {
		err = driverapi.ErrNoNetwork
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

func (d *driver) CreateEndpoint(nid, eid types.UUID, sboxKey string, epOptions interface{}) (*sandbox.Info, error) {
	var (
		ipv6Addr *net.IPNet
		err      error
	)

	// Get the network handler and make sure it exists
	d.Lock()
	n := d.network
	config := d.config
	d.Unlock()
	if n == nil {
		return nil, driverapi.ErrNoNetwork
	}

	// Sanity check
	n.Lock()
	if n.id != nid {
		n.Unlock()
		return nil, InvalidNetworkIDError(nid)
	}
	n.Unlock()

	// Check if endpoint id is good and retrieve correspondent endpoint
	_, ep, err := n.getEndpoint(eid)
	if err != nil {
		return nil, err
	}

	// Endpoint with that id exists either on desired or other sandbox
	if ep != nil {
		return nil, driverapi.ErrEndpointExists
	}

	// Check if valid sandbox key
	if sboxKey == "" {
		return nil, InvalidSandboxIDError(sboxKey)
	}

	// Check if endpoint already exists for this sandbox
	n.Lock()
	if _, ok := n.endpoints[sboxKey]; ok {
		n.Unlock()
		return nil, driverapi.ErrEndpointExists
	}

	// Try to convert the options to endpoint configuration
	epConfig, err := parseEndpointOptions(epOptions)
	if err != nil {
		n.Unlock()
		return nil, err
	}

	// Create and add the endpoint
	endpoint := &bridgeEndpoint{id: eid, config: epConfig}
	n.endpoints[sboxKey] = endpoint
	n.Unlock()

	// On failure make sure to remove the endpoint
	defer func() {
		if err != nil {
			n.Lock()
			delete(n.endpoints, sboxKey)
			n.Unlock()
		}
	}()

	// Generate a name for what will be the host side pipe interface
	name1, err := generateIfaceName()
	if err != nil {
		return nil, err
	}

	// Generate a name for what will be the sandbox side pipe interface
	name2, err := generateIfaceName()
	if err != nil {
		return nil, err
	}

	// Generate and add the interface pipe host <-> sandbox
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: name1, TxQLen: 0},
		PeerName:  name2}
	if err = netlink.LinkAdd(veth); err != nil {
		return nil, err
	}

	// Get the host side pipe interface handler
	host, err := netlink.LinkByName(name1)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			netlink.LinkDel(host)
		}
	}()

	// Get the sandbox side pipe interface handler
	sbox, err := netlink.LinkByName(name2)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			netlink.LinkDel(sbox)
		}
	}()

	// Add user specified attributes
	if epConfig != nil && epConfig.MacAddress != nil {
		err = netlink.LinkSetHardwareAddr(sbox, epConfig.MacAddress)
		if err != nil {
			return nil, err
		}
	}

	// Add bridge inherited attributes to pipe interfaces
	if config.Mtu != 0 {
		err = netlink.LinkSetMTU(host, config.Mtu)
		if err != nil {
			return nil, err
		}
		err = netlink.LinkSetMTU(sbox, config.Mtu)
		if err != nil {
			return nil, err
		}
	}

	// Attach host side pipe interface into the bridge
	if err = netlink.LinkSetMaster(host,
		&netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: config.BridgeName}}); err != nil {
		return nil, err
	}

	// v4 address for the sandbox side pipe interface
	ip4, err := ipAllocator.RequestIP(n.bridge.bridgeIPv4, nil)
	if err != nil {
		return nil, err
	}
	ipv4Addr := &net.IPNet{IP: ip4, Mask: n.bridge.bridgeIPv4.Mask}

	// v6 address for the sandbox side pipe interface
	if config.EnableIPv6 {
		ip6, err := ipAllocator.RequestIP(n.bridge.bridgeIPv6, nil)
		if err != nil {
			return nil, err
		}
		ipv6Addr = &net.IPNet{IP: ip6, Mask: n.bridge.bridgeIPv6.Mask}
	}

	// Store the sandbox side pipe interface
	// This is needed for cleanup on DeleteEndpoint()
	intf := &sandbox.Interface{}
	intf.SrcName = name2
	intf.DstName = containerVeth
	intf.Address = ipv4Addr

	// Update endpoint with the sandbox interface info
	endpoint.port = intf

	// Generate the sandbox info to return
	sinfo := &sandbox.Info{Interfaces: []*sandbox.Interface{intf}}

	// Set the default gateway(s) for the sandbox
	sinfo.Gateway = n.bridge.gatewayIPv4
	if config.EnableIPv6 {
		intf.AddressIPv6 = ipv6Addr
		sinfo.GatewayIPv6 = n.bridge.gatewayIPv6
	}

	return sinfo, nil
}

func (d *driver) DeleteEndpoint(nid, eid types.UUID) error {
	var err error

	// Get the network handler and make sure it exists
	d.Lock()
	n := d.network
	config := d.config
	d.Unlock()
	if n == nil {
		return driverapi.ErrNoNetwork
	}

	// Sanity Check
	n.Lock()
	if n.id != nid {
		n.Unlock()
		return InvalidNetworkIDError(nid)
	}
	n.Unlock()

	// Check endpoint id and if an endpoint is actually there
	sboxKey, ep, err := n.getEndpoint(eid)
	if err != nil {
		return err
	}
	if ep == nil {
		return EndpointNotFoundError(eid)
	}

	// Remove it
	n.Lock()
	delete(n.endpoints, sboxKey)
	n.Unlock()

	// On failure make sure to set back ep in n.endpoints, but only
	// if it hasn't been taken over already by some other thread.
	defer func() {
		if err != nil {
			n.Lock()
			if _, ok := n.endpoints[sboxKey]; !ok {
				n.endpoints[sboxKey] = ep
			}
			n.Unlock()
		}
	}()

	// Release the v4 address allocated to this endpoint's sandbox interface
	err = ipAllocator.ReleaseIP(n.bridge.bridgeIPv4, ep.port.Address.IP)
	if err != nil {
		return err
	}

	// Release the v6 address allocated to this endpoint's sandbox interface
	if config.EnableIPv6 {
		err := ipAllocator.ReleaseIP(n.bridge.bridgeIPv6, ep.port.AddressIPv6.IP)
		if err != nil {
			return err
		}
	}

	// Try removal of link. Discard error: link pair might have
	// already been deleted by sandbox delete.
	link, err := netlink.LinkByName(ep.port.SrcName)
	if err == nil {
		netlink.LinkDel(link)
	}

	return nil
}

func (d *driver) Type() string {
	return networkType
}

func parseEndpointOptions(epOptions interface{}) (*EndpointConfiguration, error) {
	if epOptions == nil {
		return nil, nil
	}
	switch opt := epOptions.(type) {
	case options.Generic:
		opaqueConfig, err := options.GenerateFromModel(opt, &EndpointConfiguration{})
		if err != nil {
			return nil, err
		}
		return opaqueConfig.(*EndpointConfiguration), nil
	case *EndpointConfiguration:
		return opt, nil
	default:
		return nil, ErrInvalidEndpointConfig
	}
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
	return "", ErrIfaceName
}
