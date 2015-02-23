package bridge

import (
	"net"

	"github.com/docker/libnetwork"
)

const (
	NetworkType = "simplebridge"
	VethPrefix  = "veth"
)

type Configuration struct {
	BridgeName         string
	AddressIPv4        *net.IPNet
	FixedCIDR          *net.IPNet
	FixedCIDRv6        *net.IPNet
	EnableIPv6         bool
	EnableIPTables     bool
	EnableIPForwarding bool
}

func init() {
	libnetwork.RegisterNetworkType(NetworkType, Create, &Configuration{})
}

/*

func Create()

- NewBridgeInterface(*Configuration) (*BridgeInterface, error)
	. Issues LinkByName on config.BridgeName
- Create BridgeSetup instance with sequence of steps
- if !bridgeInterface.Exists()
	. Add DeviceCreation (error if non-default name)
	. Add AddressIPv4: set IPv4 (with automatic election if necessary)
- General case
	. Add option EnableIPv6 if no IPv6 on bridge (disable_ipv6=0 + set IPv6)
	. Verify configured addresses (with v4 and v6 updated in config)
	. Add FixedCIDR v4: register subnet on IP Allocator
	. Add FixedCIDR v6: register subnet on IP Allocator, route
- Add IPTables setup
- Add IPForward setup (depends on FixedCIDRv6)
- err := bridgeSetup.Apply()

*/

func Create(config *Configuration) (libnetwork.Network, error) {
	bridgeIntfc := NewInterface(config)
	bridgeSetup := NewBridgeSetup(bridgeIntfc)

	// If the bridge interface doesn't exist, we need to start the setup steps
	// by creating a new device and assigning it an IPv4 address.
	bridgeAlreadyExists := bridgeIntfc.Exists()
	if !bridgeAlreadyExists {
		bridgeSetup.QueueStep(SetupDevice)
		bridgeSetup.QueueStep(SetupBridgeIPv4)
	}

	// Conditionnally queue setup steps depending on configuration values.
	for _, step := range []struct {
		Condition bool
		Fn        SetupStep
	}{
		// Enable IPv6 on the bridge if required. We do this even for a
		// previously  existing bridge, as it may be here from a previous
		// installation where IPv6 wasn't supported yet and needs to be
		// assigned an IPv6 link-local address.
		{config.EnableIPv6, SetupBridgeIPv6},

		// We ensure that the bridge has the expectedIPv4 and IPv6 addresses in
		// the case of a previously existing device.
		{bridgeAlreadyExists, SetupVerifyConfiguredAddresses},

		// Setup the bridge to allocate containers IPv4 addresses in the
		// specified subnet.
		{config.FixedCIDR != nil, SetupFixedCIDRv4},

		// Setup the bridge to allocate containers global IPv6 addresses in the
		// specified subnet.
		{config.FixedCIDRv6 != nil, SetupFixedCIDRv6},

		// Setup IPTables.
		{config.EnableIPTables, SetupIPTables},

		// Setup IP forwarding.
		{config.EnableIPForwarding, SetupIPForwarding},
	} {
		if step.Condition {
			bridgeSetup.QueueStep(step.Fn)
		}
	}

	// Apply the prepared list of steps, and abort at the first error.
	bridgeSetup.QueueStep(SetupDeviceUp)
	if err := bridgeSetup.Apply(); err != nil {
		return nil, err
	}

	return &bridgeNetwork{*config}, nil
}

/*
func Create(config *Configuration) (libnetwork.Network, error) {
	var (
		addrv4  netlink.Addr
		addrsv6 []netlink.Addr
	)

	b := &bridgeNetwork{Config: *config}
	if b.Config.BridgeName == "" {
		b.Config.BridgeName = DefaultBridge
	}

	link, err := netlink.LinkByName(b.Config.BridgeName)
	if err != nil {
		// The bridge interface doesn't exist, but we only attempt to create it
		// if using the default name.
		if b.Config.BridgeName != DefaultBridge {
			return nil, err
		}

		// Create the bridge interface.
		if addrv4, addrsv6, err = createBridge(&b.Config); err != nil {
			return nil, err
		}
	} else {
		// The bridge interface exists: start by getting its configured
		// addresses and verify if it matches the requested configuration.
		addrv4, addrsv6, err = getInterfaceAddr(link)
		if err != nil {
			return nil, err
		}

		if b.Config.AddressIPv4 != "" {
			bridgeIP, _, err := net.ParseCIDR(b.Config.AddressIPv4)
			if err != nil {
				return nil, err
			}
			if !addrv4.IP.Equal(bridgeIP) {
				return nil, fmt.Errorf("Bridge IP %s does not match requested configuration %s", addrv4.IP, bridgeIP)
			}
		}

		// A bridge might exist but not have any IPv6 addr associated with it
		// yet (for example, an existing Docker installation that has only been
		// used with IPv4 and docker0 already is set up). In that case, we can
		// perform the bridge init for IPv6 here, else we will error out below
		// if --ipv6=true.
		if len(addrsv6) == 0 && config.EnableIPv6 {
			if err := setupIPv6Bridge(iface, config); err != nil {
				return nil, err
			}
		}
	}

	return b, nil
}
*/

/*
func createBridge(config *Configuration) (netlink.Addr, []netlink.Addr, error) {
	// Formats an error return with default values.
	fmtError := func(format string, params ...interface{}) (netlink.Addr, []netlink.Addr, error) {
		return netlink.Addr{}, nil, fmt.Errorf(format, params...)
	}

	// Elect a subnet for the bridge interface.
	bridgeIPNet, err := electBridgeNetwork(config)
	if err != nil {
		return fmtError("Failed to elect bridge network: %v", err)
	}
	log.Debugf("Creating bridge interface %q with network %s", config.BridgeName, bridgeIPNet)

	// We attempt to create the bridge, and ignore the returned error if it is
	// already existing.
	iface, err := createBridgeInterface(config.BridgeName)
	if err != nil && !os.IsExist(err) {
		return netlink.Addr{}, nil, err
	}

	// Configure bridge IPv4.
	if err := netlink.AddrAdd(iface, &netlink.Addr{bridgeIPNet, ""}); err != nil {
		return fmtError("Failed to add address %s to bridge: %v", bridgeIPNet, err)
	}

	// Configure bridge IPv6.
	if config.EnableIPv6 {
		if err := setupIPv6Bridge(iface, config); err != nil {
			return fmtError("Failed to setup bridge IPv6: %v", err)
		}
	}

	// Up the bridge interface.
	if err := netlink.LinkSetUp(iface); err != nil {
		return fmtError("Failed to up network bridge: %v", err)
	}

	if config.FixedCIDRv6 != "" {
		dest, network, err := net.ParseCIDR(config.FixedCIDRv6)
		if err != nil {
			return fmtError("Invalid bridge fixed CIDR IPv6 %q: %v", config.FixedCIDRv6, err)
		}

		// Set route to global IPv6 subnet
		log.Infof("Adding route to IPv6 network %q via device %q", dest, iface)
		if err := netlink.RouteAdd(&netlink.Route{Dst: network, LinkIndex: iface.Attrs().Index}); err != nil {
			return fmtError("Could not add route to IPv6 network %q via device %q", config.FixedCIDRv6, iface)
		}
	}

	return getInterfaceAddrByName(config.BridgeName)
}
*/

type bridgeNetwork struct {
	Config Configuration
}

func (b *bridgeNetwork) Type() string {
	return NetworkType
}

func (b *bridgeNetwork) Link(name string) ([]*libnetwork.Interface, error) {
	return nil, nil
}
