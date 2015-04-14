package bridge

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/docker/libcontainer/utils"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/ipallocator"
	"github.com/docker/libnetwork/pkg/options"
	"github.com/docker/libnetwork/portmapper"
	"github.com/vishvananda/netlink"
)

const (
	networkType = "simplebridge"
	vethPrefix  = "veth"
)

var (
	once        sync.Once
	ipAllocator *ipallocator.IPAllocator
	portMapper  *portmapper.PortMapper
)

func initPortMapper() {
	once.Do(func() {
		portMapper = portmapper.New()
	})
}

// Configuration info for the "simplebridge" driver.
type Configuration struct {
	BridgeName         string
	AddressIPv4        *net.IPNet
	FixedCIDR          *net.IPNet
	FixedCIDRv6        *net.IPNet
	EnableIPv6         bool
	EnableIPTables     bool
	EnableIPMasquerade bool
	EnableICC          bool
	EnableIPForwarding bool
}

type bridgeEndpoint struct {
	id          driverapi.UUID
	addressIPv4 net.IP
	addressIPv6 net.IP
}

type bridgeNetwork struct {
	id driverapi.UUID
	// bridge interface points to the linux bridge and it's configuration
	bridge   *bridgeInterface
	endpoint *bridgeEndpoint
	sync.Mutex
}

type driver struct {
	network *bridgeNetwork
	sync.Mutex
}

func init() {
	ipAllocator = ipallocator.New()
	initPortMapper()
}

// New provides a new instance of bridge driver instance
func New() (string, driverapi.Driver) {
	return networkType, &driver{}
}

// Create a new network using simplebridge plugin
func (d *driver) CreateNetwork(id driverapi.UUID, option interface{}) error {

	var (
		config *Configuration
		err    error
	)

	d.Lock()
	if d.network != nil {
		d.Unlock()
		return fmt.Errorf("network already exists, simplebridge can only have one network")
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

	switch opt := option.(type) {
	case options.Generic:
		opaqueConfig, err := options.GenerateFromModel(opt, &Configuration{})
		if err != nil {
			return fmt.Errorf("failed to generate driver config: %v", err)
		}
		config = opaqueConfig.(*Configuration)
	case *Configuration:
		config = opt
	}

	bridgeIface := newInterface(config)
	bridgeSetup := newBridgeSetup(bridgeIface)

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
		{bridgeAlreadyExists, setupVerifyConfiguredAddresses},

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

func (d *driver) DeleteNetwork(nid driverapi.UUID) error {
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
		err = fmt.Errorf("Network %s has active endpoint %s", n.id, n.endpoint.id)
		return err
	}

	err = netlink.LinkDel(n.bridge.Link)
	return err
}

func (d *driver) CreateEndpoint(nid, eid driverapi.UUID, sboxKey string, config interface{}) (*driverapi.SandboxInfo, error) {
	var (
		ipv6Addr net.IPNet
		err      error
	)

	d.Lock()
	n := d.network
	d.Unlock()
	if n == nil {
		return nil, driverapi.ErrNoNetwork
	}

	n.Lock()
	if n.id != nid {
		n.Unlock()
		return nil, fmt.Errorf("invalid network id %s", nid)
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
		&netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: n.bridge.Config.BridgeName}}); err != nil {
		return nil, err
	}

	ip4, err := ipAllocator.RequestIP(n.bridge.bridgeIPv4, nil)
	if err != nil {
		return nil, err
	}
	ipv4Addr := net.IPNet{IP: ip4, Mask: n.bridge.bridgeIPv4.Mask}

	if n.bridge.Config.EnableIPv6 {
		ip6, err := ipAllocator.RequestIP(n.bridge.bridgeIPv6, nil)
		if err != nil {
			return nil, err
		}
		ipv6Addr = net.IPNet{IP: ip6, Mask: n.bridge.bridgeIPv6.Mask}
	}

	var interfaces []*driverapi.Interface
	sinfo := &driverapi.SandboxInfo{}

	intf := &driverapi.Interface{}
	intf.SrcName = name2
	intf.DstName = "eth0"
	intf.Address = ipv4Addr
	sinfo.Gateway = n.bridge.bridgeIPv4.IP
	if n.bridge.Config.EnableIPv6 {
		intf.AddressIPv6 = ipv6Addr
		sinfo.GatewayIPv6 = n.bridge.bridgeIPv6.IP
	}

	n.endpoint.addressIPv4 = ip4
	n.endpoint.addressIPv6 = ipv6Addr.IP
	interfaces = append(interfaces, intf)
	sinfo.Interfaces = interfaces
	return sinfo, nil
}

func (d *driver) DeleteEndpoint(nid, eid driverapi.UUID) error {
	var err error

	d.Lock()
	n := d.network
	d.Unlock()
	if n == nil {
		return driverapi.ErrNoNetwork
	}

	n.Lock()
	if n.id != nid {
		n.Unlock()
		return fmt.Errorf("invalid network id %s", nid)
	}

	if n.endpoint == nil {
		n.Unlock()
		return driverapi.ErrNoEndpoint
	}

	ep := n.endpoint
	if ep.id != eid {
		n.Unlock()
		return fmt.Errorf("invalid endpoint id %s", eid)
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

	if n.bridge.Config.EnableIPv6 {
		err := ipAllocator.ReleaseIP(n.bridge.bridgeIPv6, n.endpoint.addressIPv6)
		if err != nil {
			return err
		}
	}

	return nil
}

func generateIfaceName() (string, error) {
	for i := 0; i < 10; i++ {
		name, err := utils.GenerateRandomName("veth", 7)
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
	return "", errors.New("Failed to find name for new interface")
}
