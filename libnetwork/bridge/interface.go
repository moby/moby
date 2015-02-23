package bridge

import "github.com/vishvananda/netlink"

const (
	DefaultBridgeName = "docker0"
)

type Interface struct {
	Config *Configuration
	Link   netlink.Link
}

func NewInterface(config *Configuration) *Interface {
	i := &Interface{
		Config: config,
	}

	// Initialize the bridge name to the default if unspecified.
	if i.Config.BridgeName == "" {
		i.Config.BridgeName = DefaultBridgeName
	}

	// Attempt to find an existing bridge named with the specified name.
	i.Link, _ = netlink.LinkByName(i.Config.BridgeName)
	return i
}

// Exists indicates if the existing bridge interface exists on the system.
func (i *Interface) Exists() bool {
	return i.Link != nil
}

// Addresses returns a single IPv4 address and all IPv6 addresses for the
// bridge interface.
func (i *Interface) Addresses() (netlink.Addr, []netlink.Addr, error) {
	v4addr, err := netlink.AddrList(i.Link, netlink.FAMILY_V4)
	if err != nil {
		return netlink.Addr{}, nil, err
	}

	v6addr, err := netlink.AddrList(i.Link, netlink.FAMILY_V6)
	if err != nil {
		return netlink.Addr{}, nil, err
	}

	return v4addr[0], v6addr, nil
}
