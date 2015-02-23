package bridge

import "github.com/vishvananda/netlink"

type Interface struct {
	Config *Configuration
	Link   netlink.Link
}

func NewInterface(config *Configuration) *Interface {
	i := &Interface{
		Config: config,
	}

	// Attempt to find an existing bridge named with the specified name.
	i.Link, _ = netlink.LinkByName(i.Config.BridgeName)
	return i
}

// Exists indicates if the existing bridge interface exists on the system.
func (i *Interface) Exists() bool {
	return i.Link != nil
}
