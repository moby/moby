package sandbox

import "net"

func (i *nwIface) processInterfaceOptions(options ...IfaceOption) {
	for _, opt := range options {
		if opt != nil {
			opt(i)
		}
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
