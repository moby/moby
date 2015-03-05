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

type driver struct{}

func init() {
	libnetwork.RegisterNetworkType(networkType, &driver{}, &Configuration{})
}

// Create a new network using simplebridge plugin
func (d *driver) CreateNetwork(name string, opaqueConfig interface{}) (libnetwork.Network, error) {
	config := opaqueConfig.(*Configuration)
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

	return &bridgeNetwork{NetworkName: name, Config: *config}, nil
}
