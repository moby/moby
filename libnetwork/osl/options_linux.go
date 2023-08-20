package osl

import "net"

func (nh *neigh) processNeighOptions(options ...NeighOption) {
	for _, opt := range options {
		if opt != nil {
			opt(nh)
		}
	}
}

// LinkName returns an option setter to set the srcName of the link that should
// be used in the neighbor entry
func (n *networkNamespace) LinkName(name string) NeighOption {
	return func(nh *neigh) {
		nh.linkName = name
	}
}

// Family returns an option setter to set the address family for the neighbor
// entry. eg. AF_BRIDGE
func (n *networkNamespace) Family(family int) NeighOption {
	return func(nh *neigh) {
		nh.family = family
	}
}

func (i *nwIface) processInterfaceOptions(options ...IfaceOption) {
	for _, opt := range options {
		if opt != nil {
			opt(i)
		}
	}
}

// Bridge returns an option setter to set if the interface is a bridge.
func (n *networkNamespace) Bridge(isBridge bool) IfaceOption {
	return func(i *nwIface) {
		i.bridge = isBridge
	}
}

// Master returns an option setter to set the master interface if any for this
// interface. The master interface name should refer to the srcname of a
// previously added interface of type bridge.
func (n *networkNamespace) Master(name string) IfaceOption {
	return func(i *nwIface) {
		i.master = name
	}
}

// MacAddress returns an option setter to set the MAC address.
func (n *networkNamespace) MacAddress(mac net.HardwareAddr) IfaceOption {
	return func(i *nwIface) {
		i.mac = mac
	}
}

// Address returns an option setter to set IPv4 address.
func (n *networkNamespace) Address(addr *net.IPNet) IfaceOption {
	return func(i *nwIface) {
		i.address = addr
	}
}

// AddressIPv6 returns an option setter to set IPv6 address.
func (n *networkNamespace) AddressIPv6(addr *net.IPNet) IfaceOption {
	return func(i *nwIface) {
		i.addressIPv6 = addr
	}
}

// LinkLocalAddresses returns an option setter to set the link-local IP addresses.
func (n *networkNamespace) LinkLocalAddresses(list []*net.IPNet) IfaceOption {
	return func(i *nwIface) {
		i.llAddrs = list
	}
}

// Routes returns an option setter to set interface routes.
func (n *networkNamespace) Routes(routes []*net.IPNet) IfaceOption {
	return func(i *nwIface) {
		i.routes = routes
	}
}
