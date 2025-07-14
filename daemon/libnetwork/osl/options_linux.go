package osl

import (
	"fmt"
	"net"
	"time"
)

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

// WithSysctls sets the interface sysctls.
func WithSysctls(sysctls []string) IfaceOption {
	return func(i *Interface) error {
		i.sysctls = sysctls
		return nil
	}
}

// WithAdvertiseAddrNMsgs sets the number of unsolicited ARP/NA messages that will
// be sent to advertise a network interface's addresses.
func WithAdvertiseAddrNMsgs(nMsgs int) IfaceOption {
	return func(i *Interface) error {
		if nMsgs < AdvertiseAddrNMsgsMin || nMsgs > AdvertiseAddrNMsgsMax {
			return fmt.Errorf("AdvertiseAddrNMsgs %d is not in the range %d to %d",
				nMsgs, AdvertiseAddrNMsgsMin, AdvertiseAddrNMsgsMax)
		}
		i.advertiseAddrNMsgs = nMsgs
		return nil
	}
}

// WithAdvertiseAddrInterval sets the interval between unsolicited ARP/NA messages
// sent to advertise a network interface's addresses.
func WithAdvertiseAddrInterval(interval time.Duration) IfaceOption {
	return func(i *Interface) error {
		if interval < AdvertiseAddrIntervalMin || interval > AdvertiseAddrIntervalMax {
			return fmt.Errorf("AdvertiseAddrNMsgs %d is not in the range %v to %v milliseconds",
				interval, AdvertiseAddrIntervalMin, AdvertiseAddrIntervalMax)
		}
		i.advertiseAddrInterval = interval
		return nil
	}
}

// WithCreatedInContainer can be used to say the network driver created the
// interface in the container's network namespace (and, therefore, it doesn't
// need to be moved into that namespace.)
func WithCreatedInContainer(cic bool) IfaceOption {
	return func(i *Interface) error {
		i.createdInContainer = cic
		return nil
	}
}
