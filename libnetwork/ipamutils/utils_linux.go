// Package ipamutils provides utililty functions for ipam management
package ipamutils

import (
	"fmt"
	"net"

	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/osl"
	"github.com/docker/libnetwork/resolvconf"
	"github.com/vishvananda/netlink"
)

// ElectInterfaceAddresses looks for an interface on the OS with the specified name
// and returns its IPv4 and IPv6 addresses in CIDR form. If the interface does not exist,
// it chooses from a predifined list the first IPv4 address which does not conflict
// with other interfaces on the system.
func ElectInterfaceAddresses(name string) (*net.IPNet, []*net.IPNet, error) {
	var (
		v4Net  *net.IPNet
		v6Nets []*net.IPNet
		err    error
	)

	defer osl.InitOSContext()()

	link, _ := netlink.LinkByName(name)
	if link != nil {
		v4addr, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			return nil, nil, err
		}
		v6addr, err := netlink.AddrList(link, netlink.FAMILY_V6)
		if err != nil {
			return nil, nil, err
		}
		if len(v4addr) > 0 {
			v4Net = v4addr[0].IPNet
		}
		for _, nlAddr := range v6addr {
			v6Nets = append(v6Nets, nlAddr.IPNet)
		}
	}

	if link == nil || v4Net == nil {
		// Choose from predifined broad networks
		v4Net, err = FindAvailableNetwork(PredefinedBroadNetworks)
		if err != nil {
			return nil, nil, err
		}
	}

	return v4Net, v6Nets, nil
}

// FindAvailableNetwork returns a network from the passed list which does not
// overlap with existing interfaces in the system
func FindAvailableNetwork(list []*net.IPNet) (*net.IPNet, error) {
	// We don't check for an error here, because we don't really care if we
	// can't read /etc/resolv.conf. So instead we skip the append if resolvConf
	// is nil. It either doesn't exist, or we can't read it for some reason.
	var nameservers []string
	if rc, err := resolvconf.Get(); err == nil {
		nameservers = resolvconf.GetNameserversAsCIDR(rc.Content)
	}
	for _, nw := range list {
		if err := netutils.CheckNameserverOverlaps(nameservers, nw); err == nil {
			if err := netutils.CheckRouteOverlaps(nw); err == nil {
				return nw, nil
			}
		}
	}
	return nil, fmt.Errorf("no available network")
}
