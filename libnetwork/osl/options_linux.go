package osl

import "net"

func (nh *neigh) processNeighOptions(options ...NeighOption) {
	for _, opt := range options {
		if opt != nil {
			opt(nh)
		}
	}
}

// WithLinkName sets the srcName of the link to use in the neighbor entry.
func WithLinkName(name string) NeighOption {
	return func(nh *neigh) {
		nh.linkName = name
	}
}

// WithFamily sets the address-family for the neighbor entry. e.g. [syscall.AF_BRIDGE].
func WithFamily(family int) NeighOption {
	return func(nh *neigh) {
		nh.family = family
	}
}

func (i *Interface) processInterfaceOptions(options ...IfaceOption) error {
	for _, opt := range options {
		if opt != nil {
			// TODO(thaJeztah): use multi-error instead of returning early.
			if err := opt(i); err != nil {
				return err
			}
		}
	}
	return nil
}

// WithIsBridge sets whether the interface is a bridge.
func WithIsBridge(isBridge bool) IfaceOption {
	return func(i *Interface) error {
		i.bridge = isBridge
		return nil
	}
}

// WithMaster sets the master interface (if any) for this interface. The
// master interface name should refer to the srcName of a previously added
// interface of type bridge.
func WithMaster(name string) IfaceOption {
	return func(i *Interface) error {
		i.master = name
		return nil
	}
}

// WithMACAddress sets the interface MAC-address.
func WithMACAddress(mac net.HardwareAddr) IfaceOption {
	return func(i *Interface) error {
		i.mac = mac
		return nil
	}
}

// WithIPv4Address sets the IPv4 address of the interface.
func WithIPv4Address(addr *net.IPNet) IfaceOption {
	return func(i *Interface) error {
		i.address = addr
		return nil
	}
}

// WithIPv6Address sets the IPv6 address of the interface.
func WithIPv6Address(addr *net.IPNet) IfaceOption {
	return func(i *Interface) error {
		i.addressIPv6 = addr
		return nil
	}
}

// WithLinkLocalAddresses set the link-local IP addresses of the interface.
func WithLinkLocalAddresses(list []*net.IPNet) IfaceOption {
	return func(i *Interface) error {
		i.llAddrs = list
		return nil
	}
}

// WithRoutes sets the interface routes.
func WithRoutes(routes []*net.IPNet) IfaceOption {
	return func(i *Interface) error {
		i.routes = routes
		return nil
	}
}
