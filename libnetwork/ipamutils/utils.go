// Package ipamutils provides utililty functions for ipam management
package ipamutils

import (
	"fmt"
	"net"

	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/resolvconf"
	"github.com/vishvananda/netlink"
)

var (
	// PredefinedBroadNetworks contains a list of 31 IPv4 private networks with host size 16 and 12
	// (172.17-31.x.x/16, 192.168.x.x/20) which do not overlap with the networks in `PredefinedGranularNetworks`
	PredefinedBroadNetworks []*net.IPNet
	// PredefinedGranularNetworks contains a list of 64K IPv4 private networks with host size 8
	// (10.x.x.x/24) which do not overlap with the networks in `PredefinedBroadNetworks`
	PredefinedGranularNetworks []*net.IPNet
)

func init() {
	PredefinedBroadNetworks = initBroadPredefinedNetworks()
	PredefinedGranularNetworks = initGranularPredefinedNetworks()
}

// ElectInterfaceAddresses looks for an interface on the OS with the specified name
// and returns its IPv4 and IPv6 addresses in CIDR form. If the interface does not exist,
// it chooses from a predifined list the first IPv4 address which does not conflict
// with other interfaces on the system.
func ElectInterfaceAddresses(name string) ([]*net.IPNet, []*net.IPNet, error) {
	var v4Nets, v6Nets []*net.IPNet

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
		for _, nlAddr := range v4addr {
			v4Nets = append(v4Nets, nlAddr.IPNet)
		}
		for _, nlAddr := range v6addr {
			v6Nets = append(v6Nets, nlAddr.IPNet)
		}
	}

	if link == nil || len(v4Nets) == 0 {
		// Choose from predifined broad networks
		v4Net, err := FindAvailableNetwork(PredefinedBroadNetworks)
		if err != nil {
			return nil, nil, err
		}
		v4Nets = append(v4Nets, v4Net)
	}

	return v4Nets, v6Nets, nil
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

func initBroadPredefinedNetworks() []*net.IPNet {
	pl := make([]*net.IPNet, 0, 31)
	mask := []byte{255, 255, 0, 0}
	for i := 17; i < 32; i++ {
		pl = append(pl, &net.IPNet{IP: []byte{172, byte(i), 0, 0}, Mask: mask})
	}
	mask20 := []byte{255, 255, 240, 0}
	for i := 0; i < 16; i++ {
		pl = append(pl, &net.IPNet{IP: []byte{192, 168, byte(i << 4), 0}, Mask: mask20})
	}
	return pl
}

func initGranularPredefinedNetworks() []*net.IPNet {
	pl := make([]*net.IPNet, 0, 256*256)
	mask := []byte{255, 255, 255, 0}
	for i := 0; i < 256; i++ {
		for j := 0; j < 256; j++ {
			pl = append(pl, &net.IPNet{IP: []byte{10, byte(i), byte(j), 0}, Mask: mask})
		}
	}
	return pl
}
