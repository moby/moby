package bridge

import (
	"net"

	"github.com/docker/libnetwork"
)

const (
	networkType = "simplebridge"
	vethPrefix  = "veth"
)

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

type driver struct{}

func init() {
	libnetwork.RegisterNetworkType(networkType, &driver{}, &Configuration{})
}

// Create a new network using simplebridge plugin
func (d *driver) CreateNetwork(name string, opaqueConfig interface{}) (libnetwork.Network, error) {
	config := opaqueConfig.(*Configuration)
	bridgeIntfc := newInterface(config)
	bridgeSetup := newBridgeSetup(bridgeIntfc)

	// If the bridge interface doesn't exist, we need to start the setup steps
	// by creating a new device and assigning it an IPv4 address.
	bridgeAlreadyExists := bridgeIntfc.exists()
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
	if err := bridgeSetup.apply(); err != nil {
		return nil, err
	}

	return &bridgeNetwork{NetworkName: name, Config: *config}, nil
}
