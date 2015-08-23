package bridge

import (
	"net"

	"github.com/vishvananda/netlink"
)

const (
	// DefaultBridgeName is the default name for the bridge interface managed
	// by the driver when unspecified by the caller.
	DefaultBridgeName = "docker0"
)

// Interface models the bridge network device.
type bridgeInterface struct {
	Link        netlink.Link
	bridgeIPv4  *net.IPNet
	bridgeIPv6  *net.IPNet
	gatewayIPv4 net.IP
	gatewayIPv6 net.IP
}

// newInterface creates a new bridge interface structure. It attempts to find
// an already existing device identified by the configuration BridgeName field,
// or the default bridge name when unspecified), but doesn't attempt to create
// one when missing
func newInterface(config *networkConfiguration) *bridgeInterface {
	i := &bridgeInterface{}

	// Initialize the bridge name to the default if unspecified.
	if config.BridgeName == "" {
		config.BridgeName = DefaultBridgeName
	}

	// Attempt to find an existing bridge named with the specified name.
	i.Link, _ = netlink.LinkByName(config.BridgeName)
	return i
}

// exists indicates if the existing bridge interface exists on the system.
func (i *bridgeInterface) exists() bool {
	return i.Link != nil
}

// addresses returns a single IPv4 address and all IPv6 addresses for the
// bridge interface.
func (i *bridgeInterface) addresses() (netlink.Addr, []netlink.Addr, error) {
	v4addr, err := netlink.AddrList(i.Link, netlink.FAMILY_V4)
	if err != nil {
		return netlink.Addr{}, nil, err
	}

	v6addr, err := netlink.AddrList(i.Link, netlink.FAMILY_V6)
	if err != nil {
		return netlink.Addr{}, nil, err
	}

	if len(v4addr) == 0 {
		return netlink.Addr{}, v6addr, nil
	}
	return v4addr[0], v6addr, nil
}
