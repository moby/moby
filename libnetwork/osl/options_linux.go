package osl

import "net"

func (nh *neigh) processNeighOptions(options ...NeighOption) {
	for _, opt := range options {
		if opt != nil {
			opt(nh)
		}
	}
}

func (n *networkNamespace) LinkName(name string) NeighOption {
	return func(nh *neigh) {
		nh.linkName = name
	}
}

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

func (n *networkNamespace) Bridge(isBridge bool) IfaceOption {
	return func(i *nwIface) {
		i.bridge = isBridge
	}
}

func (n *networkNamespace) Master(name string) IfaceOption {
	return func(i *nwIface) {
		i.master = name
	}
}

func (n *networkNamespace) Address(addr *net.IPNet) IfaceOption {
	return func(i *nwIface) {
		i.address = addr
	}
}

func (n *networkNamespace) AddressIPv6(addr *net.IPNet) IfaceOption {
	return func(i *nwIface) {
		i.addressIPv6 = addr
	}
}

func (n *networkNamespace) Routes(routes []*net.IPNet) IfaceOption {
	return func(i *nwIface) {
		i.routes = routes
	}
}
