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
}

type bridgeEndpoint struct {
	id          types.UUID
	addressIPv4 net.IP
	addressIPv6 net.IP
}

type bridgeNetwork struct {
	id types.UUID
	// bridge interface points to the linux bridge and it's configuration
	bridge   *bridgeInterface
	endpoint *bridgeEndpoint
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

// New provides a new instance of bridge driver instance
func New() (string, driverapi.Driver) {
	return networkType, &driver{}
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

	d.config = config
	return nil
}

// Create a new network using bridge plugin
func (d *driver) CreateNetwork(id types.UUID, option interface{}) error {

	var (
		err error
	)

	d.Lock()
	if d.config == nil {
		d.Unlock()
		return ErrInvalidConfig
	}
	config := d.config

	if d.network != nil {
		d.Unlock()
		return ErrNetworkExists
	}
	d.network = &bridgeNetwork{id: id}
	d.Unlock()
	defer func() {
		// On failure make sure to reset d.network to nil
		if err != nil {
			d.Lock()
			d.network = nil
			d.Unlock()
		}
	}()

	bridgeIface := newInterface(config)
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

	d.network.bridge = bridgeIface
	return nil
}

func (d *driver) DeleteNetwork(nid types.UUID) error {
	var err error
	d.Lock()
	n := d.network
	d.network = nil
	d.Unlock()
	defer func() {
		if err != nil {
			// On failure set d.network back to n
			// but only if is not already take over
			// by some other thread
			d.Lock()
			if d.network == nil {
				d.network = n
			}
			d.Unlock()
		}
	}()

	if n == nil {
		err = driverapi.ErrNoNetwork
		return err
	}

	if n.endpoint != nil {
		err = &ActiveEndpointsError{nid: string(n.id), eid: string(n.endpoint.id)}
		return err
	}

	err = netlink.LinkDel(n.bridge.Link)
	return err
}

func (d *driver) CreateEndpoint(nid, eid types.UUID, sboxKey string, epOption interface{}) (*sandbox.Info, error) {
	var (
		ipv6Addr *net.IPNet
		ip6      net.IP
		err      error
	)

	d.Lock()
	n := d.network
	config := d.config
	d.Unlock()
	if n == nil {
		return nil, driverapi.ErrNoNetwork
	}

	n.Lock()
	if n.id != nid {
		n.Unlock()
		return nil, InvalidNetworkIDError(nid)
	}

	if n.endpoint != nil {
		n.Unlock()
		return nil, driverapi.ErrEndpointExists
	}
	n.endpoint = &bridgeEndpoint{id: eid}
	n.Unlock()
	defer func() {
		// On failye make sure to reset n.endpoint to nil
		if err != nil {
			n.Lock()
			n.endpoint = nil
			n.Unlock()
		}
	}()

	name1, err := generateIfaceName()
	if err != nil {
		return nil, err
	}

	name2, err := generateIfaceName()
	if err != nil {
		return nil, err
	}

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: name1, TxQLen: 0},
		PeerName:  name2}
	if err = netlink.LinkAdd(veth); err != nil {
		return nil, err
	}

	host, err := netlink.LinkByName(name1)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			netlink.LinkDel(host)
		}
	}()

	container, err := netlink.LinkByName(name2)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			netlink.LinkDel(container)
		}
	}()

	if err = netlink.LinkSetMaster(host,
		&netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: config.BridgeName}}); err != nil {
		return nil, err
	}

	ip4, err := ipAllocator.RequestIP(n.bridge.bridgeIPv4, nil)
	if err != nil {
		return nil, err
	}
	ipv4Addr := &net.IPNet{IP: ip4, Mask: n.bridge.bridgeIPv4.Mask}

	if config.EnableIPv6 {
		ip6, err := ipAllocator.RequestIP(n.bridge.bridgeIPv6, nil)
		if err != nil {
			return nil, err
		}
		ipv6Addr = &net.IPNet{IP: ip6, Mask: n.bridge.bridgeIPv6.Mask}
	}

	var interfaces []*sandbox.Interface
	sinfo := &sandbox.Info{}

	intf := &sandbox.Interface{}
	intf.SrcName = name2
	intf.DstName = containerVeth
	intf.Address = ipv4Addr
	sinfo.Gateway = n.bridge.bridgeIPv4.IP
	if config.EnableIPv6 {
		intf.AddressIPv6 = ipv6Addr
		sinfo.GatewayIPv6 = n.bridge.bridgeIPv6.IP
	}

	n.endpoint.addressIPv4 = ip4
	n.endpoint.addressIPv6 = ip6
	interfaces = append(interfaces, intf)
	sinfo.Interfaces = interfaces
	return sinfo, nil
}

func (d *driver) DeleteEndpoint(nid, eid types.UUID) error {
	var err error

	d.Lock()
	n := d.network
	config := d.config
	d.Unlock()
	if n == nil {
		return driverapi.ErrNoNetwork
	}

	n.Lock()
	if n.id != nid {
		n.Unlock()
		return InvalidNetworkIDError(nid)
	}

	if n.endpoint == nil {
		n.Unlock()
		return driverapi.ErrNoEndpoint
	}

	ep := n.endpoint
	if ep.id != eid {
		n.Unlock()
		return InvalidEndpointIDError(eid)
	}

	n.endpoint = nil
	n.Unlock()
	defer func() {
		if err != nil {
			// On failure make to set back n.endpoint with ep
			// but only if it hasn't been taken over
			// already by some other thread.
			n.Lock()
			if n.endpoint == nil {
				n.endpoint = ep
			}
			n.Unlock()
		}
	}()

	err = ipAllocator.ReleaseIP(n.bridge.bridgeIPv4, ep.addressIPv4)
	if err != nil {
		return err
	}

	if config.EnableIPv6 {
		err := ipAllocator.ReleaseIP(n.bridge.bridgeIPv6, ep.addressIPv6)
		if err != nil {
			return err
		}
	}

	return nil
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
